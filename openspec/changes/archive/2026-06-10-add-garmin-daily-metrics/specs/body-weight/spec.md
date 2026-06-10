## MODIFIED Requirements

### Requirement: Body-weight entries are stored in a dedicated table

The system SHALL persist body-weight measurements in a `body_weight_entries` table independent of meals, hydration, workouts, and products. Each row holds a positive `weight_kg`, a `logged_at` timestamp in UTC, an optional `body_fat_pct` (0–100), optional smart-scale biometrics (`muscle_mass_kg`, `body_water_pct`, `bone_mass_kg`, `bmi`), an optional free-text `note`, and audit timestamps. Multiple measurements per calendar date are allowed; the trend endpoint smooths within and across days.

#### Scenario: Table is created with the documented columns

- **WHEN** the migration set is applied to a clean database
- **THEN** `body_weight_entries` exists with columns:
  - `id` (UUID PRIMARY KEY)
  - `logged_at` (TIMESTAMPTZ NOT NULL)
  - `weight_kg` (NUMERIC(5, 2) NOT NULL, CHECK `weight_kg > 0`)
  - `body_fat_pct` (NUMERIC(4, 2) NULL, CHECK `body_fat_pct IS NULL OR (body_fat_pct >= 0 AND body_fat_pct <= 100)`)
  - `muscle_mass_kg` (NUMERIC(5, 2) NULL, CHECK `muscle_mass_kg IS NULL OR muscle_mass_kg > 0`)
  - `body_water_pct` (NUMERIC(4, 1) NULL, CHECK `body_water_pct IS NULL OR (body_water_pct >= 0 AND body_water_pct <= 100)`)
  - `bone_mass_kg` (NUMERIC(4, 2) NULL, CHECK `bone_mass_kg IS NULL OR bone_mass_kg > 0`)
  - `bmi` (NUMERIC(4, 1) NULL, CHECK `bmi IS NULL OR bmi > 0`)
  - `note` (TEXT NULL)
  - `created_at` (TIMESTAMPTZ NOT NULL DEFAULT now())
  - `updated_at` (TIMESTAMPTZ NOT NULL DEFAULT now())
- **AND** an index `body_weight_entries_logged_at_idx` exists on `(logged_at)`
- **AND** there is NO uniqueness constraint on `logged_at` or `(logged_at, weight_kg)` — multiple entries per day, even at the same instant, are allowed

#### Scenario: Biometric columns are nullable with no back-fill

- **WHEN** the migration adding `muscle_mass_kg`, `body_water_pct`, `bone_mass_kg`, and `bmi` is applied to a database with existing `body_weight_entries` rows
- **THEN** every existing row carries NULL for all four columns
- **AND** the migration succeeds without back-filling any of them
- **AND** subsequent INSERT/PATCH paths default all four to NULL when omitted

### Requirement: POST /weight logs a single measurement

The system SHALL expose `POST /weight` that creates a body-weight entry from `{weight_kg, logged_at, body_fat_pct?, muscle_mass_kg?, body_water_pct?, bone_mass_kg?, bmi?, note?}` and accepts the standard `Idempotency-Key` header.

#### Scenario: Successful log

- **WHEN** the client posts `{"weight_kg": 72.5, "logged_at": "2026-06-07T07:00:00Z"}`
- **THEN** the system creates a row and returns `201 Created` with the new entry including its generated `id`

#### Scenario: Optional body_fat_pct is accepted

- **WHEN** the client also supplies `body_fat_pct: 14.2`
- **THEN** the system stores it on the row and echoes it back

#### Scenario: Optional smart-scale biometrics are accepted

- **WHEN** the client also supplies `{"muscle_mass_kg": 58.4, "body_water_pct": 55.1, "bone_mass_kg": 3.2, "bmi": 22.4}`
- **THEN** the system stores all four and echoes them back
- **AND** omitting any of them stores NULL and the response omits the field (omitempty)

#### Scenario: Optional note is accepted

- **WHEN** the client also supplies a `note` (free-text, ≤ 500 chars)
- **THEN** the system stores it and echoes it back

#### Scenario: Missing weight is rejected

- **WHEN** the client posts a body without `weight_kg` (or with a non-numeric value)
- **THEN** the system returns `400 Bad Request` with `{"error":"weight_kg_invalid"}`

#### Scenario: Non-positive weight is rejected

- **WHEN** the client posts `weight_kg` that is zero or negative
- **THEN** the system returns `400 Bad Request` with `{"error":"weight_kg_invalid"}`

#### Scenario: Body-fat % out of range is rejected

- **WHEN** the client posts `body_fat_pct` < 0 or > 100
- **THEN** the system returns `400 Bad Request` with `{"error":"body_fat_pct_invalid"}`

#### Scenario: Out-of-range biometric is rejected

- **WHEN** the client posts `muscle_mass_kg` ≤ 0, `body_water_pct` < 0 or > 100, `bone_mass_kg` ≤ 0, or `bmi` ≤ 0
- **THEN** the system returns `400 Bad Request` with the matching error code (`muscle_mass_kg_invalid`, `body_water_pct_invalid`, `bone_mass_kg_invalid`, `bmi_invalid`)
- **AND** no row is written

#### Scenario: Note longer than 500 characters is rejected

- **WHEN** the client posts a `note` whose length exceeds 500 characters
- **THEN** the system returns `400 Bad Request` with `{"error":"note_too_long"}`

#### Scenario: logged_at far in the future is rejected

- **WHEN** the client posts `logged_at` more than 24 hours in the future
- **THEN** the system returns `400 Bad Request` with `{"error":"logged_at_too_far_future"}`

#### Scenario: Idempotent retry returns the original entry

- **WHEN** two POST requests arrive with the same `Idempotency-Key` and byte-identical body
- **THEN** the second response replays the first response body and status `201`
- **AND** only one `body_weight_entries` row exists

### Requirement: PATCH /weight/{id} updates a subset of fields

The system SHALL expose `PATCH /weight/{id}` accepting partial updates of `weight_kg`, `body_fat_pct`, `muscle_mass_kg`, `body_water_pct`, `bone_mass_kg`, `bmi`, `logged_at`, and `note`. Validation rules match the POST endpoint.

#### Scenario: Partial update changes only supplied fields

- **WHEN** the client patches `{"body_fat_pct": 13.8}` on an existing entry
- **THEN** the response shows the new body-fat %
- **AND** other fields remain unchanged

#### Scenario: Biometric fields can be patched

- **WHEN** the client patches `{"muscle_mass_kg": 59.0}` on an existing entry
- **THEN** the response shows the new muscle mass
- **AND** other fields remain unchanged

#### Scenario: Patching to an invalid weight is rejected

- **WHEN** the client patches `{"weight_kg": -1}`
- **THEN** the system returns `400 Bad Request` with `{"error":"weight_kg_invalid"}`

#### Scenario: Patch on unknown id returns 404

- **WHEN** the client patches an id that does not exist
- **THEN** the system returns `404 Not Found` with `{"error":"weight_not_found"}`
