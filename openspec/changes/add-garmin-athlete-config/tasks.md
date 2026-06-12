## 1. Migration

- [ ] 1.1 Verify migration head on disk before `task migrate:new` (arc order is B=036, A=037, C=038, D=039; the on-disk head may lag if siblings have not yet landed — out-of-band slots have collided before), then `task migrate:new NAME=add_athlete_config` (expect `040`)
- [ ] 1.2 `040_add_athlete_config.up.sql`: `CREATE TABLE athlete_config` with a fixed sentinel `id` primary key (singleton), all physiology columns nullable — `ftp_watts`, `threshold_hr`, `lactate_threshold_hr`, `max_hr` (INTEGER), `threshold_pace_sec_per_km`, `threshold_swim_pace_sec_per_100m` (NUMERIC), `hr_zone_1_max..hr_zone_5_max`, `power_zone_1_max..power_zone_5_max` (INTEGER) — with CHECKs mirroring existing conventions (`IS NULL OR > 0` where sensible), plus `created_at` / `updated_at`
- [ ] 1.3 `.down.sql`: drop `athlete_config`

## 2. Types & repo (internal/athleteconfig)

- [ ] 2.1 `types.go`: `AthleteConfig` struct with nullable pointer fields + JSON tags (omitempty), `created_at` / `updated_at`
- [ ] 2.2 `repo.go`: singleton `Get(ctx) (*AthleteConfig, error)` returning `(nil, nil)` when no row exists (distinct from DB error), against `store.Querier`
- [ ] 2.3 `repo.go`: `Upsert(ctx, *AthleteConfig)` — `INSERT … ON CONFLICT (id) DO UPDATE` on the fixed sentinel id, full-replace (absent fields → NULL), `updated_at = now()` (model on `internal/goals/repo.go`)

## 3. Service & handlers (internal/athleteconfig)

- [ ] 3.1 `service.go`: validate each field (negative / NaN / non-numeric → `athlete_config_value_invalid` with `field`); sentinel-error → API-code mapping like `goals`
- [ ] 3.2 `handlers.go`: `Register(rg)` wiring `GET /athlete-config` (returns `{"athlete_config": <config> | null}`) and `PUT /athlete-config` (full-replace, read-back the stored row); reject `Idempotency-Key` on PUT with `400 idempotency_unsupported_for_put`; apply `numfmt.Round1` to every float at the response boundary; swag annotations
- [ ] 3.3 Confirm config never leaks into `summary` Totals (unit isolation)

## 4. Wiring & MCP

- [ ] 4.1 `internal/httpserver/server.go`: instantiate the athlete-config repo/service and register routes in `Run()` (mirror the goals wiring)
- [ ] 4.2 `internal/mcpserver/`: add `athlete_config_get` read tool (`GET /athlete-config`, verbatim body forward via `toToolResult`, no `Idempotency-Key`) in a new tool group; register it
- [ ] 4.3 Bump the `mcp_integration_test` expected-tools list by exactly one (`athlete_config_get`)

## 5. garmin-bridge (apps/garmin-bridge)

- [ ] 5.1 `garmin_client.py`: add guarded fetches `get_user_profile` (and/or `get_userprofile_settings`) and `get_heart_rate_zones` wired into `fetch_day`, each via the existing `safe()` pattern
- [ ] 5.2 `mapping.py`: add `map_athlete_config` extracting FTP/thresholds/max-HR/lactate-threshold-HR/HR-zone (+ optional power-zone) boundaries defensively (absent → omitted); add it to `map_day`
- [ ] 5.3 `sync.py`: issue `PUT /athlete-config` with the mapped config on each daily sync (non-date-keyed singleton refresh, in-place overwrite); skip the PUT when the mapper returns nothing

## 6. Tests & docs

- [ ] 6.1 Expand the bridge test fixture with a user profile (FTP, thresholds) and heart-rate-zone data
- [ ] 6.2 `test_mapping.py`: assert `map_athlete_config` mapping including absent-field omission and the power-zones-absent case
- [ ] 6.3 `internal/athleteconfig` integration tests: GET-before-write returns null; first PUT creates; subsequent PUT full-replaces (absent fields cleared); partial config; HR-zone round-trip; power-zones omitted when absent; negative value → `athlete_config_value_invalid`; `Idempotency-Key` on PUT → `idempotency_unsupported_for_put`; `numfmt.Round1` on a float field
- [ ] 6.4 `task swag`, `task vet`, `task test` (or scoped `go test -count=1 ./internal/athleteconfig/...` + bridge `pytest`) all green
