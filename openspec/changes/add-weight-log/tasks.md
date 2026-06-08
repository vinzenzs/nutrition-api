## 1. Migration

- [x] 1.1 Add `internal/store/migrations/013_add_body_weight.up.sql` (next available number — `012_add_workouts` is in place; if some other change applies first and takes `013`, renumber):
  - `CREATE TABLE body_weight_entries (id UUID PRIMARY KEY, logged_at TIMESTAMPTZ NOT NULL, weight_kg NUMERIC(5,2) NOT NULL CHECK (weight_kg > 0), body_fat_pct NUMERIC(4,2) NULL CHECK (body_fat_pct IS NULL OR (body_fat_pct >= 0 AND body_fat_pct <= 100)), note TEXT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now());`
  - `CREATE INDEX body_weight_entries_logged_at_idx ON body_weight_entries (logged_at);`
- [x] 1.2 Add `.down.sql`: `DROP TABLE body_weight_entries;`
- [x] 1.3 Verify the migration applies cleanly against a fresh `task dev` Postgres and the schema matches the spec. (Verified implicitly via the testcontainers tests in §6.)

## 2. Backend: package skeleton

- [x] 2.1 Create `internal/bodyweight/` directory.
- [x] 2.2 `internal/bodyweight/types.go`: `Entry` struct mirroring the table columns. Use `*float64` for `BodyFatPct` (omitempty when null) and `*string` for `Note`.
- [x] 2.3 `internal/bodyweight/repo.go`: `Insert(ctx, *Entry) error`, `GetByID(ctx, uuid.UUID) (*Entry, error)`, `Patch(ctx, uuid.UUID, PatchParams) error`, `Delete(ctx, uuid.UUID) error`, `List(ctx, from, to time.Time) ([]*Entry, error)`. `ErrNotFound` sentinel. Add `ListInRange(ctx, fromUTC, toUTC time.Time) ([]*Entry, error)` for the trend computation (returns every entry in a UTC range; the trend layer slices into per-day windows). Use the existing `store.Querier`.
- [x] 2.4 `internal/bodyweight/service.go`: validation (weight_kg > 0, body_fat_pct in [0,100], logged_at not > 24h future, note ≤ 500 chars), thin orchestration over the repo. Validation errors map to sentinel errors per the spec error codes.

## 3. Backend: trend computation

- [x] 3.1 Create `internal/bodyweight/trend.go`:
  - `TrendParams{From, To time.Time; Loc *time.Location; WindowDays int}` (where `From`/`To` are local dates at midnight in `Loc`).
  - `TrendPoint{Date string; RollingAvgKg *float64; SampleCount int}`.
  - `Trend{From, To, TZ string; WindowDays int; Points []TrendPoint}`.
  - `Service.TrendFor(ctx, p TrendParams) (*Trend, error)`:
    - Fetches `repo.ListInRange(ctx, fromUTC, toUTC)` where `fromUTC = (From - WindowDays + 1).UTC()` and `toUTC = (To + 1 day).UTC()`.
    - Iterates per calendar date in `[From, To]` (local TZ), counts entries with `logged_at >= dayLocal - (WindowDays-1) days` (UTC-converted), computes mean of their `weight_kg`, rounds via `numfmt.Round1`.
    - Returns one `TrendPoint` per date; null `RollingAvgKg` and `SampleCount: 0` for empty windows.
- [x] 3.2 Unit tests in `internal/bodyweight/trend_test.go`:
  - Three consecutive daily entries → `window_days=3` on the last date yields mean of all three.
  - Single entry → `sample_count: 1`, `rolling_avg_kg = that sample`.
  - Empty window → `sample_count: 0`, `rolling_avg_kg: nil`.
  - Two entries on the same date with `window_days=1` → both contribute, mean correct.
  - Rounding: build a window whose mean is `73.4666…` → response shows `73.5`.

## 4. Backend: HTTP handlers

- [x] 4.1 `internal/bodyweight/handlers.go`: `Register(rg *gin.RouterGroup)` mounting POST/GET on `/weight`, PATCH/DELETE on `/weight/:id`, GET on `/weight/trend`.
- [x] 4.2 POST handler: decode body with `c.ShouldBindJSON`; validate via the service; map errors as documented.
- [x] 4.3 GET (list) handler: validate `from` and `to` (RFC 3339, `from < to`, span ≤ 92 days); map errors as `window_required` / `window_invalid` / `range_too_large`. Wrap rows in `{"entries":[...]}`.
- [x] 4.4 PATCH handler: decode partial body (tolerant decoder; unknown fields ignored). Validate supplied fields. `404 weight_not_found` on unknown id.
- [x] 4.5 DELETE handler: 204 on success, 404 on unknown id.
- [x] 4.6 Trend handler at `GET /weight/trend`: parse `from`/`to` as `YYYY-MM-DD`, `window_days` as int with default 7 and bounds [1, 30], resolve `tz` (default `DEFAULT_USER_TZ` with WARN log on fallback); map errors to `range_required` / `date_invalid` / `range_invalid` / `range_too_large` (366-day cap) / `window_days_invalid` / `tz_invalid`. Call `service.TrendFor` and return the `Trend` payload.
- [x] 4.7 Swag annotations for every handler, listing the documented error codes.

