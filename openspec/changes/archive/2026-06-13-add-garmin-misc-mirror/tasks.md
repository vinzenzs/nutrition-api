## 1. Migration

- [x] 1.1 Verify the migration head on disk before `task migrate:new`. The arc reserves `036`–`040` (B=036, A=037, C=038, D=039, F=040; E and the backfill take no slot), so this change is expected to be `041`. Confirm the highest existing number, then `task migrate:new NAME=add_garmin_misc_mirror`
- [x] 1.2 `041_add_garmin_misc_mirror.up.sql`: `CREATE TABLE devices` (`id` UUID PK, `external_id` TEXT NOT NULL, `display_name` TEXT NOT NULL, `model` TEXT NULL, `last_sync_at` TIMESTAMPTZ NULL, `battery_pct` NUMERIC(5,1) NULL with 0–100 CHECK, `firmware_version` TEXT NULL, audit timestamps; UNIQUE index on `(external_id)`)
- [x] 1.3 Same up-migration: `CREATE TABLE health_vitals` (`date` DATE PRIMARY KEY, nullable vital columns `bp_systolic`/`bp_diastolic`/`bp_pulse`/`resting_hr`/`min_hr`/`max_hr` with `> 0` CHECKs and `stress_avg`/`stress_max` with 0–100 CHECKs, audit timestamps)
- [x] 1.4 Same up-migration: `CREATE TABLE achievements` (`id` UUID PK, `external_id` TEXT NOT NULL, `kind` TEXT NOT NULL with `kind IN ('badge','challenge')` CHECK, `name` TEXT NOT NULL, `earned_at` TIMESTAMPTZ NULL, `progress_pct` NUMERIC(5,1) NULL with 0–100 CHECK, audit timestamps; UNIQUE index on `(external_id)`)
- [x] 1.5 `.down.sql`: drop `achievements`, `health_vitals`, then `devices`

## 2. devices capability (internal/devices)

- [x] 2.1 `types.go`: `Device` struct mirroring the row, JSON tags with omitempty for nullables
- [x] 2.2 `repo.go`: upsert by `external_id` (`INSERT … ON CONFLICT (external_id) DO UPDATE`), list ordered by `display_name`, single-get by id; against `store.Querier`
- [x] 2.3 `service.go`: validate `external_id`/`display_name` required, `battery_pct` 0–100; sentinel errors → API codes (`external_id_required`, `display_name_required`, `battery_pct_invalid`, `device_not_found`)
- [x] 2.4 `handlers.go`: `POST /devices` (201 insert / 200 update), `GET /devices`, `GET /devices/{id}`; `numfmt.Round1` on `battery_pct` at the boundary; `Register(rg)` + swag annotations

## 3. health-vitals capability (internal/healthvitals)

- [x] 3.1 `types.go`: `Snapshot` struct keyed by `date`, nullable vital fields with omitempty
- [x] 3.2 `repo.go`: upsert by `date` (full-replace of metric columns), list in a date window (≤ 92 days), single-get by date, against `store.Querier`
- [x] 3.3 `service.go`: validate `date` (`YYYY-MM-DD`), per-metric ranges; sentinel errors → API codes (`date_invalid`, `bp_systolic_invalid`, …, `stress_avg_invalid`, `range_too_large`, `window_required`, `health_vitals_not_found`)
- [x] 3.4 `handlers.go`: `POST /health-vitals`, `GET /health-vitals`, `GET /health-vitals/{date}`; swag annotations

## 4. achievements capability (internal/achievements)

- [x] 4.1 `types.go`: `Achievement` struct mirroring the row, omitempty nullables
- [x] 4.2 `repo.go`: upsert by `external_id`, list ordered by `earned_at` DESC (NULLs last) with optional `kind` filter, against `store.Querier`
- [x] 4.3 `service.go`: validate `external_id`/`name` required, `kind` in `{badge,challenge}`, `progress_pct` 0–100; sentinel errors → API codes (`external_id_required`, `kind_invalid`, `name_required`, `progress_pct_invalid`)
- [x] 4.4 `handlers.go`: `POST /achievements`, `GET /achievements`; `numfmt.Round1` on `progress_pct`; swag annotations

