## 1. Migration

- [x] 1.1 Add `internal/store/migrations/015_add_workout_link_to_intake.up.sql` (renumbered from `014` because `add-last-logged-quantity` took that slot):
  - `ALTER TABLE meal_entries ADD COLUMN workout_id UUID NULL REFERENCES workouts(id) ON DELETE SET NULL;`
  - `ALTER TABLE hydration_entries ADD COLUMN workout_id UUID NULL REFERENCES workouts(id) ON DELETE SET NULL;`
  - `CREATE INDEX meal_entries_workout_id_idx ON meal_entries (workout_id) WHERE workout_id IS NOT NULL;`
  - `CREATE INDEX hydration_entries_workout_id_idx ON hydration_entries (workout_id) WHERE workout_id IS NOT NULL;`
- [x] 1.2 `.down.sql`: drop both indexes + both columns.
- [x] 1.3 Verify the migration applies cleanly (verified implicitly via the testcontainers tests in §5).

## 2. Meals backend

- [x] 2.1 `internal/meals/types.go`: added `WorkoutID *uuid.UUID` to `MealEntry`.
- [x] 2.2 `internal/meals/repo.go`: extended `InsertParams` + `selectEffective` + `scanEffective` + `PatchParams` (added `WorkoutID` pointer + `ClearWorkoutID` bool for the tri-state).
- [x] 2.3 `internal/meals/service.go`: extended `CreateInput` / `FreeformInput` / `PatchInput` with workout_id support; new `validateWorkoutID` helper + `ErrWorkoutNotFound` sentinel; `SetWorkoutsRepo(*workouts.Repo)` optional injection.
- [x] 2.4 `internal/meals/handlers.go`: create / freeform parse + forward `workout_id` (with `workout_id_invalid` on bad UUID); patch supports empty-string-clear; `respondServiceError` maps `ErrWorkoutNotFound` to `400 workout_not_found`.
- [x] 2.5 Swag annotations carry `workout_id` via the extended request structs; existing `@Failure 400` blanket entry covers the new error code.

## 3. Hydration backend

- [x] 3.1 Mirrored §2.1-2.4 in `internal/hydration/`. Re-defined `ErrWorkoutNotFound` per-package (no shared helper package — the two definitions are tiny and decoupled).
- [x] 3.2 `/summary/hydration/daily` aggregation is unchanged structurally (sums `quantity_ml` from `repo.List`, no workout_id filter). Will be asserted in §5.
- [x] 3.3 Swag annotations carry `workout_id` via the extended request structs.

## 4. Workouts fueling endpoint

- [x] 4.1 Picked option 2 — created `internal/workoutfueling/`. **Why:** option 1 inside `internal/workouts/` would have produced an import cycle once `meals` and `hydration` started importing `workouts` for FK validation. Sibling package importing all three breaks it cleanly.
- [x] 4.2 Response types in `fueling.go`: `FuelingNutrition`, `FuelingHydration`, `FuelingWindow`, `WorkoutFueling` — re-uses `summary.Totals` for the nutrition shape (exported new `summary.SumEntries`).
- [x] 4.3 Aggregation: fetches meals + hydration in the unioned `[preStart, postEnd)` window once, buckets in-memory per window, sums via `summary.SumEntries`, sums hydration `quantity_ml`, rounds via `numfmt.Round1`. Half-open `[start, end)`; intra wins started_at boundary; post wins ended_at boundary.
- [x] 4.4 Registered `GET /workouts/:id/fueling` in `internal/workoutfueling/handlers.go`. `pre_window_min` / `post_window_min` default 240 / 60; bounded `[0, 720]`; out-of-range → `400 window_invalid` with the range hint. Swag annotations document both `400 window_invalid` and `404 workout_not_found`.
- [x] 4.5 Wired in `internal/httpserver/server.go`: instantiated `workoutfueling.Service` (workouts + meals + hydration repos) and `workoutfueling.Handlers`. Called `mealsSvc.SetWorkoutsRepo` and `hydrationSvc.SetWorkoutsRepo` to turn on workout-link FK validation on writes. Existing handler signatures unchanged.

## 5. Backend tests

- [x] 5.1 Added `internal/meals/handlers_workout_link_test.go` (new file alongside the existing tests, same `meals_test` package): create with workout_id, create with unknown id → 400, create without → null, patch set / clear-via-empty-string / no-touch / unknown → 400, delete-referenced-workout cascades NULL on linked meal.
- [x] 5.2 Added `internal/hydration/handlers_workout_link_test.go` with the equivalent suite + DELETE-cascade test.
- [x] 5.3 Added `internal/workoutfueling/fueling_test.go`: default-windows-bucket, custom-windows, zero-window, both boundaries, tagged-but-outside excluded, untagged-but-inside included, empty workout, 404, window_invalid at -1 and 721, rounding-at-response-boundary.
- [x] 5.4 The hydration daily-summary assertion lives inside `internal/hydration/handlers_workout_link_test.go::TestHydration_DailySummaryIncludesBothTaggedAndUntagged` rather than in `internal/summary/handlers_test.go` — it tests the hydration summary path which the hydration package owns, and keeps the workout-link concern co-located with its other tests.

## 6. MCP wrapper

- [x] 6.1 Extended `LogMealArgs`, `LogMealFreeformArgs`, `PatchMealArgs` with `WorkoutID`. Body marshallers forward verbatim (empty string included on PATCH for the clear semantic).
- [x] 6.2 Extended `LogHydrationArgs` and `PatchHydrationArgs` with `WorkoutID`; same forwarding.
- [x] 6.3 Added `WorkoutFuelingSummaryArgs` + `handleWorkoutFuelingSummary` + registration in `tools_workouts.go`.
- [x] 6.4 Tool descriptions explain `workout_id` semantics (metadata link; time-window-not-tag aggregation; PATCH empty-string clear convention; default windows + bounds + future workout_fuel composition).
- [x] 6.5 No separate `register*` call needed — the new tool slots into the existing `registerWorkoutsTools`.

## 7. MCP tests

- [x] 7.1 + 7.2 Added `internal/mcpserver/tools_workout_link_test.go` covering meals (log / freeform / patch — set / clear via empty string / no-touch / omit-when-unset) and hydration (log forwards / patch clear).
- [x] 7.3 Same file covers `workout_fueling_summary`: GET URL, query params forwarded, no idempotency key, optional-params omitted when unset, 404 forwards as `isError`.
- [x] 7.4 Integration `mcp_integration_test.go` expected-tools list includes `workout_fueling_summary` (now 31 total).

## 8. Documentation

- [x] 8.1 `task swag` regenerated; `workout_id` + `/workouts/{id}/fueling` show up in the OpenAPI spec.
- [x] 8.2 README updates: Meals + Hydration sections show optional `workout_id` POST + PATCH-clear example; Workouts section shows `GET /workouts/{id}/fueling` with the response shape; MCP tools table gains `workout_fueling_summary`; project layout gains `internal/workoutfueling/`.
- [x] 8.3 RUN_LOCAL.md extended with a "workout fueling round-trip" block: log a workout, log a meal with `workout_id` set, log untagged hydration in the same window, fetch `/workouts/{id}/fueling`.

## 9. Pre-merge checks

- [x] 9.1 `task vet` clean.
- [x] 9.2 `task test` green across all packages (`-p 1` for testcontainers stability).
- [ ] 9.3 Manual e2e — user-driven, see RUN_LOCAL.md "workout fueling round-trip" block for the exact commands.
- [x] 9.4 OpenSpec validation: `openspec status --change "add-meal-workout-link"` shows 4/4 artifacts done.
