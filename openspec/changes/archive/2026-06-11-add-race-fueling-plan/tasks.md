## 1. Migration

- [x] 1.1 `task migrate:new NAME=add_races` (verify the next free number against the current migration head before committing).
- [x] 1.2 Up migration: create `races` (`id UUID PK, name TEXT NOT NULL, race_date DATE NOT NULL, race_type TEXT NULL, location TEXT NULL, notes TEXT NULL, created_at/updated_at TIMESTAMPTZ`).
- [x] 1.3 Up migration: create `race_legs` (`id UUID PK, race_id UUID NOT NULL REFERENCES races(id) ON DELETE CASCADE, ordinal INT NOT NULL, discipline TEXT NOT NULL CHECK (discipline IN ('swim','bike','run','transition','other')), distance_m NUMERIC NULL, expected_duration_min INT NULL CHECK (expected_duration_min IS NULL OR expected_duration_min > 0), intensity TEXT NULL, UNIQUE(race_id, ordinal))`.
- [x] 1.4 Down migration: drop `race_legs` then `races`.

## 2. Package scaffolding (`internal/races/`)

- [x] 2.1 `types.go`: `Race` and `RaceLeg` structs mirroring the rows, JSON tags with `omitempty` on nullables; a `Discipline` type with the five allowed values.
- [x] 2.2 `repo.go`: CRUD against `store.Querier` — `CreateRace` (race + legs in one tx), `GetRace` (with legs ordered by ordinal), `ListRaces`, `UpdateRace`, `ReplaceLegs`, `DeleteRace`. Usable from `*pgxpool.Pool` or `pgx.Tx`.

## 3. Service: validation + CRUD

- [x] 3.1 `service.go`: sentinel errors mapping 1:1 to API codes (`race_name_required`, `race_date_invalid`, `leg_ordinal_duplicate`, `leg_discipline_invalid`, `race_not_found`, …).
- [x] 3.2 Validate on create/update: name non-empty, `race_date` parses, leg `ordinal` unique within the race, every `discipline` in the allowed set, `expected_duration_min > 0` when present.
- [x] 3.3 Service CRUD methods wrapping the repo, returning the sentinels.

## 4. Fuelling math (pure, unit-tested) — `internal/races/fueling.go`

- [x] 4.1 `FuelingParams{BodyWeightKg float64; SweatRateMlPerHr *float64}` + validation (`body_weight_kg_required`, `body_weight_kg_out_of_range` 30–200, `sweat_rate_out_of_range`).
- [x] 4.2 Implement the carb band on total race duration `D` (sum of leg durations): `D<75 → 0`, `75≤D<150 → 60`, `D≥150 → 90` g/hr; discipline intake factors `swim 0.0 / transition 0.0 / bike 1.0 / run 0.7 / other 0.8`; `carbs_g_per_hr = round(base × factor)`, `carbs_g_total = round(per_hr × duration_hr)`.
- [x] 4.3 Fluid: `min(sweat_rate, 1000)` when supplied else `600`; sodium: `round(sweat_rate/1000 × 800)` when supplied else `600`; both zeroed for swim/transition; `_total = round(per_hr × duration_hr)`.
- [x] 4.4 Per-leg `rationale` string; flag defaulted sweat rate and "duration unknown" legs (which contribute 0 to all totals).
- [x] 4.5 Race-level totals = element-wise sums of per-leg `_total` fields + `total_duration_min = D`. Keep `carbs_g_*` / `sodium_mg_*` / `fluid_ml_*` as distinct fields (unit isolation); `numfmt.Round1` at the boundary for any fractional values.
- [x] 4.6 Pure-function tests: each carb band boundary (74/75/149/150 min), every discipline factor, swim/transition zero, sweat-rate supplied vs defaulted (assert exact `1000`/`800` and `600`/`600`), unknown-duration leg, race totals = sum of legs.

## 5. Handlers + routes + swag (`handlers.go`)

- [x] 5.1 `POST /races` (create with nested legs), `GET /races`, `GET /races/{id}`, `PATCH /races/{id}` (race fields + `ReplaceLegs`), `DELETE /races/{id}`.
- [x] 5.2 `GET /races/{id}/fueling-plan` — parse `body_weight_kg` (required) + `sweat_rate_ml_per_hr` (optional), 404 on unknown race, return the computed plan.
- [x] 5.3 Map every sentinel error to its documented HTTP status + code; swag annotations on all handlers including the error codes.
- [x] 5.4 `Register(rg *gin.RouterGroup)` following the package convention.
- [x] 5.5 Per-handler integration tests against testcontainers Postgres: create→get round-trip, cascade delete removes legs, duplicate-ordinal 400, invalid-discipline 400, fueling-plan happy path + missing/out-of-range body weight + unknown race 404, and a `NotContains` assertion guarding unit isolation in the fueling response.

## 6. Wiring

- [x] 6.1 Instantiate repo/service/handlers and register routes in `internal/httpserver/server.go` (inside the authed API group, after the middleware stack).

## 7. MCP tools

- [x] 7.1 `internal/mcpserver/`: `registerRaceTools` adding `create_race`, `list_races`, `get_race`, `update_race`, `delete_race`, `plan_race_fueling` — each one HTTP call; write tools auto-derive the idempotency key via the existing helper; read tools send none.
- [x] 7.2 `plan_race_fueling` forwards `body_weight_kg` always and `sweat_rate_ml_per_hr` only when supplied.
- [x] 7.3 Bump the expected-tools list in `internal/mcpserver/mcp_integration_test.go` by the six new tools; add wrapper tests for query/body construction and response passthrough.

## 8. Docs

- [x] 8.1 `task swag` to regenerate `docs/` after the handler changes.
- [x] 8.2 README MCP tool table: add the six race tool rows.
- [x] 8.3 RUN_LOCAL.md: short walkthrough — create a race with legs, then `GET /races/{id}/fueling-plan?body_weight_kg=70&sweat_rate_ml_per_hr=900`.

## 9. Verification

- [x] 9.1 `task vet` clean.
- [x] 9.2 `task test` green (new package + MCP integration tag).
- [x] 9.3 `openspec validate add-race-fueling-plan --strict` passes.
