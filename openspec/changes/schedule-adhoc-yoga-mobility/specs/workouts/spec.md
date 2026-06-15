## MODIFIED Requirements

### Requirement: Workouts are stored in a dedicated table

The system SHALL persist workouts in a `workouts` table independent of meals, hydration, and products. Each row holds a sport, a time window (`started_at`, `ended_at`), provenance metadata, a `status` (`planned` or `completed`), optional intensity/burn signals, optional ingestion metrics (distance, average power, ambient temperature, estimated sweat loss), an optional session-group key linking the legs of a brick/multisport session, optional links to a `workout-template` and a training-plan `plan_slot` (for planned workouts originating from a plan), and audit timestamps. The table is the data shape that external writers — initially `garmin.py`, in future potentially Apple Health, Strava, or a manual UI — target via the REST endpoints, and that the training-plan materializer targets for planned sessions.

#### Scenario: Table is created with the documented columns

- **WHEN** the migration set is applied to a clean database
- **THEN** `workouts` exists with columns:
  - `id` (UUID PRIMARY KEY)
  - `external_id` (TEXT NULL)
  - `source` (TEXT NOT NULL, CHECK IN `('garmin', 'manual', 'other')`)
  - `sport` (TEXT NOT NULL, CHECK IN `('run', 'bike', 'swim', 'strength', 'yoga', 'mobility', 'other')`)
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
  - `template_id` (UUID NULL, REFERENCES `workout_templates(id)` ON DELETE SET NULL)
  - `plan_slot_id` (UUID NULL, REFERENCES `plan_slots(id)` ON DELETE SET NULL)
  - `notes` (TEXT NULL)
  - `created_at` (TIMESTAMPTZ NOT NULL DEFAULT now())
  - `updated_at` (TIMESTAMPTZ NOT NULL DEFAULT now())
- **AND** a CHECK constraint enforces `ended_at > started_at`
- **AND** an index `workouts_started_at_idx` exists on `(started_at)`
- **AND** a partial UNIQUE index exists on `(external_id) WHERE external_id IS NOT NULL`
- **AND** a partial (non-unique) index `workouts_session_group_idx` exists on `(session_group) WHERE session_group IS NOT NULL`
- **AND** a partial UNIQUE index `workouts_plan_slot_id_key` exists on `(plan_slot_id) WHERE plan_slot_id IS NOT NULL`
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

#### Scenario: sport vocabulary admits yoga and mobility

- **WHEN** the migration widening the `sport` CHECK is applied to a database with existing `workouts` rows
- **THEN** the `sport` CHECK accepts `'yoga'` and `'mobility'` in addition to `'run'`, `'bike'`, `'swim'`, `'strength'`, `'other'`
- **AND** every existing row keeps its current sport unchanged (the migration only widens the allowed set)

### Requirement: POST /workouts creates or updates a workout via external_id UPSERT

The system SHALL expose `POST /workouts` that accepts a workout body and persists it. When `external_id` is present and a row already exists with the same `external_id`, the system UPDATES that row (full-replace of the mutable fields); otherwise the system INSERTS a new row. The mutable field set includes `rpe` and `gi_distress_score` as optional integer-valued per-session signals (1..10 and 1..5 respectively), plus the optional ingestion metrics `distance_m`, `avg_power_w`, `temperature_c`, `sweat_loss_ml`, and the optional `session_group` key. This semantic lets an external writer "POST every activity it sees" without tracking what is already synced.

#### Scenario: First POST with external_id inserts a new row

- **WHEN** the client posts `{"external_id":"garmin:1234567","source":"garmin","sport":"bike","started_at":"2026-06-07T08:00:00Z","ended_at":"2026-06-07T09:30:00Z","kcal_burned":850,"tss":78}`
- **THEN** the system creates a row and returns `201 Created` with the new workout including its generated `id`

#### Scenario: Subsequent POST with same external_id updates the existing row

- **WHEN** a workout with `external_id: "garmin:1234567"` already exists
- **AND** the client posts another body with the same `external_id` but `kcal_burned: 900`
- **THEN** the system UPDATES the existing row to the new values
- **AND** returns `200 OK` with the updated workout (the same `id`)
- **AND** no duplicate row is created

#### Scenario: POST without external_id always inserts

