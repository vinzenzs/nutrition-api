## Why

`add-workouts-capability` and `add-weight-log` landed the missing endurance-training primitives as standalone tables. They were deliberately scoped to "table + CRUD only" so each could ship in a small change. The cost of that scoping is that the system can record workouts and meals *separately* but cannot answer the actual question: **what did I eat in the pre/intra/post window of this workout?**

Closing T1 #1 in `openspec/priorities.md` is structurally tiny: add a nullable `workout_id` column to `meal_entries` and `hydration_entries`, add an aggregation endpoint that buckets a workout's intake into windows, extend the existing MCP write tools with the optional field, expose one new read tool. No new capabilities; four small modifications.

Once this lands, the agent can answer fueling questions natively (no time-window math on its end), and the foundation is in place for two follow-ups already pencilled in priorities — the EA tool (T1 #4) and the `workout_fuel_entries` sibling for in-session carbs/sodium/caffeine (T1 #2).

## What Changes

- **`meal_entries.workout_id`** — nullable `UUID REFERENCES workouts(id) ON DELETE SET NULL`. Indexed for the lookup-by-workout query.
- **`hydration_entries.workout_id`** — same shape and same FK semantics.
- **Optional `workout_id` accepted on** `POST /meals`, `POST /meals/freeform`, `PATCH /meals/{id}`, `POST /hydration`, `PATCH /hydration/{id}`. Validated to reference an existing workout; otherwise `400 workout_not_found`.
- **`workout_id` returned on** every meal and hydration response (`omitempty` so existing-shape consumers see no change when the field is null).
- **PATCH clears via empty string** — `{"workout_id": ""}` unlinks; `{"workout_id": "<uuid>"}` (re)links; field omitted leaves the link unchanged. Standard nullable-pointer pattern adapted for the JSON tri-state.
- **`GET /workouts/{id}/fueling?pre_window_min=&post_window_min=`** — new aggregation endpoint. Returns three time-anchored buckets (pre / intra / post) each carrying *separate* nutrition totals and hydration totals (no cross-unit mixing in the Totals struct). Defaults: `pre_window_min=240` (4h), `post_window_min=60` (1h), both bounded [0, 720].
- **MCP wrapper changes**:
  - `log_meal`, `log_meal_freeform`, `patch_meal`, `log_hydration`, `patch_hydration` gain an optional `workout_id` field.
  - **One new tool**: `workout_fueling_summary` wrapping `GET /workouts/{id}/fueling`.
- **No schema change to `workouts`, `body_weight_entries`, `products`, `nutrition_goals`, `daily_goal_overrides`, idempotency, auth.**

## Capabilities

### Modified Capabilities

- `meals`: `meal_entries` gain an optional `workout_id` FK; the create / patch / list / get endpoints accept and return it.
- `hydration`: `hydration_entries` gain the same column; the same CRUD endpoints accept and return it.
- `workouts`: gains the `GET /workouts/{id}/fueling` aggregation endpoint that pulls meals + hydration in pre/intra/post windows around the workout.
- `mcp-server`: five existing tools gain the optional `workout_id` arg; one new tool (`workout_fueling_summary`) is added.

## Impact

- **Schema migration** at `internal/store/migrations/014_add_workout_link_to_intake.{up,down}.sql`. Two `ALTER TABLE ... ADD COLUMN` statements + two indexes on the new column.
- **No new package**: lives entirely inside the existing `internal/meals/`, `internal/hydration/`, `internal/workouts/`, and `internal/mcpserver/` packages.
- **Code changes**:
  - `internal/meals/types.go`: `WorkoutID *uuid.UUID` field with `omitempty`.
  - `internal/meals/repo.go`: persist + select the new column; new `ResolveWorkoutID` helper for validation.
  - `internal/meals/handlers.go`: decode + validate workout_id on POST/PATCH; PATCH supports the empty-string clear semantic.
  - Equivalent additions in `internal/hydration/`.
  - `internal/workouts/handlers.go`: new `/workouts/:id/fueling` route + handler.
  - `internal/workouts/fueling.go` (new file): the aggregation logic that pulls from meals + hydration over the time windows.
  - `internal/mcpserver/tools_meals.go` + `tools_hydration.go`: extend args with `WorkoutID`; pass through.
  - `internal/mcpserver/tools_workouts.go`: add `WorkoutFuelingSummaryArgs` + `handleWorkoutFuelingSummary` + register.
- **Tests**:
  - Meals + hydration: round-trip with workout_id, PATCH set/clear/no-touch, validation 400 when workout_id missing-from-db.
  - Workouts fueling: cases for pre-only, intra-only, post-only, mixed; default window lengths; custom window lengths; bounded; empty workout returns zero totals.
  - MCP: each touched tool's recorder test forwards `workout_id` when supplied, omits when not; one test per the new tool.
  - Integration test expected-tools list grows by 1 (30 → 31).
- **Documentation**: `task swag`; README "Meals" + "Hydration" + "Workouts" subsections gain mentions of the link; one new MCP-table row.

### Out of scope (explicit non-goals)

- **`workout_fuel_entries` capability** (in-session carbs / sodium / caffeine). Tracked as T1 #2 in `priorities.md`; lands as a separate sibling change after this. The `/workouts/{id}/fueling` endpoint will compose those entries in cleanly when they exist; v1 returns nutrition (from meals) + hydration (from /hydration) only.
- **EA computation** (T1 #4). Needs aggregation over `workouts.kcal_burned` across a date range, not a per-workout view; separate change.
- **Snapshot of the workout reference on meal_entries.** Workouts are events, not definitions; the `ON DELETE SET NULL` semantics mean the link gracefully degrades when a workout is removed. No reason to preserve "this meal was linked to a workout that no longer exists."
- **Filter on `GET /meals?workout_id=` and `GET /hydration?workout_id=`.** Useful but not required for the primary use case (`/workouts/{id}/fueling` covers "show me what fed this workout"). Easy follow-up if real use surfaces the need.
- **Auto-classification.** The agent infers "this snack was probably for that workout" from time/context; not the API's job.
- **Workout context in nutrition summaries** (e.g. `/summary/daily?workout_id=X`). Date-anchored summaries stay date-anchored; workout-anchored fueling has its own endpoint.
- **Per-meal-type / per-product fueling breakdown.** The aggregation returns Totals + entry counts; per-entry detail is one extra call to `GET /meals?from=…&to=…`.
- **Cross-window dedup.** If a meal falls *exactly* on a window boundary (e.g. eaten at workout-start), it lands in `intra_window` by convention (half-open `[start, end)`). Documented; not exposed as a parameter.
