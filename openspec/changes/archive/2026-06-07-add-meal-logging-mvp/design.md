## Context

This is the first change in a fresh repository — no existing code, no schema. The user is one person, building a personal nutrition log they will read and write from a mobile app they own and (later, via a separate change) an LLM coaching agent reached over MCP. Open Food Facts is the only external data source, with no auth and generous rate limits. The proposal commits us to Go + Gin + Postgres; the open design surface is around the data model, write paths, OFF resilience, time/timezone handling, idempotency, and project layout.

The "personal, single-user, two-client" framing is the key constraint. It rules out the typical multi-tenant complexity (user IDs everywhere, row-level isolation) but keeps two questions sharp:
1. What does the API look like when one client is human-driven (mobile) and the other is an LLM that will retry on transient failure and benefit from agent-shaped responses?
2. How do we treat Open Food Facts data — which is patchy, sometimes wrong, and outside our control — without polluting our own schema with its mess?

## Goals / Non-Goals

**Goals:**

- A correct and resilient personal log: every meal eaten, accurately attributed to a moment in time and a portion in grams.
- Two clean write paths: barcode-backed (mobile) and freeform (LLM). Both produce equivalently queryable `meal_entries`.
- Daily and ranged summaries that respect the user's local timezone without storing timezone-shifted timestamps.
- Idempotent writes so the LLM agent's retries are safe.
- A storage model that survives Open Food Facts changing fields or shapes upstream, without re-fetching.

**Non-Goals:**

- An MCP server, tool definitions, or any agent-side prompt design. That is a follow-up change that wraps this REST API.
- Trends, streaks, coaching, or any aggregation beyond per-day totals across an explicit window.
- OFF text search. The freeform endpoint covers the "I don't have a barcode" case structurally.
- Multi-user, OAuth, web UI, mobile UI.
- Computing nutriment estimates server-side (the LLM does this for the freeform path).

## Decisions

### 1. Two-table data model — `products` and `meal_entries` — kept distinct

A meal entry is an event in time; a product is a reusable definition. Conflating them (denormalizing nutriments onto every entry, no products table) would make barcode caching impossible and product search meaningless. Splitting them is the small bit of upfront modeling that keeps everything else simple.

```
products                                  meal_entries
─────────                                  ────────────
id            uuid pk                     id              uuid pk
barcode       text null  unique           product_id      uuid fk → products(id)  null
name          text                        logged_at       timestamptz   (UTC)
brand         text null                   quantity_g      numeric(10,3)
source        text  enum(off,manual)      meal_type       text null  enum
serving_size_g numeric null               note            text null
kcal_per_100g  numeric null               -- freeform snapshot (when product_id is null)
protein_g_per_100g numeric null           snapshot_name             text null
carbs_g_per_100g   numeric null           snapshot_kcal_per_100g    numeric null
fat_g_per_100g     numeric null           snapshot_protein_g_per_100g numeric null
fiber_g_per_100g   numeric null           snapshot_carbs_g_per_100g   numeric null
sugar_g_per_100g   numeric null           snapshot_fat_g_per_100g     numeric null
salt_g_per_100g    numeric null           snapshot_fiber_g_per_100g   numeric null
off_payload   jsonb null                  snapshot_sugar_g_per_100g   numeric null
fetched_at    timestamptz null            snapshot_salt_g_per_100g    numeric null
last_logged_at timestamptz null           idempotency_key text null  -- per-row
created_at    timestamptz                 created_at      timestamptz
updated_at    timestamptz                 updated_at      timestamptz
```

Individual nutriment columns are nullable because OFF data is patchy — every field may or may not be present per product. Storing nutriments as discrete columns (rather than a single JSON blob) makes summary math a straight SQL aggregation.

**Alternatives considered:**
- *Single `meal_entries` table, nutriments denormalized everywhere.* Rejected: barcode caching becomes ad-hoc, product search disappears, every meal entry pays the cost of a future product re-fetch.
- *Nutriments as a single JSONB column on products.* Rejected: summaries become slow and lose type safety; the win is too small for the loss.

### 2. Grams as the canonical unit

The server only accepts and stores `quantity_g`. Clients convert servings, slices, cups, or any other UI affordance into grams before sending. The product row may store `serving_size_g` to *help* clients offer "1 serving" as a quick button, but the server never does that math.

This pushes UI complexity to where it belongs (the client knows its UI) and keeps summary math trivial: `sum(kcal_per_100g × quantity_g / 100)`.

**Alternatives considered:**
- *Store `{quantity, unit}` and a conversion table.* Rejected: the conversion table becomes its own messy spec (what is "one slice of bread"?). Pushing it to clients is cleaner.

### 3. Freeform meal entries denormalize the nutriment snapshot

When the LLM logs via `POST /meals/freeform`, the supplied nutriments are stored on the meal entry itself (the `snapshot_*` columns above), not just on a generated product. This is true even when `save_as_product=true` and we *do* create a product alongside.

