# workouts Specification

## Purpose

Define a persisted catalogue of training sessions with the minimum metadata nutrition tools need — sport, time window, intensity, and burn. Workouts are a standalone primitive: the backend exposes a minimal write surface (REST endpoints for create/upsert, list, get, patch, delete, and bulk upsert), while the writer (today `garmin.py`, tomorrow potentially Apple Health, Strava, or a manual REST call) lives outside the API. The shape is source-agnostic so any external importer can target it, and `external_id` provides deduplication so a Garmin-style writer can "POST every activity it sees" without tracking what is already synced. Performance analysis (laps, splits, GPS, streams) is explicitly out of scope; this capability stores only what downstream nutrition tools need to answer "what was the athlete doing in window X?".
## Requirements
### Requirement: Workouts are stored in a dedicated table

The system SHALL persist workouts in a `workouts` table independent of meals, hydration, and products. Each row holds a sport, a time window (`started_at`, `ended_at`), provenance metadata, a `status` (`planned` or `completed`), optional intensity/burn signals, optional ingestion metrics (distance, average power, ambient temperature, estimated sweat loss), an optional session-group key linking the legs of a brick/multisport session, optional links to a `workout-template` and a training-plan `plan_slot` (for planned workouts originating from a plan), and audit timestamps. The table is the data shape that external writers — initially `garmin.py`, in future potentially Apple Health, Strava, or a manual UI — target via the REST endpoints, and that the training-plan materializer targets for planned sessions.

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

- **WHEN** the client posts a body without `sport`, or with `sport` not in `run|bike|swim|strength|other`
- **THEN** the system returns `400 Bad Request` with `{"error":"sport_invalid"}`

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

### Requirement: GET /workouts lists workouts in a window

The system SHALL expose `GET /workouts?from=<rfc3339>&to=<rfc3339>` that returns workouts whose `started_at` falls in the inclusive window, ordered by `started_at` ascending. An optional `session_group=<key>` query parameter SHALL narrow the result to workouts whose `session_group` equals the supplied key exactly — an additional AND-predicate inside the (still mandatory) window, used to fetch the legs of one brick/multisport session together.

#### Scenario: Window filtering returns only workouts in range

- **WHEN** the client calls `GET /workouts?from=2026-06-07T00:00:00Z&to=2026-06-08T00:00:00Z`
- **THEN** only workouts with `from <= started_at <= to` are returned
- **AND** workouts outside the window are excluded

#### Scenario: Missing window parameters are rejected

- **WHEN** the client omits `from` or `to`
- **THEN** the system returns `400 Bad Request` with `{"error":"window_required"}`

#### Scenario: Inverted window is rejected

- **WHEN** `from > to`
- **THEN** the system returns `400 Bad Request` with `{"error":"window_invalid"}`

#### Scenario: Window larger than 92 days is rejected

- **WHEN** the supplied window spans more than 92 days
- **THEN** the system returns `400 Bad Request` with `{"error":"range_too_large","max_days":92}`

#### Scenario: Response wraps the list

- **WHEN** the request is valid
- **THEN** the response body has the shape `{"workouts": [Workout, ...]}` (consistent with `/meals` and `/hydration`)

#### Scenario: List includes rehearsal fields per row

- **WHEN** the client lists workouts in a window containing one rehearsal-tagged ride (with `rpe`/`gi_distress_score` set) and one Garmin-imported ride (both fields `NULL`)
- **THEN** the rehearsal-tagged ride's entry includes `rpe` and `gi_distress_score`
- **AND** the Garmin-imported ride's entry omits both fields (omitempty)

#### Scenario: session_group filter returns only matching legs

- **WHEN** a window contains a bike leg and a run leg with `session_group: "garmin:9876543"` plus an unrelated swim with `session_group = NULL`
- **AND** the client calls `GET /workouts?from=…&to=…&session_group=garmin:9876543`
- **THEN** exactly the two legs are returned, ordered by `started_at` ascending (leg order)
- **AND** the swim is excluded

#### Scenario: session_group filter still requires the window

