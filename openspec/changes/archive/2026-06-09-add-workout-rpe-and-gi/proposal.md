## Why

The 70.3 build phase mandates fueling rehearsal on every long ride from Week 9 onward. The API records *what* was consumed (`workout_fuel_entries` with carbs_g, sodium_mg, caffeine_mg, etc.) but nothing about *how the session felt*. Without `gi_distress_score` and `rpe` on the workout, you can't iterate the strategy — you're logging what you ate but not whether it worked. Listed in `openspec/priorities.md` as **T2 #6D** ("GI distress / RPE on workout fueling entries") and explicitly framed as "THE primary data captured during training — race-fueling rehearsal data."

This change adds two nullable per-session integer fields to `workouts`: `rpe` (1–10 Borg CR-10 perceived effort) and `gi_distress_score` (1–5, where 1 = no distress, 5 = severe). Both stay nullable because not every workout gets rehearsed (Z1 spin doesn't need RPE/GI logged) and back-filling years of historical workouts is a non-goal. Per-product GI attribution stays in the existing `workout_fuel_entries.note` free-text field — the design.md captures the trade.

## What Changes

- **New migration** `018_add_workout_rpe_and_gi.up.sql` adding `rpe INTEGER NULL CHECK (rpe IS NULL OR (rpe BETWEEN 1 AND 10))` and `gi_distress_score INTEGER NULL CHECK (gi_distress_score IS NULL OR (gi_distress_score BETWEEN 1 AND 5))` to the `workouts` table.
- **`internal/workouts/types.go` `Workout` struct** gains the two pointer fields (`RPE *int`, `GIDistressScore *int`) with `omitempty` JSON tags — matches the existing nullable-fields pattern (KcalBurned, AvgHR, TSS, Notes).
- **`internal/workouts/repo.go`** — both Upsert and the eventual Patch path carry the new columns. selectCols extended.
- **`internal/workouts/handlers.go`** — `POST /workouts` (createRequest) + `PATCH /workouts/:id` (patchRequest) both accept the new fields; validation rejects out-of-range values with `400 rpe_invalid` (1..10) and `400 gi_distress_score_invalid` (1..5).
- **`workout_fueling` aggregation** — the existing `/workouts/{id}/fueling` response already exposes the workout subset; gains `rpe` and `gi_distress_score` echoes on the workout summary block so the agent reads the rehearsal data alongside the carbs/sodium/caffeine totals it's evaluating.
- **MCP tools** — `log_workout`, `patch_workout`, and `get_workout` schemas + descriptions updated to mention the new fields. Reading `rpe` and `gi_distress_score` in the agent's context becomes the natural prompt for "how did the fueling work?"
- **swag** regenerates `docs/` with the new optional params on the workout endpoints and the new fields on the response shapes.

## Capabilities

### Modified Capabilities
- `workouts`: the `Workout` shape requirement gains two nullable per-session fields with validation rules; `POST /workouts` and `PATCH /workouts/:id` accept them; `GET /workouts/{id}` and `GET /workouts` echo them.
- `workout-fuel`: the workout-fueling aggregation (`GET /workouts/{id}/fueling`) response shape gains the two fields on its workout summary block — the agent reads them alongside the fueling totals.
- `mcp-server`: `log_workout`, `patch_workout`, `get_workout`, and `list_workouts` tool schemas + descriptions updated. No new tools.

## Impact

- **Schema migration** (likely `018_add_workout_rpe_and_gi.up.sql`):
  - `ALTER TABLE workouts ADD COLUMN rpe INTEGER NULL CHECK (rpe IS NULL OR (rpe BETWEEN 1 AND 10));`
  - `ALTER TABLE workouts ADD COLUMN gi_distress_score INTEGER NULL CHECK (gi_distress_score IS NULL OR (gi_distress_score BETWEEN 1 AND 5));`
  - Down: drop both columns.
  - No back-fill — existing rows get NULL. Workouts logged before this change stay valid; the agent treats absence as "not rehearsed."
- **`internal/workouts/`**: `types.go` adds the two pointer fields with JSON tags. `repo.go` Upsert + selectCols updated; Patch path extended (if it exists per the existing tests). `handlers.go` createRequest + patchRequest accept the fields with range validation; handler error code map gains `rpe_invalid` and `gi_distress_score_invalid`.
- **`internal/workoutfueling/fueling.go`** (the `/workouts/{id}/fueling` aggregator): the `WorkoutFueling` response struct's workout summary block gains the two fields. The aggregator already reads the full workout row; just adds two fields to the projection.
- **`internal/mcpserver/tools_workouts.go`**: `LogWorkoutArgs` and `PatchWorkoutArgs` gain `RPE *int` and `GIDistressScore *int` with jsonschema descriptions naming the scales explicitly (`Borg CR-10` for RPE; `1 = no GI distress, 5 = severe` for the GI score). Tool descriptions for `log_workout` and `patch_workout` get one sentence each on what the fields mean and why to log them.
- **Tests**:
  - Repo tests: upsert with both fields, upsert with one field only, fields stored as NULL when omitted, range constraint enforced at the DB layer.
  - Handler tests: POST with valid + invalid values for each field; PATCH partial update; out-of-range returns the right error code; GET response includes the fields when set.
  - Workoutfueling aggregator test: the workout summary block carries RPE and GI score when the underlying workout has them.
  - MCP wrapper tests: explicit values forwarded verbatim; absent values omitted from POST/PATCH body.
- **Documentation**:
  - README "Workouts" subsection gains a one-paragraph note + a curl example PATCHing RPE + GI on an existing workout (the canonical post-ride flow).
  - RUN_LOCAL gets a fueling-rehearsal example: log workout → log workout-fuel entries during/after → PATCH the workout with RPE + GI when done.
  - `task swag` regen.

### Out of scope (explicit non-goals)

- **Per-fuel-entry GI / RPE.** GI distress is captured at workout granularity; per-product attribution stays in `workout_fuel_entries.note` as free-text the agent can reason over. The "Maurten worked, SIS didn't" comparison happens at the agent's reasoning layer, not via structured per-entry diagnostic fields. The design.md "decisions" section spells out why.
- **Auto-derived fields** (e.g. `worst_gi_in_session` on the workout response) — would belong to a later "rehearsal report" capability if the per-product comparison gets formalised.
- **Back-filling historical workouts.** Existing rows keep `NULL` for both fields; "not rehearsed" is a meaningful signal, not a data quality bug.
- **External-source overrides.** Garmin doesn't surface these fields (RPE is manual; GI is not measured). The Garmin importer leaves them untouched; the user PATCHes after the ride.
- **A `rehearsal_protocol` enum on workouts** (e.g. "race-pace nutrition rehearsal" vs "easy ride"). Tagged training context belongs to the phase-template layer (T1 #5/#1A already shipped) or to a future per-workout label; not in scope here.
- **Aggregations across sessions** (e.g. "average RPE across the last 4 long rides"). Belongs to `rolling_summary` / a future analytics capability; not this primitive.
- **A separate `effort` or `subjective_load` field beyond Borg CR-10 RPE.** RPE is the single industry-standard subjective effort signal; no need to triplicate.
