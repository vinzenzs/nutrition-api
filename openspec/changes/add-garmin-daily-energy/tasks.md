## 1. Migration

- [ ] 1.1 Verify the migration head on disk before scaffolding (`ls internal/store/migrations | sort | tail`) — the arc expects `036` (B `add-garmin-workout-detail`) to be the prior slot so this is `037`, but an out-of-band slot collision has happened before; confirm `037` is free, then `task migrate:new NAME=add_daily_summary`
- [ ] 1.2 `037_add_daily_summary.up.sql`: `CREATE TABLE daily_summary` with `date DATE PRIMARY KEY`, the eight nullable metric columns (`active_kcal`, `resting_kcal`, `total_kcal`, `steps`, `floors`, `moderate_intensity_minutes`, `vigorous_intensity_minutes` as INTEGER; `distance_m` as `NUMERIC(10, 1)`), each with a `>= 0` CHECK, plus `created_at`/`updated_at` TIMESTAMPTZ defaults — mirroring `recovery_metrics`
- [ ] 1.3 `037_add_daily_summary.down.sql`: `DROP TABLE daily_summary`

## 2. Types & repo (internal/dailysummary)

- [ ] 2.1 `types.go`: `DailySummary`/`Entry` struct mirroring the row with `*` pointer fields + JSON tags `omitempty` for the nullable metrics
- [ ] 2.2 `repo.go`: upsert-by-date (`INSERT … ON CONFLICT (date) DO UPDATE`, full-replace of metric columns), single-get by date, list by `[from, to]` ordered ascending, delete by date — all against `store.Querier`
- [ ] 2.3 `service.go`: validate date and non-negative metrics; sentinel errors mapping 1:1 to API codes (`date_invalid`, `<field>_invalid`, `window_required`, `range_too_large`, `daily_summary_not_found`)

## 3. Handlers (internal/dailysummary)

- [ ] 3.1 `handlers.go`: `POST /daily-summary` (upsert, 201 insert / 200 update), `GET /daily-summary/{date}` (404 when absent), `GET /daily-summary?from=&to=` (92-day cap), `DELETE /daily-summary/{date}` (204 / 404); `Register(rg *gin.RouterGroup)`
- [ ] 3.2 Apply `numfmt.Round1` to `distance_m` at the response boundary; integers returned as-is; omitempty on NULL fields
- [ ] 3.3 swag annotations on the request/response structs

## 4. Wiring & MCP

- [ ] 4.1 `internal/httpserver/server.go`: instantiate the daily-summary repo + service and register routes in `Run()`, alongside the sibling snapshot capabilities
- [ ] 4.2 `internal/mcpserver`: add `registerDailySummaryTools` with a single read tool `daily_summary_get` mirroring `GET /daily-summary/{date}` 1:1 (verbatim body via `toToolResult`); call it from the server's registration list
- [ ] 4.3 Bump the `mcp_integration_test` expected-tools list by exactly one (`daily_summary_get`)

## 5. garmin-bridge (apps/garmin-bridge)

- [ ] 5.1 `garmin_client.py`: add a guarded `get_user_summary(date)` fetch wired into `fetch_day` via the existing `safe()` pattern
- [ ] 5.2 `mapping.py`: add `map_daily_summary(raw)` mapping `activeKilocalories→active_kcal`, `bmrKilocalories→resting_kcal`, `totalKilocalories→total_kcal`, `totalSteps→steps`, `floorsAscended→floors`, `moderateIntensityMinutes→moderate_intensity_minutes`, `vigorousIntensityMinutes→vigorous_intensity_minutes`, `totalDistanceMeters→distance_m`; defensive extraction (absent → omitted)
- [ ] 5.3 `sync.py`: POST the mapped daily-summary body to `/daily-summary` in the date-keyed snapshot flow (the `_SNAPSHOT_ROUTES`-style path), skipping the POST when the mapped body is empty

## 6. Tests & docs

- [ ] 6.1 Expand `apps/garmin-bridge/tests/fixtures/garmin_day.json` with a `get_user_summary` payload
- [ ] 6.2 `test_mapping.py`: assert `map_daily_summary` field mapping, including absent-field omission and empty-payload handling
- [ ] 6.3 `internal/dailysummary` integration tests against testcontainers Postgres: first-POST inserts (201), second-POST same date updates (200) with omitted fields reset to NULL, invalid date and negative metric rejected, window filtering + missing-window + >92-day rejection, single-get 200/404, delete 204/404, `distance_m` rounded at the boundary
- [ ] 6.4 `task swag`, `task vet`, `task test` (or scoped `go test -count=1 ./internal/dailysummary/...` + bridge `pytest`) all green
