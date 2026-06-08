## 1. Internal package skeleton

- [x] 1.1 Create `internal/energy/` directory.
- [x] 1.2 `internal/energy/types.go`: request struct (`AvailabilityParams{From, To, TZ, LeanMassKg *float64, BodyFatPct *float64}`) and response structs:
  - `Composition{FFM_kg float64; Source string; BodyWeightKg *float64; BodyWeightSource *string; CompositionEstimated bool ",omitempty"}`
  - `Day{Date string; IntakeKcal, ExerciseEnergyKcal, EA float64; Band string; MissingBurnWorkoutIDs []uuid.UUID; CompleteData bool}`
  - `Window{AvgEA *float64; Band *string; DaysWithCompleteData, TotalDays int}`
  - `Availability{From, To time.Time; TZ string; Days []Day; Window Window; Composition Composition}`
- [x] 1.3 `internal/energy/service.go`: `Service` constructor takes `*meals.Repo`, `*workouts.Repo`, `*bodyweight.Repo`. Single public method `Compute(ctx, AvailabilityParams) (*Availability, error)`.
- [x] 1.4 Validation rules in the service:
  - `lean_mass_kg > 0 && finite` else `ErrLeanMassInvalid` → `400 lean_mass_kg_invalid`
  - `0 <= body_fat_pct < 100 && finite` else `ErrBodyFatInvalid` → `400 body_fat_pct_invalid`
  - `from < to`, both RFC 3339, span ≤ 92 days — surfaced by the handler as `window_invalid` / `range_too_large` (consistent with hydration / workouts / weight endpoints).

## 2. Body weight + FFM resolution

- [x] 2.1 `internal/energy/composition.go`: `resolveBodyWeight(ctx, weightRepo, from, to) (kg *float64, source *string, err)` per Requirement: Body weight resolution — rolling 7d avg first (entries in `[to-7d, to)`), then in-window mean, then last-before-window.
- [x] 2.2 `resolveFFM(ctx, weightRepo, from, to, params) (ffmKg float64, source string, weightKg *float64, weightSource *string, estimated bool, err)` per Requirement: FFM resolution — explicit lean → explicit body-fat → stored body-fat from most-recent in-window entry → 85% fallback → `ErrWeightDataMissing` → `400 weight_data_missing`.
- [x] 2.3 Add `bodyweight.Repo.LatestBefore(ctx, ts) (*Entry, error)` if not already present — needed for the last-before-window fallback. Skip if a List + filter is cleaner; document either choice in the task note.
- [x] 2.4 Composition tests: table-driven across all four resolution tiers + the no-data error + invalid-input errors. Boundary case: explicit lean mass with zero stored weight entries → succeeds; `BodyWeightKg` is nil in response.

## 3. Day-bucket aggregation

