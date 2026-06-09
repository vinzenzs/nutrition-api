## Context

The MPS-per-meal story has been the second-most-cited "the API can't see what I care about" question from the agent transcripts (after Tier-1 EA, which `add-energy-availability` just delivered). It's also the lowest-cost: zero new tables, zero migrations, one new read endpoint that buckets existing `meal_entries` rows.

The structural pattern is identical to `add-rolling-window-summaries` and `add-energy-availability`: read endpoint, calendar-day buckets in the requested TZ, body-weight resolution via the four-tier rule already implemented in `internal/energy/composition.go`. Each meal entry becomes one row; per-row the response computes `mps_effective: protein_g >= mps_threshold_g` where `mps_threshold_g = 0.3 × body_weight_kg`.

The bulk of the design decisions are about (a) what "a meal" means in the row sense (per-entry vs per-meal-type aggregation), (b) the gap-since-previous semantics, (c) body-weight resolution, and (d) where this endpoint sits in the API surface area.

## Goals / Non-Goals

**Goals:**

- One row per `meal_entries` row on the date — no implicit grouping.
- Each row carries `protein_g`, `mps_effective: bool`, `gap_minutes_since_previous: int | null`.
- Body-weight resolution with reported source (`explicit` / `rolling_7d_avg` / `last_before_date`).
- Calendar-day boundaries in the requested TZ, matching daily / range / rolling.
- Hard-code MPS threshold at 0.3 g/kg/meal (Loucks/Phillips literature value).
- Pure read; no schema; no idempotency-key.

**Non-Goals:**

- Per-meal-type aggregation (`range_summary?group_by=meal_type` already does that).
- Range / rolling protein distribution (agent composes if needed).
- Leucine-specific tracking.
- Streak / nudge surface.
- MPS-threshold tuning per user.
- Auto-clustering of close-together entries into a single "meal."

## Decisions

### 1. One row per `meal_entries` row — no implicit grouping

A user who logs three breakfast components (skyr, oats, honey) at 07:30 in the local TZ gets **three rows** in the response, not one row labelled "breakfast." Reasoning:

- The API has no canonical definition of "meal" beyond the row. The phone occasionally logs a recipe as one row and the components as separate rows depending on which entry path was used (`POST /meals` with a recipe id vs `POST /meals/freeform` chained).
- Collapsing client-side is trivial: sum `protein_g` across rows where `meal_type` is identical and `logged_at` is within 30 minutes. Collapsing server-side requires either a threshold the agent can't see or a `meal_type` requirement that breaks when entries lack the field.
- Per-row data is honest about what was logged. The agent does the human-friendly framing.

**Alternatives considered:**

- *Group by `meal_type`.* Rejected — `meal_type` is optional on `meal_entries`. A user who never sets it would get every row collapsed into one "untyped" bucket.
- *Group by `meal_type` + 30-minute clustering.* Rejected — invents a heuristic the spec then has to define. Better: agent owns the heuristic.
- *Return both per-row AND per-meal-type aggregates.* Rejected — duplicates data, increases response size with no new signal.

### 2. `mps_effective` is computed against body weight at the date, not at the meal

`mps_threshold_g = 0.3 × body_weight_kg` is a window-level value, not a per-meal value. Body weight is reported once at the response root with its resolution source.

**Why not per-meal weight?** Weight doesn't change meaningfully meal-to-meal. Resolving per-meal would re-introduce daily noise that the rolling-7d average exists to smooth out. A single window-level `mps_threshold_g` is honest about the granularity the data supports.

### 3. Body-weight resolution — four-tier rule (re-use the energy package's pattern)

Mirrors `internal/energy/composition.go`'s `resolveBodyWeight`:

1. **Explicit `body_weight_kg` query param** wins over everything. `body_weight_source: "explicit"`.
2. **Rolling 7-day average** ending at `date` (entries in `[date − 7d, date + 1d)` local TZ). `body_weight_source: "rolling_7d_avg"`.
3. **Last entry strictly before the date** if no in-window data. `body_weight_source: "last_before_date"`.
4. **No weight data at all** → `400 weight_data_missing`. The agent must pass `body_weight_kg` explicitly OR `add-weight-log` has to be populated.

