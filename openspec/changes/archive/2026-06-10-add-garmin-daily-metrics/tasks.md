# add-garmin-daily-metrics — tasks

> Slices 1–2 (the two new capabilities) are independent and can land before the
> modifications in slices 3–5. Verify the next free migration slots before
> committing — head is `019`, so `020`–`023` are expected but confirm.

## 1. recovery-metrics capability

- [x] 1.1 Migration `020_add_recovery_metrics`: CREATE TABLE `recovery_metrics` (`date` DATE PRIMARY KEY; nullable `sleep_seconds`, `sleep_score`, `hrv_ms`, `resting_hr`, `stress_avg`, `body_battery_charged`, `body_battery_drained`, `training_readiness` with the documented CHECKs; `created_at`/`updated_at`). Down: DROP TABLE.
- [x] 1.2 `internal/recoverymetrics/types.go`: `Snapshot` struct (Date as `civil`/string `YYYY-MM-DD`, all metrics `*int`/`*float64` with omitempty) mirroring the bodyweight package shape
- [x] 1.3 `repo.go`: `Upsert` (`INSERT … ON CONFLICT (date) DO UPDATE` full-replace), `GetByDate`, `List(from, to)` ordered by date asc, `DeleteByDate`; `selectCols` + `scanSnapshot`
- [x] 1.4 `service.go`: sentinel errors (`ErrDateInvalid`, `ErrSleepScoreInvalid`, `ErrStressAvgInvalid`, `ErrRestingHRInvalid`, `ErrTrainingReadinessInvalid`, `ErrBodyBattery*Invalid`, `ErrSleepSecondsInvalid`, `ErrHRVInvalid`); range/positivity validation; `Upsert`, `Get`, `ListWindow`, `Delete`
- [x] 1.5 `handlers.go`: `POST /recovery-metrics` (upsert, 201 insert / 200 update), `GET /recovery-metrics` (window + 92-day cap + `window_required`/`range_too_large`), `GET /recovery-metrics/{date}` (404 `recovery_metrics_not_found`), `DELETE /recovery-metrics/{date}`; `Register(rg)`; swag annotations
- [x] 1.6 Wire into `internal/httpserver/server.go` `Run()` (repo→service→handlers→Register, behind auth+idempotency)
- [x] 1.7 Tests: upsert insert+update round-trip, omitted-field NULL on re-POST (full-replace), each range-validation error, window list + caps, get/delete 404 paths, unit-isolation (no kcal/vo2max keys)

## 2. fitness-metrics capability

- [x] 2.1 Migration `021_add_fitness_metrics`: CREATE TABLE `fitness_metrics` (`date` DATE PRIMARY KEY; nullable `vo2max_running`, `vo2max_cycling`, `race_predictor_{5k,10k,half,full}_seconds`, `acute_load`, `chronic_load` with CHECKs; audit cols). Down: DROP TABLE.
- [x] 2.2 `internal/fitnessmetrics/types.go`: `Snapshot` struct (race predictors as `*int` seconds; VO2max/load as `*float64`; omitempty)
- [x] 2.3 `repo.go`: `Upsert` (ON CONFLICT date), `GetByDate`, `List`, `DeleteByDate`
- [x] 2.4 `service.go`: sentinel errors + positivity validation; `Upsert`/`Get`/`ListWindow`/`Delete`
- [x] 2.5 `handlers.go`: `POST`/`GET` list/`GET {date}`/`DELETE {date}` mirroring recovery-metrics; error code `fitness_metrics_not_found`; swag
- [x] 2.6 Wire into `server.go` `Run()`
- [x] 2.7 Tests: upsert round-trip, full-replace NULL, validation errors, window caps, get/delete 404, unit-isolation, and the "no `acwr` column / field" assertion

## 3. body-weight biometrics