- [x] 3.1 `internal/energy/aggregate.go`: `bucketByDate(meals []*meals.MealEntry, workouts []*workouts.Workout, loc *time.Location) map[civil.Date]dayBucket` — meals matched by `LoggedAt.In(loc).Date()`, workouts by `StartedAt.In(loc).Date()`. Use `time.Time` plus formatted strings rather than a separate `civil.Date` dep — Go std-lib only.
- [x] 3.2 Enumerate all calendar days in `[from, to)` in `loc` and emit one `Day` per day even if the bucket is empty — empty days return zeros + `band = low` (Requirement: Empty calendar days appear).
- [x] 3.3 Sum `meals.kcal` per day (existing `summary.SumEntries` accepts `[]*meals.MealEntry` and returns kcal; reuse).
- [x] 3.4 Sum `workouts.kcal_burned` per day across only the workouts that have it set. Workouts with NULL `kcal_burned` go into `MissingBurnWorkoutIDs`.
- [x] 3.5 `CompleteData = len(MissingBurnWorkoutIDs) == 0` (note: a day with zero workouts qualifies as complete — there's nothing missing).

## 4. EA computation + band classification

- [x] 4.1 `internal/energy/ea.go`: `func computeEA(intakeKcal, burnedKcal, ffmKg float64) float64 { return (intakeKcal - burnedKcal) / ffmKg }`. Returns `0.0` if ffm is exactly zero (defensive; the resolver guarantees > 0 in practice).
- [x] 4.2 `func classifyBand(ea float64) string` per Requirement: Loucks band classification — `< 30 → low`, `30..45 → sub_optimal`, `>= 45 → adequate`. Closed-low / open-high at both boundaries.
- [x] 4.3 Window aggregate: `avg_ea = mean(day.ea for day in days if day.complete_data)`; null + nil band when zero complete days. `total_days = len(days)`, `days_with_complete_data = count of complete days`.
- [x] 4.4 Boundary tests: 29.9 → low, 30.0 → sub_optimal, 44.9 → sub_optimal, 45.0 → adequate.

## 5. HTTP handler

- [x] 5.1 `internal/energy/handlers.go`: `Handlers.Register(rg *gin.RouterGroup)` mounting `GET /energy/availability`.
- [x] 5.2 Parse query params: `from` / `to` (RFC 3339), `tz` (load via `time.LoadLocation`; default to `cfg.DefaultUserTZ` injected via the Handlers constructor), `lean_mass_kg` (parse to `*float64` with `strconv.ParseFloat`; omit if absent), `body_fat_pct` (same).
- [x] 5.3 Map service errors to documented codes — `window_required`, `window_invalid`, `range_too_large`, `tz_invalid`, `lean_mass_kg_invalid`, `body_fat_pct_invalid`, `weight_data_missing`.
- [x] 5.4 Round all numeric fields at the response boundary via `numfmt.Round1` (matches the per-entry rounding rule used by hydration / fueling).
- [x] 5.5 Swag annotations listing every error code; documents the response shape (use composition + day + window struct refs).

## 6. Wiring

- [x] 6.1 `internal/httpserver/server.go`: instantiate `energy.Service` with `mealsRepo`, `workoutsRepo`, `bodyWeightRepo`; instantiate `energy.Handlers` with the service + `cfg.DefaultUserTZ`; register on the API group (auth middleware applies).
- [x] 6.2 No idempotency middleware change — the GET handler does not consume `Idempotency-Key`.

## 7. Backend tests

- [x] 7.1 `internal/energy/composition_test.go` — every FFM resolution rule + body-weight resolution rule covered with `storetest.NewPool` fixtures.
- [x] 7.2 `internal/energy/ea_test.go` — `computeEA` numerics + `classifyBand` at exact boundaries (table-driven).
- [x] 7.3 `internal/energy/handlers_test.go` — end-to-end through Gin:
  - Happy path: 7-day window with one meal/day + one workout/day, body-weight + body-fat % in window → expected `days[]` shape, `window.avg_ea` correct, all bands present.
  - Missing-burn flagging: insert a workout with `kcal_burned = NULL`; assert `missing_burn_workout_ids` non-empty, `complete_data: false`, and `window.avg_ea` excludes the day.
  - Empty days: window covers 7 days with intake only on 3 → 7 day-rows, empty days show zeros + `band: "low"`.
  - TZ boundaries: meal at `22:30Z` + `tz=Europe/Berlin` (UTC+2) attributes to the *next* local day. Workout that spans midnight by `started_at` belongs to the start day.
  - `tz=Europe/Berlin` round-trips in the response.
  - Error codes: each documented `400` is reachable (`window_required`, `window_invalid`, `range_too_large`, `tz_invalid`, `lean_mass_kg_invalid`, `body_fat_pct_invalid`, `weight_data_missing`).
  - FFM resolution path is exposed correctly in `composition.source` for all four tiers.

## 8. MCP wrapper

- [x] 8.1 `internal/mcpserver/tools_energy.go`: one input struct `WeeklyEnergySummaryArgs{From, To string; TZ string; LeanMassKg *float64; BodyFatPct *float64}`.
- [x] 8.2 `handleWeeklyEnergySummary` builds the `url.Values` query forwarding only the supplied params (omit `tz` / `lean_mass_kg` / `body_fat_pct` when unset). No `Idempotency-Key`. Forwards the response body via `toToolResult`.
- [x] 8.3 `registerEnergyTools(server, c)` — single `mcp.AddTool` call.
- [x] 8.4 Wire `registerEnergyTools` in `internal/mcpserver/server.go`.
- [x] 8.5 Tool description names all three Loucks bands with their thresholds, the FFM resolution order, and the `missing_burn_workout_ids` semantic (per Requirement: Tool description explains).

## 9. MCP tests

- [x] 9.1 `internal/mcpserver/tools_energy_test.go` using the recorder pattern. Cover:
  - GET URL + method + endpoint path.
  - `from`/`to` always included; `tz` / `lean_mass_kg` / `body_fat_pct` only when set.
  - No `Idempotency-Key` header.
  - REST `200` body passes through verbatim.
  - REST `400 weight_data_missing` is forwarded as `isError: true`.
- [x] 9.2 `internal/mcpserver/mcp_integration_test.go` expected-tools list: add `weekly_energy_summary`. Tool count grows by 1 (was 35 after add-workout-fuel, now 36).

## 10. Documentation

- [x] 10.1 `task swag` regenerates OpenAPI with the new endpoint + response shape.
- [x] 10.2 `README.md`:
  - New "Energy availability" subsection placed after "Body weight" (both derive from composition data). Example call with `lean_mass_kg`, example call falling back to stored body-fat %, example call hitting the `weight_data_missing` 400 to show the documented failure mode.
  - Add `weekly_energy_summary` to the MCP tools table.
  - Project layout: add `internal/energy/`.
- [x] 10.3 `RUN_LOCAL.md` walkthrough extension: with `task dev` running, log a weight (with body-fat %), log 3 days of meals + 3 workouts with kcal_burned, then `GET /energy/availability?from=...&to=...&tz=Europe/Berlin` and observe the bands + composition source. Bonus: a second call passing explicit `lean_mass_kg` to show the override path.

## 11. Pre-merge checks

- [x] 11.1 `task vet` clean.
- [x] 11.2 `task test` green per-package — `internal/energy/`, `internal/mcpserver/`, `internal/bodyweight/`, `internal/httpserver/` all pass. Full-module `go test -p 1 ./...` flaked on testcontainers pool ping in `hydration` and `products` (same flake observed under add-workout-fuel); both pass when re-run in isolation.
- [x] 11.3 Manual e2e:
  - POST a body-weight entry with `body_fat_pct: 16`.
  - POST a workout with `kcal_burned: 600`.
  - POST a few meals summing to 2400 kcal.
  - `GET /energy/availability?from=<today>&to=<tomorrow>&tz=Europe/Berlin` → assert `days[0].ea ≈ (2400-600) / (weight × 0.84)`, `band` matches, `composition.source = "stored_body_fat"`.
  - Re-call with `lean_mass_kg=58` → `composition.source = "explicit_lean_mass"`, `ffm_kg = 58`, EA recomputed.
  - POST a second workout with `kcal_burned: null` and re-call → `missing_burn_workout_ids` non-empty, `complete_data: false`.
- [x] 11.4 OpenSpec validation: `openspec status --change "add-energy-availability"` shows 4/4 artifacts done.
