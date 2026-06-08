# products Specification

## Purpose

Define product lookup, manual creation, search, retrieval, and last-logged tracking for the nutrition API.

## Requirements

### Requirement: Barcode lookup against Open Food Facts with local cache

The system SHALL expose `POST /products/lookup/{barcode}` that returns a product for the given barcode, fetching from Open Food Facts on first access and serving from local cache thereafter. An empty barcode path segment (`POST /products/lookup/`) SHALL be rejected with `400 barcode_required` so the caller distinguishes "the route exists, your input is wrong" from a not-found.

#### Scenario: First lookup for an unknown barcode succeeds

- **WHEN** the client calls `POST /products/lookup/3017624010701` and the barcode is not yet cached
- **THEN** the system fetches `https://world.openfoodfacts.org/api/v2/product/3017624010701.json`
- **AND** parses available nutriments (kcal, protein, carbs, fat, fiber, sugar, salt per 100g, plus iron_mg, calcium_mg, vitamin_d_mcg, vitamin_b12_mcg, vitamin_c_mg, magnesium_mg, potassium_mg, zinc_mg per 100g) into typed columns
- **AND** stores the entire OFF response JSON in `off_payload`
- **AND** sets `source = "off"` and `fetched_at = now()`
- **AND** returns `200 OK` with the product row (id, barcode, name, brand, nutriments, serving_size_g, source, fetched_at)

#### Scenario: Subsequent lookup serves from cache without re-fetching

- **WHEN** the client calls `POST /products/lookup/{barcode}` for a barcode already in the products table
- **THEN** the system returns the cached product immediately
- **AND** does not issue any HTTP request to Open Food Facts

#### Scenario: Forced refresh re-fetches from OFF

- **WHEN** the client calls `POST /products/lookup/{barcode}?refresh=true` for a cached barcode
- **THEN** the system fetches from Open Food Facts
- **AND** updates the parsed columns (macros and micros) and `off_payload`
- **AND** updates `fetched_at` to now
- **AND** returns the refreshed product

#### Scenario: Unknown barcode returns actionable not-found

- **WHEN** the client looks up a barcode that Open Food Facts does not recognize (response `status: 0`)
- **THEN** the system returns `404 Not Found`
- **AND** the response body is `{"error":"product_not_found","barcode":"<barcode>","next":"POST /meals/freeform"}`
- **AND** no product row is created

#### Scenario: Empty barcode returns barcode_required

- **WHEN** the client calls `POST /products/lookup/` with no path segment after `lookup/` (or with a path segment that resolves to an empty string)
- **THEN** the system returns `400 Bad Request`
- **AND** the response body is `{"error":"barcode_required"}`
- **AND** the Open Food Facts API is NOT contacted

### Requirement: Manual product creation

The system SHALL expose `POST /products` that creates a reusable product from user- or client-supplied data, with no Open Food Facts involvement. The request accepts macros and the micros listed under the products-carry-micros requirement.

#### Scenario: Manual product is created with required fields

- **WHEN** the client posts `{"name":"Homemade granola","nutriments_per_100g":{"kcal":420,"protein_g":12,"carbs_g":55,"fat_g":18}}` to `POST /products`
- **THEN** the system creates a product with `source = "manual"`, `barcode = null`, and the supplied nutriments
- **AND** returns `201 Created` with the new product row including its generated id

#### Scenario: Manual product accepts a barcode

- **WHEN** the client supplies a `barcode` in the request body and that barcode is not already cached
- **THEN** the system stores the barcode on the new product
- **AND** sets `source = "manual"` (not `"off"`)

#### Scenario: Manual product with duplicate barcode is rejected

- **WHEN** the client supplies a barcode that is already present on another product
- **THEN** the system returns `409 Conflict` with `{"error":"barcode_already_exists","product_id":"<existing-id>"}`

#### Scenario: Manual product rejects source override

- **WHEN** the client supplies `source` in the request body
- **THEN** the system ignores the field and always stores `source = "manual"`

### Requirement: Product search ranked by recency of use

The system SHALL expose `GET /products/search?q=<query>` that returns products whose name or brand matches the query, ordered by most recently logged first.

#### Scenario: Matching by name returns recent first

- **WHEN** the client calls `GET /products/search?q=yogurt`
- **AND** two products match: one with `last_logged_at = yesterday`, one with `last_logged_at = last month`
- **THEN** the system returns both products with the yesterday-used one first

#### Scenario: Matching is case-insensitive and substring

- **WHEN** the client calls `GET /products/search?q=GRAN`
- **THEN** products with `name = "Homemade granola"` and `name = "Granny Smith apple"` both appear in results

