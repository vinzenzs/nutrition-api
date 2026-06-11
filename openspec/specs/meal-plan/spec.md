# meal-plan Specification

## Purpose
TBD - created by archiving change add-meal-plan. Update Purpose after archive.
## Requirements
### Requirement: Planned meals are persisted with a date, slot, product reference, and lifecycle status

The system SHALL persist a planned meal as `{plan_date, slot, product_id, quantity_g?, status, notes?}` where `slot` MUST be one of `breakfast`, `lunch`, `dinner`, `snack` and `status` MUST be one of `planned`, `eaten`, `skipped` (default `planned`). `product_id` is required and MUST reference an existing product, validated at create and update with a `404 product_not_found` on violation. Multiple planned meals MAY share the same `plan_date` and `slot`. The system SHALL expose create, single read, range list (`GET /plan?from=YYYY-MM-DD&to=YYYY-MM-DD`), update, and delete. Write endpoints SHALL participate in the standard idempotency middleware.

#### Scenario: Create a planned dinner

- **WHEN** the client POSTs `{plan_date: "2026-06-12", slot: "dinner", product_id: <existing recipe product>, quantity_g: 450}`
- **THEN** the response is `201` with the entry, `status: "planned"`, and a generated `id`

#### Scenario: Unknown product is rejected

- **WHEN** the client POSTs a planned meal whose `product_id` does not exist
- **THEN** the response is `404 product_not_found` and no row is created

#### Scenario: Range list returns the horizon ordered by date then slot

- **WHEN** the client GETs `/plan?from=2026-06-12&to=2026-06-14` with entries on all three dates
- **THEN** the response is `200` with all entries in `[from, to]` inclusive, ordered by `plan_date` ascending then slot (breakfast, lunch, dinner, snack), each carrying its product's `name` for display
- **AND** dates outside the range are absent

#### Scenario: Two planned dinners on one date are legal

- **WHEN** the client creates two `dinner` entries for the same `plan_date`
- **THEN** both are persisted and both appear in range reads

### Requirement: Marking a planned meal eaten atomically logs a real meal entry

The system SHALL expose `POST /plan/{id}/eaten` with optional body `{quantity_g?, logged_at?}`. It MUST, in a single transaction: create a meal entry through the existing meals capability (with the plan's `product_id`, the effective quantity — body override, else plan `quantity_g`, else the product's serving default —, and `logged_at` defaulting to now), set the plan entry's `status` to `eaten`, and store the created meal entry's id on the plan row as `meal_entry_id`. `logged_at` MUST NOT be in the future. The response SHALL carry both the updated plan entry and the created meal entry. Meal entries SHALL NOT be created from plan data by any other path.

#### Scenario: One-tap eaten creates the meal and flips status atomically

- **WHEN** the client POSTs `/plan/{id}/eaten` with an empty body on a `planned` entry with `quantity_g: 450`
- **THEN** a meal entry is created with that product, `quantity_g: 450`, and `logged_at` ≈ now
- **AND** the plan entry returns with `status: "eaten"` and `meal_entry_id` set to the new meal entry
- **AND** if the meal creation fails, the plan entry remains `planned` (no partial state)

#### Scenario: Future logged_at is rejected

- **WHEN** the client POSTs `/plan/{id}/eaten` with `logged_at` tomorrow
- **THEN** the response is a `400` validation error and neither write occurs

### Requirement: Lifecycle transitions are one-way with eaten terminal

The system SHALL allow `planned → eaten` (via the eaten endpoint only), `planned → skipped`, and `skipped → planned` (via update). It MUST reject marking an `eaten` entry eaten again with `409 plan_entry_already_eaten`, and MUST reject any status change away from `eaten`. Deleting a meal entry through the meals capability SHALL NOT modify the linked plan row.

#### Scenario: Double-tap on eaten is a conflict, not a duplicate meal

- **WHEN** the client POSTs `/plan/{id}/eaten` on an entry already `eaten` (with a fresh idempotency key)
- **THEN** the response is `409 plan_entry_already_eaten` and no second meal entry is created

#### Scenario: Skipped can be un-skipped

- **WHEN** the client PATCHes a `skipped` entry to `status: "planned"`
- **THEN** the response is `200` and the entry is `planned` again

#### Scenario: Eaten is terminal on the plan surface

- **WHEN** the client PATCHes an `eaten` entry to any other status
- **THEN** the response is a `409` conflict error and the entry remains `eaten`

