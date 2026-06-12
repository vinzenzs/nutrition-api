# personal-records Specification (delta)

## ADDED Requirements

### Requirement: Personal records are stored in a dedicated table keyed by Garmin PR id

The system SHALL persist Garmin personal records (e.g. fastest 5k, fastest 10k, longest ride) in a `personal_records` table, independent of meals, hydration, workouts, and products. Each row mirrors a single Garmin PR identified by its stable Garmin PR `id` (stored as `external_id`), and carries its type, value, an accompanying unit note, an optional linked activity id, and the achievement timestamp. Personal records are slowly-changing inventory, not date-keyed data: a row is upserted in place by `external_id`, never appended per day. PR values are coaching context ("you're PR-fit right now") and SHALL NOT feed any nutrition computation.

#### Scenario: Table is created with the documented columns

- **WHEN** the migration set is applied to a clean database
- **THEN** `personal_records` exists with columns:
  - `id` (UUID PRIMARY KEY)
  - `external_id` (TEXT NOT NULL) — the Garmin PR id
  - `pr_type` (TEXT NOT NULL) — e.g. `5k`, `10k`, `longest-ride`
  - `value` (NUMERIC(12, 3) NOT NULL, CHECK `value >= 0`)
  - `unit` (TEXT NOT NULL) — the unit the value is expressed in (e.g. `s` for a time, `m` for a distance)
  - `activity_id` (TEXT NULL) — the Garmin activity that set the PR, when present
  - `achieved_at` (TIMESTAMPTZ NOT NULL)
  - `created_at` (TIMESTAMPTZ NOT NULL DEFAULT now())
  - `updated_at` (TIMESTAMPTZ NOT NULL DEFAULT now())
- **AND** a UNIQUE index exists on `(external_id)` so upserts dedup by Garmin PR id

### Requirement: POST /personal-records upserts a record by external id

The system SHALL expose `POST /personal-records` that creates or updates a personal record from `{external_id, pr_type, value, unit, activity_id?, achieved_at}`, upserting by `external_id` (`INSERT … ON CONFLICT (external_id) DO UPDATE`). A first POST inserts; a later POST with the same `external_id` updates the existing row in place (a beaten PR overwrites the prior value). The endpoint accepts the standard `Idempotency-Key` header.

#### Scenario: First upsert creates the record

- **WHEN** the client posts `{"external_id":"garmin-pr-1","pr_type":"5k","value":1320,"unit":"s","achieved_at":"2026-05-20T08:00:00Z"}`
- **THEN** the system creates a row and returns `201 Created` with the new record including its generated `id`

#### Scenario: Re-upsert of the same external_id updates in place

- **WHEN** the client posts the same `external_id` with `value: 1295` and a newer `achieved_at`
- **THEN** the system returns `200 OK` with the updated record (new value and timestamp)
- **AND** exactly one `personal_records` row exists for that `external_id` (no duplicate)

#### Scenario: Optional activity_id is accepted

- **WHEN** the client also supplies `activity_id: "garmin-act-987"`
- **THEN** the system stores it and echoes it back
- **AND** omitting it stores NULL and the response omits the field (omitempty)

#### Scenario: Missing external_id is rejected

- **WHEN** the client posts a body without `external_id`
- **THEN** the system returns `400 Bad Request` with `{"error":"external_id_required"}`

#### Scenario: Missing pr_type is rejected

- **WHEN** the client posts a body without `pr_type`
- **THEN** the system returns `400 Bad Request` with `{"error":"pr_type_required"}`

#### Scenario: Missing or invalid value is rejected

- **WHEN** the client posts a body without `value`, or with `value` < 0
- **THEN** the system returns `400 Bad Request` with `{"error":"value_invalid"}`
- **AND** no row is written

#### Scenario: Missing unit is rejected

- **WHEN** the client posts a body without `unit`
- **THEN** the system returns `400 Bad Request` with `{"error":"unit_required"}`

#### Scenario: Missing achieved_at is rejected

- **WHEN** the client posts a body without `achieved_at`
- **THEN** the system returns `400 Bad Request` with `{"error":"achieved_at_required"}`

#### Scenario: Value is rounded at the response boundary

- **WHEN** a record with `value = 1295.6667` is serialized to a response
- **THEN** the value is rounded with `numfmt.Round1` to `1295.7`

### Requirement: GET /personal-records lists records

The system SHALL expose `GET /personal-records` returning all personal records wrapped as `{"personal_records": [PersonalRecord, ...]}`, ordered by `achieved_at` descending (most recent PR first). Optional `?pr_type=<type>` filters to a single PR type.

#### Scenario: List returns all records

- **WHEN** the client calls `GET /personal-records`
- **THEN** the response is `200 OK` with body shape `{"personal_records": [PersonalRecord, ...]}`
- **AND** records are ordered by `achieved_at` descending

#### Scenario: pr_type filter narrows the list

- **WHEN** the client calls `GET /personal-records?pr_type=5k`
- **THEN** only records with `pr_type = "5k"` are returned

#### Scenario: Nullable activity_id is omitted when unset

- **WHEN** a record has NULL `activity_id`
- **THEN** the `activity_id` key is omitted from that record (omitempty)

### Requirement: Personal records are unit-isolated from nutrition summaries

The system SHALL NOT include personal-record data in the nutrition daily summary (`GET /summary/daily`) or the nutrition range summary. Personal-record responses SHALL NOT contain any nutriment or hydration field, and PR values SHALL NOT contribute to any nutriment, hydration, or energy total.

#### Scenario: Nutrition summary does not include personal records

- **WHEN** the client calls `GET /summary/daily?date=…&tz=…`
- **THEN** the response body does not include `pr_type`, `value`, or any personal-record field

#### Scenario: Personal-record responses do not include nutriment fields

- **WHEN** the client calls `GET /personal-records`
- **THEN** the response body does not include `kcal`, `protein_g`, `total_ml`, or any other nutriment / hydration field