- [x] 3.1 Migration `022_add_weight_biometrics`: ALTER TABLE `body_weight_entries` ADD `muscle_mass_kg`, `body_water_pct`, `bone_mass_kg`, `bmi` (nullable, with CHECKs). Down: DROP the four columns. No back-fill.
- [x] 3.2 `internal/bodyweight/types.go`: add the four `*float64` fields with omitempty to `Entry`
- [x] 3.3 `repo.go`: extend insert/select/scan + the PATCH SET-builder for the four fields
- [x] 3.4 `service.go`: sentinel errors (`ErrMuscleMassInvalid`, `ErrBodyWaterPctInvalid`, `ErrBoneMassInvalid`, `ErrBMIInvalid`); validation (positivity, body_water_pct 0–100); extend Create + Patch inputs
- [x] 3.5 `handlers.go`: POST + PATCH accept the four fields; map validation to the matching 400 codes; swag updated
- [x] 3.6 Tests: POST stores+echoes biometrics, omitted→NULL/omitempty, each invalid-value 400, PATCH a biometric field, existing-row migration reads NULL

## 4. workouts planned/completed status

- [x] 4.1 Migration `023_add_workout_status`: ALTER TABLE `workouts` ADD `status TEXT NOT NULL DEFAULT 'completed' CHECK (status IN ('planned','completed'))`; existing rows take the default; add `workouts_status_idx` on `(status)`. Down: drop index + column.
- [x] 4.2 `types.go`: add `Status` field (string, always present — no omitempty); a `ValidStatus`/`ParseStatus` helper + `StatusPlanned`/`StatusCompleted` consts
- [x] 4.3 `repo.go`: include `status` in selectCols/INSERT/UPSERT/scan; add `status` to the `List` filter predicate (optional `*string`)
- [x] 4.4 `service.go`: `ErrStatusInvalid`; validate status in Create/Patch; **condition the future-date guard on status** (completed: 24h; planned: +1y; planned past-date allowed); add status to CreateInput/PatchInput; default to `completed` when omitted
- [x] 4.5 `handlers.go`: POST/PATCH accept `status` (`status_invalid` on bad value); `GET /workouts?status=` filter (window still required); add `status` to the PATCH mutable allowlist; swag `@Failure`/`@Param` updated
- [x] 4.6 Confirm planned rows don't distort reads: add a test asserting a planned future workout is excluded from energy-availability burn and from `/workouts/{id}/fueling`; if it leaks, filter `status='completed'` in those read paths (resolves design Open Question)
- [x] 4.7 Tests: planned future POST accepted, completed future POST rejected, planned >1y rejected, bad status 400, list `?status=` filter, PATCH promote planned→completed, existing rows back-filled to completed

## 5. daily-context + MCP + docs

- [x] 5.1 `internal/dailycontext/service.go`: add `recovery` + `fitness` blocks (same-day-or-null, via the new repos' `GetByDate`) to the bundle; echo the new biometric fields on the `weight` block; extend the parallel fan-out
- [x] 5.2 daily-context tests: bundle includes recovery+fitness when present, null (no carryover) when absent, weight block echoes biometrics
- [x] 5.3 `internal/mcpserver/tools_recoverymetrics.go`: `log_/list_/get_/delete_recovery_metrics` (mirror the bodyweight tool file); register in `server.go`
- [x] 5.4 `internal/mcpserver/tools_fitnessmetrics.go`: `log_/list_/get_/delete_fitness_metrics`; register
- [x] 5.5 Extend `tools_weight.go` (`log_weight`/`patch_weight` biometric args) and `tools_workouts.go` (`log_workout`/`patch_workout` `status`, `list_workouts` `status` filter) + descriptions
- [x] 5.6 Bump the expected-tools list in `mcp_integration_test.go` (+8 tools); MCP wrapper tests for the new tools + weight/workout additions (forwarded-when-set, omitted-when-absent, date-addressed get/delete)
- [x] 5.7 `task swag` — regenerate `docs/`
- [x] 5.8 README: sections for recovery-metrics + fitness-metrics (push-by-date examples), the weight biometrics, and planned-workout `status`; RUN_LOCAL: a daily Garmin push example hitting the new endpoints
- [x] 5.9 `task vet` + full `task test` green
- [x] 5.10 Out-of-repo coordination note (no code here): `garmin.py` push path maps its `cmd_coach` reads (sleep/HRV/RHR/stress/body-battery/readiness; VO2max/race-predictions/load; full weigh-in biometrics; calendar planned sessions) onto the new endpoints, plus a planned→completed reconciliation rule — tracked outside this repo