#### Scenario: Never-logged products still appear, ranked after used ones

- **WHEN** a product matches the query but `last_logged_at IS NULL`
- **THEN** the product appears in results ordered after all products with a non-null `last_logged_at`

#### Scenario: Empty query is rejected

- **WHEN** the client calls `GET /products/search?q=` or omits `q`
- **THEN** the system returns `400 Bad Request` with `{"error":"q_required"}`

### Requirement: Single product retrieval

The system SHALL expose `GET /products/{id}` that returns a product by id.

#### Scenario: Existing product is returned

- **WHEN** the client calls `GET /products/{id}` with a valid product id
- **THEN** the system returns `200 OK` with the full product row

#### Scenario: Missing product returns 404

- **WHEN** the client calls `GET /products/{id}` with an id that does not exist
- **THEN** the system returns `404 Not Found` with `{"error":"product_not_found"}`

### Requirement: Last-logged tracking updates on meal entry creation

The system SHALL update `products.last_logged_at` to the meal entry's `logged_at` whenever a `meal_entry` is created or updated with that product, but only if the new value is greater than the current `last_logged_at`. When `last_logged_at` advances, the system SHALL ALSO update `products.last_logged_quantity_g` to the meal entry's `quantity_g`. The two columns advance together or not at all.

#### Scenario: New meal entry advances last_logged_at

- **WHEN** a meal entry is created with `product_id = X` and `logged_at = 2026-06-06T12:00:00Z`
- **AND** `products[X].last_logged_at` is null or earlier than `2026-06-06T12:00:00Z`
- **THEN** `products[X].last_logged_at` is set to `2026-06-06T12:00:00Z`
- **AND** `products[X].last_logged_quantity_g` is set to the meal entry's `quantity_g`

#### Scenario: Older meal entry does not regress last_logged_at

- **WHEN** a meal entry is created with `logged_at` earlier than `products[X].last_logged_at`
- **THEN** `products[X].last_logged_at` is unchanged
- **AND** `products[X].last_logged_quantity_g` is unchanged

#### Scenario: Freeform meal with save_as_product creates the product and sets last_logged_at

- **WHEN** the LLM calls `POST /meals/freeform` with `save_as_product: true` and the supplied name does not match an existing product
- **THEN** the system creates a `products` row with `source = "manual"` and the supplied nutriments
- **AND** sets `last_logged_at` to the meal entry's `logged_at`
- **AND** sets `last_logged_quantity_g` to the meal entry's `quantity_g`

### Requirement: Products carry optional micronutrient columns

The `products` table SHALL gain optional per-100g columns for the following micronutrients: `iron_mg`, `calcium_mg`, `vitamin_d_mcg`, `vitamin_b12_mcg`, `vitamin_c_mg`, `magnesium_mg`, `potassium_mg`, `zinc_mg`. Each is nullable and follows the same null-tolerant pattern as the existing macros: absent in source data means null, never zero.

#### Scenario: Manual product creation accepts micros

- **WHEN** the client posts `{"name":"Fortified oat milk","nutriments_per_100g":{"kcal":48,"protein_g":1.5,"calcium_mg":120,"vitamin_d_mcg":1.5,"vitamin_b12_mcg":0.4}}` to `POST /products`
- **THEN** the system stores the supplied micros in their typed columns
- **AND** leaves unsupplied micro columns as null

#### Scenario: Manual product without any micros stores them all as null

- **WHEN** the client posts a manual product with only macros
- **THEN** all micro columns are stored as null
- **AND** the response includes only the populated fields under `nutriments_per_100g`

#### Scenario: Micros are returned on retrieval when non-null

- **WHEN** the client calls `GET /products/{id}` for a product with `iron_mg = 4.5`
- **THEN** the response `nutriments_per_100g` includes `iron_mg: 4.5`
- **AND** omits the columns that are null

### Requirement: Composite recipe products

The system SHALL expose `POST /products/recipes` that creates a composite product made of N component products plus per-component grams. Nutriments-per-100g are computed at creation time from components and stored on the new product row, so downstream meal-logging math uses the same effective-nutriment columns as any other product. The component list MUST NOT contain duplicate `product_id` values — if the same product appears more than once, the request is rejected so the user (or agent) sums the quantities and supplies one entry per product.

#### Scenario: Recipe creation computes nutriments from components

