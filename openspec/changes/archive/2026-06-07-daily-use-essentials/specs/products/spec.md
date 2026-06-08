## ADDED Requirements

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

The system SHALL expose `POST /products/recipes` that creates a composite product made of N component products plus per-component grams. Nutriments-per-100g are computed at creation time from components and stored on the new product row, so downstream meal-logging math uses the same effective-nutriment columns as any other product.

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

## MODIFIED Requirements

### Requirement: Barcode lookup against Open Food Facts with local cache

The system SHALL expose `POST /products/lookup/{barcode}` that returns a product for the given barcode, fetching from Open Food Facts on first access and serving from local cache thereafter.

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