Implementation note: the energy package owns the resolver. If it cleanly factors into `internal/bodyweight/resolve.go` as a shared helper, hoist it; otherwise duplicate the small function (it's ~30 lines and the duplication is more honest than a premature shared package). Decide during impl based on what feels natural.

**Alternatives considered:**

- *Per-meal weight (most-recent entry at-or-before each meal).* Rejected per Decision 2.
- *Require explicit `body_weight_kg`.* Rejected — fails the "agent uses the data already in the system" principle.

### 4. `gap_minutes_since_previous` — null on first meal, otherwise integer minutes

Ordered by `logged_at` ascending. First row's gap is `null`. Subsequent rows show `int(now − previous logged_at, in minutes)`. Same-second logs get `0`.

Reasoning: the MPS-trigger story has a 3–5h sweet spot. Gap < 180 min → meals not independent triggers. Gap > 300 min → MPS window closed before the next dose. The endpoint surfaces the raw number; "this gap is short / long / fine" framing is agent-side.

**Alternatives considered:**

- *Compute a `gap_effective: "short" | "ok" | "long"` enum.* Rejected — bands hide the actual number from agents that want to chart trends; raw minutes are denser signal.
- *Include `gap_minutes_to_next` as well.* Rejected — derivable from the next row; would double the payload.

### 5. `logged_at_hour` field for circadian context

Each row carries `logged_at_hour: 0..23` in the requested `tz`. Cheap to compute, real signal: agents care about "the user logs lunch at 14:00 but breakfast at 11:00 — gap fine, but circadian-wise breakfast is closer to brunch."

Could be derived agent-side from `logged_at` + `tz`, but exposing it explicitly:
- Avoids agent timezone math (a known fragility).
- Makes the response self-contained for downstream tooling.

### 6. Empty-day shape — `meals: []`, `mps_effective_meal_count: 0`, weight resolution still attempted

A day with zero logged meals returns:

```json
{
  "date": "2026-06-09",
  "tz": "Europe/Berlin",
  "body_weight_kg": 72.5,
  "body_weight_source": "rolling_7d_avg",
  "mps_threshold_g": 21.75,
  "total_protein_g": 0,
  "meal_count": 0,
  "mps_effective_meal_count": 0,
  "meals": []
}
```

Weight resolution still runs because the threshold is informational even without meals — the agent can say "today's threshold would be 21.75 g per meal" in a coaching message.

If weight resolution fails on an empty day, return `400 weight_data_missing` same as a populated day. Consistency over special-cases.

### 7. Hard-code MPS threshold at 0.3 g/kg/meal

Phillips (2014) / Loucks (2007) / Stokes (2018) all converge on this value. Variability across published reviews is ±0.05 g/kg/meal, well within the noise the data carries anyway.

A `mps_threshold_g_per_kg` parameter is a non-breaking follow-up if real use shows the friction. v1: no parameter.

### 8. No `range` variant on the endpoint

A multi-day protein-distribution view is composable from N calls to this endpoint plus client-side aggregation. The single-day endpoint stays simple. If usage shows the agent calls it 7× per "weekly review" conversation, revisit; not before.

### 9. Endpoint path — `/summary/protein-distribution`

Sits next to `/summary/daily`, `/summary/range`, `/summary/rolling`, `/summary/hydration/daily`. The "summary" prefix keeps the URL space coherent. No deeper nesting (`/summary/protein/distribution/daily`) — adds segments without adding info.

### 10. MCP tool name — `protein_distribution`

Short, matches the endpoint suffix. The tool description should call out:

- The MPS-per-meal rule (~0.3 g/kg/meal).
- The gap heuristic (3–5h sweet spot; this is the timing context, not a warning).
- The body-weight resolution order (explicit > stored rolling > last-before-date).
- `mps_effective_meal_count / meal_count` is the headline number an agent should surface to the user.

## Risks / Trade-offs

- **The MPS threshold (0.3 g/kg/meal) is a literature value, not a personal target.** An older athlete or one in a hard cutting block may benefit from 0.4 g/kg/meal. → Mitigation: documented; `mps_threshold_g_per_kg` follow-up is cheap if real use surfaces it.
- **Rows-per-entry is verbose for users who log component-style** (3 rows for one breakfast). → Mitigation: the agent collapses on output. Server-side collapsing is a heuristic landmine.
- **Body weight from the rolling-7d average can lag a real change** (e.g. immediately after a cutting-block weight drop). → Mitigation: explicit `body_weight_kg` override is the escape hatch; the source is reported so the agent can flag the assumption.
- **Empty-day responses look weird** (rich shape, zero meals). → Mitigation: rich-but-honest is better than 404-on-empty (matches the other summary endpoints' pattern).
- **`gap_minutes_since_previous` on cross-midnight workflows** — a meal at 23:30 on June 8 and one at 00:30 on June 9 are both their respective days' first meal. The 60-minute gap doesn't appear anywhere. → Documented; the agent composes across-day windows if it cares.
- **The endpoint encourages "score your day on MPS" behaviours** which can over-tune meal timing in ways that aren't useful for non-strength-focused athletes. → Out of scope for the API; agent-side framing.

## Migration Plan

- Forward: no schema change. Code-only. Deploy lands the endpoint behind the existing `BearerAuth`.
- Rollback: revert the binary.
- No feature flag — additive, read-only.

## Open Questions

- Whether to expose `kcal` per row alongside `protein_g`. Tentative: no — the question this endpoint serves is per-meal MPS, not per-meal energy. The agent calls `daily_summary` for the per-meal-type kcal breakdown.
- Whether `meal_type` should be normalized when null (e.g. infer from `logged_at_hour`). Tentative: no — surface the raw value; let the agent infer if it cares.
- Whether to include a window-level `protein_per_kg_per_day` field (`total_protein_g / body_weight_kg`) for the daily target framing. Tentative: yes, cheap — adds the "how am I doing against the daily 1.6–2.2 g/kg/day target" signal at no marginal cost. Decide during spec.
- Whether the MCP tool should default to today's date when omitted. Tentative: yes — matches the pattern other date-anchored tools use (`daily_summary` requires explicit date but agents always pass today's date anyway; defaulting saves a round-trip on the most common query).
