## Context

The endurance-training side of this API has been steadily filling in: meals (kcal in), workouts (kcal out + duration), body weight (with optional body-fat %). Each in isolation answers a narrow question. Energy Availability is the first computation that *connects* them — and it happens to be the single most clinically-validated marker for whether an athlete in a deficit is in a safe zone or not.

The unit is `kcal per kg of fat-free mass (FFM) per day`. Loucks (2007 + 2018 IOC consensus) bands:

```
EA ≥ 45  kcal/kg FFM/day  →  adequate / optimal
EA 30–45 kcal/kg FFM/day  →  sub-optimal (chronic deficit risks)
EA < 30  kcal/kg FFM/day  →  low — endocrine/bone/menstrual consequences
```

The formula is straightforward: `EA = (intake_kcal - exercise_energy_expenditure) / FFM_kg`. The whole game in the implementation is being honest about the three input qualities (intake, burn, composition) and surfacing where the data is missing rather than silently zeroing it.

This change is deliberately small — pure composition, no schema. The clean way to ship it is as one `internal/energy/` package + one MCP tool. Mirrors `internal/raceprep/` in structure: stateless service, single read endpoint, agent does anything stateful (e.g. persistence) on its end.

## Goals / Non-Goals

**Goals:**

- Compute per-day EA values + window aggregate from existing primitives without persisting derived state.
- Honest FFM derivation with an explicit resolution order: explicit lean mass → explicit body-fat % → stored body-fat % (most-recent weight entry in-window) → 85% fallback. Always tell the caller which path was used.
- Surface incomplete-data days loudly: a day with one untracked workout shows up with `missing_burn_workout_ids: ["..."]` and an `ea` value the caller knows to treat as an overestimate.
- Loucks band classification at every level (per-day + window-average).
- Single canonical body-weight value across the window (rolling 7-day avg of `body_weight_entries`), not per-day weight values (daily noise corrupts EA more than averaging does).

**Non-Goals:**

- Persisting EA snapshots (same reason `plan_carb_load` is stateless — recomputation with new body-fat data should be free).
- Anomaly detection / coaching recommendations ("EA has been < 30 for 14 days — see a coach"). Agent-side synthesis from this series.
- Estimation of `kcal_burned` for workouts that have it null. The signal is *missing data*; we don't paper over it.
- Sport-specific or sex-specific EA bands (e.g. female-athlete triad thresholds). Loucks is the validated baseline; sport-specific tuning is a separate change.
- Macro-level decomposition (carbs/fat/protein contribution to intake).
- Goal-weight or projection math.
- Goal integration. The `nutrition_goals` `kcal_target` doesn't enter the EA formula; an athlete on goal can still be at low EA if their burn is high. The two numbers stay independent.

## Decisions

### 1. FFM resolution order — explicit overrides win, fallback is loud

```
1. lean_mass_kg param (explicit)
2. body_fat_pct param + window-avg body weight
3. body_fat_pct from most recent body_weight_entries row in [from, to)
   + window-avg body weight
4. 85% of window-avg body weight + composition_source=estimated_85pct
   + composition_estimated=true flag at response root
```

Reasoning per tier:

- **Tier 1 (explicit lean mass)** is the highest-trust path. An athlete who's been DEXA-scanned has a real number; honor it.
- **Tier 2 (explicit body-fat %)** lets the agent compose with a number it just read elsewhere (e.g. a smart scale value not yet in the API) without forcing storage first.
- **Tier 3 (stored body-fat %)** is the common case: the user's `add-weight-log` entries include `body_fat_pct` from a smart scale. Pull the most-recent reading inside the requested window.
- **Tier 4 (85% fallback)** is for users without body-fat data. The 85% figure is a generic "average athletic male body composition" estimate. We flag it loudly because EA computed against an estimated FFM has an error bar measured in tens-of-kcal/kg.

**Alternatives considered:**

- *Refuse to compute without explicit composition data.* Rejected — the 85% fallback is well-established in sports-nutrition tooling and is better than silently failing; the response makes the estimation transparent.
- *Use rolling body-fat % across multiple entries.* Rejected — body-fat measurements are noisy; the most-recent in-window reading is a closer point estimate than a rolling avg over 3 weeks ago + today.
- *Per-day FFM.* Rejected — FFM doesn't change day-to-day. A single window value is honest about the granularity the data supports.

### 2. Single body-weight value per window — rolling 7-day avg

`body_weight_entries` is noisy: glycogen swings 1–2 kg, post-meal water 0.5–1 kg, hotel scales add a unit-conversion error. The 7-day rolling average is the standard fix (already exposed via `weight_trend`).

Resolution:

```
1. If from-to window is ≥ 7 days: rolling 7-day avg ending at the requested `to`,
   using entries in [to-7d, to)
2. If window < 7 days: rolling avg across whatever in-window entries exist
3. If no in-window entries: pick the most-recent entry before `from`,
   flag body_weight_source=last_before_window
4. If no weight entries at all: 400 weight_data_missing
```

**Alternatives considered:**

- *Per-day weight.* Rejected — same noise problem as for FFM.
- *Force the caller to supply body_weight_kg.* Rejected — the data is already in the API; making the caller redundantly pass it invites stale-data bugs.

### 3. Calendar-day boundaries respect the requested TZ

EA is conceptually "today" in the user's local sense — a workout finished at 23:30 belongs to today's `burned`, not tomorrow's. The endpoint accepts `tz` (default = `DEFAULT_USER_TZ`); calendar-day buckets are resolved in that TZ; meals/workouts are matched by their `logged_at` / `started_at` in that TZ.

This is consistent with `daily_summary` and `daily_hydration_summary` — same pattern, same `tz` parameter, same default.

