# devices Specification (delta)

## ADDED Requirements

### Requirement: Device inventory is stored in a dedicated table keyed by Garmin device id

The system SHALL persist Garmin device inventory (watches, bike computers, scales, and other paired hardware) in a `devices` table, independent of meals, hydration, workouts, and products. Each row mirrors a single Garmin device identified by its stable Garmin device id (stored as `external_id`), and carries its display name, model, last sync time, and optional battery level and firmware version. Devices are slowly-changing inventory, not date-keyed data: a row is upserted in place by `external_id`, never appended per day. Device fields are reference/coaching context (battery and firmware nudges) and SHALL NOT feed any nutrition, fueling, energy, or hydration computation.

#### Scenario: Table is created with the documented columns

- **WHEN** the migration set is applied to a clean database
- **THEN** `devices` exists with columns:
  - `id` (UUID PRIMARY KEY)
  - `external_id` (TEXT NOT NULL) — the Garmin device id
  - `display_name` (TEXT NOT NULL)
  - `model` (TEXT NULL)
  - `last_sync_at` (TIMESTAMPTZ NULL)
  - `battery_pct` (NUMERIC(5, 1) NULL, CHECK `battery_pct IS NULL OR (battery_pct >= 0 AND battery_pct <= 100)`)
  - `firmware_version` (TEXT NULL)
  - `created_at` (TIMESTAMPTZ NOT NULL DEFAULT now())
  - `updated_at` (TIMESTAMPTZ NOT NULL DEFAULT now())
- **AND** a UNIQUE index exists on `(external_id)` so upserts dedup by Garmin device id

### Requirement: POST /devices upserts a device record by external id

The system SHALL expose `POST /devices` that creates or updates a device record from `{external_id, display_name, model?, last_sync_at?, battery_pct?, firmware_version?}`, upserting by `external_id` (`INSERT … ON CONFLICT (external_id) DO UPDATE`). A first POST inserts; a later POST with the same `external_id` updates the existing row in place (a fresh sync advances `last_sync_at` and `battery_pct`). The endpoint accepts the standard `Idempotency-Key` header.

#### Scenario: First upsert creates the device row

- **WHEN** the client posts `{"external_id":"garmin-dev-fenix7","display_name":"Fenix 7","model":"fenix7","battery_pct":86.0}`
- **THEN** the system creates a row and returns `201 Created` with the new record including its generated `id`

#### Scenario: Re-upsert of the same external_id updates in place

- **WHEN** the client posts the same `external_id` with `battery_pct: 41.0` and a newer `last_sync_at`
- **THEN** the system returns `200 OK` with the updated record (new battery and sync time)
- **AND** exactly one `devices` row exists for that `external_id` (no duplicate)

#### Scenario: Missing external_id is rejected

- **WHEN** the client posts a body without `external_id`
- **THEN** the system returns `400 Bad Request` with `{"error":"external_id_required"}`

#### Scenario: Missing display_name is rejected

- **WHEN** the client posts a body without `display_name`
- **THEN** the system returns `400 Bad Request` with `{"error":"display_name_required"}`

#### Scenario: Out-of-range battery is rejected

- **WHEN** the client posts `battery_pct` < 0 or > 100
- **THEN** the system returns `400 Bad Request` with `{"error":"battery_pct_invalid"}`
- **AND** no row is written

#### Scenario: Battery is rounded at the response boundary

- **WHEN** a device record with `battery_pct = 86.44` is serialized to a response
- **THEN** the value is rounded with `numfmt.Round1` to `86.4`

### Requirement: GET /devices lists device records

The system SHALL expose `GET /devices` returning all device records wrapped as `{"devices": [DeviceRecord, ...]}`, ordered by `display_name` ascending.

#### Scenario: List returns all devices

- **WHEN** the client calls `GET /devices`
- **THEN** the response is `200 OK` with body shape `{"devices": [DeviceRecord, ...]}`
- **AND** each record includes `external_id` and `display_name`, plus `model`, `last_sync_at`, `battery_pct`, and `firmware_version` when set

#### Scenario: Nullable fields are omitted when unset

- **WHEN** a device record has NULL `model`, `last_sync_at`, `battery_pct`, and `firmware_version`
- **THEN** those keys are omitted from that record (omitempty)

### Requirement: GET /devices/{id} returns a single device record

The system SHALL expose `GET /devices/{id}` returning the device record by its backend `id`.

#### Scenario: Existing id returns the record

- **WHEN** the client calls `GET /devices/<existing-id>`
- **THEN** the response is `200 OK` with the device record body

#### Scenario: Unknown id returns 404

- **WHEN** the client calls `GET /devices/<unknown-id>`
- **THEN** the system returns `404 Not Found` with `{"error":"device_not_found"}`

### Requirement: Devices are unit-isolated from nutrition summaries

The system SHALL NOT include device data in the nutrition daily summary (`GET /summary/daily`) or the nutrition range summary. Device responses SHALL NOT contain any nutriment or hydration field, and device fields SHALL NOT contribute to any nutriment, hydration, or energy total.

#### Scenario: Nutrition summary does not include devices

- **WHEN** the client calls `GET /summary/daily?date=…&tz=…`
- **THEN** the response body does not include `battery_pct`, `firmware_version`, or any device-related field

#### Scenario: Device responses do not include nutriment fields

- **WHEN** the client calls `GET /devices` or `GET /devices/{id}`
- **THEN** the response body does not include `kcal`, `protein_g`, `total_ml`, or any other nutriment / hydration field
