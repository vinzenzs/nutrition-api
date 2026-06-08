## 1. Project skeleton

- [x] 1.1 Initialize `go mod init github.com/vinzenzs/nutrition-api`
- [x] 1.2 Add dependencies: `github.com/gin-gonic/gin`, `github.com/jackc/pgx/v5`, `github.com/jackc/pgx/v5/pgxpool`, `github.com/golang-migrate/migrate/v4` (with `database/postgres` and `source/file` drivers), `github.com/google/uuid`
- [x] 1.3 Add test-only dependencies: `github.com/testcontainers/testcontainers-go`, `github.com/testcontainers/testcontainers-go/modules/postgres`, `github.com/stretchr/testify`
- [x] 1.4 Create directory skeleton: `cmd/api/`, `internal/{products,meals,summary,off,auth,idempotency,store}/`, `migrations/`, `testdata/off/`
- [x] 1.5 Add `.env.example` documenting `DATABASE_URL`, `MOBILE_API_TOKEN`, `AGENT_API_TOKEN`, `DEFAULT_USER_TZ`, `OFF_TIMEOUT_SECONDS`, `OFF_USER_AGENT_CONTACT`, `IDEMPOTENCY_TTL_HOURS`
- [x] 1.6 Add a minimal `Makefile` with `run`, `test`, `migrate-up`, `migrate-down`, `migrate-new NAME=…` targets

## 2. Database migrations

- [x] 2.1 `001_create_products.up.sql` / `.down.sql`: `products` table with columns from design (id uuid pk, barcode text unique nullable, name, brand, source enum-as-text with check constraint, per-100g nutriment columns nullable numeric, serving_size_g, off_payload jsonb, fetched_at, last_logged_at, created_at, updated_at)
- [x] 2.2 `002_create_meal_entries.up.sql` / `.down.sql`: `meal_entries` table (id, product_id fk nullable, logged_at timestamptz, quantity_g numeric, meal_type text nullable with check constraint, note, snapshot_name + snapshot_*_per_100g nullables, created_at, updated_at)
- [x] 2.3 `003_create_idempotency_records.up.sql` / `.down.sql`: `idempotency_records` table (composite pk on (client_id, method, path, key), status, response_body jsonb, request_body_hash, created_at) with index on `created_at` for cleanup
- [x] 2.4 `004_create_indexes.up.sql` / `.down.sql`: products(name) trigram or simple lower(name) index for search; products(last_logged_at desc); meal_entries(logged_at)
- [x] 2.5 Wire migrations to run on app startup behind a `MIGRATE_ON_START` flag (default true in dev, false in prod)

## 3. Shared store wiring

- [x] 3.1 `internal/store/pool.go`: build `*pgxpool.Pool` from `DATABASE_URL`, with sensible pool settings
- [x] 3.2 `internal/store/tx.go`: `WithTx(ctx, pool, fn func(pgx.Tx) error) error` helper that commits on nil return and rolls back on error
- [x] 3.3 Define a `Querier` interface (`Exec`, `Query`, `QueryRow`) satisfied by both `*pgxpool.Pool` and `pgx.Tx` so repos accept either
- [x] 3.4 Testcontainers helper in `internal/store/storetest/` that boots a Postgres container, runs migrations, returns a pool, and cleans up

## 4. Auth middleware

- [x] 4.1 `internal/auth/config.go`: load + validate `MOBILE_API_TOKEN` and `AGENT_API_TOKEN` at startup (non-empty, ≥16 bytes, distinct from each other)
- [x] 4.2 `internal/auth/middleware.go`: Gin middleware that reads `Authorization: Bearer <token>`, matches against the two tokens via constant-time compare, sets `client_id` in `gin.Context`, and rejects with `401 auth_required` / `401 auth_invalid`
- [x] 4.3 Ensure request logging redacts the raw token and emits the resolved `client_id` (handled in request log middleware in section 10; auth middleware never logs the raw token)
- [x] 4.4 Tests: valid mobile token sets context; valid agent token sets context; missing header → 401; wrong scheme → 401; unknown token → 401; startup with missing token panics; startup with equal tokens panics

## 5. Idempotency middleware

- [x] 5.1 `internal/idempotency/repo.go`: insert / read / delete records keyed on `(client_id, method, path, key)`
- [x] 5.2 `internal/idempotency/middleware.go`: Gin middleware that, on a write request with `Idempotency-Key`, computes a SHA-256 hash of the request body, looks up an existing record, replays its stored response on match, returns `409 idempotency_key_conflict` on body-hash mismatch, otherwise runs the chain and stores the response on success
- [x] 5.3 Ensure the middleware is mounted AFTER auth so unauthenticated requests cannot replay
- [x] 5.4 Ensure the middleware ignores `Idempotency-Key` on `GET`
- [x] 5.5 Startup task: register a `time.Ticker` (every 15 minutes) that calls a cleanup query removing records older than `IDEMPOTENCY_TTL_HOURS`; also run cleanup once at startup
- [x] 5.6 Tests: first request stores + returns response; replay returns stored response; conflicting body → 409; different `client_id` with same key → independent; expired record is treated as first-arrival; GET ignored

## 6. Open Food Facts client