- **WHEN** the client posts `{"name":"Morning skyr bowl","components":[{"product_id":"<skyr>","quantity_g":200},{"product_id":"<oats>","quantity_g":40},{"product_id":"<honey>","quantity_g":10}],"serving_size_g":250}` to `POST /products/recipes`
- **THEN** the system creates a product with `source = "recipe"`, the supplied name, and a generated id
- **AND** stores per-100g nutriments computed as the gram-weighted average of each component's effective nutriments
- **AND** stores `serving_size_g = 250`
- **AND** inserts one `product_components` row per component referencing the new product, the component product_id, and the grams
- **AND** returns `201 Created` with the new product row plus a `components` array echoing the input

#### Scenario: Recipe requires at least one component

- **WHEN** the client posts a recipe with empty or missing `components`
- **THEN** the system returns `400 Bad Request` with `{"error":"components_required"}`

#### Scenario: Recipe rejects duplicate component product_ids

- **WHEN** the client posts a recipe whose `components` array contains the same `product_id` more than once (e.g. `[{"product_id":"X","quantity_g":100},{"product_id":"X","quantity_g":50}]`)
- **THEN** the system returns `400 Bad Request` with body `{"error":"component_duplicate","product_id":"<X>","occurrences":<N>,"hint":"sum the quantities and supply one entry per product"}`
- **AND** no product or component rows are created

#### Scenario: Recipe rejects unknown component product ids

- **WHEN** any `product_id` in `components` does not exist
- **THEN** the system returns `404 Not Found` with `{"error":"component_not_found","product_id":"<id>"}`
- **AND** no product or component rows are created

#### Scenario: Recipe rejects non-positive component quantity

- **WHEN** any component has `quantity_g <= 0`
- **THEN** the system returns `400 Bad Request` with `{"error":"component_quantity_g_invalid","product_id":"<id>"}`

#### Scenario: Components with null nutriments propagate null

- **WHEN** a component lacks a value for `iron_mg`
- **THEN** the computed recipe `iron_mg` is null if no component supplies a value
- **AND** the computed recipe `iron_mg` reflects only the contributing components' values (gram-weighted) if some do

#### Scenario: Recipes can be retrieved with components

- **WHEN** the client calls `GET /products/{id}?expand=components` for a recipe product
- **THEN** the response includes a `components` array with `{product_id, name, quantity_g, effective_nutriments_per_100g}` for each component
- **AND** omitting `expand=components` returns the product row without the `components` array

### Requirement: Recipe nutriments can be recomputed

The system SHALL expose `POST /products/recipes/{id}/recompute` that re-derives the recipe's per-100g nutriments from its current component values. This handles the case where a component's underlying nutriments changed (e.g. an OFF refresh) after the recipe was created.

#### Scenario: Recompute updates nutriments from current component state

- **WHEN** the client posts `POST /products/recipes/{id}/recompute`
- **THEN** the system recomputes each nutriment field as the gram-weighted average of each linked component's current effective nutriments
- **AND** updates the recipe product's typed nutriment columns
- **AND** updates `updated_at` to now
- **AND** returns `200 OK` with the refreshed product

#### Scenario: Recompute on a non-recipe returns 400

- **WHEN** the client posts recompute against a product whose `source` is not `"recipe"`
- **THEN** the system returns `400 Bad Request` with `{"error":"not_a_recipe","product_id":"<id>"}`

#### Scenario: Recompute on missing recipe returns 404

- **WHEN** the client posts recompute against a product id that does not exist
- **THEN** the system returns `404 Not Found` with `{"error":"product_not_found"}`

### Requirement: Products can be listed with pagination

The system SHALL expose `GET /products` that returns the product cache as a paginated list, ordered most-recently-used first, optionally filtered by source.

#### Scenario: Default listing returns recent-first products

- **WHEN** the client calls `GET /products`
- **THEN** the system returns `200 OK` with body `{"products": [...], "total": N, "limit": 50, "offset": 0}`
- **AND** the `products` array is ordered by `last_logged_at DESC NULLS LAST, name ASC`
- **AND** `limit` defaults to `50`, `offset` defaults to `0`

#### Scenario: Source filter narrows results

- **WHEN** the client calls `GET /products?source=manual`
- **THEN** only products with `source = "manual"` are returned
- **AND** `total` reflects the filtered count

#### Scenario: Pagination via limit and offset

- **WHEN** the client calls `GET /products?limit=10&offset=10`
- **THEN** the response contains at most 10 products starting from the 11th row in the ordering
- **AND** `limit` echoes `10`, `offset` echoes `10`
- **AND** `total` reports the unpaginated total

#### Scenario: Invalid source is rejected

- **WHEN** the client supplies `source` that is not one of `off | manual | recipe`
- **THEN** the system returns `400 Bad Request` with `{"error":"source_invalid"}`

#### Scenario: Limit above the maximum is rejected

