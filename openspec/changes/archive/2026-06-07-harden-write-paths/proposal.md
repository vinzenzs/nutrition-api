## Why

A real MCP-driven test session surfaced two correctness bugs and one rough edge in the write paths:

1. **Silent state corruption on PUT replays.** The MCP wrapper auto-derives an idempotency key from tool arguments. For POST-style creates that's the right semantic ("create-once"); for PUT-style replaces like `set_goals` it lies: replays return the cached response that no longer matches DB state. Specifically: `set_goals(15 fields)` → `set_goals(kcal_only)` → `set_goals(15 fields)` returns the cached step-1 response showing 15 fields, while the DB still has only kcal. The user thinks their goals are saved when they aren't.
2. **Plain-text 404 leak.** `POST /products/lookup/` with no barcode hits Gin's default NoRoute handler and returns the literal string `404 page not found`. Every other error in the API is structured JSON; this one breaks the contract and would surprise any client that assumes `error.error` exists.
3. **Empty-barcode case is unvalidated.** Even with a proper JSON 404, `POST /products/lookup/` should return `400 barcode_required` like other validation failures, not a not-found.

This change closes those three with a tight scope: tighten the idempotency contract so PUT writes are loud-rejected, register a global JSON NoRoute handler, validate the empty-barcode case explicitly, and surface a new cross-cutting `http-error-shape` capability so the JSON-everywhere invariant has a documented home.

Explicit non-goals for this change: the body-hash canonical-JSON idea is dropped (byte-strict comparison is the Stripe-canonical behaviour and the failed test that suggested otherwise turned out to have a real body difference — two notes that actually disagreed). ETag / If-Match support is forward-pointed in the new error's hint but not implemented. Float-precision serialisation rounding stays for the separate `unify-adherence-shape` change.

## What Changes

- **Tighten idempotency to POST/PATCH/DELETE.** The middleware processes Idempotency-Key only on those three methods. On `PUT`, a non-empty `Idempotency-Key` header is rejected with `400 idempotency_unsupported_for_put` and a body of `{"error":"idempotency_unsupported_for_put","hint":"use If-Match with ETag for retry-safety"}`. Handler logic does not change; the rejection is middleware-level so it applies uniformly to every PUT route (current and future).
- **Global JSON NoRoute handler.** Gin's default plain-text 404 is replaced by `r.NoRoute(...)` returning `404 Not Found` with `{"error":"not_found"}`. Same for `r.NoMethod(...)` returning `405 Method Not Allowed` with `{"error":"method_not_allowed"}`. The new `http-error-shape` capability documents this as the invariant.
- **Explicit empty-barcode validation.** `POST /products/lookup/` (no barcode in the path) registers as a sibling route that returns `400 Bad Request` with `{"error":"barcode_required"}`. Without this, the route falls through to NoRoute and gets a 404, which is semantically wrong — the path is recognised, the parameter is missing.
- **MCP `set_goals` tool drops `idempotency_key`.** The tool's input schema removes the field entirely. No auto-derive. If a future caller sends the header through a direct REST call, the backend's middleware rejection (above) gives them an actionable error.
- **Spec tightening on MCP auto-derive.** The `mcp-server` capability's "auto-derive" requirement is rewritten to explicitly scope to the POST-style write tools (`log_meal`, `log_meal_freeform`, `create_recipe`) and to forbid the field on PUT-style tools (`set_goals` today; future tools follow the same rule).
- **Tests cover all three bugs + the missing replay test.** The replay test that was never actually run during MCP testing — two byte-identical `log_meal_freeform` bodies with the same explicit `idempotency_key` returning the original meal — is added to the e2e suite so the canonical replay path is proven, not assumed.

## Capabilities

### New Capabilities
- `http-error-shape`: All HTTP error responses (including framework-level 404s and 405s) use the JSON shape `{"error":"<code>","...":...}`. This is a cross-cutting invariant the API has informally relied on; the new capability spec gives it a documented home and lets future changes link to it.

### Modified Capabilities
- `auth`: The "Idempotency-Key header on write endpoints" requirement is tightened from "every POST, PATCH, DELETE endpoint" (currently respected by impl but loosely described) to that same explicit set, and a new scenario rejects the header on PUT with `400 idempotency_unsupported_for_put`. The remaining scenarios (replay, body conflict, client isolation, TTL, GET-ignored) are preserved verbatim.
- `products`: The "Barcode lookup against Open Food Facts with local cache" requirement gains an empty-barcode validation scenario returning `400 barcode_required`. The existing four scenarios remain unchanged.
- `mcp-server`: The "Write tools auto-derive idempotency keys when none is supplied" requirement is rewritten to scope auto-derive to POST-style tools, exclude PUT-style tools (`set_goals` today), and drop `idempotency_key` from the input schema of any PUT-style tool. Existing scenarios for `log_meal_freeform` are preserved; new scenarios cover `set_goals` not having the field and the backend rejection if a header somehow arrives.

## Impact

- **Backend changes**:
  - `internal/idempotency/middleware.go`: add the PUT branch that returns 400 when the header is present; preserve the existing GET/HEAD/OPTIONS pass-through.
  - `cmd/nutrition-api/serve.go` (or wherever the router is wired): `r.NoRoute(...)` and `r.NoMethod(...)` register JSON responders.
  - `internal/products/handlers.go`: register `POST /products/lookup/` as a sibling of the parameterised route, returning `400 barcode_required`.
- **MCP wrapper changes**:
  - `internal/mcpserver/tools_goals.go` (or wherever `set_goals` is registered): remove `idempotency_key` from the input struct and tool description; remove the auto-derive call.
- **Tests**:
  - New: bug #1 sequence (set 15 → set kcal → set 15 → get returns 15).
  - New: bug #3 sequence (empty barcode → 400 JSON; unknown route → 404 JSON; wrong method on known route → 405 JSON).
  - New: PUT with Idempotency-Key → 400 with the documented body.
  - New e2e: two byte-identical `log_meal_freeform` calls with the same explicit `idempotency_key` → 201 with the original meal id (the test the MCP session never actually ran).
- **No schema changes**. No migrations. No new dependencies.
- **Breaking change**: clients that were silently relying on the buggy PUT-with-Idempotency-Key behaviour will now see a `400`. In practice this is only the auto-derive in the MCP wrapper (which this change updates) — direct REST callers were not relying on it.

### Out of scope (explicit non-goals)
- ETag / If-Match optimistic concurrency on `PUT /goals`. The new error's `hint` field forward-points at this, but adding it is its own change (and would need a schema column).
- Canonical-JSON body hashing. The byte-strict Stripe behaviour is correct; the bug suggesting otherwise was a test error.
- `kcal_target` shape unification, float-precision rounding, recipe quality indicators, `list_products` / `delete_product` MCP tools. Those live in `unify-adherence-shape` and (planned) `add-product-management-tools`.
- Retroactive cleanup of orphaned test products in the cache (manual sweep, not in scope).
