## 1. Project setup

- [x] 1.1 Add the official Go MCP SDK dependency (`github.com/modelcontextprotocol/go-sdk`). If the SDK fails to integrate cleanly, fall back to `github.com/mark3labs/mcp-go` as documented in `design.md`.
- [x] 1.2 Create directory skeleton: `cmd/mcp/`
- [x] 1.3 Extend `.env.example` with `NUTRITION_API_URL`, `MCP_REQUEST_TIMEOUT_SECONDS` (the `AGENT_API_TOKEN` entry is already there)
- [x] 1.4 Extend the `Makefile` with `mcp-run`, `mcp-build`, and `mcp-install` targets. `mcp-install` copies the built binary to `~/.local/bin/nutrition-mcp`.

## 2. Configuration

- [x] 2.1 `cmd/mcp/config.go`: load `NUTRITION_API_URL` (default `http://localhost:8080`), `AGENT_API_TOKEN` (required, non-empty), `MCP_REQUEST_TIMEOUT_SECONDS` (default 10). Refuse to start on missing/empty `AGENT_API_TOKEN`.
- [x] 2.2 Validate the URL parses with `net/url.Parse` and rejects on failure.

## 3. REST API client

- [x] 3.1 `cmd/mcp/apiclient.go`: small typed HTTP client with `Get`, `Post`, `Patch`, `Delete` methods that share base URL + bearer header.
- [x] 3.2 Every request sets `Authorization: Bearer <AGENT_API_TOKEN>` and `User-Agent: nutrition-mcp/<version>`.
- [x] 3.3 Write requests accept an optional `idempotencyKey` argument and set the `Idempotency-Key` header when non-empty.
- [x] 3.4 Responses are returned as `(status int, body []byte, err error)` so each tool can decide how to surface them.
- [x] 3.5 Unit tests using `httptest.Server` covering: header injection, request body forwarding, response passthrough, network failure surface as `transport` error.

## 4. Error mapping

- [x] 4.1 `cmd/mcp/errors.go`: helper that turns `(status int, body []byte, err error)` into an MCP tool result. 2xx → success result with text content = body. Non-2xx → isError=true result with text content = body. Network/transport error → isError=true result with body `{"error":"transport","detail":"…"}`.
- [x] 4.2 Unit tests for each branch (2xx, 4xx, 5xx, transport).

## 5. Idempotency-key derivation

- [x] 5.1 `cmd/mcp/idempotency.go`: `Derive(toolName string, args any) string` returns `sha256_hex(toolName + "|" + canonical_json(args))`. The canonical JSON encoder must sort object keys deterministically.
- [x] 5.2 The function strips an `idempotency_key` field from `args` before hashing so that adding/removing the field never changes the derived key.
- [x] 5.3 Caller convention: each write tool calls `effectiveKey(input.IdempotencyKey, "tool_name", input)` which returns the explicit key if present, otherwise the derived one.
- [x] 5.4 Unit tests: same args → same key; different args → different keys; explicit key wins; idempotency_key field is excluded from derivation.

## 6. Tool definitions: products

- [x] 6.1 `cmd/mcp/tools_products.go`: register `lookup_product_by_barcode` with input `{barcode: string (required), refresh: bool?}` and description that captures: "Looks up a product by barcode. First call hits Open Food Facts and caches the result. Set refresh=true to force a refresh. On 404 the response body's `next` field tells you to call log_meal_freeform instead."
- [x] 6.2 Register `search_products` with input `{q: string (required)}` and description: "Search the locally cached product list by name or brand. Results are ranked by most recently logged."
- [x] 6.3 Unit tests using a stubbed apiclient: verify each tool calls the right endpoint and returns the response body as content.

## 7. Tool definitions: meals

- [x] 7.1 Register `log_meal` with input `{product_id: uuid (required), quantity_g: float (required, >0), logged_at: rfc3339 (required), meal_type?: enum, note?: string, idempotency_key?: string}`. Description points to `log_meal_freeform` for the "no product yet" case.
- [x] 7.2 Register `log_meal_freeform` with input `{name: string (required), nutriments_per_100g: {kcal?, protein_g?, carbs_g?, fat_g?, fiber_g?, sugar_g?, salt_g?}, quantity_g: float (required, >0), logged_at: rfc3339 (required), meal_type?: enum, note?: string, save_as_product?: bool, idempotency_key?: string}`. Description includes: "Logs a meal from user-supplied nutriment estimates. Use this for natural-language inputs like 'I had a banana' after estimating the macros. Set save_as_product=true to also create a reusable product."
- [x] 7.3 Register `patch_meal` with input `{meal_id: uuid (required), quantity_g?: float, logged_at?: rfc3339, meal_type?: enum, note?: string, idempotency_key?: string}`.
- [x] 7.4 Register `delete_meal` with input `{meal_id: uuid (required), idempotency_key?: string}`. Returns an empty content tool result on 204.
- [x] 7.5 Unit tests covering: happy path for each tool, idempotency-key forwarding, auto-derivation when omitted, REST error passthrough for `product_not_found` and `meal_not_found`.

## 8. Tool definitions: summary

- [x] 8.1 Register `daily_summary` with input `{date: yyyy-mm-dd (required), tz?: iana}`. Description notes that omitting `tz` uses the REST server's configured default.
- [x] 8.2 Register `range_summary` with input `{from: yyyy-mm-dd (required), to: yyyy-mm-dd (required), tz?: iana}`. Description notes the 92-day maximum range.
- [x] 8.3 Unit tests: tools call the right endpoints, optional `tz` is omitted when not supplied, `range_too_large` error body is forwarded verbatim.

## 9. Server wiring & lifecycle

- [x] 9.1 `cmd/mcp/main.go`: load config, build apiclient, register all eight tools, start the MCP SDK's stdio server, block until stdin EOF or SIGTERM.
- [x] 9.2 Run a single `GET <NUTRITION_API_URL>/healthz` smoke check at startup. Log success or failure to stderr but do not block tool registration on failure (so the agent gets a useful error on the first tool call rather than the process silently exiting).
- [x] 9.3 All stderr logging uses `slog` with JSON output so Claude Desktop's MCP log is greppable.
- [x] 9.4 Trap SIGTERM and SIGINT, return from the server loop cleanly, exit 0.

## 10. Optional MCP integration test

- [x] 10.1 `cmd/mcp/mcp_integration_test.go` behind `//go:build integration`: spawn the binary, send `initialize` and a single `tools/list` JSON-RPC frame on stdin, assert the eight expected tool names appear on stdout. Skipped by default in `go test ./...`; run with `go test -tags=integration ./cmd/mcp/`.

## 11. Documentation

- [x] 11.1 Add a section to README titled "MCP server (LLM agent integration)" that explains: what it is, how to build it (`make mcp-build`), and how to register it with Claude Desktop (sample `claude_desktop_config.json` block) and Claude Code (sample `mcp.json` block).
- [x] 11.2 Document the eight tools in a single table in the README (name + one-line description + which REST endpoint they wrap).
- [x] 11.3 Note in the README that the MCP server requires the REST API to be running (the `make run` target).

## 12. Pre-merge checks

- [x] 12.1 `go vet ./...` clean
- [x] 12.2 `go test ./...` green (excluding the integration-tagged MCP test, which runs on demand)
- [x] 12.3 Manual smoke test: register the binary with Claude Code, invoke `daily_summary` for today, confirm a response comes back without errors. (Documented checklist; not automated.)
