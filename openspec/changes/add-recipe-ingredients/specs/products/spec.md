# products — delta for add-recipe-ingredients

## ADDED Requirements

### Requirement: Recipe products carry an optional ordered ingredients list

The system SHALL accept and persist an optional `ingredients` field on `source=recipe` products: an ordered array of free-text strings stored verbatim (e.g. `"100 g Staudensellerie"`). The array MUST contain at most 100 entries, each non-empty and at most 500 characters. Supplying `ingredients` on a product whose `source` is not `recipe` MUST be rejected with `400 ingredients_require_recipe_source`. The field SHALL round-trip: accepted on `POST /products`, returned on every product read, omitted from JSON when null.

#### Scenario: Flat-imported recipe with ingredients round-trips

- **WHEN** the client POSTs a product with `source=recipe`, an `external_url`, `nutriments_per_100g`, and `ingredients: ["1 Zwiebel", "100 g Staudensellerie"]`
- **THEN** the product is created and `GET /products/{id}` returns the `ingredients` array in the same order with identical strings

#### Scenario: Ingredients on a non-recipe product are rejected

- **WHEN** the client POSTs a product with `source=off` and an `ingredients` array
- **THEN** the response is `400` with error code `ingredients_require_recipe_source`

#### Scenario: Oversized ingredients array is rejected

- **WHEN** the client POSTs a recipe product with 101 ingredient strings
- **THEN** the response is `400` with a validation error
- **AND** no product row is created

#### Scenario: Products without ingredients omit the field

- **WHEN** the client GETs a product that has no ingredients stored
- **THEN** the response body does not contain an `ingredients` key
