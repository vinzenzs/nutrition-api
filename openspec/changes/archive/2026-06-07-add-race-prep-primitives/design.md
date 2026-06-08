## Context

Carb-loading math is a textbook recipe: pick how many days you'll load, multiply body weight by a carbs-per-kg target for each of those days, multiply by a smaller number for race-morning. Three parameters, one body weight, one date — out pops a schedule. Five lines of arithmetic.

Three reasons it nevertheless belongs in the API:

1. **Agents hallucinate small numbers.** "10 g of carbs per kg" sounds like a small enough fact to remember, but agents have been observed to say "1 g/kg" or "100 g/kg" in similar contexts. The cost of an under- or over-load is real.
2. **It composes with `add-date-varying-goals`.** The natural workflow is plan-then-override: the agent calls `plan_carb_load`, then issues N `set_daily_goal_override` calls. Having the math on the same side as the goal-storage means both halves use the same numbers without serialisation drift.
3. **Tests anchor the conventions.** A handful of exact-math assertions in the test suite document "we use this convention" in code, not in agent prompts.

The compositional case is the strongest: `plan_carb_load` and `set_daily_goal_override` together form the full "set up race week" loop, and they share a code surface where the parameter shapes can be cross-checked.

## Goals / Non-Goals

**Goals:**

