# cookidoo-importer — delta for add-recipe-ingredients

## ADDED Requirements

### Requirement: Server-side Cookidoo import endpoint creates a recipe product from a URL

The system SHALL expose `POST /products/import/cookidoo` accepting `{url, serving_size_g?}`. The endpoint MUST validate that `url`'s host matches `cookidoo.<tld>` and its path matches the recipe pattern (`/recipes/recipe/<locale>/<id>`) before any outbound request, rejecting anything else with `400 invalid_cookidoo_url`. On success it SHALL fetch the page anonymously, extract the Schema.org `Recipe` JSON-LD, and create a flat-imported `source=recipe` product carrying `name`, `external_url` (the request URL), the `recipeIngredient` strings as `ingredients`, and serving metadata. The outbound client SHALL send a `User-Agent` of the form `nutrition-cookidoo/<version>` and use a configurable timeout.

#### Scenario: Successful import creates a recipe product with ingredients

- **WHEN** the client POSTs `{url: "https://cookidoo.de/recipes/recipe/de-DE/r386806", serving_size_g: 450}`
- **AND** the page's JSON-LD contains a `Recipe` block with `name`, `recipeIngredient`, and per-serving `nutrition`
- **THEN** the response is `201` with a product having `source=recipe`, `external_url` equal to the request URL, the full ordered `ingredients` array, `serving_size_g: 450`, and `nutriments_per_100g` converted from the per-serving values (per-serving × 100 / 450)

#### Scenario: Import without serving_size_g skips nutriments honestly

- **WHEN** the client POSTs only `{url}` for a page whose JSON-LD nutrition is per-serving
- **THEN** the product is created with ingredients and metadata but no `nutriments_per_100g`
- **AND** the response carries `needs_nutriments: true` and a `nutrition_per_serving` echo of the parsed JSON-LD values so the caller can convert and PATCH later

#### Scenario: Non-Cookidoo URL is rejected before any fetch

- **WHEN** the client POSTs `{url: "https://evil.example.com/recipes/recipe/de-DE/r1"}`
- **THEN** the response is `400 invalid_cookidoo_url`
- **AND** no outbound HTTP request is made

#### Scenario: Page without Recipe JSON-LD maps to a typed upstream error

- **WHEN** the fetched page contains no `Recipe` JSON-LD block (or the fetch fails or times out)
- **THEN** the response is `502 cookidoo_unavailable` with a reason distinguishing fetch failure from missing JSON-LD
- **AND** no product row is created

### Requirement: Re-importing an already-imported URL is idempotent-ensure

The system SHALL treat `POST /products/import/cookidoo` for a URL whose `external_url` already exists on a product as an ensure operation: it MUST return `200` with the existing product and `already_imported: true`, and MUST NOT modify the existing product or create a duplicate.

#### Scenario: Duplicate import returns the existing product untouched

- **WHEN** a recipe product already exists with `external_url = U` and manually corrected nutriments
- **AND** the client POSTs `{url: U}` again
- **THEN** the response is `200` with the existing product, its corrected nutriments intact, and `already_imported: true`

## MODIFIED Requirements

### Requirement: Save POSTs to nutrition-api as a flat recipe product

The system SHALL convert the popup's form state into a `POST /products` request body and send it to the configured API base URL using the configured token. When the page JSON-LD provided a `recipeIngredient` array, the extension SHALL include it verbatim as `ingredients` in the request body and display a read-only ingredient count in the popup.

#### Scenario: Save produces the canonical request shape

- **WHEN** the user clicks Save with a complete form
- **THEN** the extension sends `POST <API_BASE_URL>/products` with the headers `Authorization: Bearer <TOKEN>` and `Content-Type: application/json`
- **AND** the body is `{"name": ..., "source": "recipe", "external_url": ..., "serving_size_g": ..., "nutriments_per_100g": { ... }}` where `nutriments_per_100g` contains only the fields the user provided (omitted keys for empty fields)
- **AND** when JSON-LD provided `recipeIngredient`, the body additionally carries `"ingredients": [...]` in page order

#### Scenario: Popup summarizes captured ingredients

- **WHEN** the extracted JSON-LD contains 20 `recipeIngredient` strings
- **THEN** the popup shows "20 ingredients captured" (read-only; individual strings are not editable in the popup)

#### Scenario: Server-side errors are surfaced

- **WHEN** the backend responds with a non-2xx status
- **THEN** the popup shows the response status and the JSON error body verbatim
