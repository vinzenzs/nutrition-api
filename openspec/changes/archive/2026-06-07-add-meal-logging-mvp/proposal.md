## Why

There is no backend yet for logging what the user eats. The user wants a personal nutrition log that they can write to from two clients — a mobile app (for barcode scans and quick entry) and, later, an LLM agent doing food coaching — and read back as daily and ranged summaries. Open Food Facts gives us product data for barcodes for free, so a thin Go service over Postgres is enough to ship a useful v1 without external dependencies beyond OFF.

## What Changes

- Add a Go + Gin HTTP service backed by Postgres (driver: `pgx`, migrations: `golang-migrate`).
- Persist two distinct concepts: `products` (reusable food/product definitions, cached from OFF or manually created) and `meal_entries` (logged eating events). Grams are the canonical unit — clients convert servings/units before sending.
- Lookup-and-cache integration with Open Food Facts: `POST /products/lookup/{barcode}` fetches from `https://world.openfoodfacts.org/api/v2/product/{barcode}.json` on first scan, caches the parsed product plus the raw JSON payload, serves subsequent scans from cache.
- "Flow B" barcode write path: client looks up the product, confirms with the user, then logs the meal entry separately (`POST /meals` with `{product_id, quantity_g, logged_at, …}`).
- LLM-friendly freeform write path: `POST /meals/freeform` accepts `{name, nutriments_per_100g, quantity_g, logged_at, save_as_product?}` so the agent can log meals the user describes in natural language by supplying its own structured nutriment estimate. Snapshot is denormalized onto the meal entry so historical totals remain stable.
- Read endpoints: `GET /products/{id}`, `GET /products/search?q=…` (ranked by `last_logged_at` so recent foods surface first — the agent's recall path), `GET /meals?from=…&to=…&meal_type=…`, `GET /summary/daily?date=…&tz=…`, `GET /summary/range?from=…&to=…&tz=…`. Summaries compute the day window in a user-supplied IANA timezone over UTC-stored timestamps.
- Edit/delete: `PATCH /meals/{id}` and `DELETE /meals/{id}`.
- Auth: two static bearer tokens (`MOBILE_API_TOKEN`, `AGENT_API_TOKEN`) from env. Middleware tags each request with the client identity for audit logging; v1 does not branch authorization on client.
- Idempotency: every write endpoint accepts an optional `Idempotency-Key` header. The key + response are stored for ~24h so retries (especially from the LLM agent) return the original response instead of double-logging.
- OFF resilience contract: `status:0` returns `404 product_not_found` with an actionable `next` hint pointing the agent to the freeform endpoint; timeouts return `504` with a retry hint; partial/messy OFF payloads (missing fields, kJ-only, free-text serving sizes) store what is parseable and null the rest, keeping the raw JSON for later re-extraction.

## Capabilities

### New Capabilities
- `products`: Reusable food/product definitions. Barcode lookup against Open Food Facts with local caching, manual product creation, and name/brand search ranked by recency of use.
- `meals`: Logged eating events. CRUD over meal entries, freeform (LLM-friendly) write path, and daily/range summaries computed in a user-supplied timezone.
- `off-integration`: Resilience contract for the Open Food Facts client — not-found handling, timeouts, partial-schema tolerance, and raw payload retention.
- `auth`: Two-token bearer authentication and idempotency-key handling for write endpoints.

### Modified Capabilities
<!-- None — this is the first change in the repo. -->

## Impact

- **New code**: `cmd/api/`, `internal/products/`, `internal/meals/`, `internal/summary/`, `internal/off/`, `internal/auth/`, `internal/idempotency/`, `migrations/`.
- **New dependencies**: `github.com/gin-gonic/gin`, `github.com/jackc/pgx/v5`, `github.com/golang-migrate/migrate/v4`, `github.com/testcontainers/testcontainers-go` (test-only).
- **External services**: Open Food Facts API (`world.openfoodfacts.org`). No auth required, generous rate limits for personal use, cached aggressively.
- **Configuration (env)**: `DATABASE_URL`, `MOBILE_API_TOKEN`, `AGENT_API_TOKEN`, `DEFAULT_USER_TZ`, `OFF_TIMEOUT_SECONDS` (default `5`), `IDEMPOTENCY_TTL_HOURS` (default `24`).
- **Out of scope (separate follow-up changes)**: MCP server wrapping this REST API for the LLM agent; trends/coaching endpoints; OFF text search (the freeform endpoint covers the LLM's use case); multi-user; web UI.