## 5. garmin-control activity operations (internal/garmincontrol)

- [x] 5.1 Add bridge-client functions: `bridgeGetActivityGear`, `bridgeDownloadWorkout`, `bridgeUploadActivity`, `bridgeRenameActivity`, `bridgeDeleteActivity` (each one bridge call; delete treats already-absent as no-op success)
- [x] 5.2 Handlers: `GET /garmin/activity/{activity_id}/gear`, `GET /garmin/workout/{garmin_workout_id}/download` (optional `format`, base64 envelope verbatim), `POST /garmin/activity/upload`, `PATCH /garmin/activity/{activity_id}` (require `name`), `DELETE /garmin/activity/{activity_id}`; each returns `503 garmin_disabled` when the bridge URL is unset; swag annotations
- [x] 5.3 Confirm authentication is enforced on all five (same middleware as the existing control endpoints)

## 6. Wiring & MCP

- [x] 6.1 `internal/httpserver/server.go`: instantiate + register the three new capability repos/services/handlers and the new control routes
- [x] 6.2 `internal/mcpserver`: three read tools (`devices_list`, `health_vitals_list`, `achievements_list`) mirroring the list endpoints 1:1 (no `Idempotency-Key`)
- [x] 6.3 `internal/mcpserver/tools_garmin.go`: five activity control tools (`garmin_get_activity_gear`, `garmin_download_workout`, `garmin_upload_activity`, `garmin_rename_activity`, `garmin_delete_activity`); write tools auto-derive an idempotency key via `effectiveIdempotencyKey`; reads send none
- [x] 6.4 Bump the `mcp_integration_test` expected-tools list by **eight** (three read + five control-plane) and assert each new tool is present

## 7. garmin-bridge (apps/garmin-bridge)

- [x] 7.1 `garmin_client.py`: add guarded `safe()` fetches into `fetch_day` — `get_devices`/`get_device_last_used`, `get_blood_pressure`/`get_heart_rates`/`get_all_day_stress`, `get_earned_badges`/`get_adhoc_challenges`
- [x] 7.2 `garmin_client.py`: add the five on-demand activity ops (`get_activity_gear`, `download_workout` → base64 envelope, `upload_activity`, `set_activity_name`, `delete_activity` with idempotent 404-as-no-op), alongside the workout-library ops
- [x] 7.3 `mapping.py`: add `map_devices` (list of upsert bodies), `map_health_vitals` (one date-keyed snapshot, dropped when empty), `map_achievements` (list, `external_id` namespaced by `kind`); each defensive (absent → omitted); wire into `map_day`
- [x] 7.4 `sync.py`: add `/health-vitals` to the date-keyed snapshot routes; add per-item upsert loops for `/devices` and `/achievements` (modelled on the `/weight` loop); each tolerant of per-item failure
- [x] 7.5 `app.py`: expose the five activity control bridge endpoints the backend control handlers call

## 8. Tests & docs

- [x] 8.1 Expand `apps/garmin-bridge/tests/fixtures/garmin_day.json` with device, blood-pressure/all-day-HR/stress, and badge/challenge sections
- [x] 8.2 `test_mapping.py`: assert `map_devices`/`map_health_vitals`/`map_achievements` mapping, including absent-field omission and the `kind`-namespaced `external_id`
- [x] 8.3 `internal/devices`, `internal/healthvitals`, `internal/achievements` integration tests against testcontainers Postgres: upsert insert/update, list ordering/filter, single-get 404, validation error codes, unit-isolation assertions (no nutriment fields)
- [x] 8.4 `internal/garmincontrol` handler tests: each of the five activity endpoints (verbatim forward, `503 garmin_disabled` when bridge off, delete already-absent → success, `PATCH` missing-name → 400)
- [x] 8.5 `task swag`, `task vet`, `task test` (or scoped `go test -count=1 ./internal/devices/... ./internal/healthvitals/... ./internal/achievements/... ./internal/garmincontrol/...` + bridge `pytest`) all green