Why: future edits to the product (correcting the LLM's estimate, say) must not silently rewrite history. The summary for last Tuesday must compute from what was eaten on last Tuesday, not from today's best estimate.

A meal entry's effective nutriments are: `coalesce(snapshot_*, product.*_per_100g)`. The query is column-by-column coalesce in SQL.

**Alternatives considered:**
- *Only ever reference the live product.* Rejected: violates "the past should not change under your feet."
- *Store a versioned snapshot of the entire product row in a separate table.* Rejected: too heavy for v1, and the column-per-nutriment snapshot inline is enough.

### 4. Flow B for the barcode path (`lookup` then `log`), one-shot for freeform

Mobile barcode write is a two-call sequence: `POST /products/lookup/{barcode}` first, then `POST /meals` with the returned `product_id`. The lookup is idempotent and read-mostly; the meal write happens only after the user confirms the product and picks a portion. This handles barcode misreads, OFF returning the wrong variant, and the "I'd like to log this retroactively" case cleanly.

The freeform LLM write is a single call: `POST /meals/freeform` takes name + nutriments + quantity + time and logs in one step. The LLM already has the user's confirmation as part of its prompt loop — a separate "preview" round-trip would just waste tokens.

**Alternatives considered:**
- *One-shot `POST /meals {barcode, quantity_g}` for mobile.* Rejected: poor UX when OFF doesn't know the barcode, when scans misread, and when logging retroactively. Saves one round-trip at the cost of every edge case being awkward.

### 5. Open Food Facts integration: cache aggressively, store raw, fail actionably

The OFF client lives in `internal/off/`. Behavior contract:

- First lookup for a barcode: `GET https://world.openfoodfacts.org/api/v2/product/{barcode}.json` with `User-Agent: nutrition-api/<version> (+contact)`, `OFF_TIMEOUT_SECONDS` (default 5) timeout.
- Successful response (`status:1`) → parse the subset of fields we care about into typed columns, store the entire response JSON in `off_payload`, set `fetched_at = now()`. Return the product row.
- `{status:0}` → return `404 product_not_found` with body `{"error":"product_not_found","barcode":"…","next":"POST /meals/freeform"}`. The LLM agent reads `next` and switches paths; the mobile app shows "we don't know this product" with a manual-entry option.
- Timeout / 5xx from OFF → return `504 upstream_timeout` with `{"error":"upstream_timeout","retry_after_seconds":30}`. Do not cache failures.
- Subsequent lookup for a cached barcode → serve from `products` immediately, do not re-hit OFF. `?refresh=true` forces a re-fetch and updates the parsed columns and `off_payload`.

OFF's JSON is messy: nutriments may be missing, present in kJ instead of kcal, named inconsistently across product versions, and `serving_size` is free text like `"30g"`, `"≈ 2 slices"`, or `"1 cup (240ml)"`. Parsing strategy:

- Pull the canonical kcal value with the OFF-documented preference (`energy-kcal_100g` first, then derive from `energy_100g` / 4.184 if only kJ is present, else null).
- Each macro is parsed independently — missing fields become null, not zero.
- `serving_size` is parsed best-effort with a tolerant regex (leading number + unit). On parse failure: log a warning, leave `serving_size_g` null, keep the raw text reachable via `off_payload`.
- The raw `off_payload` is the source of truth for anything we missed. When we discover a new edge case, we can re-derive without hitting OFF.

**Alternatives considered:**
- *Parse OFF strictly and reject products with missing fields.* Rejected: that would reject most of OFF.
- *Don't store the raw payload.* Rejected: we will absolutely find OFF fields we wish we had captured, and re-fetching at that point is wasteful.

### 6. Bearer auth with two static tokens, audit-logged client identity

Single-user system, two clients. Two env vars: `MOBILE_API_TOKEN`, `AGENT_API_TOKEN`. The auth middleware accepts either as `Authorization: Bearer <token>`, sets a request context value `client = "mobile" | "agent"`, and rejects everything else with `401`.

v1 does not branch authorization on client — both clients can call any endpoint. The client tag is logged on every request so we can review activity. Splitting tokens now means we can rotate or revoke independently when (not if) the agent's token ends up somewhere it shouldn't.

**Alternatives considered:**
- *One shared token.* Rejected: rotating one means breaking the other.
- *DB-backed `api_keys` table.* Rejected for v1 — overkill for two keys; revisit if we need fine-grained scopes.

### 7. Idempotency keys for write endpoints

Every write endpoint (`POST /products`, `POST /products/lookup/{barcode}`, `POST /meals`, `POST /meals/freeform`, `PATCH /meals/{id}`, `DELETE /meals/{id}`) accepts an optional `Idempotency-Key` header. The middleware in `internal/idempotency/`:

- Computes `(client_id, http_method, path, idempotency_key)` as a composite key.
- On first arrival, runs the handler, stores `(key → http_status, response_body, created_at)` in an `idempotency_records` table.
- On replay within `IDEMPOTENCY_TTL_HOURS` (default 24), returns the stored response without re-running the handler.
- After TTL: the record is purged by a periodic cleanup (simple `DELETE … WHERE created_at < now() - interval` run on startup and via a slow background tick).

A request that supplies the same key but a different body returns `409 idempotency_key_conflict` — guards against a buggy retry-with-different-input.

**Alternatives considered:**
- *Skip idempotency for v1, rely on clients not retrying.* Rejected: the LLM agent will retry on transient errors, and double-logging breakfast is a silently wrong daily summary, which is the worst failure mode for a nutrition log.
- *Stripe-style header processing at the middleware layer for all endpoints, including reads.* Rejected: reads are naturally idempotent, no value added.

### 8. Time and timezone

All `timestamptz` columns are stored UTC. Summary endpoints accept a `tz` query parameter (IANA name, e.g. `Europe/Berlin`) and compute the day window in that timezone. The default if omitted is `DEFAULT_USER_TZ` from env.

`logged_at` is always client-supplied (mobile knows the user's local moment, the LLM knows what time the user said they ate). The server validates it as a parseable RFC 3339 timestamp and rejects logged_at more than 24h in the future as a sanity check.

**Alternatives considered:**
- *Store `logged_at` as local time + tz column.* Rejected: pushes timezone math into every query. UTC + a single `tz` param at read time is the boring correct answer.

### 9. Project layout

```
cmd/api/                 main, wiring, server lifecycle
internal/products/       repo, service, handlers
internal/meals/          repo, service, handlers
internal/summary/        service, handlers (read-only over meals + products)
internal/off/            HTTP client + JSON fixtures for tests
internal/auth/           bearer middleware, client-id context value
internal/idempotency/    middleware, repo, cleanup
internal/store/          shared pgx pool wiring, transaction helpers
migrations/              golang-migrate SQL files (NNN_*.up.sql / .down.sql)
testdata/off/            recorded OFF JSON fixtures keyed by barcode
```

The `internal/*/` packages each own their HTTP handlers, business logic, and repo. No shared "models" package — types live with their owner package. The `internal/store/` package exposes the pgx pool and a `WithTx` helper; everything else takes a `Querier` (`*pgxpool.Pool | pgx.Tx`).

### 10. Testing strategy

- **Unit-light, integration-heavy.** Single-user toy backend, so the integration tests are the spec.
- Postgres for tests runs in a testcontainers-managed container. The pool is rebuilt per package; migrations run once.
- The OFF client is tested against `testdata/off/*.json` recorded from real responses (good, missing-fields, kJ-only, malformed serving_size, status:0). No live HTTP in CI.
- Handler-level tests use `httptest` + a fresh DB per test class, asserting on full JSON responses (golden-style with normalised timestamps).

## Risks / Trade-offs

- **Stale OFF cache.** Cached products will not reflect upstream OFF corrections. *Mitigation:* `?refresh=true` on lookup forces a re-fetch; the raw payload is also kept, so we can re-extract fields without going back to the network.
- **Freeform LLM estimates can be wildly wrong.** A hallucinated 200 kcal banana ruins the day's totals. *Mitigation:* `PATCH /meals/{id}` lets the user correct after the fact; the snapshot columns mean an after-the-fact product correction does not change history.
- **Idempotency table grows unbounded.** *Mitigation:* TTL purge on startup and via slow background tick. Indexed on `(client_id, key)` for lookups and `(created_at)` for cleanup.
- **Two static tokens leaked is two compromises.** *Mitigation:* env-set tokens, never logged in plaintext; rotation is restart-with-new-env. If this becomes a problem, swap in the `api_keys` table later — the auth middleware is the only thing that needs to change.
- **OFF schema drift.** OFF changes field names over time. *Mitigation:* parse defensively, null on miss, keep raw payload. We'll see the drift in `off_payload` before our typed columns silently degrade.
- **Timezone foot-gun.** `tz` defaulting to env means a request that meant "today in Berlin" while the user is in Tokyo gives subtly wrong sums. *Mitigation:* both clients are expected to always pass an explicit `tz`; the env default exists for ad-hoc curl, not normal traffic. Log a warning when the default is used.

## Migration Plan

- This is a greenfield change. Initial migrations create `products`, `meal_entries`, `idempotency_records` from empty.
- Rollback strategy: `golang-migrate down` to baseline. For v1 there is no production data to preserve — the rollback is destructive, by definition acceptable.

## Open Questions

- Do we want a structured `OFF_USER_AGENT_CONTACT` env var so the OFF client identifies itself politely with a contact? Recommended yes, but trivial to add. Defaulting to `nutrition-api/0.1.0 (+https://github.com/vinzenzs/nutrition-api)` is fine for now.
- Background cleanup of expired idempotency records: simple `time.Ticker` in the API process for v1 vs. a separate worker later. Starting with in-process is fine; revisit if the API process becomes multi-instance.