- **WHEN** the client posts a body without an `external_id` (e.g. `{"source":"manual","sport":"strength","started_at":"…","ended_at":"…"}`)
- **THEN** the system INSERTS a new row with `external_id: NULL`
- **AND** returns `201 Created`
- **AND** two such POSTs produce two distinct rows even if the bodies are identical (manual writes have no implicit dedup)

#### Scenario: source is required and validated

- **WHEN** the client posts a body without `source`, or with `source` not in the documented enum
- **THEN** the system returns `400 Bad Request` with `{"error":"source_invalid"}`

#### Scenario: sport is required and validated

- **WHEN** the client posts a body without `sport`, or with `sport` not in `run|bike|swim|strength|yoga|mobility|other`
- **THEN** the system returns `400 Bad Request` with `{"error":"sport_invalid"}`

#### Scenario: yoga and mobility are accepted sports

- **WHEN** the client posts a body with `sport: "yoga"` or `sport: "mobility"` and an otherwise valid payload
- **THEN** the system persists the row with that sport and returns `201 Created`
- **AND** the response echoes the sport unchanged (no coercion to `other`/`strength`)

#### Scenario: started_at and ended_at are required and validated

- **WHEN** the client posts a body where `started_at` or `ended_at` is missing or unparseable as RFC 3339
- **THEN** the system returns `400 Bad Request` with `{"error":"window_invalid"}`

#### Scenario: ended_at must be after started_at

- **WHEN** the client posts a body where `ended_at <= started_at`
- **THEN** the system returns `400 Bad Request` with `{"error":"window_invalid"}`

#### Scenario: started_at far in the future is rejected

- **WHEN** the client posts `started_at` more than 24 hours in the future
- **THEN** the system returns `400 Bad Request` with `{"error":"started_at_too_far_future"}`

#### Scenario: kcal_burned if supplied must be positive

- **WHEN** the client posts `kcal_burned` that is zero or negative
- **THEN** the system returns `400 Bad Request` with `{"error":"kcal_burned_invalid"}`

#### Scenario: avg_hr if supplied must be positive integer

- **WHEN** the client posts `avg_hr` that is zero, negative, or non-integer
- **THEN** the system returns `400 Bad Request` with `{"error":"avg_hr_invalid"}`

#### Scenario: tss if supplied must be non-negative

- **WHEN** the client posts `tss` that is negative
- **THEN** the system returns `400 Bad Request` with `{"error":"tss_invalid"}`

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

#### Scenario: Garmin import path passes through with NULLs on rehearsal fields

- **WHEN** the Garmin importer POSTs a workout body that does not include `rpe` or `gi_distress_score` (Garmin does not surface either)
- **THEN** the row is created with both fields `NULL`
- **AND** the user can subsequently PATCH the row to add the rehearsal signals

#### Scenario: POST with all five ingestion metrics stores them

- **WHEN** the client posts `{"external_id":"garmin:555","source":"garmin","sport":"bike","started_at":"2026-06-13T08:00:00Z","ended_at":"2026-06-13T11:00:00Z","distance_m":80500,"avg_power_w":182,"temperature_c":27.5,"sweat_loss_ml":2400,"session_group":"garmin:554"}`
- **THEN** the system creates a row carrying all five values
- **AND** returns `201 Created` with the response body echoing all five fields
- **AND** `distance_m` and `sweat_loss_ml` are rounded to 1 decimal place at the response boundary

#### Scenario: distance_m if supplied must be positive

- **WHEN** the client posts `distance_m` that is zero or negative
- **THEN** the system returns `400 Bad Request` with `{"error":"distance_m_invalid"}`
- **AND** no row is inserted

#### Scenario: avg_power_w if supplied must be a positive integer

- **WHEN** the client posts `avg_power_w` that is zero, negative, or non-integer
- **THEN** the system returns `400 Bad Request` with `{"error":"avg_power_w_invalid"}`
- **AND** no row is inserted

#### Scenario: temperature_c if supplied must be within [-40, 60]

- **WHEN** the client posts `temperature_c` of `-41` or `61` or `98.6`
- **THEN** the system returns `400 Bad Request` with `{"error":"temperature_c_invalid","range":{"min":-40,"max":60}}`
- **AND** no row is inserted

#### Scenario: temperature_c accepts negative values in range

- **WHEN** the client posts `temperature_c: -5.5` (winter session)
- **THEN** the row is created with `temperature_c = -5.5`

#### Scenario: sweat_loss_ml if supplied must be positive

