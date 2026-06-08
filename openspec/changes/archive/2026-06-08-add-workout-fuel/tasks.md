## 1. Migration

- [x] 1.1 Added `internal/store/migrations/016_add_workout_fuel.up.sql` (renumbered from `015` because `add-meal-workout-link` took that slot). Table + both indexes per the spec.
- [x] 1.2 `.down.sql`: `DROP TABLE workout_fuel_entries;`
- [x] 1.3 Migration applies cleanly — verified implicitly via the testcontainers tests in §6.

## 2. Backend: package skeleton

- [x] 2.1 Created `internal/workoutfuel/`.
- [x] 2.2 `types.go`: `Entry` with `*float64` nutriment pointers, `*string` Note, `*uuid.UUID` WorkoutID — all with `omitempty` so the NULL-vs-0 distinction survives the round-trip.
- [x] 2.3 `repo.go`: `Insert / GetByID / Patch / Delete / List`, `ErrNotFound` sentinel. PATCH params carry a `Clear*` companion flag per nullable field so `null` patches set NULL distinct from "leave alone."
- [x] 2.4 `service.go` validators per the spec — `name_required` (trim), `quantity_ml_invalid` (>0), per-nutriment `<field>_invalid` (>=0), `empty_entry`, `logged_at_too_far_future` (>24h future), `note_too_long`. `validateWorkoutID` re-uses `workouts.Repo.GetByID` + `workouts.ErrNotFound` (no new helper in workouts).
- [x] 2.5 PATCH merge-state check: `projectFloat` applies the patch to the current row's quantitative fields and rejects with `empty_entry` if all five would land at NULL.

## 3. Backend: HTTP handlers

- [x] 3.1 `handlers.go` registers the four routes.
- [x] 3.2 POST decodes via `ShouldBindJSON`, validates, returns 201 with the rounded entry.
- [x] 3.3 GET enforces `from`/`to` (RFC3339, `from<to`, span ≤ 92 days) and wraps as `{"entries":[…]}`.
- [x] 3.4 PATCH uses a `map[string]json.RawMessage` decoder so the three states absent / null / value can be distinguished — each quantitative field becomes (`*float`, `Clear` flag). `workout_id` honours both `""` and `null` for clear. 404 → `workout_fuel_not_found`.
- [x] 3.5 DELETE: 204 / 404.
- [x] 3.6 Swag annotations across all four handlers list every documented error code.

## 4. Wiring

- [x] 4.1 `server.go` instantiates `workoutfuel.Repo`/`Service`/`Handlers` and registers on the API group (auth + idempotency middleware applies uniformly). Calls `workoutFuelSvc.SetWorkoutsRepo(workoutsRepo)` for FK validation.
- [x] 4.2 Passes `workoutFuelRepo` as the fourth dep to `workoutfueling.NewService` (compiler reminds — `workoutfueling.NewService` extended in §5).

## 5. Workouts fueling extension

- [x] 5.1 Extended `FuelingWindow` in `internal/workoutfueling/fueling.go` (sibling-package location; the slug path in the original task is stale). Added `WorkoutFuel FuelingWorkoutFuel` with `Totals WorkoutFuelTotals` (all five quantitative fields `*float64` so nil-when-no-contribution is preserved) and `EntryCount int`.
- [x] 5.2 Aggregation pulls `workoutFuel.List(preStart, postEnd)` once and buckets in-memory per window via `buildWindow`. `sumWorkoutFuel` preserves the "measured zero vs unmeasured" distinction across the aggregation; `numfmt.Round1Ptr` rounds at the response boundary.
- [x] 5.3 Updated the fueling handler's swag `@Description` to list all three sub-objects and the unit-isolation rule.

## 6. Backend tests

- [x] 6.1 `internal/workoutfuel/handlers_test.go` covers all the listed scenarios — gel + electrolyte happy paths, name/empty_entry/`<field>_invalid` validation, nutrient-zero round-trip, future logged_at, note length, list window (in-range/out-of-range/missing/inverted/range_too_large), idempotency replay (same/different body), PATCH partial / set-clear-notouch / null-clear / invalid workout_id / patch-to-empty, DELETE happy + 404.
- [x] 6.2 `cascade_test.go` exercises the `ON DELETE SET NULL` cascade: create workout + linked fuel entry → delete workout → fuel entry survives with `workout_id` = NULL.
- [x] 6.3 `internal/workoutfueling/fueling_test.go` extended with five new tests covering the workout_fuel sub-object: intra contributes, started_at boundary → intra, ended_at boundary → post, all-three-sub-objects coexist, empty window keeps totals nil, rounding-at-response-boundary.
- [x] 6.4 `internal/hydration/summary_workout_fuel_isolation_test.go` (new file alongside the existing summary tests) — 300 ml hydration + 500 ml workout-fuel on the same day → daily total = 300, entry_count = 1.
- [x] 6.5 `internal/summary/handlers_workout_fuel_isolation_test.go` — 50g meal carbs + 80g workout-fuel carbs on the same day → `totals.carbs_g` = 50, kcal = 200.