- **WHEN** the client calls `GET /workouts?session_group=garmin:9876543` without `from`/`to`
- **THEN** the system returns `400 Bad Request` with `{"error":"window_required"}` (the filter composes with, and does not replace, the window contract)

#### Scenario: session_group filter matching nothing returns an empty list

- **WHEN** no workout in the window carries the supplied `session_group`
- **THEN** the response is `200 OK` with `{"workouts": []}`

#### Scenario: List includes ingestion metrics per row (omitempty)

- **WHEN** the client lists a window containing one Garmin-imported ride with `distance_m`, `avg_power_w`, `temperature_c`, `sweat_loss_ml` set and one manual gym session with all five NULL
- **THEN** the ride's entry includes the set fields
- **AND** the gym session's entry omits all five keys

### Requirement: GET /workouts/{id} returns a single workout

The system SHALL expose `GET /workouts/{id}` returning the workout row, including the scalar performance and HR-zone fields inline (when set) and the nested `splits` and `sets` detail arrays (each ordered by its index; empty arrays omitted). The response carries `rpe` and `gi_distress_score` when set on the underlying row; all detail and rehearsal fields follow the omitempty pattern when NULL/absent.

#### Scenario: Existing id returns the workout

- **WHEN** the client calls `GET /workouts/<existing-id>`
- **THEN** the response is `200 OK` with the workout body

#### Scenario: Unknown id returns 404

- **WHEN** the client calls `GET /workouts/<unknown-id>`
- **THEN** the system returns `404 Not Found` with `{"error":"workout_not_found"}`

#### Scenario: GET on a workout with rpe and gi_distress_score returns them

- **WHEN** a workout has `rpe = 7` and `gi_distress_score = 2`
- **AND** the client calls `GET /workouts/{id}`
- **THEN** the response is `200 OK` with body that includes `"rpe": 7` and `"gi_distress_score": 2`

#### Scenario: GET on a workout with NULL rehearsal fields omits them

- **WHEN** a workout has both fields `NULL`
- **AND** the client calls `GET /workouts/{id}`
- **THEN** the response body does NOT include the `rpe` or `gi_distress_score` keys

#### Scenario: GET returns scalar, zone, and nested detail when present

- **WHEN** a workout has elevation/normalized-power/zone columns set and three `workout_splits` rows
- **AND** the client calls `GET /workouts/{id}`
- **THEN** the body includes the scalar fields, `secs_in_zone_1..5`, and a `splits` array of three entries ordered by `split_index`
- **AND** a strength workout with sets returns a `sets` array ordered by `set_index`

#### Scenario: GET omits detail keys when no detail exists

- **WHEN** a workout has no detail columns set and no split/set rows
- **AND** the client calls `GET /workouts/{id}`
- **THEN** the scalar/zone keys are omitted and no empty `splits`/`sets` arrays appear

### Requirement: PATCH /workouts/{id} updates the mutable subset

The system SHALL expose `PATCH /workouts/{id}` accepting partial updates of `name`, `notes`, `kcal_burned`, `avg_hr`, `tss`, `rpe`, `gi_distress_score`, `distance_m`, `avg_power_w`, `temperature_c`, `sweat_loss_ml`, and `session_group`. Validation rules match the POST endpoint for the same fields. The fields `source`, `external_id`, `sport`, `started_at`, and `ended_at` are IMMUTABLE via PATCH. PATCH supports tri-state semantics on the two integer rehearsal fields AND on the five ingestion fields: `unchanged` when absent from the body, `set` when present with a value, and `cleared to NULL` when present with explicit JSON `null`.

#### Scenario: Partial update changes only supplied mutable fields

- **WHEN** the client patches `{"tss":85,"notes":"FTP changed last month; updated TSS"}` on an existing workout
- **THEN** the response shows the new TSS and notes
- **AND** other fields remain unchanged

#### Scenario: Patching an immutable field is rejected

- **WHEN** the client patches a body containing any of `source`, `external_id`, `sport`, `started_at`, `ended_at`
- **THEN** the system returns `400 Bad Request` with `{"error":"field_immutable","field":"<offending-field>"}`

#### Scenario: Patching to a negative tss is rejected

