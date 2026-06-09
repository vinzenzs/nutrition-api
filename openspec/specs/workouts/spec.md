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

The system SHALL expose `POST /workouts` that accepts a workout body and persists it. When `external_id` is present and a row already exists with the same `external_id`, the system UPDATES that row (full-replace of the mutable fields); otherwise the system INSERTS a new row. The mutable field set includes `rpe` and `gi_distress_score` as optional integer-valued per-session signals (1..10 and 1..5 respectively). This semantic lets an external writer "POST every activity it sees" without tracking what is already synced.

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

#### Scenario: List includes rehearsal fields per row

- **WHEN** the client lists workouts in a window containing one rehearsal-tagged ride (with `rpe`/`gi_distress_score` set) and one Garmin-imported ride (both fields `NULL`)
- **THEN** the rehearsal-tagged ride's entry includes `rpe` and `gi_distress_score`
- **AND** the Garmin-imported ride's entry omits both fields (omitempty)

### Requirement: GET /workouts/{id} returns a single workout

The system SHALL expose `GET /workouts/{id}` returning the workout row. The response carries `rpe` and `gi_distress_score` when set on the underlying row; both follow the omitempty pattern when NULL.

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

### Requirement: PATCH /workouts/{id} updates the mutable subset

The system SHALL expose `PATCH /workouts/{id}` accepting partial updates of `name`, `notes`, `kcal_burned`, `avg_hr`, `tss`, `rpe`, and `gi_distress_score`. Validation rules match the POST endpoint for the same fields. The fields `source`, `external_id`, `sport`, `started_at`, and `ended_at` are IMMUTABLE via PATCH. PATCH supports tri-state semantics on the two integer rehearsal fields: `unchanged` when absent from the body, `set` when present with an integer value, and `cleared to NULL` when present with explicit JSON `null`.

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
