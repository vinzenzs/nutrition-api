# achievements Specification

## Purpose
TBD - created by archiving change add-garmin-misc-mirror. Update Purpose after archive.
## Requirements
### Requirement: Achievements are stored in a dedicated table keyed by Garmin id

The system SHALL persist Garmin earned badges and ad-hoc challenges in an `achievements` table, independent of meals, hydration, workouts, and products. Each row mirrors a single Garmin badge or challenge identified by its stable Garmin id (stored as `external_id`), discriminated by a `kind` (`badge` or `challenge`), and carries its name, an optional earned timestamp, and an optional progress percentage (for in-progress challenges). Achievements are slowly-changing inventory, not date-keyed data: a row is upserted in place by `external_id`, never appended per day. Achievements are coaching/reference context ("you just earned the 100-rides badge") and SHALL NOT feed any nutrition, fueling, energy, or hydration computation. Personal records are NOT stored here — they live in the separate `personal-records` capability.

#### Scenario: Table is created with the documented columns

- **WHEN** the migration set is applied to a clean database
- **THEN** `achievements` exists with columns:
  - `id` (UUID PRIMARY KEY)
  - `external_id` (TEXT NOT NULL) — the Garmin badge/challenge id
  - `kind` (TEXT NOT NULL, CHECK `kind IN ('badge','challenge')`)
  - `name` (TEXT NOT NULL)
  - `earned_at` (TIMESTAMPTZ NULL)
  - `progress_pct` (NUMERIC(5, 1) NULL, CHECK `progress_pct IS NULL OR (progress_pct >= 0 AND progress_pct <= 100)`)
  - `created_at` (TIMESTAMPTZ NOT NULL DEFAULT now())
  - `updated_at` (TIMESTAMPTZ NOT NULL DEFAULT now())
- **AND** a UNIQUE index exists on `(external_id)` so upserts dedup by Garmin id

### Requirement: POST /achievements upserts a record by external id

The system SHALL expose `POST /achievements` that creates or updates an achievement from `{external_id, kind, name, earned_at?, progress_pct?}`, upserting by `external_id` (`INSERT … ON CONFLICT (external_id) DO UPDATE`). A first POST inserts; a later POST with the same `external_id` updates the existing row in place (a challenge's `progress_pct` advances, a badge's `earned_at` is set when completed). The endpoint accepts the standard `Idempotency-Key` header.

#### Scenario: First upsert creates the achievement

- **WHEN** the client posts `{"external_id":"garmin-badge-100rides","kind":"badge","name":"100 Rides","earned_at":"2026-05-30T12:00:00Z"}`
- **THEN** the system creates a row and returns `201 Created` with the new record including its generated `id`

#### Scenario: Re-upsert of the same external_id updates in place

- **WHEN** the client posts the same `external_id` for a challenge with `progress_pct: 80.0`
- **THEN** the system returns `200 OK` with the updated record (new progress)
- **AND** exactly one `achievements` row exists for that `external_id` (no duplicate)

#### Scenario: Optional fields are accepted and omitted when unset

- **WHEN** the client posts a challenge without `earned_at` but with `progress_pct: 40.0`
- **THEN** the system stores `progress_pct` and leaves `earned_at` NULL
- **AND** the response omits the `earned_at` key (omitempty)

#### Scenario: Missing external_id is rejected

- **WHEN** the client posts a body without `external_id`
- **THEN** the system returns `400 Bad Request` with `{"error":"external_id_required"}`

#### Scenario: Invalid kind is rejected

- **WHEN** the client posts `kind: "trophy"` (not one of badge/challenge)
- **THEN** the system returns `400 Bad Request` with `{"error":"kind_invalid"}`
- **AND** no row is written

#### Scenario: Missing name is rejected

- **WHEN** the client posts a body without `name`
- **THEN** the system returns `400 Bad Request` with `{"error":"name_required"}`

#### Scenario: Out-of-range progress is rejected

- **WHEN** the client posts `progress_pct` < 0 or > 100
- **THEN** the system returns `400 Bad Request` with `{"error":"progress_pct_invalid"}`
- **AND** no row is written

#### Scenario: Progress is rounded at the response boundary

- **WHEN** a record with `progress_pct = 80.46` is serialized to a response
- **THEN** the value is rounded with `numfmt.Round1` to `80.5`

### Requirement: GET /achievements lists records

The system SHALL expose `GET /achievements` returning all achievement records wrapped as `{"achievements": [Achievement, ...]}`, ordered by `earned_at` descending with NULL `earned_at` (in-progress challenges) last. Optional `?kind=badge|challenge` filters to a single kind.

#### Scenario: List returns all achievements

- **WHEN** the client calls `GET /achievements`
- **THEN** the response is `200 OK` with body shape `{"achievements": [Achievement, ...]}`
- **AND** earned records are ordered by `earned_at` descending

#### Scenario: kind filter narrows the list

- **WHEN** the client calls `GET /achievements?kind=challenge`
- **THEN** only records with `kind = "challenge"` are returned

#### Scenario: Nullable fields are omitted when unset

- **WHEN** a record has NULL `earned_at` and NULL `progress_pct`
- **THEN** those keys are omitted from that record (omitempty)

### Requirement: Achievements are unit-isolated from nutrition summaries

The system SHALL NOT include achievement data in the nutrition daily summary (`GET /summary/daily`) or the nutrition range summary. Achievement responses SHALL NOT contain any nutriment or hydration field, and achievement fields SHALL NOT contribute to any nutriment, hydration, or energy total.

#### Scenario: Nutrition summary does not include achievements

- **WHEN** the client calls `GET /summary/daily?date=…&tz=…`
- **THEN** the response body does not include `kind`, `progress_pct`, or any achievement field

#### Scenario: Achievement responses do not include nutriment fields

- **WHEN** the client calls `GET /achievements`
- **THEN** the response body does not include `kcal`, `protein_g`, `total_ml`, or any other nutriment / hydration field

