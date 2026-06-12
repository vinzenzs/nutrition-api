## 1. Migration

- [x] 1.1 Verify migration head on disk (`ls internal/store/migrations | tail`) before `task migrate:new NAME=add_gear_and_personal_records` — arc order fixes this at `039`, but confirm the highest number on disk first (out-of-band work has occasionally taken a slot)
- [x] 1.2 `039_add_gear_and_personal_records.up.sql`: `CREATE TABLE gear` (`external_id` UNIQUE, `gear_type` CHECK in `('shoes','bike','other')`, `display_name` NOT NULL, `total_distance_m`/`total_activities` nullable non-negative, `retired` BOOLEAN DEFAULT false, `date_begin`/`date_end` DATE NULL, audit timestamps)
- [x] 1.3 Same up-migration: `CREATE TABLE personal_records` (`external_id` UNIQUE, `pr_type` NOT NULL, `value` NUMERIC NOT NULL CHECK `>= 0`, `unit` NOT NULL, `activity_id` TEXT NULL, `achieved_at` TIMESTAMPTZ NOT NULL, audit timestamps)
- [x] 1.4 `.down.sql`: drop `personal_records`, then `gear`

## 2. gear capability (internal/gear)

- [x] 2.1 `types.go`: `Gear` struct mirroring the row with omitempty on nullable fields (`total_distance_m`, `total_activities`, `date_begin`, `date_end`)
- [x] 2.2 `repo.go`: upsert against `store.Querier` (`INSERT … ON CONFLICT (external_id) DO UPDATE`), list (with optional `retired` filter, ordered by `display_name`), get-by-id
- [x] 2.3 `service.go`: validate `gear_type` enum, required `external_id`/`display_name`, non-negative distance/activities; sentinel errors → API codes (`gear_type_invalid`, `external_id_required`, `display_name_required`, `total_distance_m_invalid`, `total_activities_invalid`, `gear_not_found`)
- [x] 2.4 `handlers.go`: `POST /gear` (upsert, 201 on insert / 200 on update, `Idempotency-Key`), `GET /gear` (list + `?retired=`), `GET /gear/{id}`; `numfmt.Round1` on `total_distance_m` at the boundary; `Register(rg *gin.RouterGroup)` + swag annotations
- [x] 2.5 `handlers_test.go`: per-handler integration tests (upsert insert+update no-dup, enum/required validation, list + retired filter, round-trip, 404)

## 3. personal-records capability (internal/personalrecords)

- [x] 3.1 `types.go`: `PersonalRecord` struct with omitempty on `activity_id`
- [x] 3.2 `repo.go`: upsert by `external_id`, list (optional `pr_type` filter, ordered by `achieved_at` DESC) against `store.Querier`
- [x] 3.3 `service.go`: validate required `external_id`/`pr_type`/`value`/`unit`/`achieved_at`, non-negative `value`; sentinel errors → API codes (`external_id_required`, `pr_type_required`, `value_invalid`, `unit_required`, `achieved_at_required`)
- [x] 3.4 `handlers.go`: `POST /personal-records` (upsert, `Idempotency-Key`), `GET /personal-records` (list + `?pr_type=`); `numfmt.Round1` on `value`; `Register(...)` + swag annotations
- [x] 3.5 `handlers_test.go`: per-handler integration tests (upsert insert+update no-dup, validation, list + pr_type filter, activity_id omitempty)

## 4. Wiring & MCP

- [x] 4.1 `internal/httpserver/server.go`: instantiate both repos/services and register `gear` + `personal-records` routes in `Run()` (middleware stack unchanged)
- [x] 4.2 `internal/mcpserver/server.go`: add `registerGearTools` (`gear_list` → `GET /gear`) and `registerPersonalRecordsTools` (`personal_records_list` → `GET /personal-records`), each forwarding the body verbatim via `toToolResult`
- [x] 4.3 Bump the `mcp_integration_test` expected-tools list by **two** (`gear_list`, `personal_records_list`)

## 5. garmin-bridge (apps/garmin-bridge)

- [x] 5.1 `garmin_client.py`: add guarded `get_gear`, `get_gear_stats`, `get_personal_records` fetches (via the existing `safe()` pattern) into the daily fetch
- [x] 5.2 `mapping.py`: map gear (join `get_gear` with `get_gear_stats` by gear id → `/gear` items) and personal records (→ `/personal-records` items); defensive extraction (absent → omitted), `gear_type` fallback to `other`
- [x] 5.3 `sync.py`: upsert each gear / PR item via `POST /gear` / `POST /personal-records`, tolerant of per-capability failure (mirror the existing weight loop's partial-failure summary)

## 6. Tests & docs

- [x] 6.1 Expand the bridge fixture with gear (shoes + bike, with stats) and personal records
- [x] 6.2 `test_mapping.py`: assert gear join + PR mapping, including absent-stats omission and `gear_type` fallback
- [x] 6.3 `task swag`, `task vet`, `task test` (or scoped `go test -count=1 ./internal/gear/... ./internal/personalrecords/...` + bridge `pytest`) all green