- **WHEN** the client supplies `limit > 200`
- **THEN** the system returns `400 Bad Request` with `{"error":"limit_too_large","max":200}`

#### Scenario: Negative limit or offset is rejected

- **WHEN** the client supplies `limit < 0` or `offset < 0`
- **THEN** the system returns `400 Bad Request` with `{"error":"pagination_invalid"}`

### Requirement: Products can be deleted with history preservation

The system SHALL expose `DELETE /products/{id}` that permanently removes a product row, preserving any logged meal entries' nutriment history via snapshot materialisation before nulling the FK.

#### Scenario: Delete on a product with no references succeeds

- **WHEN** the client calls `DELETE /products/{id}` for a product that is not referenced by any `product_components` row
- **THEN** the system returns `204 No Content`
- **AND** the product row is removed from `products`

#### Scenario: Delete materialises a snapshot into any historical meals

- **WHEN** the product was referenced by `meal_entries` rows whose `snapshot_name` is null (the standard non-freeform meal-logging path)
- **THEN** before deleting the product, the system copies `name` and every nutriment column from the product into the corresponding `snapshot_*` columns on those meal entries
- **AND** the meal entries' `product_id` becomes null via the FK's `ON DELETE SET NULL` cascade
- **AND** subsequent summary calls compute totals from the materialised snapshot exactly as they did before the deletion (no silent loss of historical totals)

#### Scenario: Delete does not overwrite an existing freeform snapshot

- **WHEN** a `meal_entries` row already has a non-null `snapshot_name` (a freeform meal that nevertheless linked to this product)
- **THEN** the materialisation step uses `COALESCE(snapshot_*, products.*)` and does NOT overwrite the freeform snapshot
- **AND** the meal entry retains its original snapshot after the product is deleted

#### Scenario: Delete is blocked when the product is a component of any recipe

- **WHEN** the client calls `DELETE /products/{id}` for a product that is referenced by one or more `product_components.component_product_id` rows
- **THEN** the system returns `409 Conflict` with body `{"error":"product_in_use_as_component","recipes":[{"id":"<rid>","name":"<rname>"}, ...],"hint":"delete the listed recipes first, or replace this product within them"}`
- **AND** the product row is NOT deleted
- **AND** no meal entries are modified

#### Scenario: Delete on an unknown id returns 404

- **WHEN** the client calls `DELETE /products/{id}` for an id that does not exist
- **THEN** the system returns `404 Not Found` with `{"error":"product_not_found"}`

#### Scenario: Delete is idempotent under retry

- **WHEN** the client repeats `DELETE /products/{id}` after a successful delete
- **THEN** the system returns `404 product_not_found`
- **AND** does not corrupt any other product row

### Requirement: Products carry an optional external_url

The system SHALL persist an optional `external_url` per product, used to record the upstream source of products that came from somewhere other than direct manual entry, the OFF lookup, or an internal recipe composition.

#### Scenario: Migration adds the column

- **WHEN** the migration set is applied to a clean database
- **THEN** the `products` table has a column `external_url TEXT NULL` with no default

#### Scenario: External URL is returned on product reads

- **WHEN** the client calls `GET /products/{id}` for a product whose `external_url` is set
- **THEN** the response body includes an `external_url` field carrying the stored string

#### Scenario: Null external URL is omitted from responses

- **WHEN** the client calls `GET /products/{id}` for a product whose `external_url` is null
- **THEN** the response body does not include the `external_url` field (or includes it as null, per the existing pointer-emit convention used elsewhere in the schema)

### Requirement: POST /products accepts source and external_url

The system SHALL accept optional `source` and `external_url` fields on `POST /products`. When omitted, source defaults to `manual` and external_url defaults to null, preserving existing behaviour.

#### Scenario: Manual product with neither field

- **WHEN** the client posts `{"name":"Homemade granola","nutriments_per_100g":{"kcal":420}}`
- **THEN** the created product has `source = "manual"` and `external_url = null`
- **AND** the response status is `201 Created`

#### Scenario: Flat-imported recipe with source=recipe and external_url

