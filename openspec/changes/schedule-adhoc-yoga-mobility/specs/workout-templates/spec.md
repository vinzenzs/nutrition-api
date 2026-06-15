## MODIFIED Requirements

### Requirement: Workout templates are stored in a dedicated table

The system SHALL persist reusable workout templates in a `workout_templates`
table independent of `workouts`. Each row holds a `sport` (reusing the
`workouts` sport vocabulary), a `name`, an optional `description`, an optional
author-supplied `estimated_duration_sec`, an ordered list of `steps` stored as
JSONB, and audit timestamps. Templates are first-party authored — there is no
`external_id` or `source` column and no uniqueness constraint on `name`.

#### Scenario: Table is created with the documented columns

- **WHEN** the migration set is applied to a clean database
- **THEN** `workout_templates` exists with columns:
  - `id` (UUID PRIMARY KEY)
  - `sport` (TEXT NOT NULL, CHECK IN `('run', 'bike', 'swim', 'strength', 'yoga', 'mobility', 'other')`)
  - `name` (TEXT NOT NULL)
  - `description` (TEXT NULL)
  - `estimated_duration_sec` (INTEGER NULL, CHECK `estimated_duration_sec IS NULL OR estimated_duration_sec > 0`)
  - `steps` (JSONB NOT NULL, CHECK `jsonb_typeof(steps) = 'array' AND jsonb_array_length(steps) > 0`)
  - `created_at` (TIMESTAMPTZ NOT NULL DEFAULT now())
  - `updated_at` (TIMESTAMPTZ NOT NULL DEFAULT now())
- **AND** an index exists on `(sport)` to support the list filter
- **AND** the `sport` CHECK reuses the `workouts` sport vocabulary, including `'yoga'` and `'mobility'`

#### Scenario: sport vocabulary admits yoga and mobility

- **WHEN** a template is created with `sport: "yoga"` or `sport: "mobility"`
- **THEN** the row persists with that sport and it is returned unchanged on read
- **AND** the value passes the same validation used for `workouts` sports
