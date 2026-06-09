## MODIFIED Requirements

### Requirement: Workouts are stored in a dedicated table

The system SHALL persist workouts in a `workouts` table independent of meals, hydration, and products. Each row holds a sport, a time window (`started_at`, `ended_at`), provenance metadata, optional intensity/burn signals, optional subjective post-session signals (RPE + GI distress), and audit timestamps. The table is the data shape that external writers — initially `garmin.py`, in future potentially Apple Health, Strava, or a manual UI — target via the REST endpoints.

#### Scenario: Table is created with the documented columns

- **WHEN** the migration set is applied to a clean database
- **THEN** `workouts` exists with columns:
  - `id` (UUID PRIMARY KEY)
  - `external_id` (TEXT NULL)
  - `source` (TEXT NOT NULL, CHECK IN `('garmin', 'manual', 'other')`)
  - `sport` (TEXT NOT NULL, CHECK IN `('run', 'bike', 'swim', 'strength', 'other')`)
  - `name` (TEXT NULL)
  - `started_at` (TIMESTAMPTZ NOT NULL)
  - `ended_at` (TIMESTAMPTZ NOT NULL)
  - `kcal_burned` (NUMERIC(10, 1) NULL, CHECK `kcal_burned IS NULL OR kcal_burned > 0`)
  - `avg_hr` (INTEGER NULL, CHECK `avg_hr IS NULL OR avg_hr > 0`)
  - `tss` (NUMERIC(10, 2) NULL, CHECK `tss IS NULL OR tss >= 0`)
  - `rpe` (INTEGER NULL, CHECK `rpe IS NULL OR (rpe BETWEEN 1 AND 10)`)
  - `gi_distress_score` (INTEGER NULL, CHECK `gi_distress_score IS NULL OR (gi_distress_score BETWEEN 1 AND 5)`)
  - `notes` (TEXT NULL)
  - `created_at` (TIMESTAMPTZ NOT NULL DEFAULT now())
  - `updated_at` (TIMESTAMPTZ NOT NULL DEFAULT now())
- **AND** a CHECK constraint enforces `ended_at > started_at`
- **AND** an index `workouts_started_at_idx` exists on `(started_at)`
- **AND** a partial UNIQUE index exists on `(external_id) WHERE external_id IS NOT NULL`
- **AND** there is NO `intensity` column (TSS is the intensity signal; downstream tools derive bands at call time)

#### Scenario: rpe and gi_distress_score are nullable per session

- **WHEN** the migration is applied to a database with existing `workouts` rows
- **THEN** every existing row carries `rpe = NULL` and `gi_distress_score = NULL`
- **AND** the migration succeeds without back-filling either column
- **AND** subsequent INSERT/UPSERT/PATCH paths default both fields to NULL when omitted

### Requirement: POST /workouts creates or updates a workout via external_id UPSERT

The system SHALL expose `POST /workouts` that accepts a workout body and persists it. When `external_id` is present and a row already exists with the same `external_id`, the system UPDATES that row (full-replace of the mutable fields); otherwise the system INSERTS a new row. The mutable field set includes `rpe` and `gi_distress_score` as optional integer-valued per-session signals. This semantic lets an external writer "POST every activity it sees" without tracking what is already synced.

#### Scenario: POST with rpe and gi_distress_score stores the values

- **WHEN** the client posts `{"source":"manual","sport":"bike","started_at":"2026-07-15T08:00:00Z","ended_at":"2026-07-15T09:30:00Z","rpe":7,"gi_distress_score":2}`
- **THEN** the system creates a row with `rpe = 7` and `gi_distress_score = 2`
- **AND** returns `201 Created` with the response body echoing both fields

#### Scenario: POST omitting rpe and gi_distress_score stores NULL

- **WHEN** the client posts a workout body that omits both fields
- **THEN** the row is created with both columns `NULL`
- **AND** the response body's JSON omits both fields (omitempty pattern matching `kcal_burned`, `avg_hr`, `tss`, `notes`)

#### Scenario: POST with rpe out of range is rejected

- **WHEN** the client posts `{"source":"manual","sport":"bike",…,"rpe":0}` or `{"…","rpe":11}` or `{"…","rpe":-1}`
- **THEN** the system returns `400 Bad Request` with `{"error":"rpe_invalid","range":{"min":1,"max":10}}`
- **AND** no row is inserted

#### Scenario: POST with gi_distress_score out of range is rejected

- **WHEN** the client posts a workout body with `gi_distress_score` set to `0` or `6` or `-2` or `100`
- **THEN** the system returns `400 Bad Request` with `{"error":"gi_distress_score_invalid","range":{"min":1,"max":5}}`
- **AND** no row is inserted

#### Scenario: POST with non-integer rpe / gi_distress_score is rejected

- **WHEN** the client posts with `rpe: "seven"` or `rpe: 7.5` or `gi_distress_score: "mild"`
- **THEN** the system returns `400 Bad Request` with `{"error":"rpe_invalid"}` or `{"error":"gi_distress_score_invalid"}` respectively
- **AND** no row is inserted

