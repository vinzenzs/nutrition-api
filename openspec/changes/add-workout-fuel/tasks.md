## 1. Migration

- [ ] 1.1 Add `internal/store/migrations/015_add_workout_fuel.up.sql` (next available — `014_add_workout_link_to_intake` from `add-meal-workout-link` is the predecessor; renumber if another change takes 015 first):
  - `CREATE TABLE workout_fuel_entries` with the column set, CHECK constraints, and defaults documented in `specs/workout-fuel/spec.md`.
  - `CREATE INDEX workout_fuel_entries_logged_at_idx ON workout_fuel_entries (logged_at);`
  - `CREATE INDEX workout_fuel_entries_workout_id_idx ON workout_fuel_entries (workout_id) WHERE workout_id IS NOT NULL;`
- [ ] 1.2 `.down.sql`: `DROP TABLE workout_fuel_entries;`
- [ ] 1.3 Verify the migration applies cleanly against a fresh `task dev` Postgres and the schema matches.

## 2. Backend: package skeleton

- [ ] 2.1 Create `internal/workoutfuel/` directory.
- [ ] 2.2 `internal/workoutfuel/types.go`: `Entry` struct mirroring the table columns. Use `*float64` for every nullable nutriment field with `omitempty`; `*string` for `Note`; `*uuid.UUID` for `WorkoutID` with `omitempty`.
- [ ] 2.3 `internal/workoutfuel/repo.go`: `Insert(ctx, *Entry) error`, `GetByID(ctx, uuid.UUID) (*Entry, error)`, `Patch(ctx, uuid.UUID, PatchParams) error`, `Delete(ctx, uuid.UUID) error`, `List(ctx, from, to time.Time) ([]*Entry, error)`. `ErrNotFound` sentinel. Use the existing `store.Querier`.
- [ ] 2.4 `internal/workoutfuel/service.go`: validation rules:
  - `name` required (non-empty after trim)
  - `quantity_ml` > 0 when supplied (else `quantity_ml_invalid`)
  - each nutriment field >= 0 when supplied (else `<field>_invalid`)
  - at least one of `{quantity_ml, carbs_g, sodium_mg, potassium_mg, caffeine_mg}` must be set (else `empty_entry`)
  - `logged_at` not > 24h future (else `logged_at_too_far_future`)
  - `note` ≤ 500 chars (else `note_too_long`)
  - If `workout_id` set: validate it exists via a `workouts.Repo.Exists(ctx, uuid)` helper; on miss → `ErrWorkoutNotFound` (re-use the sentinel from `add-meal-workout-link`).
- [ ] 2.5 PATCH's at-least-one-quantitative validation: re-validate the merged-state row (existing values + patch deltas) and reject if the result has all five quantitative fields null.

## 3. Backend: HTTP handlers

- [ ] 3.1 `internal/workoutfuel/handlers.go`: `Register(rg *gin.RouterGroup)` mounting POST/GET on `/workout-fuel`, PATCH/DELETE on `/workout-fuel/:id`.
- [ ] 3.2 POST handler: decode body with `c.ShouldBindJSON`; validate via service; map errors to the documented codes. Insert and return `201` with the created entry.
- [ ] 3.3 GET (list) handler: validate `from` and `to` (RFC 3339, `from < to`, span ≤ 92 days); map errors as `window_required` / `window_invalid` / `range_too_large`. Wrap rows in `{"entries":[...]}`.
- [ ] 3.4 PATCH handler: decode partial body (tolerant decoder; unknown fields ignored). `workout_id` honours the `""` clear sentinel (same pattern as meals/hydration after `add-meal-workout-link`). Validate via service. 404 `workout_fuel_not_found` on unknown id.
- [ ] 3.5 DELETE handler: 204 on success, 404 on unknown id.
- [ ] 3.6 Swag annotations for every handler, listing the documented error codes.

## 4. Wiring

