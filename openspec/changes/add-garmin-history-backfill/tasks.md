## 1. garmin-bridge: backfill route + range loop + pacing

- [ ] 1.1 `config.py`: add `backfill_day_delay_seconds` (default a few seconds, e.g. `3`) and `backfill_max_days` (default ~120), read from env `BACKFILL_DAY_DELAY_SECONDS` / `BACKFILL_MAX_DAYS`
- [ ] 1.2 `app.py`: add `BackfillRequest {from, to}` model and `POST /sync/backfill` handler — parse/validate the inclusive date range, reject spans > `backfill_max_days` with `400 range_too_large` (including the cap), return `409 login_required` when no token (mirroring `/sync`)
- [ ] 1.3 `sync.py` (or `app.py`): iterate dates oldest→newest, calling the existing token-load + `gc.fetch_day` + `sync.sync_day` per day; wrap each day so a failure is recorded `{date, ok:false, error}` and the loop continues
- [ ] 1.4 Insert `time.sleep(backfill_day_delay_seconds)` between consecutive days (skip when `0`); confirm the plain-`def` handler runs in the threadpool so the sleep does not stall the loop
- [ ] 1.5 Build the response: `days[]` (each day's `sync_day` summary or its error) + roll-up `days_total`/`days_ok`/`days_failed`; status `200` if all ok else `207`

## 2. garmin-control: proxy endpoint

- [ ] 2.1 `handlers.go` / new proxy handler: add `POST /garmin/backfill` reading the `{from,to}` body and forwarding to the bridge `POST /sync/backfill` verbatim (status + body), `503 garmin_disabled` when disabled, auth required — mirror the `loginMFA`/`calendar` forward pattern
- [ ] 2.2 Use a backfill-appropriate (longer than the 30s interactive) timeout for this proxy path so a paced run is not cut off
- [ ] 2.3 Register the route in `Register(rg)`; add swag annotations (`@Tags garmin`, `@Router /garmin/backfill [post]`, `503 garmin_disabled`)

## 3. MCP: garmin_backfill tool

- [ ] 3.1 `tools_garmin.go`: add `GarminBackfillArgs {From, To, IdempotencyKey}` and `handleGarminBackfill` — marshal `{from,to}`, derive the key via `effectiveIdempotencyKey("garmin_backfill", args)`, one `c.Post("/garmin/backfill", ...)`, return `toToolResult`
- [ ] 3.2 Register the tool in `registerGarminTools` with a description covering bounded/paced/resumable + idempotent re-runs
- [ ] 3.3 `mcp_integration_test.go`: add `"garmin_backfill"` to the expected-tools list (+1)

## 4. Deploy config

- [ ] 4.1 `values.yaml`: surface `BACKFILL_DAY_DELAY_SECONDS` / `BACKFILL_MAX_DAYS` for the bridge with safe defaults (document them); leave the steady-state rolling-window CronJob unchanged

## 5. Tests & docs

- [ ] 5.1 Bridge `pytest`: happy-path multi-day backfill (each day synced via `sync_day`), one-bad-day-continues (`207`), range-cap rejection (`400 range_too_large`, nothing written), idempotent re-run, delay=0 path
- [ ] 5.2 garmin-control test: forward to bridge verbatim, `207` pass-through, `503 garmin_disabled` when bridge URL unset, auth required
- [ ] 5.3 `task swag` after the new garmin-control handler; `task vet`; `task test` (or scoped `go test -count=1 ./internal/garmincontrol/... ./internal/mcpserver/...`) + bridge `pytest` green
- [ ] 5.4 Confirm NO migration was added (head unchanged) — backfill reuses existing upsert/UPSERT/child-replace paths
