## 1. Schema migration

- [x] 1.1 Confirm next migration slot with `ls internal/store/migrations/`. At time of proposal `017` was last; expected next is `018`. Verify before writing.
- [x] 1.2 Create `internal/store/migrations/018_add_workout_rpe_and_gi.up.sql`:
  - `ALTER TABLE workouts ADD COLUMN rpe INTEGER NULL CHECK (rpe IS NULL OR (rpe BETWEEN 1 AND 10));`
  - `ALTER TABLE workouts ADD COLUMN gi_distress_score INTEGER NULL CHECK (gi_distress_score IS NULL OR (gi_distress_score BETWEEN 1 AND 5));`
- [x] 1.3 Create `internal/store/migrations/018_add_workout_rpe_and_gi.down.sql` dropping both columns (drop order doesn't matter — no FKs).
- [x] 1.4 Run `task migrate:up` against the local dev pool, then `task migrate:down`, then `task migrate:up` to leave applied. Confirm both directions clean up.

## 2. Workouts package — types + repo

- [x] 2.1 Update `internal/workouts/types.go` `Workout` struct to add `RPE *int json:"rpe,omitempty"` and `GIDistressScore *int json:"gi_distress_score,omitempty"`. Place them adjacent to `KcalBurned`, `AvgHR`, `TSS` to mirror the existing nullable-fields cluster.
- [x] 2.2 Update `internal/workouts/repo.go` `selectCols` constant to include `rpe, gi_distress_score`. Update every `scanWorkout` / `rows.Scan` call site to read the two new fields (the scan function is reused across GetByID, List, Upsert-readback — touch all of them).
- [x] 2.3 Update `internal/workouts/repo.go` Upsert SQL: add `rpe`, `gi_distress_score` to the INSERT column list, the VALUES list, and the ON CONFLICT DO UPDATE SET clause. Update `Upsert`'s arg pack to include `w.RPE, w.GIDistressScore`.
- [x] 2.4 Update the patch path in `internal/workouts/repo.go` (`Patch` or `PatchParams`-driven). Add `RPE *int`, `GIDistressScore *int` to `PatchParams` plus tri-state sentinels: `ClearRPE bool`, `ClearGIDistressScore bool`. `HasUpdates` covers them. The Patch SQL builder appends `rpe = NULL` / `rpe = $N` based on which is set, mirroring the existing `ClearDefaultTemplateID` pattern from `add-training-phases-and-templates`.

## 3. Workouts package — handlers + validation

- [x] 3.1 Update `internal/workouts/handlers.go` `createRequest` to accept `RPE *int` and `GIDistressScore *int`. Update `LogWorkout` handler to copy them onto the `Workout` it persists.
- [x] 3.2 Add `validateRPE(*int)` and `validateGIDistressScore(*int)` helpers in `internal/workouts/service.go` (or wherever validation lives) — return sentinel errors `ErrRPEInvalid` (1..10) and `ErrGIDistressScoreInvalid` (1..5). Both: nil → ok; integer in range → ok; out of range → sentinel; non-int payload → handled at JSON-decode boundary.
- [x] 3.3 Wire validation in `LogWorkout` and `PatchWorkout` handlers. Map sentinels to `400 rpe_invalid` and `400 gi_distress_score_invalid` with `range: {min, max}` body shape (mirror `body_weight_kg_invalid` from race-prep).
- [x] 3.4 PATCH endpoint tri-state handling: the patch body uses `json.RawMessage` for the two fields, decoded twice — first into `*int` for the value, then peek the raw bytes to distinguish `null` from absent. Alternative: a custom `NullableInt { Set bool; Null bool; Value int }` type with `UnmarshalJSON`. Pick whichever fits the existing codebase style (workout-link uses empty-string-sentinel; this is the first numeric-tri-state field). Document the choice in the PR description.
- [x] 3.5 Update swag annotations on `POST /workouts` and `PATCH /workouts/:id` for the two new fields and the new error codes.

## 4. workoutfueling — surface rehearsal fields on /workouts/{id}/fueling

- [x] 4.1 Update `internal/workoutfueling/fueling.go` `WorkoutFueling` struct to add `RPE *int json:"rpe,omitempty"` and `GIDistressScore *int json:"gi_distress_score,omitempty"` at the top level (alongside `WorkoutID`, `StartedAt`, `EndedAt`).
- [x] 4.2 Update `Service.BuildFor` (or wherever the workout row is fetched) to copy `w.RPE` and `w.GIDistressScore` onto the response.
- [x] 4.3 Update swag annotations on `GET /workouts/{id}/fueling` to document the two new optional response fields.

## 5. Tests — workouts package

- [x] 5.1 Repo tests in `internal/workouts/repo_test.go`: Upsert with both fields set / one field set / neither set; GetByID/List round-trips preserve the values; CHECK constraint enforced at DB layer (try inserting `rpe = 11` directly via SQL and expect failure — gives defence-in-depth coverage).
- [x] 5.2 Patch tests: set both, set one + leave other unchanged, clear one via tri-state, clear both, range validation on set.
- [x] 5.3 Handler tests in `internal/workouts/handlers_test.go`: `POST /workouts` happy path with both fields → 201 + echo; out-of-range `rpe`/`gi_distress_score` → 400 with right error code + `range`; non-integer payload → 400 invalid_json or specific error code; omitting both fields → 201 with neither in response. PATCH: set, clear-via-null, leave-unchanged, range-violation-rolls-back-other-field.
- [x] 5.4 GET tests: GET on a row with both fields includes them; GET on a NULL row omits them (omitempty contract).

## 6. Tests — workoutfueling aggregator

- [x] 6.1 Update `internal/workoutfueling/fueling_test.go`: seed a workout with `rpe = 7` and `gi_distress_score = 2`; call `BuildFor`; assert the response carries both fields at the top level.
- [x] 6.2 Seed a workout with NULL on both; assert both fields are omitted from the JSON-serialized response (use `assert.NotContains(body, "rpe")` and same for the other).

## 7. MCP tools

- [x] 7.1 Update `internal/mcpserver/tools_workouts.go` `LogWorkoutArgs` and `PatchWorkoutArgs` to add `RPE *int` and `GIDistressScore *int` with jsonschema descriptions naming the scales explicitly (Borg CR-10 for RPE; 1=no distress, 5=severe for GI).
- [x] 7.2 Update the Patch wrapper to encode tri-state correctly: present-with-integer → JSON integer; present-with-explicit-null → JSON null (clear); absent → field omitted from body. Use `json.RawMessage` or the NullableInt type chosen in 3.4.
- [x] 7.3 Update `log_workout` and `patch_workout` tool descriptions (the long-form text in `registerWorkoutsTools`) to name what the fields mean and when to log them. One sentence per field.
- [x] 7.4 Wrapper tests in `internal/mcpserver/tools_workouts_test.go`: log_workout with both fields → body contains them; log_workout without fields → body omits them; patch_workout with rpe set → body has `"rpe": <int>`; patch_workout with `Clear*` semantics → body has `"rpe": null`.

## 8. Documentation

- [x] 8.1 Run `task swag` to regenerate `docs/swagger.{json,yaml,docs.go}`.
- [x] 8.2 Update `README.md` "Workouts" subsection: short paragraph explaining the rehearsal-data fields + a curl example showing `PATCH /workouts/{id}` setting `rpe` and `gi_distress_score` (the canonical post-ride flow).
- [x] 8.3 Update `RUN_LOCAL.md`'s workout walkthrough with a fueling-rehearsal example: log workout (manual or Garmin-imported) → log workout-fuel entries → PATCH the workout with RPE + GI when done → GET `/workouts/{id}/fueling` and confirm the rehearsal signals appear alongside fueling totals.

## 9. Verify and hand off

- [x] 9.1 Run `task test` — all packages green. (Re-run any package showing the documented testcontainers parallel-boot flake alone with `-p 1`.)
- [x] 9.2 Run `task build` and exercise via curl: POST a manual workout with both fields → confirm shape; PATCH on an existing row to add both; GET `/workouts/{id}/fueling` and confirm the fields appear at the top level.
- [x] 9.3 Verify `openspec status --change "add-workout-rpe-and-gi"` reports 4/4 artifacts done and all tasks done.
- [x] 9.4 Commit per CLAUDE.md's "commit after every /opsx:apply" convention: `feat(workouts): add rpe + gi_distress_score for fueling rehearsal data` — include migration, types/repo/handler updates, workoutfueling change, MCP tool updates, doc regen, and the change directory.
- [x] 9.5 Ready for `/opsx:archive add-workout-rpe-and-gi` — at archive time the two delta specs sync into main specs: modified `openspec/specs/workouts/spec.md` and modified (additive) `openspec/specs/mcp-server/spec.md`.