- **WHEN** the client posts `{"name":"Lasagne Bolognese","source":"recipe","external_url":"https://cookidoo.de/recipes/recipe/de-DE/r-xxxxxxx","serving_size_g":350,"nutriments_per_100g":{"kcal":166,"protein_g":9.4}}`
- **THEN** the created product has `source = "recipe"`, the supplied `external_url`, `nutriment_computed_at = null` (this row's nutriments did not come from a component recompute), and no `product_components` rows
- **AND** the response status is `201 Created`

#### Scenario: Invalid source is rejected

- **WHEN** the client posts a body with `source` set to anything other than `"manual"` or `"recipe"`
- **THEN** the system returns `400 Bad Request` with body `{"error":"source_invalid"}`
- **AND** no product row is created

#### Scenario: External URL length is bounded

- **WHEN** the client posts an `external_url` longer than 2048 characters
- **THEN** the system returns `400 Bad Request` with body `{"error":"external_url_too_long"}`
- **AND** no product row is created

#### Scenario: External URL can accompany a manual product

- **WHEN** the client posts `{"name":"Grandma's apple cake","source":"manual","external_url":"https://family.example/cakes/apple"}`
- **THEN** the created product has `source = "manual"` and the supplied `external_url`
- **AND** the response status is `201 Created`

### Requirement: Existing product behaviour is preserved when neither field is supplied

The system SHALL behave identically to its prior contract for clients that do not supply `source` or `external_url` on `POST /products`.

#### Scenario: Existing client unaffected

- **WHEN** an existing client (e.g. the mobile app prior to this change) posts a body without `source` or `external_url`
- **THEN** the created product has `source = "manual"` (the prior default) and `external_url = null`
- **AND** all other response fields are unchanged

### Requirement: Recipe products may be either composed or flat-imported

The system SHALL permit two valid shapes for `source=recipe` products: composed (with `product_components` rows and `nutriment_computed_at` set by the recompute pipeline) and flat-imported (with `external_url` set, no components, and `nutriment_computed_at` null).

#### Scenario: Composed recipe is recognisable

- **WHEN** a recipe product has one or more `product_components` rows
- **THEN** its `nutriment_computed_at` is non-null (set by the recompute pipeline that produced its nutriments)

#### Scenario: Flat-imported recipe is recognisable

- **WHEN** a recipe product was created via `POST /products` with `source=recipe`, an `external_url`, and `nutriments_per_100g` supplied directly
- **THEN** it has no `product_components` rows
- **AND** its `nutriment_computed_at` is null
- **AND** its `external_url` is non-null

### Requirement: Products carry the most recently logged quantity

The `products` table SHALL carry an optional `last_logged_quantity_g NUMERIC(10, 3)` column whose value mirrors the `quantity_g` of the most-recent `meal_entries` row linked to that product. The column moves in lockstep with `last_logged_at`: when one advances, both advance; when neither advances (backdated entries), both stay unchanged. The field SHALL be serialized as `last_logged_quantity_g` on every product-returning JSON response, omitted when null.

#### Scenario: New meal entry advances both last_logged_at and quantity

- **WHEN** a meal entry is created with `product_id = X`, `quantity_g = 200`, and `logged_at = 2026-06-06T12:00:00Z`
- **AND** `products[X].last_logged_at` is null or earlier than `2026-06-06T12:00:00Z`
- **THEN** `products[X].last_logged_at` becomes `2026-06-06T12:00:00Z`
- **AND** `products[X].last_logged_quantity_g` becomes `200`

#### Scenario: Older meal entry does not regress quantity

- **WHEN** a meal entry is created with `logged_at` earlier than `products[X].last_logged_at`
- **THEN** `products[X].last_logged_quantity_g` is unchanged

#### Scenario: Freeform meal with save_as_product captures initial quantity

- **WHEN** `POST /meals/freeform` is called with `save_as_product: true` and supplies `quantity_g = 120`
- **AND** the system creates a new product row for the supplied name
- **THEN** the new product's `last_logged_quantity_g` is `120`
- **AND** its `last_logged_at` is the meal entry's `logged_at`

#### Scenario: PATCH quantity_g on the most-recent meal updates the product

- **WHEN** the client PATCHes `quantity_g` to `300` on a meal entry whose `logged_at` equals `products[X].last_logged_at`
- **THEN** `products[X].last_logged_quantity_g` becomes `300`

#### Scenario: PATCH quantity_g on an older meal does NOT update the product

- **WHEN** the client PATCHes `quantity_g` on a meal entry whose `logged_at` is earlier than `products[X].last_logged_at`
- **THEN** `products[X].last_logged_quantity_g` is unchanged
- **AND** the meal entry's stored `quantity_g` reflects the patched value

#### Scenario: Deleting the most-recent meal does not revert quantity

- **WHEN** the meal entry whose `logged_at` equals `products[X].last_logged_at` is deleted
- **THEN** `products[X].last_logged_quantity_g` is unchanged (the product still "remembers" the most recent quantity)

#### Scenario: The field is omitted from JSON when null

- **WHEN** a client retrieves a product that has never been logged (`last_logged_at` is null)
- **THEN** the JSON response omits the `last_logged_quantity_g` field entirely