- One function, one endpoint, one MCP tool — the smallest possible surface.
- Pure math: stateless, no DB, no migration, no per-user storage.
- Every parameter has a sensible default; only `race_date` and `body_weight_kg` are required.
- Bounds-check parameters to catch obvious typos (`body_weight_kg = 70000` because someone confused kg with grams).
- Response is structured for direct piping into `set_daily_goal_override`: each entry has the target carbs in grams and the date.
- Race date in the past is an error (the schedule wouldn't help anyone).

**Non-Goals:**

- Stored race calendar / `races` table.
- Race-type presets that hide parameters behind a string.
- During-race fuelling plans, recovery-window targets, gut-training tracking.
- Auto-apply: the API does not write goal overrides on the user's behalf — the agent does, explicitly.
- Multi-athlete or training-partner support.
- Sport-specific variation (triathlon vs marathon vs ultra). The numeric parameters cover the variation; sport is a label the agent uses to pick defaults.

## Decisions

### 1. Stateless pure function, no storage

```go
type CarbLoadParams struct {
    RaceDate            time.Time // local date, no time-of-day
    BodyWeightKg        float64
    DaysBefore          int     // default 3
    CarbsPerKgPerDay    float64 // default 10
    RaceDayCarbsPerKg   float64 // default 2
}

type CarbLoadEntry struct {
    Date           string  `json:"date"`            // YYYY-MM-DD
    DaysBefore     int     `json:"days_before"`     // 3 .. 0
    TargetCarbsG   float64 `json:"target_carbs_g"`  // body_weight_kg × multiplier
    Rationale      string  `json:"rationale"`       // human-readable label
}

type CarbLoadSchedule struct {
    RaceDate     string          `json:"race_date"`
    BodyWeightKg float64         `json:"body_weight_kg"`
    Params       CarbLoadParams  `json:"params"`     // echo of effective inputs
    Schedule     []CarbLoadEntry `json:"schedule"`
}

func PlanCarbLoad(p CarbLoadParams) (*CarbLoadSchedule, error)
```

Algorithm:

```
for i := DaysBefore; i > 0; i-- {
    schedule[d] = {date: race_date - i, days_before: i,
                   target: round1(body_weight_kg * carbs_per_kg_per_day),
                   rationale: "carb-load day <DaysBefore - i + 1>"}
}
schedule[last] = {date: race_date, days_before: 0,
                  target: round1(body_weight_kg * race_day_carbs_per_kg),
                  rationale: "race morning, pre-race meal ~3-4h before start"}
```

When `DaysBefore == 0`, the schedule contains just the race-day entry. When `DaysBefore == 7`, seven load days + one race day.

`round1` is the existing `numfmt.Round1` helper from `unify-adherence-shape` — consistent precision with everywhere else.

**Alternatives considered:**

- *Store races in a `races` table; `plan_carb_load` takes a `race_id`.* Rejected — per the proposal's non-goal: agent's calendar already knows the race date. A `races` table is data we don't yet need.
- *Race-type enum (sprint/70.3/ironman) selecting curated defaults.* Rejected — hides the assumptions; the three numeric parameters cover the entire space with full transparency.

### 2. Bounds validation, not value clamping

Reject out-of-range parameters with `400 …_invalid` rather than silently clamping. Bounds:

```
body_weight_kg            30 ≤ x ≤ 200    (catches kg-vs-grams typos either way)
days_before                0 ≤ x ≤ 7      (carb-load > 7 days isn't a real protocol)
carbs_per_kg_per_day       1 ≤ x ≤ 20     (catches "did I mean grams or pounds?")
race_day_carbs_per_kg      0 ≤ x ≤ 10     (zero is valid — "skip the pre-race meal")
race_date                  ≥ today        (past-date carb-load helps no one)
```

Error codes per spec: `body_weight_kg_invalid`, `days_before_invalid`, `carbs_per_kg_per_day_invalid`, `race_day_carbs_per_kg_invalid`, `race_date_in_past`. Plus `race_date_invalid` if the YYYY-MM-DD parse fails.

**Alternatives considered:**

- *Clamp silently to the valid range.* Rejected — agents would never see they were off; loud rejection beats silent fix-up (consistent with `harden-write-paths` posture).
- *No bounds; trust the agent.* Rejected — the "kg vs grams" typo is exactly the failure case where loud bounds-checking earns its weight.

### 3. Endpoint is GET, no auth-special-casing

```
GET /race-prep/carb-load?race_date=&body_weight_kg=&days_before=&carbs_per_kg_per_day=&race_day_carbs_per_kg=
```

Pure-function semantics → GET. Registered under the existing authed API group, so the standard bearer middleware applies (consistent with every other read endpoint). No idempotency middleware involvement — GET requests bypass it already.

`race_date` is a `YYYY-MM-DD` string in the query param. The handler parses it as a date in the configured `DEFAULT_USER_TZ` (so "tomorrow" means tomorrow in the user's local time, not UTC). This matters mostly at the date boundary; for users far from UTC the convention prevents "wait, I asked for 2026-07-24 and the schedule starts on 2026-07-23 with `days_before=3`."

**Alternatives considered:**

- *POST with a JSON body.* Considered for parameter clarity. Rejected — GET keeps it cacheable, bookmarkable, and matches the pure-function semantics; query params are fine for ≤ 5 inputs.

### 4. Response includes echoed params for traceability

The response carries `params: {days_before, carbs_per_kg_per_day, race_day_carbs_per_kg}` reflecting the effective inputs (defaults applied). This lets the agent — and any human reading the response — see exactly which protocol produced the schedule, without re-computing or guessing.

`race_date` and `body_weight_kg` echo at the top level so the agent can correlate the schedule with the call without parsing back.

### 5. MCP tool description nudges the agent toward the override workflow

```
plan_carb_load — Compute the carb-load schedule for a race.

Returns a daily schedule of carb targets in grams: 'days_before' load days
plus race day. The natural follow-up is to translate each entry into a
goal override via set_daily_goal_override, so adherence on those days
reflects the carb-load target.

For sprint tri / short races, consider days_before=1 or 2 (carb-load benefit
plateaus). For 70.3 use the default 3. For Ironman consider 3-4 days.
The carbs_per_kg_per_day default of 10 sits in the middle of the documented
8-12 g/kg range; lower for athletes who handle GI distress.
```

The description names the cross-tool workflow explicitly so an agent that runs `plan_carb_load` once has the next step visible without needing to remember it.

### 6. No DB, no migration, no state — but tests anchor the math

Although the feature has no persistence, the tests are load-bearing: they document "we use ~10 g/kg as the default carb-load multiplier" in code, with exact numeric assertions. If a future contributor (human or agent) tweaks the math, the tests fail loudly. The capability spec encodes the same invariants in plain English so both surfaces agree.

## Risks / Trade-offs

- **The default carbs_per_kg_per_day = 10 is one point in a documented range.** Some athletes do better at 7, some need 12. *Mitigation:* the parameter is exposed; agent picks based on context. The default is the middle of the textbook 8-12 g/kg.
- **No race-day fuelling, no recovery window.** The API doesn't tell the user when to eat gels on the bike or how much protein to take after the race. *Mitigation:* both are explicitly agent territory per the user's stated principle. If real use shows they should also be primitives, two separate small changes.
- **Stateless means we can't track adherence to the carb-load.** Tomorrow's schedule isn't connected to yesterday's actual intake. *Mitigation:* the cross-tool workflow with `set_daily_goal_override` provides exactly that link — agent calls plan, applies overrides, daily summary reports adherence against the (now overridden) goals.
- **Bounds-checking might surprise an agent at the edges.** Body weight of 25 kg (a small child) is a legitimate value the API will reject. *Mitigation:* this app is single-user adult endurance-training-focused; the bounds are right for the user. If multi-user is ever a thing, bounds become per-user config.

## Migration Plan

- Stateless feature; nothing to migrate. Forward and rollback both: add/remove the route registration.

## Open Questions

- Whether the schedule entries should also include `kcal` (carbs × 4) to make goal-override construction trivial. Tentative answer: no — the agent multiplies by 4 if it cares, and including kcal invites confusion about what we're computing (total kcal? carb kcal? other macros' contribution?). One number per row keeps the contract clean.
- Whether `plan_carb_load` should warn when the race date is more than 14 days out (carb-loading that far ahead is meaningless). Tentative answer: no for v1 — the agent can ignore the schedule and call again closer to race day. A warning would also pollute the response shape.
- Whether to extend with a `recovery_window_macros(body_weight_kg, workout_intensity)` primitive in this change. Tentative answer: no, separate concern, separate change if needed. Keep this one tight.
