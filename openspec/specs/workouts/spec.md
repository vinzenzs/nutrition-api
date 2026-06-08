# workouts Specification

## Purpose

Define a persisted catalogue of training sessions with the minimum metadata nutrition tools need — sport, time window, intensity, and burn. Workouts are a standalone primitive: the backend exposes a minimal write surface (REST endpoints for create/upsert, list, get, patch, delete, and bulk upsert), while the writer (today `garmin.py`, tomorrow potentially Apple Health, Strava, or a manual REST call) lives outside the API. The shape is source-agnostic so any external importer can target it, and `external_id` provides deduplication so a Garmin-style writer can "POST every activity it sees" without tracking what is already synced. Performance analysis (laps, splits, GPS, streams) is explicitly out of scope; this capability stores only what downstream nutrition tools need to answer "what was the athlete doing in window X?".

## Requirements

### Requirement: Workouts are stored in a dedicated table

The system SHALL persist workouts in a `workouts` table independent of meals, hydration, and products. Each row holds a sport, a time window (`started_at`, `ended_at`), provenance metadata, optional intensity/burn signals, and audit timestamps. The table is the data shape that external writers — initially `garmin.py`, in future potentially Apple Health, Strava, or a manual UI — target via the REST endpoints.

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
  - `notes` (TEXT NULL)
  - `created_at` (TIMESTAMPTZ NOT NULL DEFAULT now())
  - `updated_at` (TIMESTAMPTZ NOT NULL DEFAULT now())
- **AND** a CHECK constraint enforces `ended_at > started_at`
- **AND** an index `workouts_started_at_idx` exists on `(started_at)`
- **AND** a partial UNIQUE index exists on `(external_id) WHERE external_id IS NOT NULL`
- **AND** there is NO `intensity` column (TSS is the intensity signal; downstream tools derive bands at call time)

### Requirement: POST /workouts creates or updates a workout via external_id UPSERT

The system SHALL expose `POST /workouts` that accepts a workout body and persists it. When `external_id` is present and a row already exists with the same `external_id`, the system UPDATES that row (full-replace of the mutable fields); otherwise the system INSERTS a new row. This semantic lets an external writer "POST every activity it sees" without tracking what is already synced.

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

### Requirement: GET /workouts lists workouts in a window

The system SHALL expose `GET /workouts?from=<rfc3339>&to=<rfc3339>` that returns workouts whose `started_at` falls in the inclusive window, ordered by `started_at` ascending.

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

### Requirement: GET /workouts/{id} returns a single workout

The system SHALL expose `GET /workouts/{id}` returning the workout row.

#### Scenario: Existing id returns the workout

- **WHEN** the client calls `GET /workouts/<existing-id>`
- **THEN** the response is `200 OK` with the workout body

#### Scenario: Unknown id returns 404

- **WHEN** the client calls `GET /workouts/<unknown-id>`
- **THEN** the system returns `404 Not Found` with `{"error":"workout_not_found"}`

### Requirement: PATCH /workouts/{id} updates the mutable subset

The system SHALL expose `PATCH /workouts/{id}` accepting partial updates of `name`, `notes`, `kcal_burned`, `avg_hr`, and `tss`. Validation rules match the POST endpoint for the same fields. The fields `source`, `external_id`, `sport`, `started_at`, and `ended_at` are IMMUTABLE via PATCH.

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

The system SHALL expose `POST /workouts/bulk` that accepts a batch of workouts and persists each one independently with the same upsert semantics as `POST /workouts`. Per-item validation and persistence failures are reported per-item; the overall response is `200 OK` whenever the request body is well-formed and within the size cap. Partial failure is allowed.

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
