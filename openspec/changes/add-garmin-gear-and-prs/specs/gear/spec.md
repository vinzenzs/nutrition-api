# gear Specification (delta)

## ADDED Requirements

### Requirement: Gear inventory is stored in a dedicated table keyed by Garmin gear id

The system SHALL persist Garmin gear (shoes, bikes, and other equipment) in a `gear` table, independent of meals, hydration, workouts, and products. Each row mirrors a single piece of Garmin gear identified by its stable Garmin gear `uuid` (stored as `external_id`), and carries its type, display name, accumulated distance and activity count, retirement state, and optional begin/end dates. Gear is slowly-changing inventory, not date-keyed data: a row is upserted in place by `external_id`, never appended per day. Gear distance and counts are coaching context (gear-retirement reminders) and SHALL NOT feed any nutrition computation.

#### Scenario: Table is created with the documented columns

- **WHEN** the migration set is applied to a clean database
- **THEN** `gear` exists with columns:
  - `id` (UUID PRIMARY KEY)
  - `external_id` (TEXT NOT NULL) — the Garmin gear uuid
  - `gear_type` (TEXT NOT NULL, CHECK `gear_type IN ('shoes','bike','other')`)
  - `display_name` (TEXT NOT NULL)
  - `total_distance_m` (NUMERIC(12, 1) NULL, CHECK `total_distance_m IS NULL OR total_distance_m >= 0`)
  - `total_activities` (INTEGER NULL, CHECK `total_activities IS NULL OR total_activities >= 0`)
  - `retired` (BOOLEAN NOT NULL DEFAULT false)
  - `date_begin` (DATE NULL)
  - `date_end` (DATE NULL)
  - `created_at` (TIMESTAMPTZ NOT NULL DEFAULT now())
  - `updated_at` (TIMESTAMPTZ NOT NULL DEFAULT now())
- **AND** a UNIQUE index exists on `(external_id)` so upserts dedup by Garmin gear id

### Requirement: POST /gear upserts a gear record by external id

The system SHALL expose `POST /gear` that creates or updates a gear record from `{external_id, gear_type, display_name, total_distance_m?, total_activities?, retired?, date_begin?, date_end?}`, upserting by `external_id` (`INSERT … ON CONFLICT (external_id) DO UPDATE`). A first POST inserts; a later POST with the same `external_id` updates the existing row in place. The endpoint accepts the standard `Idempotency-Key` header.

#### Scenario: First upsert creates the gear row

- **WHEN** the client posts `{"external_id":"garmin-gear-abc","gear_type":"shoes","display_name":"Daily Trainers","total_distance_m":780000,"total_activities":120}`
- **THEN** the system creates a row and returns `201 Created` with the new record including its generated `id`

#### Scenario: Re-upsert of the same external_id updates in place

- **WHEN** the client posts the same `external_id` with `total_distance_m: 812000` and `retired: true`
- **THEN** the system returns `200 OK` with the updated record (new distance, `retired: true`)
- **AND** exactly one `gear` row exists for that `external_id` (no duplicate)

#### Scenario: Missing external_id is rejected

- **WHEN** the client posts a body without `external_id`
- **THEN** the system returns `400 Bad Request` with `{"error":"external_id_required"}`

#### Scenario: Invalid gear_type is rejected

- **WHEN** the client posts `gear_type: "kayak"` (not one of shoes/bike/other)
- **THEN** the system returns `400 Bad Request` with `{"error":"gear_type_invalid"}`
- **AND** no row is written

#### Scenario: Missing display_name is rejected

- **WHEN** the client posts a body without `display_name`
- **THEN** the system returns `400 Bad Request` with `{"error":"display_name_required"}`

#### Scenario: Negative distance or activity count is rejected

- **WHEN** the client posts `total_distance_m` < 0 or `total_activities` < 0
- **THEN** the system returns `400 Bad Request` with the matching error code (`total_distance_m_invalid`, `total_activities_invalid`)
- **AND** no row is written

#### Scenario: Distance is rounded at the response boundary

- **WHEN** a gear record with `total_distance_m = 812345.67` is serialized to a response
- **THEN** the value is rounded with `numfmt.Round1` to `812345.7`

### Requirement: GET /gear lists gear records

The system SHALL expose `GET /gear` returning all gear records wrapped as `{"gear": [GearRecord, ...]}`, ordered by `display_name` ascending. Optional `?retired=true|false` filters by retirement state.

#### Scenario: List returns all gear

- **WHEN** the client calls `GET /gear`
- **THEN** the response is `200 OK` with body shape `{"gear": [GearRecord, ...]}`
- **AND** each record includes `external_id`, `gear_type`, `display_name`, and the distance/activity/retired fields when set

#### Scenario: Retired filter narrows the list

- **WHEN** the client calls `GET /gear?retired=true`
- **THEN** only records with `retired = true` are returned

#### Scenario: Nullable fields are omitted when unset

- **WHEN** a gear record has NULL `total_distance_m`, `total_activities`, `date_begin`, and `date_end`
- **THEN** those keys are omitted from that record (omitempty)

### Requirement: GET /gear/{id} returns a single gear record

The system SHALL expose `GET /gear/{id}` returning the gear record by its backend `id`.

#### Scenario: Existing id returns the record

- **WHEN** the client calls `GET /gear/<existing-id>`
- **THEN** the response is `200 OK` with the gear record body

#### Scenario: Unknown id returns 404

- **WHEN** the client calls `GET /gear/<unknown-id>`
- **THEN** the system returns `404 Not Found` with `{"error":"gear_not_found"}`

### Requirement: Gear is unit-isolated from nutrition summaries

The system SHALL NOT include gear data in the nutrition daily summary (`GET /summary/daily`) or the nutrition range summary. Gear responses SHALL NOT contain any nutriment or hydration field, and gear distance SHALL NOT contribute to any nutriment, hydration, or energy total.

#### Scenario: Nutrition summary does not include gear

- **WHEN** the client calls `GET /summary/daily?date=…&tz=…`
- **THEN** the response body does not include `total_distance_m`, `gear_type`, or any gear-related field

#### Scenario: Gear responses do not include nutriment fields

- **WHEN** the client calls `GET /gear` or `GET /gear/{id}`
- **THEN** the response body does not include `kcal`, `protein_g`, `total_ml`, or any other nutriment / hydration field
