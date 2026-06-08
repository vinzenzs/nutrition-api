## MODIFIED Requirements

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
