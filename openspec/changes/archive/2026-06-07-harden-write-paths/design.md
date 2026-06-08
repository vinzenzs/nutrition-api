## Context

A targeted MCP-driven test session produced a clean three-finding report. After triage and one bug-retraction, the three remaining issues are:

1. **Idempotency replay on PUT returns stale state.** Root cause: the MCP wrapper auto-derives an idempotency key from tool arguments and the backend middleware caches and replays based on it. For POST-style creates ("ensure this happened at least once") the semantic is correct; for PUT-style replaces ("the resource is now in this state") it is silently wrong — the cached response can lie when intermediate writes changed state.
2. **Plain-text 404 leaks from Gin's default NoRoute handler** when the path doesn't match any registered route (e.g. `POST /products/lookup/` with the path parameter empty).
3. **Empty barcode is not validated.** It should produce `400 barcode_required`, consistent with every other input validation in the API.

The user's testing report was disciplined enough to also retract a fourth finding (a 409 they originally read as a bug, which on inspection was correct byte-strict body-hash conflict behaviour). That retraction is load-bearing for this change's scope: we explicitly do NOT introduce canonical-JSON body hashing, because the byte-strict Stripe semantic is right.

The harden work is correctness-must-ship. It deliberately stays narrow so it can be reviewed and merged quickly. Adjacent concerns (API-aesthetics shape unification, missing CRUD tools) are explicitly punted.

## Goals / Non-Goals

**Goals:**

- Eliminate the silent state-corruption mode on `set_goals` (and any future PUT-style write).
- Eliminate the plain-text 404 by making JSON the universal error shape.
- Validate the empty-barcode case with a structured 400.
- Add the missing replay smoke test that proves byte-identical retries of `log_meal_freeform` return the original meal id (the test that never actually ran during the MCP session).
- Keep the change small enough to review in one sitting.

**Non-Goals:**

- ETag / If-Match optimistic concurrency. Forward-pointed in the new error's `hint` field; deferred.
- Canonical-JSON body hashing. Byte-strict comparison is correct.
- Aesthetic / breaking shape changes (kcal_target, float precision, adherence-row consistency). Those live in `unify-adherence-shape`.
- New MCP tools (`list_products`, `delete_product`) and the duplicate-component decision in `create_recipe`. Those live in `add-product-management-tools`.
- A migration to retire silently-orphaned test products in the cache.

## Decisions

### 1. Reject `Idempotency-Key` on PUT with `400 idempotency_unsupported_for_put`

The middleware in `internal/idempotency/middleware.go` already short-circuits on `GET`, `HEAD`, `OPTIONS`. We add a parallel branch: when `method == "PUT"` AND the header is non-empty, abort with `400` and body:

```json
{"error":"idempotency_unsupported_for_put","hint":"use If-Match with ETag for retry-safety"}
```

Three reasons this is the right shape:

- **Loud beats silent.** Bug #1 is the canonical case of "the framework let me ship code that looks safe but isn't." Loud rejection puts the design rule in the developer's face the first time they try; silent ignore lets them ship and only discover it under exactly the conditions that produce data corruption.
- **One-time cost.** `set_goals` is called weekly per user, not in tight retry loops. The "remove the key" friction is paid once per integration; the cost of silent-no-op is paid forever.
- **Future-pointed.** The `hint` field tells the caller where the real retry-safety lives (ETag) without committing us to ship it yet. When a bulk-migration or batch-tool consumer asks for retry-safe writes, optimistic concurrency is the right answer — and pointing at that path now means we don't have to retrofit around silent-ignore semantics later.

**Alternatives considered:**

- *Accept-but-ignore the header on PUT.* Rejected — see "loud beats silent" above.
- *Honour the header on PUT but invalidate the cache when any intermediate write to the same resource lands.* Rejected — requires per-resource invalidation bookkeeping, doesn't solve the conceptual mismatch (PUT replaces state; replay returns an old state representation).
- *Only short-circuit if current DB state matches the cached response.* Rejected — requires reading the resource on every replay, defeating the optimisation, and "matches" is non-trivial to define.

### 2. Global JSON NoRoute and NoMethod handlers

Gin defaults to `404 page not found` (plain text) and `405 Method Not Allowed` (plain text). We register:

```go
r.NoRoute(func(c *gin.Context) {
    c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
})
r.NoMethod(func(c *gin.Context) {
    c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "method_not_allowed"})
})
```

These are framework-level catches that protect every current and future route from the "plain text leak" class of bug. The `http-error-shape` spec documents this as a cross-cutting invariant, so future work that adds new routes inherits the JSON-everywhere contract without having to remember it.

**Alternatives considered:**

- *Inline 404 handling per capability.* Rejected — repetitive and easy to forget on new routes.
- *A custom 404 body that varies per capability (e.g. `meal_not_found` on `GET /meals/...`)*. Rejected for this change — the specific 404s for known routes are already handled at the handler layer (e.g. `GET /meals/{id}` returns `meal_not_found`). The NoRoute path is for genuinely unknown paths and should be generic.

### 3. Explicit `POST /products/lookup/` route returning `400 barcode_required`

The current parametrised route `POST /products/lookup/:barcode` does not match the empty case. Two fixes are possible:

- Register a sibling route at `POST /products/lookup/` that returns 400.
- Validate inside the existing handler when `c.Param("barcode") == ""` (which doesn't happen today because Gin doesn't route there at all).