- **WHEN** the client posts `sweat_loss_ml` that is zero or negative
- **THEN** the system returns `400 Bad Request` with `{"error":"sweat_loss_ml_invalid"}`
- **AND** no row is inserted

#### Scenario: session_group must be non-empty and bounded when supplied

- **WHEN** the client posts `session_group` that is empty, whitespace-only, or longer than 255 characters
- **THEN** the system returns `400 Bad Request` with `{"error":"session_group_invalid"}`
- **AND** no row is inserted

#### Scenario: Two legs of a brick share a session_group

- **WHEN** the importer posts a bike leg and a run leg, both with `session_group: "garmin:9876543"` (the multisport parent activity's id)
- **THEN** both rows persist with the same `session_group` value
- **AND** each row keeps its own real `sport`, time window, and metrics (no merged pseudo-workout is created)

#### Scenario: UPSERT full-replace covers the ingestion metrics

- **WHEN** a workout with `external_id: "garmin:555"` exists with `sweat_loss_ml = 2400`
- **AND** the client re-POSTs the same `external_id` with a body that omits `sweat_loss_ml`
- **THEN** the row's `sweat_loss_ml` becomes `NULL` (full-replace of the mutable field set, matching the existing UPSERT semantics)

### Requirement: POST /workouts/bulk upserts an array with per-item results

The system SHALL expose `POST /workouts/bulk` that accepts a batch of workouts and persists each one independently with the same upsert semantics as `POST /workouts`. Each batch item MAY carry optional nested `splits` and `sets` arrays; when present, the item's workout row and its child rows are written in a single transaction, and on an `external_id` match the child rows are fully REPLACED (delete-then-reinsert) so a re-sync never accumulates duplicate laps or sets. Per-item validation and persistence failures are reported per-item; the overall response is `200 OK` whenever the request body is well-formed and within the size cap. Partial failure is allowed.

#### Scenario: Mixed batch produces per-item results

- **WHEN** the client posts `{"workouts": [valid_1, valid_2_with_existing_external_id, invalid_3]}`
- **THEN** the system returns `200 OK` with body shape:
  ```
  {
    "results": [
      {"index": 0, "id": "<uuid>", "created": true},
      {"index": 1, "id": "<uuid>", "created": false},
      {"index": 2, "error": "<code>"}
    ]
  }
  ```
- **AND** the valid items are persisted (item 0 inserted, item 1 updated)
- **AND** the invalid item is NOT persisted
- **AND** later items continue processing even when an earlier item failed

#### Scenario: Each item uses the same external_id UPSERT semantics

- **WHEN** the batch contains an item with an `external_id` matching an existing row
- **THEN** that item's `results` entry has `created: false` and the existing row's `id`
- **AND** the row is updated to the batch item's values

#### Scenario: Nested splits and sets are written and replaced on re-sync

- **WHEN** a batch item carries `external_id: "garmin:1234567"` with `splits: [s0, s1, s2]` and is posted
- **THEN** the workout row is upserted and three `workout_splits` rows are written in the same transaction
- **AND** re-posting the same `external_id` with `splits: [s0', s1']` REPLACES the children, leaving exactly two split rows (the prior three deleted)
- **AND** a strength item carrying `sets: [...]` writes `workout_sets` rows under the same replace-on-resync semantics

#### Scenario: A child-write failure fails only its own item

- **WHEN** one batch item's nested split/set data is invalid
- **THEN** that item's transaction rolls back and its `results` entry carries the error
- **AND** other items in the batch are persisted normally (partial failure preserved)

#### Scenario: Empty array is rejected

- **WHEN** the client posts `{"workouts": []}`
- **THEN** the system returns `400 Bad Request` with `{"error":"bulk_empty"}`

#### Scenario: Batches larger than 100 items are rejected

- **WHEN** the `workouts` array contains more than 100 items
- **THEN** the system returns `400 Bad Request` with `{"error":"bulk_too_large","max":100}`
- **AND** NO items are persisted

#### Scenario: Missing or non-array workouts field is rejected

- **WHEN** the client posts a body without `workouts`, or with `workouts` not a JSON array
- **THEN** the system returns `400 Bad Request` with `{"error":"bulk_invalid"}`

#### Scenario: Per-item errors use the same codes as single POST

- **WHEN** one batch item has `sport: "pilates"` (a value outside the documented sport vocabulary)
- **THEN** its `results` entry is `{"index": <i>, "error": "sport_invalid"}` (matching the single-item POST error code)
