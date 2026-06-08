## ADDED Requirements

### Requirement: Barcode lookup against Open Food Facts with local cache

The system SHALL expose `POST /products/lookup/{barcode}` that returns a product for the given barcode, fetching from Open Food Facts on first access and serving from local cache thereafter.

#### Scenario: First lookup for an unknown barcode succeeds

- **WHEN** the client calls `POST /products/lookup/3017624010701` and the barcode is not yet cached
- **THEN** the system fetches `https://world.openfoodfacts.org/api/v2/product/3017624010701.json`
- **AND** parses available nutriments (kcal, protein, carbs, fat, fiber, sugar, salt per 100g) into typed columns
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
- **AND** updates the parsed columns and `off_payload`
- **AND** updates `fetched_at` to now
- **AND** returns the refreshed product

#### Scenario: Unknown barcode returns actionable not-found

- **WHEN** the client looks up a barcode that Open Food Facts does not recognize (response `status: 0`)
- **THEN** the system returns `404 Not Found`
- **AND** the response body is `{"error":"product_not_found","barcode":"<barcode>","next":"POST /meals/freeform"}`
- **AND** no product row is created

### Requirement: Manual product creation

The system SHALL expose `POST /products` that creates a reusable product from user- or client-supplied data, with no Open Food Facts involvement.

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

The system SHALL update `products.last_logged_at` to the meal entry's `logged_at` whenever a `meal_entry` is created or updated with that product, but only if the new value is greater than the current `last_logged_at`.

#### Scenario: New meal entry advances last_logged_at

- **WHEN** a meal entry is created with `product_id = X` and `logged_at = 2026-06-06T12:00:00Z`
- **AND** `products[X].last_logged_at` is null or earlier than `2026-06-06T12:00:00Z`
- **THEN** `products[X].last_logged_at` is set to `2026-06-06T12:00:00Z`

#### Scenario: Older meal entry does not regress last_logged_at

- **WHEN** a meal entry is created with `logged_at` earlier than `products[X].last_logged_at`
- **THEN** `products[X].last_logged_at` is unchanged

#### Scenario: Freeform meal with save_as_product creates the product and sets last_logged_at

- **WHEN** the LLM calls `POST /meals/freeform` with `save_as_product: true` and the supplied name does not match an existing product
- **THEN** the system creates a `products` row with `source = "manual"` and the supplied nutriments
- **AND** sets `last_logged_at` to the meal entry's `logged_at`