- **WHEN** the client patches `{"tss":-1}`
- **THEN** the system returns `400 Bad Request` with `{"error":"tss_invalid"}`

#### Scenario: Patch on unknown id returns 404

- **WHEN** the client patches an id that does not exist
- **THEN** the system returns `404 Not Found` with `{"error":"workout_not_found"}`

#### Scenario: PATCH sets rpe and gi_distress_score on an existing workout

- **WHEN** a workout exists with both fields `NULL`
- **AND** the client patches `{"rpe": 7, "gi_distress_score": 2}`
- **THEN** the row's `rpe = 7` and `gi_distress_score = 2`
- **AND** the response is `200 OK` with the updated workout

#### Scenario: PATCH absent rehearsal fields leaves them unchanged

- **WHEN** a workout has `rpe = 7` and `gi_distress_score = 2`
- **AND** the client patches `{"notes": "felt strong"}` (no rpe / no gi_distress_score)
- **THEN** the row's `rpe` and `gi_distress_score` are unchanged
- **AND** `notes` is updated to `"felt strong"`

#### Scenario: PATCH explicit null clears the rehearsal field to NULL

- **WHEN** a workout has `rpe = 7`
- **AND** the client patches `{"rpe": null}`
- **THEN** the row's `rpe` becomes `NULL`
- **AND** subsequent GET responses omit the `rpe` field

#### Scenario: PATCH rpe out of range is rejected without touching other fields

