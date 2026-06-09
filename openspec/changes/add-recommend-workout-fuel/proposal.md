## Why

`plan_carb_load` answers "how should I eat in the 1–4 days *before* my race." `daily_summary` + the phase template answer "what's my macro target for *today's training block*." `workout_fueling_summary` answers "what *did* I take during this ride." None of them answer the question an endurance athlete asks every single morning: **"I have a 90-minute Z2 ride tomorrow — what should I eat before, during, and after?"**

Today that question requires the agent to pull the workout shape from `workouts`, the user's body weight from the resolver, the training phase from the phases tool, then do per-zone CHO/hr math from memory — and even then the literature ratios (Jeukendrup, Burke, ISSN consensus) live only in the agent's training data, not in the API's contract. Every session pays that round-trip + math tax, and the math goes subtly wrong on edge cases (a 50-min Z4 effort with no CHO target because the agent split the band at 45 min vs 60 min).

`recommend_workout_fuel` cashes in the literature ratios as a stateless tool, taking the inputs from primitives that already exist:

- **Sport + duration + intensity** → either `workout_id` (pulls from the row) or explicit params.
- **Body weight** → the 4-tier rolling-7d resolver shared with EA, protein-distribution, and race-prep.

Closes T2 #10 in `openspec/priorities.md`. After the 2026-06-08/09 Tier-1 cluster + Tier-2 protein-distribution, this is the most "client-side math repeated in every conversation" gap left on the priorities list.

## What Changes

- **New `GET /race-prep/recommend-workout-fuel` endpoint**: query params
  - **Workout-mode**: `workout_id=<uuid>` — pull `sport`, `duration_min` (`ended_at - started_at`), and intensity from the row.
  - **Explicit-mode**: `sport=<enum>&duration_min=<int>&intensity_zone=<int>` — for "tomorrow's planned session" queries where no workout row exists yet.
  - **Common (optional)**: `body_weight_kg=<float>` overrides the resolver.
  - Exactly one of `workout_id` OR (`sport` + `duration_min` + `intensity_zone`) must be present.
- **Response shape**:
  ```json
  {
    "inputs": {
      "sport": "bike", "duration_min": 90, "intensity_zone": 3,
      "body_weight_kg": 72.0, "body_weight_source": "rolling_7d_avg",
      "workout_id": "..." // present only in workout-mode
    },
    "pre_workout": {
      "window_minutes_before": [60, 120],
      "carbs_g": 108,
      "carbs_g_per_kg": 1.5,
      "rationale": "..."
    },
    "intra_workout": {
      "applicable": true,           // false for sessions under 45 min
      "carbs_g_per_hour": 60,
      "carbs_g_total": 90,
      "fluid_ml_per_hour": 600,
      "sodium_mg_per_hour": 500,
      "rationale": "..."
    },
    "post_workout": {
      "window_minutes_after": [0, 60],
      "carbs_g": 72,                // 1 g/kg for the first hour
      "protein_g": 21.6,            // 0.3 g/kg (matches the MPS threshold from add-protein-distribution)
      "rationale": "..."
    },
    "notes": [
      "Sodium 300–800 mg/hr is the validated range; 500 mg/hr is a moderate sweater in cool conditions.",
      "Carbs/hr: < 45 min none required, 45–90 min 30 g/hr, 90–180 min 60 g/hr (single transportable), > 180 min 90 g/hr (multiple transportable glucose+fructose).",
      "For races > 90 min, also run plan_carb_load for the 24–72h pre-loading schedule."
    ]
  }
  ```
- **New MCP tool `recommend_workout_fuel`** wrapping the endpoint. Single tool, two input modes.
- **Stateless** — same shape as `plan_carb_load`. No writes; no `Idempotency-Key`; recommendations are derived numbers, not stored entries. If the user wants to commit a plan, they log via `log_workout_fuel` separately.

## Capabilities

### Modified Capabilities

- `race-prep`: Adds the `recommend-workout-fuel` requirement alongside the existing carb-load read + apply requirements.
- `mcp-server`: Adds one new tool requirement for `recommend_workout_fuel`.

## Impact

