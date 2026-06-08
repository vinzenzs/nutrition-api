## 1. Backend: idempotency middleware rejects PUT

- [x] 1.1 In `internal/idempotency/middleware.go`, add a branch before the existing GET/HEAD/OPTIONS short-circuit (or as a parallel check): when `c.Request.Method == http.MethodPut` AND `c.GetHeader(HeaderName) != ""`, abort with status `400` and body `{"error":"idempotency_unsupported_for_put","hint":"use If-Match with ETag for retry-safety"}`. Use `c.AbortWithStatusJSON`.
- [x] 1.2 Verify the middleware still passes through PUT requests that do NOT carry the header. The header check must use `c.GetHeader(idempotency.HeaderName)` and treat both an absent header and an empty string as "no header".
- [x] 1.3 Confirm via inspection that the existing `Idempotency-Key on GET requests is ignored` scenario still passes — the new PUT branch must not interfere with the GET path.

## 2. Backend: global JSON NoRoute and NoMethod

- [x] 2.1 In the router setup (currently `internal/httpserver/` based on the layout — locate the file that calls `gin.New()`), register `r.NoRoute(func(c *gin.Context) { c.JSON(http.StatusNotFound, gin.H{"error": "not_found"}) })`.
- [x] 2.2 Register `r.NoMethod(func(c *gin.Context) { c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "method_not_allowed"}) })`. Note that for `NoMethod` to fire, Gin needs `gin.Engine.HandleMethodNotAllowed = true` — set it on the engine immediately after `gin.New()`.
- [x] 2.3 Confirm the existing handler-level 404s (e.g. `meal_not_found`, `product_not_found`) still take precedence — those return from the handler with their own JSON body before NoRoute fires.

## 3. Backend: empty barcode returns 400

- [x] 3.1 In `internal/products/handlers.go` (or wherever `POST /products/lookup/:barcode` is registered), add a sibling route at `POST /products/lookup/` that returns `400 Bad Request` with `{"error":"barcode_required"}`.
- [x] 3.2 Verify the existing parametrised route's handler still runs for non-empty barcodes. The new sibling must not shadow the parametrised case.
- [x] 3.3 Update the handler's swag annotation to document the new `400 barcode_required` failure mode.

## 4. MCP wrapper: set_goals drops idempotency_key

- [x] 4.1 In the `set_goals` tool registration (likely in `internal/mcpserver/tools_goals.go`), remove the `IdempotencyKey` field from the input struct. Also remove any matching `jsonschema:"…"` tag and any field-level handling code.
- [x] 4.2 In the same handler, remove the `effectiveIdempotencyKey(...)` call. The `Patch` invocation (or whatever HTTP method the wrapper uses to hit `PUT /goals`) must not pass any idempotency key.
- [x] 4.3 Update the tool's `Description` to include one sentence: that retries of `set_goals` may land twice on transient network failure, and that ETag/If-Match retry-safety is forward-pointed.
- [x] 4.4 Confirm the wrapper still uses `PUT` (not `PATCH` or `POST`) on `set_goals`. If the wrapper currently uses a non-PUT method, that's a separate bug that needs its own fix; flag it.

## 5. Tests

- [x] 5.1 New middleware test in `internal/idempotency/middleware_test.go`: `PUT /anything` with `Idempotency-Key: anykey` → 400 with `{"error":"idempotency_unsupported_for_put","hint":"use If-Match with ETag for retry-safety"}`. Run against the existing testcontainers-backed setup; no schema additions needed since the rejection is pre-handler.
- [x] 5.2 New middleware test: `PUT /anything` without the header → 200 (or whatever the dummy handler returns), and zero rows inserted into `idempotency_records`.
- [x] 5.3 New router test: `GET /this/does/not/exist` returns 404 with `Content-Type: application/json; charset=utf-8` and body `{"error":"not_found"}`.
- [x] 5.4 New router test: `PATCH /healthz` returns 405 with `Content-Type: application/json; charset=utf-8` and body `{"error":"method_not_allowed"}`.
- [x] 5.5 New handler test in `internal/products/handlers_test.go`: `POST /products/lookup/` returns 400 with `{"error":"barcode_required"}`.
- [x] 5.6 New handler test for the existing nutrition-goals package (location may vary): the bug #1 repro — set 15 fields → set only kcal → set 15 fields → get returns all 15. Without the rest of this change (middleware not yet rejecting), this test would still pass because direct handler calls bypass the wrapper auto-derive — flag in the test comment that the corresponding MCP repro is in section 5.8.
- [x] 5.7 New e2e test addition in `internal/e2e/`: the missing replay smoke. Two `POST /meals/freeform` calls with the same explicit `Idempotency-Key: <some-key>` and byte-identical bodies. Assert status 201 on both and that the `id` field of the response body is identical across the two calls.
- [x] 5.8 New MCP integration test in `internal/mcpserver/`: drive the `set_goals` tool twice via the MCP JSON-RPC surface with no `idempotency_key` field anywhere in the request. Verify both calls hit the backend with NO `Idempotency-Key` header (mock the backend to capture headers) and that the backend handler runs twice — i.e., no replay short-circuit fires.

## 6. Documentation

- [x] 6.1 Update `internal/mcpserver/README.md` (or the equivalent doc surfaced via swag) to note that `set_goals` is retry-unsafe and to mention the future ETag path.
- [x] 6.2 Update the OpenAPI annotations on `PUT /goals` and the new `POST /products/lookup/` route via `task swag` so the regenerated `docs/` reflects the new error codes.

## 7. Pre-merge checks

- [x] 7.1 `task vet` clean.
- [x] 7.2 `task test` green.
- [ ] 7.3 Manual: run `task dev`; reproduce bug #1 via curl (`set_goals` 15-field → kcal-only → 15-field; verify GET returns 15); reproduce bug #3 (`POST /products/lookup/` → 400 JSON); confirm the missing replay smoke via curl (`POST /meals/freeform` twice with same explicit `Idempotency-Key` and identical bodies → 201 + same id).
- [x] 7.4 OpenSpec validation: `openspec status --change "harden-write-paths"` shows 4/4 artifacts done.