- **WHEN** the client patches `{"rpe": 11, "gi_distress_score": 3}`
- **THEN** the system returns `400 Bad Request` with `{"error":"rpe_invalid","range":{"min":1,"max":10}}`
- **AND** no field is updated (transactional validation — the GI score is NOT written even though it's valid)

#### Scenario: PATCH sets ingestion metrics on an existing workout

- **WHEN** a workout exists with all five ingestion fields `NULL`
- **AND** the client patches `{"sweat_loss_ml": 1850, "temperature_c": 31}`
- **THEN** the row's `sweat_loss_ml = 1850` and `temperature_c = 31`
- **AND** the other three ingestion fields remain `NULL`
- **AND** the response is `200 OK` with the updated workout

#### Scenario: PATCH explicit null clears an ingestion field

- **WHEN** a workout has `session_group = "garmin:9876543"` (grouped by mistake)
- **AND** the client patches `{"session_group": null}`
- **THEN** the row's `session_group` becomes `NULL`
- **AND** subsequent GET responses omit the `session_group` field
- **AND** the same null-clears semantics apply to `distance_m`, `avg_power_w`, `temperature_c`, and `sweat_loss_ml`

#### Scenario: PATCH ingestion field validation matches POST

- **WHEN** the client patches `{"temperature_c": 98.6}` or `{"distance_m": -100}` or `{"session_group": ""}`
- **THEN** the system returns `400 Bad Request` with the corresponding error code (`temperature_c_invalid`, `distance_m_invalid`, `session_group_invalid`)
- **AND** no field is updated

### Requirement: DELETE /workouts/{id} removes the row

The system SHALL expose `DELETE /workouts/{id}` that permanently removes a workout.

#### Scenario: Delete returns 204 on success

- **WHEN** the client deletes an existing workout
- **THEN** the system returns `204 No Content` with an empty body
- **AND** subsequent GETs for that id return `404 workout_not_found`

#### Scenario: Delete of unknown id returns 404

- **WHEN** the client deletes an id that does not exist
- **THEN** the system returns `404 Not Found` with `{"error":"workout_not_found"}`

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

- **WHEN** one batch item has `sport: "yoga"`
- **THEN** its `results` entry is `{"index": <i>, "error": "sport_invalid"}` (matching the single-item POST error code)

### Requirement: Workouts are source-tagged but source-agnostic in shape

The system SHALL accept workouts from any external writer through the same endpoint and shape. The `source` field records provenance (`garmin`, `manual`, `other`) but does NOT affect persistence semantics, validation rules, or returned shape — a Garmin-sourced workout and a manual workout are stored in the same table with the same columns.

#### Scenario: A manual workout has the same response shape as a Garmin workout

- **WHEN** one workout is posted with `source: "manual"` and another with `source: "garmin"` (otherwise-identical fields)
- **THEN** both responses have the same JSON keys
- **AND** both appear in `GET /workouts` results equally

#### Scenario: source: "other" is accepted for unanticipated writers

- **WHEN** a future writer (e.g. Apple Health bridge) posts with `source: "other"`
- **THEN** the system accepts the workout
- **AND** the writer is responsible for any source-specific conventions outside the API

### Requirement: GET /workouts/{id}/fueling returns pre/intra/post intake windows

The system SHALL expose `GET /workouts/{id}/fueling?pre_window_min=<int>&post_window_min=<int>` returning three time-anchored buckets — pre, intra, post — each carrying **three** separate aggregations for entries whose `logged_at` falls within the corresponding window: a **nutrition** sub-object (from `meal_entries`), a **hydration** sub-object (from `hydration_entries`), and a **workout_fuel** sub-object (from `workout_fuel_entries`). The windows are derived from the workout's `started_at` and `ended_at` plus the supplied (or defaulted) pre/post minutes. Aggregation is time-window-based: any entry whose `logged_at` falls in a window is included regardless of its `workout_id` value.

#### Scenario: Default windows are 240 min pre / 60 min post

- **WHEN** the client calls `GET /workouts/{id}/fueling` without `pre_window_min` or `post_window_min`
- **THEN** `pre_window.minutes` is `240`
- **AND** `post_window.minutes` is `60`

#### Scenario: Response shape carries three separate sub-objects per window

- **WHEN** the response is well-formed
- **THEN** each window object has the shape `{start, end, minutes, nutrition: {totals, entry_count}, hydration: {total_ml, entry_count}, workout_fuel: {totals, entry_count}}`
- **AND** the `nutrition.totals` shape matches `/summary/daily.totals` (macros + nullable micros)
- **AND** the `workout_fuel.totals` shape carries `{quantity_ml, carbs_g, sodium_mg, potassium_mg, caffeine_mg}` — each field nullable, summed across contributing entries
- **AND** units never mix: no ml inside `nutrition.totals`; no kcal inside `hydration` or `workout_fuel`; no per-100g nutriments inside `workout_fuel`

#### Scenario: Workout-fuel sub-object sums contributing entries

- **WHEN** two workout-fuel entries fall in the intra window with `{carbs_g: 25, sodium_mg: 100}` and `{carbs_g: 25, sodium_mg: 200, quantity_ml: 500}`
- **THEN** `intra_window.workout_fuel.totals` is `{carbs_g: 50, sodium_mg: 300, quantity_ml: 500}`
- **AND** `intra_window.workout_fuel.entry_count` is `2`
- **AND** `quantity_ml` is summed across only those entries that supplied it; `null + 500 = 500`, not `null`

#### Scenario: Workout-fuel sub-object is present even when there are no contributing entries

- **WHEN** no workout-fuel entries fall in a particular window
- **THEN** `workout_fuel.entry_count` is `0`
- **AND** `workout_fuel.totals` carries zeros (or nulls) for every field — the sub-object is NOT omitted

#### Scenario: Pre-window covers [started_at − pre_window_min, started_at)

- **WHEN** a meal is logged 30 minutes before the workout's `started_at`
- **AND** `pre_window_min >= 30`
- **THEN** the meal contributes to `pre_window.nutrition.totals`
- **AND** the meal does NOT contribute to `intra_window` or `post_window`

#### Scenario: Intra-window covers [started_at, ended_at)

- **WHEN** a hydration entry is logged at a time T with `started_at <= T < ended_at`
- **THEN** the entry contributes to `intra_window.hydration.total_ml`

#### Scenario: Post-window covers [ended_at, ended_at + post_window_min)

- **WHEN** a meal is logged 30 minutes after `ended_at`
- **AND** `post_window_min >= 30`
- **THEN** the meal contributes to `post_window.nutrition.totals`

#### Scenario: Boundary at started_at lands in intra_window

- **WHEN** a workout-fuel entry is logged at exactly `workout.started_at`
- **THEN** the entry contributes to `intra_window.workout_fuel` (not `pre_window`)
- **AND** the response documents the half-open convention

#### Scenario: Boundary at ended_at lands in post_window

- **WHEN** a workout-fuel entry is logged at exactly `workout.ended_at`
- **THEN** the entry contributes to `post_window.workout_fuel` (intra window is `[started_at, ended_at)`)

#### Scenario: Entries with workout_id but outside the time window are excluded

- **WHEN** any intake row (meal, hydration, or workout-fuel) has `workout_id = X` but is logged 8 hours before workout X's `started_at`
- **AND** `pre_window_min = 240` (4h, default)
- **THEN** the row does NOT appear in the fueling totals for any window
- **AND** the response shape is unchanged (no "tagged-but-outside" bucket)

#### Scenario: Entries without workout_id but inside the time window are included

- **WHEN** any intake row has `workout_id = NULL` but `logged_at` falls inside the pre-window
- **THEN** the row contributes to `pre_window.<sub-object>.totals` (time-window matching, not tag matching)

#### Scenario: Empty windows return zero totals and entry_count

- **WHEN** a workout has no meals, hydration, or workout-fuel in any window
- **THEN** every window returns `entry_count: 0` and zero totals across all three sub-objects
- **AND** the response status is `200 OK`

#### Scenario: Workout not found returns 404

- **WHEN** the client calls `GET /workouts/<unknown-uuid>/fueling`
- **THEN** the system returns `404 Not Found` with `{"error":"workout_not_found"}`

#### Scenario: pre_window_min and post_window_min are bounded [0, 720]

- **WHEN** `pre_window_min` or `post_window_min` is outside `[0, 720]`
- **THEN** the system returns `400 Bad Request` with `{"error":"window_invalid","range":{"min":0,"max":720}}`

#### Scenario: pre_window_min = 0 returns an empty pre-window

- **WHEN** the client passes `pre_window_min=0`
- **THEN** `pre_window.minutes` is `0`
- **AND** every sub-object's `entry_count` is `0`
- **AND** the same applies symmetrically for `post_window_min=0`

#### Scenario: Numeric fields are rounded at the response boundary

- **WHEN** any aggregated total resolves to `419.7666…`
- **THEN** the response shows `419.8` (matching the existing nutrient-rounding rule)
- **AND** hydration `total_ml` and workout_fuel `quantity_ml` are rounded to 1 decimal place

### Requirement: GET /workouts/{id}/fueling surfaces rehearsal signals on the workout

The system SHALL include `rpe`, `gi_distress_score`, `sweat_loss_ml`, and `temperature_c` on the `GET /workouts/{id}/fueling` response so the agent can read the rehearsal-outcome signals and the sweat/heat context alongside the fueling totals in a single call. The fields are echoed at the top level of the response, alongside `workout_id`, `started_at`, `ended_at`, and follow the same omitempty rule as everywhere else — absent when NULL on the underlying workout row. `distance_m`, `avg_power_w`, and `session_group` are deliberately NOT echoed here: they are not inputs to fueling-adequacy judgment, and the capability excludes performance analysis.

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

#### Scenario: Fueling response carries sweat_loss_ml and temperature_c when set

- **WHEN** a workout has `sweat_loss_ml = 2400` and `temperature_c = 27.5`
- **AND** the client calls `GET /workouts/{id}/fueling`
- **THEN** the response body includes `"sweat_loss_ml": 2400` and `"temperature_c": 27.5` at the top level
- **AND** the agent can compare `sweat_loss_ml` against the summed fluid intake (`hydration.total_ml` + `workout_fuel.totals.quantity_ml` across the windows) in one call

#### Scenario: Fueling response omits sweat/heat context when NULL and never echoes performance fields

- **WHEN** a workout has `sweat_loss_ml` and `temperature_c` `NULL` but `distance_m` and `avg_power_w` set
- **AND** the client calls `GET /workouts/{id}/fueling`
- **THEN** the response body omits `sweat_loss_ml` and `temperature_c`
- **AND** the response body does NOT contain `distance_m`, `avg_power_w`, or `session_group` keys regardless of their values on the row

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

### Requirement: Planned workouts can originate from a training-plan slot via a slot-keyed upsert

The system SHALL support upserting a planned workout from a training-plan slot,
keyed on `plan_slot_id`. Such a row SHALL carry `status='planned'`, the slot's
template's `sport` and `name`, a `template_id`, and a `plan_slot_id`. Because the
key is `plan_slot_id` and imported activities never carry one, this path is
disjoint from the existing `external_id` UPSERT path: the two never collide.
Repeated upserts on the same slot SHALL update the same row rather than create a
new one. The upsert's update SHALL apply only where the existing row's `status`
is `planned`, so a workout already marked `completed` is never reverted or
overwritten by re-materialization.

#### Scenario: The slot upsert does not overwrite a completed workout

- **WHEN** a planned workout for a slot has been marked `completed` and the slot
  is upserted again
- **THEN** the existing completed row is left unchanged (the update is guarded by
  `status='planned'`)

#### Scenario: A planned workout upserts by slot, not external_id

- **WHEN** the training-plan materializer upserts a planned workout for a given
  `plan_slot_id` twice
- **THEN** exactly one planned `workouts` row exists for that slot, updated in place

#### Scenario: The slot-keyed and external_id paths do not collide

- **WHEN** a completed activity (with `external_id`, no `plan_slot_id`) and a
  planned workout (with `plan_slot_id`, no `external_id`) exist for the same date
- **THEN** both rows persist independently, each addressable by its own key

### Requirement: Workouts track Garmin scheduling identifiers

The system SHALL add two nullable columns to `workouts`: `garmin_workout_id`
(the id of the structured workout created in the Garmin library) and
`garmin_schedule_id` (the id of the calendar entry that schedules it). Both are
opaque Garmin identifiers — stored and echoed, never parsed. They are populated
when a planned workout is pushed to the watch and cleared when it is
unscheduled, enabling clean unschedule and re-push without double-creating in the
Garmin library.

#### Scenario: Columns exist after migration

- **WHEN** the migration set is applied to a clean database
- **THEN** `workouts` has `garmin_workout_id` (TEXT NULL) and `garmin_schedule_id` (TEXT NULL)

#### Scenario: Ids are set on push and cleared on unschedule

- **WHEN** a planned workout is pushed to the watch and later unscheduled
- **THEN** both ids are populated by the push
- **AND** both ids are null after the unschedule

### Requirement: Garmin imports reconcile against open planned workouts

The system SHALL reconcile an ingested completed Garmin activity against an open
planned workout. When a completed activity is ingested via `POST /workouts` or
`POST /workouts/bulk` with `source='garmin'` and its `external_id` is not already
stored, the system SHALL attempt to match exactly one **open planned workout** —
a row with `status='planned'`, `external_id IS NULL`, the same sport, and the
same **local calendar day** as the activity's start. On exactly one match the
system SHALL **fulfill** that planned workout in place: set its `external_id`,
`source`, and actual metrics from the activity, flip `status` to `completed`, and
retain its `template_id` and `plan_slot_id`; no new row is created. On no match
the system SHALL insert a standalone completed row (the prior behavior). On more
than one candidate the system SHALL insert a standalone completed row and mark it
as needing a link rather than guess. The match SHALL run only on first sight; a
subsequent re-sync of the same activity follows the existing `external_id` UPSERT
path.

#### Scenario: A completed import fulfills the matching planned workout

- **WHEN** a `garmin` activity is ingested for a sport and local day on which
  exactly one open planned workout exists
- **THEN** that planned workout is updated to `status='completed'` with the
  activity's `external_id`, `source`, and actual metrics
- **AND** its `template_id` and `plan_slot_id` are retained
- **AND** no second row is created

#### Scenario: No matching planned workout creates a standalone row

- **WHEN** a `garmin` activity is ingested and no open planned workout matches its
  sport and local day
- **THEN** a standalone completed workout is created (the prior behavior)

#### Scenario: Ambiguous match is flagged, not guessed

- **WHEN** a `garmin` activity matches more than one open planned workout of the
  same sport on the same local day
- **THEN** a standalone completed workout is created and marked as needing a link
- **AND** no planned workout is auto-fulfilled

#### Scenario: Re-sync of a fulfilled activity is idempotent

- **WHEN** the daily sync re-sends an activity whose `external_id` is already
  stored (on a fulfilled planned row)
- **THEN** ingestion follows the existing `external_id` UPSERT path and updates
  that row in place
- **AND** reconciliation does not run again

#### Scenario: Matching uses local calendar day and exact sport

- **WHEN** an activity starts late in the local evening
- **THEN** it is matched against planned workouts on that local date (not the UTC
  date)
- **AND** only planned workouts of the same sport are considered

### Requirement: Explicit fulfill and unfulfill endpoints

The system SHALL expose `POST /workouts/{plannedId}/fulfill` accepting a
`completed_id`, which merges an existing completed activity into an existing
planned workout (copying `external_id`, `source`, and actuals onto the planned
row, flipping it to `completed`, removing the redundant standalone row, and
clearing any needs-link flag); and `POST /workouts/{id}/unfulfill`, which
reverses a merge (clearing `external_id` and actuals and restoring
`status='planned'`). The planned row is the surviving identity in a merge so its
`plan_slot_id` remains stable.

#### Scenario: Manual fulfill merges two existing rows

- **WHEN** a client `POST`s `/workouts/{plannedId}/fulfill` with the id of a
  standalone completed activity of the same session
- **THEN** the planned workout becomes `completed` with the activity's
  `external_id` and actuals
- **AND** the standalone completed row is removed
- **AND** the planned workout's `plan_slot_id` is unchanged

#### Scenario: Unfulfill restores the planned workout

- **WHEN** a client `POST`s `/workouts/{id}/unfulfill` on a fulfilled workout
- **THEN** its `external_id` and actual metrics are cleared and `status` returns
  to `planned`
- **AND** the row retains its `template_id` and `plan_slot_id`

#### Scenario: Fulfill clears the needs-link flag

- **WHEN** a flagged (needs-link) completed activity is merged via `fulfill`
- **THEN** the needs-link flag is cleared on the surviving row

### Requirement: Workouts carry per-activity detail columns and child tables

The system SHALL persist richer per-activity detail alongside each workout: scalar performance fields, HR-zone time, and ambient-weather fields (humidity, wind — complementing the existing `temperature_c`) as columns on the `workouts` row, and per-lap splits and per-set strength data in dedicated child tables. This narrows the capability's prior "no performance analysis" exclusion to **streams/GPS only** — laps, HR-zone distribution, strength sets, and in-session weather are now in scope because they feed nutrition fueling math (carbohydrate-oxidation rate, glycogen cost, and especially sweat-rate, where humidity is a primary driver alongside temperature), not generic performance analytics. All detail is nullable/optional: "not measured" remains a meaningful state, never a data-quality bug.

#### Scenario: Detail columns are added to the workouts table

- **WHEN** the migration set is applied to a clean database
- **THEN** `workouts` carries the additional nullable columns:
  - `elevation_gain_m` (NUMERIC(8, 1) NULL, CHECK `elevation_gain_m IS NULL OR elevation_gain_m >= 0`)
  - `elevation_loss_m` (NUMERIC(8, 1) NULL, CHECK `elevation_loss_m IS NULL OR elevation_loss_m >= 0`)
  - `normalized_power_w` (INTEGER NULL, CHECK `normalized_power_w IS NULL OR normalized_power_w > 0`)
  - `intensity_factor` (NUMERIC(4, 2) NULL, CHECK `intensity_factor IS NULL OR intensity_factor >= 0`)
  - `avg_cadence` (INTEGER NULL, CHECK `avg_cadence IS NULL OR avg_cadence > 0`)
  - `avg_stride_m` (NUMERIC(5, 2) NULL, CHECK `avg_stride_m IS NULL OR avg_stride_m > 0`)
  - `max_hr` (INTEGER NULL, CHECK `max_hr IS NULL OR max_hr > 0`)
  - `aerobic_te` (NUMERIC(3, 1) NULL, CHECK `aerobic_te IS NULL OR aerobic_te >= 0`)
  - `anaerobic_te` (NUMERIC(3, 1) NULL, CHECK `anaerobic_te IS NULL OR anaerobic_te >= 0`)
  - `secs_in_zone_1` … `secs_in_zone_5` (INTEGER NULL each, CHECK `IS NULL OR >= 0`)
  - `humidity_pct` (NUMERIC(5, 1) NULL, CHECK `humidity_pct IS NULL OR (humidity_pct BETWEEN 0 AND 100)`)
  - `wind_speed_mps` (NUMERIC(5, 1) NULL, CHECK `wind_speed_mps IS NULL OR wind_speed_mps >= 0`)
- **AND** every existing row carries NULL for all of them
- **AND** the migration succeeds without back-filling any of them

#### Scenario: workout_splits child table is created

- **WHEN** the migration is applied to a clean database
- **THEN** `workout_splits` exists with columns:
  - `id` (UUID PRIMARY KEY)
  - `workout_id` (UUID NOT NULL, REFERENCES `workouts(id)` ON DELETE CASCADE)
  - `split_index` (INTEGER NOT NULL, CHECK `split_index >= 0`)
  - `distance_m` (NUMERIC(10, 1) NULL)
  - `duration_s` (NUMERIC(10, 1) NULL)
  - `avg_hr` (INTEGER NULL)
  - `avg_power_w` (INTEGER NULL)
  - `avg_speed_mps` (NUMERIC(8, 3) NULL)
  - `elevation_gain_m` (NUMERIC(8, 1) NULL)
- **AND** an index `workout_splits_workout_id_idx` exists on `(workout_id)`
- **AND** a UNIQUE index exists on `(workout_id, split_index)`

#### Scenario: workout_sets child table is created

- **WHEN** the migration is applied to a clean database
- **THEN** `workout_sets` exists with columns:
  - `id` (UUID PRIMARY KEY)
  - `workout_id` (UUID NOT NULL, REFERENCES `workouts(id)` ON DELETE CASCADE)
  - `set_index` (INTEGER NOT NULL, CHECK `set_index >= 0`)
  - `exercise_name` (TEXT NULL)
  - `exercise_category` (TEXT NULL)
  - `reps` (INTEGER NULL, CHECK `reps IS NULL OR reps >= 0`)
  - `weight_kg` (NUMERIC(6, 2) NULL, CHECK `weight_kg IS NULL OR weight_kg >= 0`)
  - `duration_s` (NUMERIC(10, 1) NULL)
- **AND** an index `workout_sets_workout_id_idx` exists on `(workout_id)`
- **AND** a UNIQUE index exists on `(workout_id, set_index)`

#### Scenario: Detail floats are rounded at the response boundary

- **WHEN** a workout with detail is serialized to a response
- **THEN** every nutrient/measurement float in the scalar, zone, split, and set fields is rounded with `numfmt.Round1` at the boundary, consistent with the rest of the workouts shape
- **AND** the detail fields are NEVER merged into `summary`'s Totals struct (unit isolation preserved)

#### Scenario: List carries scalar and zone fields but never nested detail

- **WHEN** workouts in a window have detail columns set and child split/set rows
- **AND** the client calls `GET /workouts?from=…&to=…`
- **THEN** each listed workout includes the scalar performance and `secs_in_zone_*` fields when set (omitempty when NULL)
- **AND** no `splits` or `sets` arrays appear in the list response (nested detail is single-get only, to keep list payloads bounded)

#### Scenario: Detail attaches to the reconciled row, not a duplicate

- **WHEN** a planned workout exists for the day and a Garmin import carrying nested `splits`/`sets` reconciles into it (the existing reconciliation merges the import planned→completed in place, keeping `template_id`/`plan_slot_id`)
- **THEN** the scalar/zone columns and the child split/set rows attach to the surviving reconciled workout row (the planned row that was completed), NOT to a second inserted row
- **AND** a subsequent re-sync of the same activity replaces that row's children in place (no duplication across the reconcile + re-sync seam)
- **AND** the `external_id` lands on the reconciled row so future imports continue to match it

