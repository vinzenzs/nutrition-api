## MODIFIED Requirements

### Requirement: Workouts are stored in a dedicated table

The system SHALL persist workouts in a `workouts` table independent of meals, hydration, and products. Each row holds a sport, a time window (`started_at`, `ended_at`), provenance metadata, a `status` (`planned` or `completed`), optional intensity/burn signals, optional ingestion metrics (distance, average power, ambient temperature, estimated sweat loss), an optional session-group key linking the legs of a brick/multisport session, and audit timestamps. The table is the data shape that external writers — initially `garmin.py`, in future potentially Apple Health, Strava, or a manual UI — target via the REST endpoints.

#### Scenario: Table is created with the documented columns

- **WHEN** the migration set is applied to a clean database
- **THEN** `workouts` exists with columns:
  - `id` (UUID PRIMARY KEY)
  - `external_id` (TEXT NULL)
  - `source` (TEXT NOT NULL, CHECK IN `('garmin', 'manual', 'other')`)
  - `sport` (TEXT NOT NULL, CHECK IN `('run', 'bike', 'swim', 'strength', 'other')`)
  - `status` (TEXT NOT NULL DEFAULT `'completed'`, CHECK IN `('planned', 'completed')`)
  - `name` (TEXT NULL)
  - `started_at` (TIMESTAMPTZ NOT NULL)
  - `ended_at` (TIMESTAMPTZ NOT NULL)
  - `kcal_burned` (NUMERIC(10, 1) NULL, CHECK `kcal_burned IS NULL OR kcal_burned > 0`)
  - `avg_hr` (INTEGER NULL, CHECK `avg_hr IS NULL OR avg_hr > 0`)
  - `tss` (NUMERIC(10, 2) NULL, CHECK `tss IS NULL OR tss >= 0`)
  - `rpe` (INTEGER NULL, CHECK `rpe IS NULL OR (rpe BETWEEN 1 AND 10)`)
  - `gi_distress_score` (INTEGER NULL, CHECK `gi_distress_score IS NULL OR (gi_distress_score BETWEEN 1 AND 5)`)
  - `distance_m` (NUMERIC(10, 1) NULL, CHECK `distance_m IS NULL OR distance_m > 0`)
  - `avg_power_w` (INTEGER NULL, CHECK `avg_power_w IS NULL OR avg_power_w > 0`)
  - `temperature_c` (NUMERIC(4, 1) NULL, CHECK `temperature_c IS NULL OR (temperature_c BETWEEN -40 AND 60)`)
  - `sweat_loss_ml` (NUMERIC(10, 1) NULL, CHECK `sweat_loss_ml IS NULL OR sweat_loss_ml > 0`)
  - `session_group` (TEXT NULL)
  - `notes` (TEXT NULL)
  - `created_at` (TIMESTAMPTZ NOT NULL DEFAULT now())
  - `updated_at` (TIMESTAMPTZ NOT NULL DEFAULT now())
- **AND** a CHECK constraint enforces `ended_at > started_at`
- **AND** an index `workouts_started_at_idx` exists on `(started_at)`
- **AND** a partial UNIQUE index exists on `(external_id) WHERE external_id IS NOT NULL`
- **AND** a partial (non-unique) index `workouts_session_group_idx` exists on `(session_group) WHERE session_group IS NOT NULL`
- **AND** there is NO `intensity` column (TSS is the intensity signal; downstream tools derive bands at call time)

#### Scenario: rpe and gi_distress_score are nullable per session

- **WHEN** the migration is applied to a database with existing `workouts` rows
- **THEN** every existing row carries `rpe = NULL` and `gi_distress_score = NULL`
- **AND** the migration succeeds without back-filling either column
- **AND** subsequent INSERT/UPSERT/PATCH paths default both fields to NULL when omitted

#### Scenario: Ingestion-metric columns are nullable with no back-fill

- **WHEN** the migration adding `distance_m`, `avg_power_w`, `temperature_c`, `sweat_loss_ml`, and `session_group` is applied to a database with existing `workouts` rows
- **THEN** every existing row carries NULL for all five columns
- **AND** the migration succeeds without back-filling any of them
- **AND** subsequent INSERT/UPSERT/PATCH paths default all five fields to NULL when omitted ("not measured" / "not grouped" is a meaningful state, not a data-quality bug)

#### Scenario: status defaults to completed and back-fills existing rows

- **WHEN** the migration adding `status` is applied to a database with existing `workouts` rows
- **THEN** every existing row takes `status = 'completed'` via the column DEFAULT (existing rows are all completed activities)
- **AND** a POST that omits `status` stores `'completed'`
- **AND** the `status` column is NOT NULL and always present on responses (no omitempty)

## ADDED Requirements

### Requirement: Workouts carry a planned/completed status lifecycle

The system SHALL treat `status` as a mutable workout field with values `planned` and `completed` (default `completed`). `status` conditions the future-date guard: a `completed` workout keeps the existing rule (rejected when `started_at` is more than 24 hours in the future), while a `planned` workout MAY have a `started_at` in the future up to one year ahead (beyond that, `started_at_too_far_future` still fires). A `planned` workout MAY have a past `started_at` (a plan already underway). `ended_at > started_at` holds for both. Reconciling a planned session into the completed activity that fulfils it is the writer's responsibility (Garmin's scheduled-workout id and activity id differ); the API only provides the `status` field, the `status` filter, and PATCH/DELETE to support whatever reconciliation the writer chooses.

#### Scenario: POST a planned workout in the future is accepted

- **WHEN** the client posts `{"source":"garmin","sport":"bike","status":"planned","started_at":"<3 weeks from now>","ended_at":"<3 weeks from now + 2h>"}`
- **THEN** the system returns `201 Created` with `status: "planned"`
- **AND** the future-date guard does NOT reject it

#### Scenario: POST a completed workout in the future is still rejected

- **WHEN** the client posts a body with `status` omitted (or `"completed"`) and `started_at` more than 24 hours in the future
- **THEN** the system returns `400 Bad Request` with `{"error":"started_at_too_far_future"}`

#### Scenario: A planned workout more than a year out is rejected

- **WHEN** the client posts `status: "planned"` with `started_at` more than 12 months in the future
- **THEN** the system returns `400 Bad Request` with `{"error":"started_at_too_far_future"}`

#### Scenario: Invalid status value is rejected

- **WHEN** the client posts `status: "scheduled"` (not in the enum)
- **THEN** the system returns `400 Bad Request` with `{"error":"status_invalid"}`

#### Scenario: GET /workouts filters by status

- **WHEN** a window contains one `planned` and one `completed` workout
- **AND** the client calls `GET /workouts?from=…&to=…&status=planned`
- **THEN** only the planned workout is returned
- **AND** omitting `status` returns both (no implicit filter)
- **AND** the window parameters remain required

#### Scenario: PATCH can promote a planned workout to completed

- **WHEN** a `planned` workout exists
- **AND** the client patches `{"status":"completed"}`
- **THEN** the row's `status` becomes `completed`
- **AND** patching an invalid status value returns `400 status_invalid`

#### Scenario: Planned workouts do not distort energy or fueling aggregates

- **WHEN** a `planned` workout (no `kcal_burned`, future-dated) coexists with completed workouts
- **THEN** it contributes nothing to energy-availability burn sums (it has no `kcal_burned`)
- **AND** it does not appear inside any `GET /workouts/{id}/fueling` window for a real (completed) session
- **AND** read paths that must exclude plans filter on `status = 'completed'`
