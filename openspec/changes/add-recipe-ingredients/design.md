# Design: add-recipe-ingredients

## Context

Cookidoo recipes enter the system today via the Chrome MV3 extension as flat-imported `source=recipe` products (`external_url` set, no components, `nutriment_computed_at` null — an established spec shape). The extension reads Schema.org `Recipe` JSON-LD but only maps name/servings/nutriments; `recipeIngredient` is discarded. A 2026-06-11 spike against `cookidoo.de/recipes/recipe/de-DE/r386806` confirmed anonymous HTTP GET returns the full JSON-LD: `name`, 20 `recipeIngredient` strings, `nutrition` (per-serving), `recipeYield` ("6 Portionen"), `totalTime` (ISO-8601 duration). The upcoming chat agent needs a deterministic, server-side way to pull a Cookidoo recipe into the product library mid-conversation.

## Goals / Non-Goals

**Goals:**
- Persist ingredient strings on recipe products, verbatim and ordered.
- One-call server-side import: Cookidoo URL in, recipe product out.
- Extension parity: future extension imports also carry ingredients.
- Honest nutriment semantics: never fabricate per-100g values from per-serving data.

**Non-Goals:**
- No ingredient→product linking or quantity parsing ("100 g Staudensellerie" stays one string). The shopping-list agent does interpretation; the API records primitives.
- No backfill job (the recipe library is currently empty).
- No generic recipe-site importer — Cookidoo URL patterns only.
- No scraping of login-gated content (cooking instructions); JSON-LD on the public page is the entire contract.

## Decisions

### D1: Ingredients as `jsonb` array of strings on `products`

A nullable `ingredients jsonb` column holding `["1 Zwiebel", ...]`. Alternatives: (a) `product_components` rows — wrong, components reference other products and drive the recompute pipeline; ingredient strings are display/shopping data, not nutrition inputs; (b) separate `product_ingredients` table — over-normalized for an ordered string list with no per-row semantics; (c) `text[]` — viable, but jsonb matches how other optional structured blobs would evolve and round-trips through `omitempty` JSON cleanly. Validation: max 100 entries, each non-empty ≤ 500 chars, only permitted when `source=recipe` (400 `ingredients_require_recipe_source` otherwise).

### D2: Import endpoint lives in `internal/products`, fetch/parse client is its own package

`POST /products/import/cookidoo` registers under the products route group (it creates a product; the products handler owns product creation invariants). The outbound fetch + JSON-LD extraction goes in `internal/cookidoo` — same pattern as `internal/off`: small client struct, injected `http.Client`, typed sentinel errors (`ErrNotCookidooURL`, `ErrFetchFailed`, `ErrNoRecipeJSONLD`), a `User-Agent` of the form `nutrition-cookidoo/<version>`, and a configurable timeout. URL validation: host must match `cookidoo.<tld>` and path must match the recipe pattern (`/recipes/recipe/<locale>/<id>`) before any outbound call.

### D3: Per-serving → per-100g conversion is caller-supplied or skipped

Cookidoo `nutrition` is per portion with no mass. The endpoint takes optional `serving_size_g`; when present, per-100g nutriments = per-serving × 100 / serving_size_g, and `serving_size_g` is stored. When absent, the product is created **without nutriments** and the response carries `nutrition_per_serving` (echoed from JSON-LD) plus `needs_nutriments: true` so the caller (chat agent or human) can follow up with the existing `PATCH /products/{id}`. Alternative rejected: server-side mass estimation — guessing inside a deterministic API endpoint violates "agent does synthesis, API records primitives".

### D4: Duplicate handling by `external_url`

If a product already exists with the same `external_url`, the import returns `200` with the existing product and `already_imported: true` instead of creating a duplicate (and does NOT overwrite manual corrections). Alternative — 409 conflict — rejected: the agent's natural flow is "ensure this recipe is in the library", which is idempotent-ensure, not strict-create.

### D5: Extension sends `ingredients` through the existing flat-import path

`POST /products` accepts the new optional `ingredients` array (D1 validation applies). The extension popup shows the parsed ingredient count (read-only summary, not 20 editable fields) and includes the array on save.

## Risks / Trade-offs

- [Cookidoo markup changes or starts blocking server IPs] → the typed `ErrNoRecipeJSONLD`/`ErrFetchFailed` map to a 502 `cookidoo_unavailable` with the reason; the extension path still works as fallback since it reads the user's own browser session.
- [German locale strings ("6 Portionen", comma decimals) parse brittlely] → parse defensively: extract leading integer from yield, tolerate `,`/`.` decimals in nutrition strings; anything unparseable becomes null rather than an error.
- [Verbatim ingredient strings are language-mixed and unnormalized] → accepted; normalization is explicitly the consuming agent's job.
- [SSRF surface on a URL-taking endpoint] → strict host/path allowlist (D2) before any fetch; no redirects followed off `cookidoo.<tld>`.

## Migration Plan

One additive nullable column; `.down.sql` drops it. No data migration (library empty). Deploy is a plain binary swap; rollback safe because the column is optional everywhere.

## Open Questions

- None blocking. Locale coverage beyond `cookidoo.de` (`.at`, `.ch`, `.com`) is handled by the host pattern but only `.de` is spike-verified.
