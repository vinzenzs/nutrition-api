## 1. Backend: schema + types

- [x] 1.1 Add a new migration `NNN_add_products_external_url.up.sql` / `.down.sql` that ALTERs `products` to add `external_url TEXT NULL`. The `down` migration drops the column.
- [x] 1.2 Update `internal/products/types.go` `Product` struct to add `ExternalURL *string` with the JSON tag `external_url,omitempty`. Keep the existing field order; the new field sits next to `Brand` so the serialised shape is intuitive.
- [x] 1.3 Update `internal/products/repo.go` `selectAllColumns` to include `external_url`. Update the `Insert` SQL and its argument list to round-trip the column (add a `$28` placeholder; verify all the renumbering — pay attention to the existing macros + micros block). Update `scanProduct` to scan into `&p.ExternalURL`.
- [x] 1.4 Verify that neither `UpdateFromOFF` nor `UpdateRecipeNutriments` touches `external_url`. OFF re-fetches must not clobber a (hypothetical) externally-set URL on an OFF product; recipe recompute must not clobber a Cookidoo URL on a recipe row.

## 2. Backend: handler validation

- [x] 2.1 Extend `internal/products/handlers.go` `createManualRequest` (consider renaming it `createProductRequest` to drop the "manual" implication, but keep it for v1 if it's mechanical) with two optional fields: `Source *string` and `ExternalURL *string`.
- [x] 2.2 In the handler, validate `Source`:
  - nil or empty → `products.SourceManual`
  - `"manual"` → `products.SourceManual`
  - `"recipe"` → `products.SourceRecipe`
  - anything else → `400 source_invalid`
- [x] 2.3 Validate `ExternalURL`:
  - nil → leave as nil
  - non-nil → reject if length > 2048 with `400 external_url_too_long`
  - non-nil → trim whitespace; reject if the trimmed value is empty after trim with `400 external_url_invalid`
- [x] 2.4 Plumb both fields into the service-layer `CreateManualInput` (rename the type if mechanical — `CreateProductInput`) and onward into the `Product` struct on insert.
- [x] 2.5 Update the swagger annotations on `createManual` (now also handles recipes) to document the two new fields.

## 3. Backend: tests

- [x] 3.1 Add a handler test: POST a recipe product with `source=recipe` + `external_url` succeeds, response includes `external_url` and `source: "recipe"`, GET-by-id returns the same.
- [x] 3.2 Add a handler test: POST with `source=cookidoo` is rejected with `400 source_invalid`.
- [x] 3.3 Add a handler test: POST with `external_url` 2049 chars long is rejected with `400 external_url_too_long`.
- [x] 3.4 Add a handler test: POST with `external_url: ""` after trim is rejected with `400 external_url_invalid`.
- [x] 3.5 Add a handler test: existing manual-product POST (no `source`, no `external_url`) still works and produces `source: "manual"`, no `external_url`.
- [x] 3.6 Add a handler test: search returns the imported recipe by name and includes `external_url` in the result.
- [x] 3.7 `go vet ./...` clean; `go test ./...` green.

## 4. Chrome extension scaffold

- [x] 4.1 Create `extensions/cookidoo/` directory.
- [x] 4.2 Write `manifest.json` (MV3) with:
  - `manifest_version: 3`
  - `name: "nutrition-api: Cookidoo importer"`, `version: "0.1.0"`
  - `host_permissions: ["https://cookidoo.*/*", "https://*.cookidoo.*/*"]`
  - `permissions: ["storage", "activeTab"]`
  - `action: { default_popup: "popup.html", default_icon: { 16: ..., 48: ..., 128: ... } }`
  - `options_page: "options.html"`
  - `content_scripts: [{ matches: ["https://cookidoo.*/*recipes/recipe/*", "https://*.cookidoo.*/*recipes/recipe/*"], js: ["content_script.js"], run_at: "document_idle" }]`
- [x] 4.3 Add three placeholder PNG icons under `extensions/cookidoo/icons/` at 16, 48, and 128 px (a simple letter or fork glyph is fine for v1).

## 5. Content script: JSON-LD extraction

- [x] 5.1 `content_script.js`: query all `<script type="application/ld+json">` elements; for each, try `JSON.parse` (skip on parse error). Find the first node whose `@type === "Recipe"` or whose `@type` is an array containing `"Recipe"`. If a top-level `@graph` exists, walk it for a Recipe entry.
- [x] 5.2 Normalise the recipe into a shape the popup expects:
  ```
  { name, image, url, servings, servingSizeG, nutrimentsPerServing, parseWarnings }
  ```
  - `servings` = parse leading integer from `recipeYield` (Cookidoo formats vary — handle both `"4"` and `"4 servings"`).
  - `servingSizeG` = parse gram quantity from `nutrition.servingSize` if it ends in `"g"`; else null.
  - `nutrimentsPerServing` = parse `calories` (kcal — convert from kJ if needed), `proteinContent`, `carbohydrateContent`, `fatContent`, `fiberContent`, `sugarContent`, `sodiumContent` (convert mg→g salt via `salt_g = sodium_mg × 2.5 / 1000`).
  - `parseWarnings` collects strings for any field we couldn't parse, shown in the popup.
- [x] 5.3 Persist the normalised recipe to `chrome.storage.session` keyed by tab id (so the popup can read it without re-running parsing).
- [x] 5.4 Listen for `chrome.runtime` messages from the popup (`{ type: "get_recipe", tabId }`) and respond with the stored object — needed for popups that open after a SPA route change.

## 6. Popup: preview + save

- [x] 6.1 `popup.html`: form with name, servings (number), serving_size_g (number, required), and per-100g fields for every nutriment column the backend supports (macros + the eight micros from `daily-use-essentials`). A Save button (disabled until serving_size_g > 0 and token is configured). A "no recipe detected" banner shown when applicable. An error region for backend / network errors.
- [x] 6.2 `popup.js` on load:
  - Read options from `chrome.storage.sync`; if `token` is empty, disable Save and show "configure options" with a link to `chrome.runtime.openOptionsPage()`.
  - Get the current tab id; ask the content script for its stored recipe via `chrome.tabs.sendMessage`.
  - Populate form fields from the recipe, leaving any unparsed fields blank.
  - Auto-compute per-100g previews from per-serving × (100 / serving_size_g) when both serving_size_g and a per-serving value are present; the user can override the auto-computed value in the form.
- [x] 6.3 `popup.js` on Save:
  - Build the POST body: `name`, `source: "recipe"`, `external_url: <tab.url>`, `serving_size_g: <form>`, `nutriments_per_100g: { only the fields the user filled in }`.
  - Issue `fetch(API_URL + "/products", { method: "POST", headers, body })`.
  - On 2xx: show success state with a link to the created product (`<API_URL>/products/<id>`), Save button replaced by Close.
  - On non-2xx: render `${response.status} ${response.body}` in the error region; leave form editable.
  - On network failure: render `could not reach ${API_URL}` with the error string; leave form editable.

## 7. Options page

- [x] 7.1 `options.html`: API URL input (text, default `http://localhost:8080`), Token input (password), Token type radio (`mobile` default, `agent`), Save button.
- [x] 7.2 `options.js`:
  - On load: read existing values from `chrome.storage.sync`; populate the form.
  - On Save: validate that API URL is a syntactically valid URL, that Token is non-empty, then persist to `chrome.storage.sync`.
  - Surface a "saved" toast for 2 seconds.

## 8. Documentation

- [x] 8.1 Write `extensions/cookidoo/README.md` covering:
  - Limits up front (Chrome only, JSON-LD dependent, no auto-sync, no Cookidoo login).
  - Install (Chrome → `chrome://extensions` → Developer mode → Load unpacked → pick the `extensions/cookidoo/` directory).
  - Options walkthrough (URL + token + token type).
  - "Try it" recipe URL and the expected flow.
- [x] 8.2 Add a one-paragraph pointer in `RUN_LOCAL.md`'s "Other useful tasks" section linking to `extensions/cookidoo/README.md`.

## 9. Manual end-to-end check

- [x] 9.1 With `task dev` running and the extension loaded, open a Cookidoo recipe page, click the toolbar button, verify the popup pre-fills, click Save, then `curl -H "Authorization: Bearer $MOBILE_API_TOKEN" http://localhost:8080/products/search?q=<recipe name>` and confirm the imported recipe is returned with `source: "recipe"` and the correct `external_url`.
- [x] 9.2 Log a meal against the imported product via the existing mobile or agent path; confirm the meal's effective nutriments reflect the imported recipe's per-100g values.

## 10. Pre-merge checks

- [x] 10.1 `go vet ./...` clean.
- [x] 10.2 `go test ./...` green.
- [x] 10.3 `task --list` still shows every documented target (no regressions in Taskfile).
- [x] 10.4 `git status` clean of extension build artifacts — confirm `.gitignore` does not need new entries (no compiled artifacts from the extension since it ships unbundled).