The sibling-route approach is cleaner: it puts the validation at the routing layer where it belongs, doesn't pollute the existing handler with a dead branch, and produces the right semantic (400 = "the route exists, your input is wrong" vs 404 = "no such route").

**Alternatives considered:**

- *Trust the new NoRoute catch.* Rejected — that returns `404 not_found`, which is wrong. The route is recognised, the path parameter is missing — that's a validation error, not a not-found.

### 4. `set_goals` MCP tool drops `idempotency_key` from its input schema

The tool's input struct removes the field entirely; the `effectiveIdempotencyKey` call disappears; the tool description picks up one sentence noting that `set_goals` is retry-unsafe today and pointing future work at ETag.

The backend rejection (Decision 1) is the single source of truth. The wrapper change is just "don't expose the field" — there's nothing to filter, no code path that could accidentally pass it.

**Alternatives considered:**

- *Keep the field but always strip it before passing to the backend.* Rejected — confusing for the agent (it sees a field that does nothing), and adds a wrapper-level rule the backend already enforces.

### 5. The `http-error-shape` capability is a new top-level capability

The user weighed three placement options:
- A. One ADDED requirement per affected capability spec. Repetitive.
- B. New `http-error-shape` capability with one requirement. Cleaner; introduces a cross-cutting cap.
- C. Tuck into `auth/spec.md`. Minimal scope creep, slightly less discoverable.

We're going with B. The "all errors are JSON" rule isn't an auth concern; the bearer-error shape just happens to be one example of it. A dedicated capability gives the invariant its own home, makes it the obvious place to look when adding new error codes or shapes, and lets future cross-cutting HTTP concerns (rate-limit responses, request-id propagation) land alongside it without rearranging existing specs.

The capability has one requirement and three scenarios: known errors carry `{"error":"<code>", ...}`, unknown paths fall through to NoRoute and return JSON, and known paths with wrong methods return JSON.

### 6. Test additions

The new tests reproduce all three bugs and add the missing replay smoke:

- **Bug #1 repro (unit-level, against `goals` handler)**: `setX(15-field)` → `setKcal(1-field)` → `setX(15-field)` → `get` returns all 15. Without the fix, `get` returns only kcal.
- **Bug #3 reproduction (handler test against router)**: `POST /products/lookup/` returns 400 with `{"error":"barcode_required"}`.
- **NoRoute coverage (handler test)**: `GET /this/does/not/exist` returns `{"error":"not_found"}` with content-type `application/json`.
- **NoMethod coverage (handler test)**: `PATCH /healthz` (or any wrong-method-on-known-route case) returns `{"error":"method_not_allowed"}`.
- **PUT rejection (middleware test)**: `PUT /goals` with `Idempotency-Key: anything` returns 400 with the documented body. Without the header it proceeds normally.
- **Missing replay smoke (e2e in `internal/e2e/`)**: two `POST /meals/freeform` calls with the same explicit `Idempotency-Key` and byte-identical bodies return `201` and the same `meal_id`. This is the test the MCP session never actually ran.

## Risks / Trade-offs

- **Breaking change for clients that silently relied on PUT-with-Idempotency-Key.** *Mitigation:* the only known caller is the MCP wrapper's `set_goals` tool, which this change fixes in lockstep. Direct REST callers were not relying on the behaviour because it was buggy in the way they would have noticed.
- **The error code `idempotency_unsupported_for_put` may surprise an integrator who reads the auth spec without reading the new MODIFIED requirement.** *Mitigation:* the auth spec delta keeps the existing "POST/PATCH/DELETE" wording explicit and adds the new PUT-rejection scenario adjacent to it. Discoverable in the same section.
- **Global NoRoute could mask routing bugs during development.** A 404 from a typo in a registered route now looks the same as a 404 from a genuinely unknown URL. *Mitigation:* the response body is consistent JSON either way; the route diff is in the URL, which logs already carry. Minor cost.
- **The new `http-error-shape` capability is one requirement.** Capabilities with a single requirement can feel over-engineered. *Mitigation:* the cross-cutting nature is real (the rule affects every capability that returns errors); a dedicated home prevents the rule from rotting into "everyone's spec says different things."
- **No ETag rollout means PUT writes have no retry-safety story today.** *Mitigation:* `set_goals` is rare (weekly cadence per user); the cost of "your retry might land twice if you network-blink mid-PUT" is small. ETag is the right answer when the cost grows; the error hint already points at it.

## Migration Plan

- Backward-compat: clients sending `Idempotency-Key` on PUT (today, only the MCP wrapper) will get `400` instead of the previous (buggy) replay behaviour. The wrapper change ships in the same commit so the breakage is internal-only.
- Rollback: the middleware change is the only user-visible breaking edit. Reverting it restores the buggy behaviour, but also drops the `400` response, so any caller adapted to the new contract will start sending the header again "for retry safety" — which silently does nothing. Cleaner to fix-forward.
- No schema, no migration files, no env vars.

## Open Questions

- Whether to also reject `Idempotency-Key` on a future hypothetical `PUT /<resource>` not yet defined. The middleware rule applies to all PUT methods generically, so the answer is "yes, automatically." A spec scenario captures that the rule is method-shaped, not path-shaped.
- Whether the `r.NoMethod` body should include the allowed methods (Gin can populate the `Allow` response header automatically). For v1 we just return the structured JSON; the header is set by Gin regardless. Consumers who care can read the header.
- Whether `http-error-shape` should grow to cover correlation-id propagation, rate-limit body shape, and `Retry-After` semantics in a single capability. Out of scope here, but the placement decision (B) makes those future additions natural.