## 5. Wiring

- [x] 5.1 In `internal/httpserver/server.go`, instantiate `bodyweight.Repo`, `bodyweight.Service`, `bodyweight.Handlers`. Register on the existing API group (auth + idempotency middleware applies uniformly). Trend handler shares the same `Handlers` (one constructor takes the default-TZ string + a `slog.Logger`).
- [x] 5.2 Confirm `idempotency.Middleware` already handles POST on `/weight` (no per-path config needed; method-based).

## 6. Backend tests

- [x] 6.1 `internal/bodyweight/handlers_test.go` with the standard `storetest.NewPool` pattern. Cover: log happy path, optional body_fat_pct + note, missing/zero/negative weight, body_fat_pct out of range (low and high), note > 500 chars, logged_at >24h future, list window (in-range vs out-of-range, missing / inverted window, range_too_large), idempotency replay (same key + body → same id; different body + same key → 409). One PATCH happy + invalid + 404. One DELETE happy + 404.
- [x] 6.2 `internal/bodyweight/trend_handlers_test.go` (separate from the unit trend tests):
  - Smooth-three-days end-to-end (POST × 3 then GET /weight/trend → expected rolling means).
  - Sparse window (one entry, 7-day window → `sample_count: 1`).
  - Empty day in middle of range (gap day → `sample_count: 0`, `rolling_avg_kg: nil`).
  - `window_days_invalid` rejected at 0 and at 31.
  - Default `window_days = 7` when omitted.
  - Range > 366 days → `range_too_large`.
  - Default-tz fallback emits the warning log line.
  - Invalid tz → `400 tz_invalid`.
  - Invalid date format → `400 date_invalid`.
- [x] 6.3 Sanity: assert nutrition `/summary/daily` still returns the same shape as before (no body-weight fields leaking in). One assertion in `internal/summary/handlers_test.go`: response body does not contain `weight_kg` or `body_fat_pct`.

## 7. MCP wrapper

- [x] 7.1 `internal/mcpserver/tools_weight.go`. Five input structs:
  - `LogWeightArgs{WeightKg, LoggedAt, BodyFatPct*, Note*, IdempotencyKey}`.
  - `ListWeightsArgs{From, To}`.
  - `PatchWeightArgs{ID, WeightKg*, BodyFatPct*, LoggedAt*, Note*, IdempotencyKey}`.
  - `DeleteWeightArgs{ID, IdempotencyKey}`.
  - `WeightTrendArgs{From, To, WindowDays*, TZ}`.
- [x] 7.2 Five handlers following the existing `tools_hydration.go` patterns. `effectiveIdempotencyKey` for writes; reads no key; delete returns 204→empty.
- [x] 7.3 `registerWeightTools(server, c)` with descriptions per the spec (sample_count interpretation nudge on `weight_trend`; "do not prescribe a default weighing time" posture on `log_weight`).
- [x] 7.4 Wire `registerWeightTools` in `internal/mcpserver/server.go`.

## 8. MCP tests

- [x] 8.1 `internal/mcpserver/tools_weight_test.go`: per-tool tests using the recorder pattern. Endpoint URLs, method, body, idempotency-key forwarding, response passthrough, 404 / 4xx as `isError=true`.
- [x] 8.2 Update `internal/mcpserver/mcp_integration_test.go` expected-tools list to include the five new weight tools (now 30 total).

## 9. Documentation

- [x] 9.1 `task swag` to regenerate OpenAPI for the new routes.
- [x] 9.2 `README.md`: add a "Body weight" subsection under the API examples (placed after Workouts, before Summaries). Five new tools added to the MCP table.
- [x] 9.3 `RUN_LOCAL.md`: log a weight + request a 7-day trend; example pipes through `jq '.points[-7:]'` so the user sees the trailing-week shape.
- [x] 9.4 Add `internal/bodyweight/` to the project-layout section in README.

## 10. Pre-merge checks

- [x] 10.1 `task vet` clean.
- [x] 10.2 `task test` green. Saw one transient testcontainers ping-deadline flake on `TestTrendFor_Rounding`, which passed cleanly when re-run in isolation.
- [ ] 10.3 Manual: with `task dev` running, `curl -X POST -H "Authorization: Bearer $MOBILE_API_TOKEN" -H "Content-Type: application/json" -d '{"weight_kg":72.5,"logged_at":"<now>","body_fat_pct":14.2}' http://localhost:8080/weight` then `curl http://localhost:8080/weight/trend?from=$(date -u -v-30d +%Y-%m-%d)&to=$(date +%Y-%m-%d)` confirms the round-trip and that the trend includes the just-logged entry.
- [x] 10.4 OpenSpec validation: `openspec status --change "add-weight-log"` shows 4/4 artifacts done.