### 4. Window aggregation — simple mean of valid days

For the window summary `avg_ea`:

```
avg_ea = mean(day.ea for day in days if day has complete data)
```

The `window.days_with_complete_data` field exposes how many days contributed. Days with `missing_burn_workout_ids` are excluded from the mean — including them would dilute the signal with optimistic EA values.

**Alternatives considered:**

- *SUM(intake_kcal - burned_kcal) / (days * FFM).* Mathematically cleaner but gives the same answer (FFM is constant across days) and the daily-mean form is what the Loucks literature reports.
- *Median.* Rejected — the EA series is short (typical 7-day rolling window); mean is fine and more transparent.
- *Weight by intake-kcal.* Rejected — gives "heavy intake days dominate the trend" which is the wrong intuition (a day with high intake AND high burn is still informative).

### 5. Missing burn data — flag, don't impute

When `workouts.kcal_burned` is NULL for a workout in the day, the response lists that workout's id in `day.missing_burn_workout_ids` and **still computes the EA value**, treating the missing burn as 0 for that workout only. The caller (agent) sees:

```json
{
  "date": "2026-06-04",
  "intake_kcal": 2400,
  "burned_kcal": 600,       // sum of the workouts that HAD kcal_burned
  "ea": 60.0,
  "band": "adequate",
  "missing_burn_workout_ids": ["..."],
  "complete_data": false
}
```

Reasoning: silently zeroing makes a low-data day look healthier than it is, which is the most dangerous failure mode for this metric. Marking incomplete is loud-over-silent.

**Alternatives considered:**

- *Refuse to return EA for any incomplete day.* Rejected — the user still wants to see the partial picture; nulling out the field would force the agent to guess.
- *Refuse to return EA for the whole window if any day is incomplete.* Rejected — would push 80%+ of real users into the failure path; their non-Garmin workouts are reality, not an exception.
- *Auto-impute burn from `sport + duration + body weight`.* Rejected — estimation is the writer's job (Garmin already does it for synced sessions). A missing field is real signal; faking it would mask the data quality problem this tool is meant to surface.

### 6. Band classification at boundaries — closed-low, open-high

```
ea < 30        → "low"
30 <= ea < 45  → "sub_optimal"
ea >= 45       → "adequate"
```

A round 30.0 is sub-optimal (just above the danger zone); a round 45.0 is adequate. Picks the *safer* side at each boundary — encourages the athlete to clear the threshold rather than sit on it.

### 7. Window cap = 92 days (same as other list windows)

Matches the cap on `/meals`, `/hydration`, `/weight`, `/workout-fuel`. Three-month window is plenty for EA tracking; the rolling weekly view is the practical one.

### 8. No idempotency, no migration, no auth changes

Pure read. Lives on the existing auth-guarded `/` group; no new middleware; no schema work.

### 9. Package shape — sibling to summary / raceprep

```
internal/energy/
    types.go      // request + response structs
    service.go    // composition (FFM resolver, day buckets, band classifier)
    handlers.go   // single GET handler
    service_test.go (or handlers_test.go) + table-driven FFM + band tests
```

`Service` constructor takes `*meals.Repo`, `*workouts.Repo`, `*bodyweight.Repo`. That's three repos but no new package dependencies — `summary` already pulls meals+goals; `raceprep` is stateless; this sits in the same shape.

## Risks / Trade-offs

- **The 85% FFM fallback can be 5–10 kcal/kg off**, which crosses Loucks band boundaries for some athletes. → Mitigation: the response carries `composition_estimated: true` and `composition.source: "estimated_85pct"` so the agent surfaces "low-confidence EA" framing when speaking to the user.
- **Missing-burn workouts can still skew the per-day EA upward**, but at least the caller knows. → Mitigation: `missing_burn_workout_ids` is explicit; the window aggregate excludes those days.
- **Calendar-day boundary semantics affect EA at day-boundary workouts.** A late-night ride finishing at 00:15 belongs to the next day under most TZs. → Mitigation: deterministic rule — match by `started_at` in the requested TZ; documented in the spec.
- **Body-fat % from consumer smart scales has a real error bar (±2–4%).** EA inherits that error. → Mitigation: explicit-override path lets the agent surface a more trusted DEXA number when one exists; this is documented.
- **EA without specifying a goal block can mislead** (a "low EA" reading during a deliberate race-week taper isn't the same as chronic low EA). → Mitigation: out of scope for this change; the agent is the right place to add training-phase context. Future tie-in once T1 #5 / #1A (templates/phases) lands.
- **Window cap of 92 days** limits longitudinal views. → Mitigation: agent can chain windows; the cap matches every other list endpoint in the API for consistency.

## Migration Plan

- Forward: no schema change. Code-only. Deploy lands the endpoint behind the existing `BearerAuth`.
- Rollback: revert the binary. No data to clean up.
- No feature flag needed — the endpoint is additive and read-only.

## Open Questions

- Whether the response should include the goal-weight `kcal_target` from `nutrition_goals` as a context field (caller could compare EA to "what I think I need" without a second call). Tentative: yes, as an `informational.goal_kcal_target` field — but documented as informational because EA and `kcal_target` are independent metrics. Decide during implementation.
- Whether to support `granularity=weekly` to return one row per ISO week instead of per day. Tentative: no for v1 — the agent can roll up; if real use shows the per-day payload is too large for the context window, add it as a follow-up flag.
- Whether to expose `EEE` (Exercise Energy Expenditure) per day separately from `burned_kcal` so the caller can sanity-check the SUM. Tentative: yes — `day.burned_kcal` IS the EEE; the field name should be `exercise_energy_kcal` to match the formula's naming. Decide during the spec pass.