#### Scenario: Garmin import path passes through with NULLs

- **WHEN** the Garmin importer POSTs a workout body that does not include `rpe` or `gi_distress_score` (Garmin does not surface either)
- **THEN** the row is created with both fields `NULL`
- **AND** the user can subsequently PATCH the row to add the rehearsal signals

### Requirement: PATCH /workouts/{id} updates the mutable subset

The system SHALL expose `PATCH /workouts/{id}` accepting a partial body. The mutable subset includes `rpe`, `gi_distress_score`, plus the existing mutable fields. PATCH supports tri-state semantics on the new integer fields: a field is `unchanged` when absent from the body, `set` when present with an integer value, and `cleared to NULL` when present with explicit JSON `null`.

#### Scenario: PATCH sets rpe and gi_distress_score on an existing workout

- **WHEN** a workout exists with both fields `NULL`
- **AND** the client patches `{"rpe": 7, "gi_distress_score": 2}`
- **THEN** the row's `rpe = 7` and `gi_distress_score = 2`
- **AND** the response is `200 OK` with the updated workout

#### Scenario: PATCH absent fields leaves them unchanged

- **WHEN** a workout has `rpe = 7` and `gi_distress_score = 2`
- **AND** the client patches `{"notes": "felt strong"}` (no rpe / no gi_distress_score)
- **THEN** the row's `rpe` and `gi_distress_score` are unchanged
- **AND** `notes` is updated to `"felt strong"`

#### Scenario: PATCH null clears the field to NULL

- **WHEN** a workout has `rpe = 7`
- **AND** the client patches `{"rpe": null}`
- **THEN** the row's `rpe` becomes `NULL`
- **AND** subsequent GET responses omit the `rpe` field

#### Scenario: PATCH rpe out of range is rejected without touching other fields

- **WHEN** the client patches `{"rpe": 11, "gi_distress_score": 3}`
- **THEN** the system returns `400 Bad Request` with `{"error":"rpe_invalid","range":{"min":1,"max":10}}`
- **AND** no field is updated (transactional validation — the GI score is NOT written even though it's valid)

### Requirement: GET /workouts/{id} returns a single workout

The system SHALL expose `GET /workouts/{id}` returning the workout row with all populated fields, including `rpe` and `gi_distress_score` when set. Fields with `NULL` values are omitted from the response per the existing omitempty pattern.

#### Scenario: GET on a workout with rpe and gi_distress_score returns them

- **WHEN** a workout has `rpe = 7` and `gi_distress_score = 2`
- **AND** the client calls `GET /workouts/{id}`
- **THEN** the response is `200 OK` with body that includes `"rpe": 7` and `"gi_distress_score": 2`

#### Scenario: GET on a workout with NULL rehearsal fields omits them

- **WHEN** a workout has both fields `NULL`
- **AND** the client calls `GET /workouts/{id}`
- **THEN** the response body does NOT include the `rpe` or `gi_distress_score` keys

### Requirement: GET /workouts lists workouts in a window

The system SHALL expose `GET /workouts?from=&to=` returning every workout whose `started_at` falls in the window, ordered by `started_at` descending. The response items include `rpe` and `gi_distress_score` per the same omitempty pattern as the single-item GET.

#### Scenario: List includes rehearsal fields per row

- **WHEN** the client lists workouts in a window containing one rehearsal-tagged ride and one Garmin-imported ride
- **THEN** the rehearsal-tagged ride's entry includes `rpe` and `gi_distress_score`
- **AND** the Garmin-imported ride's entry omits both fields

### Requirement: GET /workouts/{id}/fueling surfaces rehearsal signals on the workout

The system SHALL include `rpe` and `gi_distress_score` on the `GET /workouts/{id}/fueling` response so the agent can read the rehearsal-outcome signals alongside the fueling totals in a single call. The two fields are echoed at the top level of the response, alongside `workout_id`, `started_at`, `ended_at`, and follow the same omitempty rule as everywhere else — absent when NULL on the underlying workout row.

#### Scenario: Fueling response carries rpe and gi_distress_score when set

- **WHEN** a workout has `rpe = 7` and `gi_distress_score = 2`
- **AND** the client calls `GET /workouts/{id}/fueling`
- **THEN** the response body includes `"rpe": 7` and `"gi_distress_score": 2` at the top level (alongside `workout_id`, `started_at`, `ended_at`, `pre_window`, `intra_window`, `post_window`)

#### Scenario: Fueling response omits the fields when NULL

- **WHEN** a workout has both fields `NULL`
- **AND** the client calls `GET /workouts/{id}/fueling`
- **THEN** the response body omits the `rpe` and `gi_distress_score` keys
- **AND** the fueling window shapes are otherwise unchanged

#### Scenario: Fueling endpoint requires no new query params for the new fields

- **WHEN** the client calls `GET /workouts/{id}/fueling`
- **THEN** the existing `pre_window_min` / `post_window_min` query semantics apply unchanged
- **AND** no `include_rehearsal` opt-in is required — the fields are always present (or always omitted via omitempty)