- [ ] 4.1 In `internal/httpserver/server.go`, instantiate `workoutfuel.Repo`, `workoutfuel.Service`, `workoutfuel.Handlers`. Register on the existing API group (auth + idempotency middleware applies uniformly).
- [ ] 4.2 Pass `workoutfuel.Repo` into the workouts fueling aggregation (extends what `add-meal-workout-link` set up — `internal/workouts/fueling.go` constructor gains a third repo dep).

## 5. Workouts fueling extension

- [ ] 5.1 Extend the `FuelingWindow` type (in `internal/workouts/fueling.go`) with a new `WorkoutFuel FuelingWorkoutFuel` sub-object. `FuelingWorkoutFuel{Totals WorkoutFuelTotals; EntryCount int}` where `WorkoutFuelTotals{QuantityMl, CarbsG, SodiumMg, PotassiumMg, CaffeineMg *float64}` — pointer fields so nil-when-no-contribution is preserved.
- [ ] 5.2 Extend the aggregation loop in the fueling handler to also fetch `workoutfuel.Repo.List(start, end)` per window and sum into the new sub-object. Round via `numfmt.Round1` at the response boundary.
- [ ] 5.3 Update the fueling handler's swag annotations to document the new `workout_fuel` sub-object.

## 6. Backend tests

- [ ] 6.1 `internal/workoutfuel/handlers_test.go` with the standard `storetest.NewPool` pattern. Cover: log happy path (gel + electrolyte drink), missing name, empty_entry (no quantitative field), each `<field>_invalid` rejection, `quantity_ml = 0` rejection, nutriment field = 0 accepted, logged_at > 24h future, note > 500 chars, list window (in-range, out-of-range, missing/inverted window, range_too_large), idempotency replay (same key + body → same id; different body + same key → 409), PATCH partial / `workout_id` set/clear/no-touch / invalid `workout_id` / patch-to-empty rejected, DELETE happy + 404.
- [ ] 6.2 `internal/workoutfuel/cascade_test.go` (or a section of the same file): create a workout + a workout-fuel entry linked to it, delete the workout, fetch the fuel entry, confirm `workout_id` is now NULL.
- [ ] 6.3 `internal/workouts/fueling_test.go` extend with cases:
  - Workout-fuel entry inside intra window contributes to `intra_window.workout_fuel.totals`.
  - Workout-fuel entry on the `started_at` boundary lands in `intra_window`.
  - Workout-fuel entry on the `ended_at` boundary lands in `post_window`.
  - Mixed window with all three sub-objects populated.
  - Workout-fuel `entry_count` is 0 (and totals carry zeros/nulls) when no entries fall in a window.
  - Rounding test: assemble totals producing `.x666` values; assert rounded to 1dp.
- [ ] 6.4 `internal/hydration/summary_handlers_test.go`: assert `/summary/hydration/daily` does NOT include workout-fuel ml — insert a hydration entry (300 ml) and a workout-fuel entry (500 ml) on the same date, assert `total_ml: 300`.
- [ ] 6.5 `internal/summary/handlers_test.go`: assert `/summary/daily` does NOT include workout-fuel carbs — insert a meal (50g carbs) and a workout-fuel entry (80g carbs) on the same date, assert `totals.carbs_g: 50`.

## 7. MCP wrapper

- [ ] 7.1 `internal/mcpserver/tools_workout_fuel.go`. Four input structs:
  - `LogWorkoutFuelArgs{Name, LoggedAt, QuantityMl*, CarbsG*, SodiumMg*, PotassiumMg*, CaffeineMg*, Note*, WorkoutID, IdempotencyKey}`.
  - `ListWorkoutFuelArgs{From, To}`.
  - `PatchWorkoutFuelArgs{ID, Name*, QuantityMl*, CarbsG*, SodiumMg*, PotassiumMg*, CaffeineMg*, Note*, LoggedAt*, WorkoutID, IdempotencyKey}` (every editable field as `*…`; `WorkoutID` is a plain string to support empty-string-clear).
  - `DeleteWorkoutFuelArgs{ID, IdempotencyKey}`.