- [x] 6.1 `internal/off/client.go`: typed `Client` with `Fetch(ctx context.Context, barcode string) (*Product, error)` against `https://world.openfoodfacts.org/api/v2/product/{barcode}.json`, configurable timeout, `User-Agent: nutrition-api/<version> (+<contact>)`
- [x] 6.2 `internal/off/parse.go`: parse JSON into a typed product struct. Handle: kcal preference (`energy-kcal_100g`, derive from `energy_100g/4.184` if only kJ), nullable nutriments, best-effort `serving_size` regex parse, raw payload retained alongside parsed fields
- [x] 6.3 Error taxonomy: `ErrProductNotFound`, `ErrUpstreamTimeout`, `ErrUpstreamServerError`, `ErrUpstreamUnexpected` (with status code)
- [x] 6.4 Record fixtures in `testdata/off/`: `3017624010701.json` (Nutella, fully populated), `missing_nutriments.json`, `kj_only.json`, `unparseable_serving_size.json`, `not_found.json` (status:0)
- [x] 6.5 Tests using fixtures via a stubbed `http.RoundTripper`: parse correctness, kJ→kcal conversion, missing fields → nil, serving_size parse and tolerated failure, status:0 → ErrProductNotFound, timeout → ErrUpstreamTimeout, 5xx → ErrUpstreamServerError, unexpected 4xx → ErrUpstreamUnexpected
- [x] 6.6 Verify a warning log line is emitted when serving_size cannot be parsed

## 7. Products module

- [x] 7.1 `internal/products/types.go`: `Product` struct mirroring the schema; `Nutriments` value type
- [x] 7.2 `internal/products/repo.go`: `Insert`, `Upsert` (by id for refresh), `GetByID`, `GetByBarcode`, `Search(q string, limit int)`, `TouchLastLoggedAt(id, ts)` (monotonic update with `WHERE last_logged_at IS NULL OR last_logged_at < $2`)
- [x] 7.3 `internal/products/service.go`: `Lookup(barcode, refresh bool)` orchestrates cache-check → OFF fetch → upsert → return; `CreateManual(input)` enforces barcode-uniqueness conflict
- [x] 7.4 `internal/products/handlers.go`: `POST /products/lookup/{barcode}` (returns 404 with `product_not_found` body on `ErrProductNotFound`, 504 on upstream errors, 502 on unexpected 4xx), `POST /products`, `GET /products/{id}`, `GET /products/search?q=…`
- [x] 7.5 Handler tests with a real Postgres (testcontainers) and a stubbed OFF client: cache hit vs miss, refresh, not-found response shape, manual create, duplicate barcode → 409, search ranking by last_logged_at, missing q → 400

## 8. Meals module

- [x] 8.1 `internal/meals/types.go`: `MealEntry` and `EffectiveNutriments` types; `MealType` enum with parse/validate
- [x] 8.2 `internal/meals/repo.go`: `Insert`, `InsertFreeform`, `GetByID`, `List(from, to, mealType)`, `Patch(id, patch)`, `Delete(id)`. Selects include effective nutriments via `coalesce(snapshot_*, products.*)` joins.
- [x] 8.3 `internal/meals/service.go`: validation (quantity_g > 0, logged_at not >24h future, valid meal_type, product_id exists, name required for freeform, nutriments non-negative). Coordinates `products.TouchLastLoggedAt` after insert/patch. `InsertFreeform(input)` creates the product when `save_as_product=true` and links it.
- [x] 8.4 `internal/meals/handlers.go`: `POST /meals`, `POST /meals/freeform`, `GET /meals/{id}`, `GET /meals` (window query), `PATCH /meals/{id}`, `DELETE /meals/{id}`. Error bodies match spec wording exactly (`product_id_required`, `quantity_g_invalid`, `logged_at_too_far_future`, `meal_type_invalid`, `name_required`, `nutriments_invalid` with `field`, `window_required`, `window_invalid`, `meal_not_found`).
- [x] 8.5 Handler tests covering every scenario in `specs/meals/spec.md` against a real Postgres

## 9. Summary module

- [x] 9.1 `internal/summary/service.go`: `Daily(date, tz)` computes the half-open day window in `tz`, runs a single aggregation query over `meal_entries` joined with `products`, returns totals + entries list. `Range(from, to, tz)` iterates over the inclusive date range and aggregates per day in a single SQL pass.
- [x] 9.2 `internal/summary/handlers.go`: `GET /summary/daily?date=…&tz=…` and `GET /summary/range?from=…&to=…&tz=…`. Apply `DEFAULT_USER_TZ` fallback; validate tz via `time.LoadLocation`; reject invalid date format, inverted ranges, and ranges >92 days with the spec's error bodies.
- [x] 9.3 Log a WARN line when the fallback `DEFAULT_USER_TZ` is used (helps catch missing-tz mistakes in clients)
- [x] 9.4 Handler tests against real Postgres: typical day, day spanning DST, day with no entries, range with empty days, range >92 days → 400, invalid tz → 400, invalid date → 400, fallback tz behavior

## 10. Wiring & server lifecycle

- [x] 10.1 `cmd/api/main.go`: load config (env), validate, build pool, run migrations (if enabled), build OFF client, build repos/services, register Gin router with middleware order: recovery → request-log → auth → idempotency → handlers
- [x] 10.2 Graceful shutdown: trap SIGINT/SIGTERM, stop accepting new connections, drain in-flight with a 10s timeout, close pool
- [x] 10.3 Health endpoints (public, no auth): `GET /healthz` returns 200 always; `GET /readyz` pings the DB pool and returns 200/503
- [x] 10.4 Structured logging: one JSON line per request with `client_id`, status, latency, route, idempotency key (hashed, not raw)

## 11. End-to-end happy-path test

- [x] 11.1 Single `e2e_test.go` that boots the full server against a testcontainers Postgres + a stubbed OFF transport, then exercises: lookup new barcode → log meal → daily summary shows it → patch quantity → daily summary updates → freeform log → range summary includes both → idempotent replay of a meal log returns the same id → conflicting-body replay returns 409

## 12. Pre-merge checks

- [x] 12.1 `go vet ./...` clean
- [x] 12.2 `go test ./...` green
- [x] 12.3 README updated with: what this is, how to run (env vars, `make migrate-up && make run`), example curl for each endpoint, link to this change folder
