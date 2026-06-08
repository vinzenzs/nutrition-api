## 1. Migration

- [x] 1.1 Add `internal/store/migrations/010_add_hydration.up.sql`: `CREATE TABLE hydration_entries (id UUID PRIMARY KEY, logged_at TIMESTAMPTZ NOT NULL, quantity_ml NUMERIC(10,1) NOT NULL CHECK (quantity_ml > 0), note TEXT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now()); CREATE INDEX hydration_entries_logged_at_idx ON hydration_entries (logged_at);`
- [x] 1.2 Add `.down.sql`: `DROP TABLE hydration_entries;`
- [x] 1.3 Verify the migration applies cleanly against a fresh `task dev` Postgres and the schema looks as documented.

## 2. Backend: package skeleton

- [x] 2.1 Create `internal/hydration/` directory.
- [x] 2.2 `internal/hydration/types.go`: `Entry` struct mirroring the table columns. Use `*string` for `Note` so `omitempty` drops it from JSON when null.
- [x] 2.3 `internal/hydration/repo.go`: `Insert(ctx, e *Entry) error`, `GetByID(ctx, id uuid.UUID) (*Entry, error)`, `Patch(ctx, id, PatchParams) error`, `Delete(ctx, id) error`, `List(ctx, from, to time.Time) ([]*Entry, error)`. `ErrNotFound` sentinel for unknown ids. Use the existing `store.Querier`.
- [x] 2.4 `internal/hydration/service.go`: validation (quantity_ml > 0, logged_at not > 24h future, note ≤ 500 chars), thin orchestration over the repo.

## 3. Backend: HTTP handlers

- [x] 3.1 `internal/hydration/handlers.go`: `Register(rg *gin.RouterGroup)` mounting POST/GET/PATCH/DELETE on `/hydration` and `/hydration/:id`.
- [x] 3.2 POST handler: decode body with `c.ShouldBindJSON`; validate via the service; map errors:
  - `quantity_ml_invalid`, `logged_at_too_far_future`, `note_too_long` → 400 with the documented codes.
  - Insert and return 201 with the created entry.
- [x] 3.3 GET (list) handler: validate `from` and `to` (RFC 3339, `from < to`, span ≤ 92 days); map errors as `window_required` / `window_invalid` / `range_too_large` matching the meals shape. Wrap rows in `{"entries":[...]}`.
- [x] 3.4 PATCH handler: decode partial body, validate the same way as POST, map `404 hydration_not_found` on unknown id.
- [x] 3.5 DELETE handler: 204 on success, 404 on unknown id.
- [x] 3.6 Swag annotations for every handler, listing the documented error codes.

## 4. Backend: daily summary

- [x] 4.1 `internal/hydration/summary.go`: `DailyParams{Date time.Time; Loc *time.Location}` and `Daily{Date, TZ string; TotalMl float64; EntryCount int; Entries []*Entry}`.
- [x] 4.2 `Service.DailyFor(ctx, p DailyParams) (*Daily, error)` computes the day window in `p.Loc`, calls `repo.List` with the UTC bounds, sums `quantity_ml`, and returns the populated struct.
- [x] 4.3 `internal/hydration/summary_handlers.go`: register `GET /summary/hydration/daily` with the same TZ-resolution helper used by the existing summary handlers (default-tz fallback + WARN log when used). Validate `date` against `2006-01-02`; invalid → `400 date_invalid`. Invalid `tz` → `400 tz_invalid`.
- [x] 4.4 Apply `numfmt.Round1` to `TotalMl` at the response boundary (consistent with the existing nutrient-rounding rule).

## 5. Wiring