- [ ] 7.2 Four handlers following the existing `tools_hydration.go` patterns. `effectiveIdempotencyKey` for writes; reads no key; delete returns 204→empty.
- [ ] 7.3 `registerWorkoutFuelTools(server, c)` with descriptions per the spec — particularly the routing-rule explanation on `log_workout_fuel` (plain water → hydration; carbs/electrolytes/caffeine → workout-fuel) and the `caffeine_mg: 0` vs NULL semantic.
- [ ] 7.4 Wire `registerWorkoutFuelTools` in `internal/mcpserver/server.go`.
- [ ] 7.5 Update the `workout_fueling_summary` tool's description (in `internal/mcpserver/tools_workouts.go`) to mention the third sub-object `workout_fuel` in each window's response.

## 8. MCP tests

- [ ] 8.1 `internal/mcpserver/tools_workout_fuel_test.go`: per-tool tests using the recorder pattern. Cover endpoint URLs, method, body, idempotency-key forwarding (explicit + derived), response passthrough, 404 / 4xx as `isError=true`. Specifically:
  - `log_workout_fuel` posts the body with optional fields when supplied, omits them when not.
  - `log_workout_fuel` forwards `workout_id` when supplied.
  - `list_workout_fuel` builds the query string from `from` / `to`, sends no Idempotency-Key.
  - `patch_workout_fuel` omits unset fields from the body; forwards `workout_id: ""` for clear.
  - `delete_workout_fuel`: 204 → empty content; 404 → isError.
- [ ] 8.2 Update `internal/mcpserver/mcp_integration_test.go` expected-tools list to include the four new tools. Tool count grows by 4 (current count + 4).

## 9. Documentation

- [ ] 9.1 `task swag` to regenerate OpenAPI for the new routes + the extended fueling endpoint response shape.
- [ ] 9.2 `README.md`:
  - Add a "Workout fuel" subsection under the API examples (placed after "Hydration", before "Workouts" — so the reader sees both as parallel capture surfaces).
  - Show: POST a gel, POST an electrolyte drink, POST with `workout_id`, GET list, PATCH change `sodium_mg`.
  - Add the four new tools to the MCP tools table.
  - Add `internal/workoutfuel/` to the project-layout section.
  - Update the "Workouts" subsection example for `/workouts/{id}/fueling` to show the response with all three sub-objects.
- [ ] 9.3 `RUN_LOCAL.md`: extend the API walkthrough — log a workout, log a workout-fuel entry during the intra window (`workout_id` set), then `GET /workouts/{id}/fueling` and observe the `intra_window.workout_fuel.totals` populated. Bonus: a second `GET /summary/hydration/daily` showing that the workout-fuel ml does NOT bleed into the daily hydration total (the documented unit-isolation rule, observed live).

## 10. Pre-merge checks

- [ ] 10.1 `task vet` clean.
- [ ] 10.2 `task test` green (use `-p 1` if testcontainers parallel boot flakes surface).
- [ ] 10.3 Manual e2e: with `task dev` running:
  - POST a workout, capture its id.
  - POST a workout-fuel entry inside the intra window with `workout_id` set, carbs and sodium populated.
  - GET `/workouts/{id}/fueling` → assert `intra_window.workout_fuel.entry_count = 1` and totals match.
  - GET `/summary/hydration/daily?date=…` → assert `total_ml` does NOT include the workout-fuel ml.
  - PATCH the workout-fuel entry's `workout_id` to `""`, re-fetch the fueling summary — entry still appears in the time window (tag-independence rule), confirming aggregation is time-based.
- [ ] 10.4 OpenSpec validation: `openspec status --change "add-workout-fuel"` shows 4/4 artifacts done.
