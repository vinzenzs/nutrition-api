## 1. Migration

- [ ] 1.1 Confirm migration head is `035` on disk, then `task migrate:new NAME=add_workout_detail` (expect `036`)
- [ ] 1.2 `036_add_workout_detail.up.sql`: `ALTER TABLE workouts ADD COLUMN` the 9 scalar fields (`elevation_gain_m`, `elevation_loss_m`, `normalized_power_w`, `intensity_factor`, `avg_cadence`, `avg_stride_m`, `max_hr`, `aerobic_te`, `anaerobic_te`) + `secs_in_zone_1..5` + the 2 weather fields (`humidity_pct`, `wind_speed_mps`), all nullable with CHECKs mirroring existing column conventions
- [ ] 1.3 Same up-migration: `CREATE TABLE workout_splits` (`workout_id` FK `ON DELETE CASCADE`, `split_index`, metrics; index on `workout_id`; UNIQUE `(workout_id, split_index)`)
- [ ] 1.4 Same up-migration: `CREATE TABLE workout_sets` (`workout_id` FK `ON DELETE CASCADE`, `set_index`, exercise fields; index on `workout_id`; UNIQUE `(workout_id, set_index)`)
- [ ] 1.5 `.down.sql`: drop `workout_sets`, `workout_splits`, then drop the added columns

## 2. Types & repo (internal/workouts)

- [ ] 2.1 `types.go`: add the scalar + zone pointer fields to `Workout` (omitempty), and `Split`/`Set` structs + `Splits []Split` / `Sets []Set` on the single-get shape
- [ ] 2.2 `repo.go`: extend the INSERT/UPSERT to write the new scalar+zone columns
- [ ] 2.3 `repo.go`: child writes against `store.Querier` (works on pool OR `pgx.Tx`) — insert splits/sets, and a replace helper (`DELETE WHERE workout_id` + reinsert)
- [ ] 2.4 `repo.go`: wrap single + bulk upsert-with-children in a transaction so parent + children commit atomically per item
- [ ] 2.5 `repo.go`: single-get reads the child rows ordered by index; list query selects scalar+zone columns but NOT children

## 3. Service & handlers (internal/workouts)

- [ ] 3.1 `service.go`: validate nested split/set inputs (indices, non-negative metrics); reuse existing sentinel-error → API-code mapping
- [ ] 3.2 `handlers.go`: accept nested `splits`/`sets` on POST and bulk items; map `""`/absent per existing conventions; apply `numfmt.Round1` to every new float at the response boundary
- [ ] 3.3 `handlers.go`: single-get returns scalar+zone inline and nested arrays (empty omitted); list returns scalar+zone, no nested arrays
- [ ] 3.4 Update swag annotations on the affected request/response structs

## 4. Wiring & MCP

- [ ] 4.1 Verify `internal/httpserver/server.go` wiring needs no change (same package); adjust only if repo construction signature changed
- [ ] 4.2 Confirm the MCP get-workout tool forwards the enriched body verbatim (no new tool); review `mcp_integration_test` expected-tools list (expect unchanged)

## 5. garmin-bridge (apps/garmin-bridge)

- [ ] 5.1 `garmin_client.py`: add guarded per-activity fetches (`get_activity_hr_in_timezones`, `get_activity_splits`, `get_activity_exercise_sets`, `get_activity_weather`) wired into `fetch_day`, each via the existing `safe()` pattern
- [ ] 5.2 `mapping.py`: extend `map_workouts` to emit the scalar + weather fields from the activity summary / `get_activity_weather` and the zone/split/set detail as nested arrays; defensive extraction (absent → omitted, e.g. indoor activities have no weather)
- [ ] 5.3 `sync.py`: confirm the bulk post carries the nested detail unchanged (no per-activity round-trips)

## 6. Tests & docs

- [ ] 6.1 Expand `apps/garmin-bridge/tests/fixtures/garmin_day.json` with a run (splits + zones + scalars + weather) and a strength activity (sets); include an indoor activity with no weather
- [ ] 6.2 `test_mapping.py`: assert the new scalar/zone/weather/split/set mapping, including absent-field omission
- [ ] 6.3 `internal/workouts` integration tests: nested write on POST + bulk, replace-on-resync (no duplicate children), single-get returns detail, list omits nested arrays, child-write failure fails only its item
- [ ] 6.4 Reconcile-seam test: a Garmin import with nested detail that reconciles into a planned workout attaches its splits/sets/scalars to the surviving reconciled row (not a duplicate), and a re-sync replaces those children in place — exercise against the existing reconciliation path in `internal/workouts/reconcile*`
- [ ] 6.5 `task swag`, `task vet`, `task test` (or scoped `go test -count=1 ./internal/workouts/...` + bridge `pytest`) all green
