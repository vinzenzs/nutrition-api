## ADDED Requirements

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
