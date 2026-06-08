## Why

Cookidoo (Vorwerk's Thermomix recipe platform) has no public API, but its recipe pages embed Schema.org `Recipe` JSON-LD for SEO — the same standard most major recipe sites use. The user cooks from Cookidoo regularly and wants logged meals to reflect those recipes without retyping macros. A small Chrome extension can read the JSON-LD from any open Cookidoo recipe page (in the user's already-authenticated session) and POST the parsed recipe into nutrition-api as a `source=recipe` product with its source URL preserved. The user then logs servings against it from the mobile app or the LLM agent like any other product. The whole integration is one Chrome extension, one new nullable column, and a small extension to `POST /products`.

## What Changes

- **Backend: extend products with `external_url`.**
  - New migration adds `products.external_url TEXT NULL`.
  - `Product.ExternalURL *string` is added to the Go type and serialized JSON shape.
  - `selectAllColumns`, `Insert`, and the (existing) `UpdateFromOFF` / `UpdateRecipeNutriments` repository methods are updated to round-trip the new column (the OFF and recompute paths leave it untouched).
- **Backend: `POST /products` accepts `source` and `external_url`.**
  - Request body grows two optional fields: `source` (one of `manual` | `recipe`, default `manual`) and `external_url` (string, max ~2KB).
  - Server validates `source` against the allow-list and rejects unknown values with `400 source_invalid`.
  - A `source=recipe` product is allowed to be created **flat** (with `nutriments_per_100g` supplied directly and no components) — this is the Cookidoo path. This explicitly complements the existing composed-recipe path (recipe with `product_components` and `nutriment_computed_at` set by the existing recompute pipeline). A flat-imported recipe has `nutriment_computed_at: null`.
  - Response includes `external_url`.
- **Chrome extension at `extensions/cookidoo/`** (Manifest V3).
  - Content script registered on Cookidoo recipe URLs (`https://cookidoo.*/*recipes/recipe/*`) extracts the page's `<script type="application/ld+json">` whose `@type === "Recipe"`. Parses Schema.org fields: `name`, `recipeYield`, `nutrition`, `image`, `url`.
  - Popup (toolbar button) opens a preview dialog showing the parsed name, servings, serving size in grams, and macros — all editable. The user clicks **Save** to POST to nutrition-api.
  - Options page stores: `API base URL`, `API token`, `token type` (mobile / agent — default mobile). Persisted via `chrome.storage.sync`.
  - Conversion: extension computes `nutriments_per_100g` from Schema.org per-serving values using `serving_size_g`. If `servingSize` is not in grams (e.g. "1 cup"), the popup forces the user to enter the serving weight before saving.
- **Documentation.**
  - `extensions/cookidoo/README.md` covers installing the unpacked extension in Chrome, the options needed, and the limits (Chrome only, Cookidoo JSON-LD-dependent, manual fallback).
  - `RUN_LOCAL.md` adds a one-paragraph pointer to the Cookidoo importer for users who want it.

## Capabilities

### New Capabilities
- `cookidoo-importer`: A Chrome MV3 extension that reads Schema.org `Recipe` JSON-LD from Cookidoo recipe pages and saves them to nutrition-api as `source=recipe` products with their Cookidoo URL preserved.

### Modified Capabilities
- `products`: Adds the `external_url` field on products and extends `POST /products` to accept `source` (allow-list: `manual`, `recipe`) and `external_url`. Existing manual-product behaviour is unchanged when neither new field is supplied.

## Impact

- **New code (backend)**:
  - Migration NNN: `ALTER TABLE products ADD COLUMN external_url TEXT NULL;`
  - `Product.ExternalURL *string` (and JSON tag).
  - Insert + Select projection in `internal/products/repo.go` carry the new column.
  - `POST /products` request struct adds `Source *string` and `ExternalURL *string`; service maps to typed `Source` value with validation.
- **New code (extension)** at `extensions/cookidoo/`:
  - `manifest.json` (MV3, host_permissions: `https://cookidoo.*/*`).
  - `content_script.js` — extracts JSON-LD.
  - `popup.html` / `popup.js` — preview + save form.
  - `options.html` / `options.js` — API URL + token + token type.
  - `icons/` — 16/48/128 PNGs.
  - `README.md` — install + options + limits.
- **No changes to**:
  - The meal-logging path, MCP server, OFF integration, summary endpoints, auth, idempotency. The extension hits the existing `POST /products`; logging is unchanged.
  - The existing composed-recipe code (`UpdateRecipeNutriments`, `product_components`). The flat-imported recipe is a distinct row whose nutriments come from Cookidoo, not from components.
- **Dependencies**: none new in Go. The extension is plain JS, no bundler, no npm dependencies — keeps it inspectable.
- **Schema constraints**: the `products.source` check constraint already permits `recipe`, so the migration is the single `external_url` column add.
- **External services**: Cookidoo is consulted only by the user's browser (via the extension content script reading the open page). No server-side traffic.

### Out of scope (explicit non-goals)
- Firefox / Safari / Edge — Chrome only for v1.
- Auto-dedup by `external_url`. If you re-import the same recipe, a second product row is created — manual cleanup is fine.
- Backend scraping of Cookidoo (login, cookies, session) — extension does the read inside the user's authenticated browser session.
- Importing the ingredient list as `product_components`. The flat-import path uses Cookidoo's already-computed nutriments; ingredient parsing is out of scope.
- "Cook this batch and log the first serving now" UX in the extension. Recipe-as-product is enough; the mobile app and the agent already handle per-serving logging.
- Recipe sync — changes upstream on Cookidoo do not propagate back to the saved product.
- Schema-importer generalisation to other recipe sites (Chefkoch, NYT Cooking, BBC GoodFood). The backend already supports any source via the new field; broadening the extension's host_permissions and content-script URL matcher is a follow-up change.