- [x] 5.1 In `cmd/nutrition-api/serve.go`, instantiate `hydration.Repo`, `hydration.Service`, `hydration.Handlers`, `hydration.SummaryHandlers`. Register both on the existing API group (so auth + idempotency middleware applies uniformly).
- [x] 5.2 Confirm `idempotency.Middleware` already handles POST on `/hydration` (no per-path config needed; it's method-based).

## 6. Backend tests

- [x] 6.1 `internal/hydration/handlers_test.go` with the standard `storetest.NewPool` pattern. Cover: log happy path, optional note, missing/zero/negative quantity, note > 500 chars, logged_at >24h future, window query (in-range vs out-of-range), missing/inverted window, range_too_large, idempotency replay (same key + body → same id; different body + same key → 409).
- [x] 6.2 `internal/hydration/summary_handlers_test.go`: daily summary totals across multiple entries in a TZ-shifted window, empty day, default-tz fallback emits the warning log line, invalid tz / invalid date rejections, rounding test (build a total like 2249.999 via an inserted entry, confirm the response shows 2250).
- [x] 6.3 Sanity: assert nutrition `/summary/daily` still returns the same shape as before (no hydration fields leaking in). One assertion in `internal/summary/handlers_test.go` is enough — "the response body does not contain the substring `total_ml`."

## 7. MCP wrapper

- [x] 7.1 `internal/mcpserver/tools_hydration.go`. Five input structs: `LogHydrationArgs{QuantityMl, LoggedAt, Note, IdempotencyKey}`, `ListHydrationArgs{From, To}`, `PatchHydrationArgs{ID, QuantityMl, LoggedAt, Note, IdempotencyKey}` (all editable fields as `*…`), `DeleteHydrationArgs{ID, IdempotencyKey}`, `DailyHydrationSummaryArgs{Date, TZ}`.
- [x] 7.2 Five handlers (`handleLogHydration`, …) following the existing tools_meals.go patterns. Use `effectiveIdempotencyKey` for write tools; reads pass no key.
- [x] 7.3 `registerHydrationTools(server, c)` registers all five with descriptions per the spec:
  - `log_hydration`: "Record a volume of fluid the user drank at a specific time. The optional `note` carries beverage context (e.g. 'water', 'iced coffee', 'electrolytes'). Use this for ANY drink — water, coffee, sports drinks. For beverages with nutriments (Coke, juice), additionally log via log_meal_freeform with the macros."
  - `daily_hydration_summary`: "Return the total ml and per-entry list for one calendar day. This is the volume-only summary — separate from daily_summary, which is the nutrient-only summary. Combine both when the user asks 'how did I do today?'"
  - others: standard CRUD descriptions.
- [x] 7.4 Wire `registerHydrationTools` in `cmd/nutrition-api/mcp.go` (or wherever tools are registered).

## 8. MCP tests

- [x] 8.1 `internal/mcpserver/tools_hydration_test.go`: per-tool tests using `newRecordingClient` / `newRecordingBodyClient`. Cover endpoint URLs, method, body, idempotency-key forwarding, response passthrough, 404 / 4xx forwarding with `isError=true`.
- [x] 8.2 Update `internal/mcpserver/mcp_integration_test.go` expected-tools list to include `log_hydration`, `list_hydration`, `patch_hydration`, `delete_hydration`, `daily_hydration_summary` (now 17 total).

## 9. Documentation

- [x] 9.1 `task swag` to regenerate OpenAPI for the new routes.
- [x] 9.2 `README.md`: add a "Hydration" subsection under the API examples (after "Meals", before "Summaries") with one example each of log, list, daily summary. Add the five new tools to the MCP table.
- [x] 9.3 `RUN_LOCAL.md`: tiny addition to the API walkthrough — log a glass of water, fetch the daily total. One block under "Trying the API end-to-end."

## 10. Pre-merge checks

- [x] 10.1 `task vet` clean.
- [x] 10.2 `task test` green (use `-p 1` if the testcontainers parallel boot flakes).
- [x] 10.3 Manual: with `task dev` running, `curl -X POST -H "Authorization: Bearer $MOBILE_API_TOKEN" -H "Content-Type: application/json" -d '{"quantity_ml":500,"logged_at":"<now>"}' http://localhost:8080/hydration` then `curl http://localhost:8080/summary/hydration/daily?date=$(date +%Y-%m-%d)` confirms the round-trip.
- [x] 10.4 OpenSpec validation: `openspec status --change "add-hydration-tracking"` shows 4/4 artifacts done.
