## ADDED Requirements

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

## MODIFIED Requirements

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