- **Prerequisites (all already shipped)**: `add-workouts-capability` (`workout_id` mode), `add-weight-log` + the 4-tier resolver, `add-protein-distribution` (MPS threshold reused for post-workout protein recommendation).
- **No schema migration**. Pure computation over a literature table + the resolver.
- **New code**:
  - `internal/raceprep/recommend.go`: types (`FuelRecommendation`, `PreWorkout`, `IntraWorkout`, `PostWorkout`, `RecommendParams`) + service method `RecommendFor(ctx, RecommendParams) (*FuelRecommendation, error)`. Sits next to the existing `CarbLoadFor` / `CarbLoadApply`.
  - `internal/raceprep/handlers.go`: third route `GET /race-prep/recommend-workout-fuel` next to the existing two.
  - `internal/mcpserver/tools_raceprep.go`: add `RecommendWorkoutFuelArgs` + handler + register.
  - The body-weight resolver: protein-distribution duplicates the energy one; race-prep should reuse the **same duplicated 4-tier function** (single-date semantics) by hoisting from `internal/summary/protein.go` into `internal/bodyweight/resolve.go`. Third caller justifies the lift now — the rule of three.
- **Modified code**:
  - `internal/httpserver/server.go`: pass `workoutsRepo` + `bodyWeightRepo` to the existing `raceprep.NewService(...)` (constructor needs extending; the existing setter pattern would also work but the raceprep service was constructor-style).
- **Tests**:
  - Literature-band coverage: each duration × intensity bucket maps to the documented CHO/hr value.
  - Per-sport modifiers: bike vs run vs strength produce the documented differences.
  - Workout-mode integration: real `workouts` row → correct inputs derived.
  - Explicit-mode happy + validation (sport/zone/duration ranges).
  - Body-weight resolution paths (mirror the protein-distribution tests; quick sanity-checks).
  - Exactly-one-mode validation: `workout_id` + `sport` together → 400.
- **Documentation**: `task swag`; README "Race prep" subsection gets the recommend example; MCP tools table gains `recommend_workout_fuel`. RUN_LOCAL.md gets a one-liner.
- **No idempotency middleware impact** — read-only.

### Out of scope (explicit non-goals)

- **Caffeine timing recommendations.** Caffeine is mostly a race-day thing (3–6 mg/kg, 45–60 min before); recommending it for daily training over-doses an athlete. Cover in a follow-up if real use shows the gap.
- **Multi-transportable vs single-transportable carb-source guidance.** The notes mention the rule (single < 90 min, multi-transportable > 180 min) but the response doesn't prescribe specific product types (Maurten 320 vs SiS Beta Fuel). Agent-side.
- **Heat / altitude / environmental modifiers.** A 90-min Z3 ride in 35 °C needs more sodium than 15 °C. Real factor, but adding it requires environmental context the agent often doesn't have. Document as a multiplier in notes; revisit if real use shows the friction.
- **Sweat-rate-personalized sodium.** T3 #6 (sweat rate test workflow) is the right way to do this; without that data, 300–800 mg/hr is the literature range and the response uses a sport-typical midpoint.
- **Auto-applying recommendations into `workout_fuel_entries`.** Recommendations are derived; entries are actuals. Conflating them would distort the rehearsal-data analytics that `add-workout-fuel` was built for. The agent calls `log_workout_fuel` separately if the user wants to commit.
- **Brick / multi-discipline sessions.** A 60-min swim followed by a 120-min bike is two sessions; the tool answers per-session. Triathlon-specific brick logic is a separate proposal if it ever earns the surface.
- **Phase-aware modifiers.** Phase + template already shift the daily macro targets; layering phase modifiers into this single-session tool would double-count. Race-week loading is owned by `plan_carb_load`. Document, don't implement.
- **Persistence of recommendations.** Same reasoning as `plan_carb_load` — stateless computation; let the agent persist as `coach_recommendation` (T2 #6F) if and when that ships.
- **Strength / non-endurance protocols.** Strength sessions get a minimal response (`intra_workout.applicable: false`, post-protein only). Hypertrophy-specific peri-workout protein loading is a separate topic.
- **Intra-workout protein.** Some endurance-fueling research recommends 0.25 g/kg protein/hr for sessions > 3h. Niche; out of scope for v1.