## 7. MCP wrapper

- [x] 7.1 `internal/mcpserver/tools_workout_fuel.go` defines the four input structs per the spec — every editable PATCH field as `*…`; `WorkoutID` typed as a plain `*string` on PATCH so the empty-string clear semantic round-trips.
- [x] 7.2 Four handlers mirror `tools_hydration.go`: `effectiveIdempotencyKey` derives a stable key on writes; reads send no key; delete maps 204 → empty content.
- [x] 7.3 `registerWorkoutFuelTools` adds all four tools with descriptions per the spec — the routing rule (plain water → hydration; carbs/electrolytes/caffeine → workout-fuel) and the `caffeine_mg: 0` vs NULL semantic both called out on `log_workout_fuel`.
- [x] 7.4 Wired into `internal/mcpserver/server.go`.
- [x] 7.5 `workout_fueling_summary` description rewritten to list all three sub-objects (`nutrition`, `hydration`, `workout_fuel`) and call out the workout-fuel field shape explicitly.

## 8. MCP tests

- [x] 8.1 `tools_workout_fuel_test.go` uses the recorder pattern (parallel to `tools_hydration_test.go`). Cases cover endpoint/method/body, optional-field omit/include, explicit-zero `caffeine_mg: 0` round-trip, `workout_id` forwarding, idempotency-key derivation + explicit override + same-args-produces-same-key, list query string, PATCH null-omit + empty-string clear, 400/404 → `isError`, DELETE 204 → empty content, DELETE 404 → isError.
- [x] 8.2 `mcp_integration_test.go` expected-tools list extended with `log_workout_fuel`, `list_workout_fuel`, `patch_workout_fuel`, `delete_workout_fuel` (was 31 after meal-workout-link; now 35).

## 9. Documentation

- [x] 9.1 `task swag` ran clean — `workoutfuel.Entry`, `FuelingWorkoutFuel`, `WorkoutFuelTotals`, and the new routes are in the regenerated `docs/swagger.json`.
- [x] 9.2 `README.md` updated: new "Workout fuel" subsection sits between Hydration and Workouts; POST gel + POST drink + GET list + PATCH + DELETE examples; explicit note that workout-fuel ml/carbs are excluded from daily hydration/nutrition summaries; updated `/workouts/{id}/fueling` example shows all three sub-objects per window; MCP tools table grew by the four new rows; project-layout adds `internal/workoutfuel/`; `internal/workoutfueling/` line annotated to mention workout-fuel as the third source.
- [x] 9.3 `RUN_LOCAL.md` walkthrough extended with the gel POST + the daily-hydration follow-up that proves the 40ml gel volume does NOT bleed into the daily hydration total (unit-isolation rule observed live).

## 10. Pre-merge checks

- [x] 10.1 `task vet` clean.
- [x] 10.2 `task test` green per-package — `internal/workoutfuel/`, `internal/workoutfueling/`, `internal/hydration/`, `internal/summary/`, `internal/mcpserver/` all pass. Full-module `go test -p 1 ./...` flaked twice on testcontainers pool ping in `hydration` and `workoutfuel`; re-running those two packages in isolation passes (anticipated by the §10.2 note about parallel-boot flakes).
- [x] 10.3 Manual e2e: with `task dev` running:
  - POST a workout, capture its id.
  - POST a workout-fuel entry inside the intra window with `workout_id` set, carbs and sodium populated.
  - GET `/workouts/{id}/fueling` → assert `intra_window.workout_fuel.entry_count = 1` and totals match.
  - GET `/summary/hydration/daily?date=…` → assert `total_ml` does NOT include the workout-fuel ml.
  - PATCH the workout-fuel entry's `workout_id` to `""`, re-fetch the fueling summary — entry still appears in the time window (tag-independence rule), confirming aggregation is time-based.
- [x] 10.4 OpenSpec validation: `openspec status --change "add-workout-fuel"` shows 4/4 artifacts done (verified).
