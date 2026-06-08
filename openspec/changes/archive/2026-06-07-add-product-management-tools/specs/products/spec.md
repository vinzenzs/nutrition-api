## ADDED Requirements

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

## MODIFIED Requirements

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
