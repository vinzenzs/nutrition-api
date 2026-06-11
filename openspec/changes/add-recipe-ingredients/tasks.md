# Tasks: add-recipe-ingredients

## 1. Storage & products surface

- [x] 1.1 Check current migration head, then `task migrate:new NAME=add_product_ingredients` — nullable `ingredients jsonb` on `products`, down drops it
- [x] 1.2 Add `Ingredients []string` to `internal/products` types with `omitempty` JSON tag; thread through repo create/read/list scans
- [x] 1.3 Service validation: ≤100 entries, each non-empty ≤500 chars, `source=recipe` required → sentinel error mapping to `400 ingredients_require_recipe_source`
- [x] 1.4 Accept `ingredients` on `POST /products`; return on all product reads; handler tests for round-trip, rejection on non-recipe source, oversized array, omission when null

## 2. Cookidoo fetch/parse client

- [x] 2.1 New `internal/cookidoo` package: client with injected `http.Client`, configurable timeout, `User-Agent: nutrition-cookidoo/<version>`
- [x] 2.2 URL validation (host `cookidoo.<tld>`, path `/recipes/recipe/<locale>/<id>`) with `ErrNotCookidooURL`; no redirects off the allowlisted host
- [x] 2.3 JSON-LD extraction: find `<script type="application/ld+json">` blocks, parse, select `@type=Recipe` (string or array form); typed `ErrNoRecipeJSONLD` / `ErrFetchFailed`
- [x] 2.4 Defensive field parsing: name, `recipeIngredient`, per-serving nutrition (tolerate `,`/`.` decimals and unit suffixes like "589 kcal"), yield leading-integer, ISO-8601 `totalTime`; unparseable fields become null
- [x] 2.5 Unit tests against fixture HTML captured from the spike (happy path, no-JSON-LD page, malformed nutrition strings)

## 3. Import endpoint

- [x] 3.1 `POST /products/import/cookidoo` handler in `internal/products` with swag annotations; wire cookidoo client in `internal/httpserver`
- [x] 3.2 Conversion path: with `serving_size_g` → per-100g nutriments; without → create sans nutriments, respond `needs_nutriments: true` + `nutrition_per_serving` echo
- [x] 3.3 Idempotent-ensure on existing `external_url`: `200` + `already_imported: true`, existing product untouched
- [x] 3.4 Error mapping: `400 invalid_cookidoo_url`, `502 cookidoo_unavailable` (reason distinguishes fetch vs parse)
- [x] 3.5 Handler integration tests (testcontainers) incl. duplicate-import and no-outbound-call-on-invalid-URL (assert via stub client)

## 4. MCP tool

- [x] 4.1 Register `import_cookidoo_recipe` in `internal/mcpserver` (one HTTP call, auto idempotency key, description per spec)
- [x] 4.2 Bump expected-tools list in `mcp_integration_test.go`

## 5. Extension & docs

- [x] 5.1 Extension content script captures `recipeIngredient`; popup shows read-only count; save body includes `ingredients`
- [x] 5.2 `task swag` regenerate; `task vet` + full `task test` green
