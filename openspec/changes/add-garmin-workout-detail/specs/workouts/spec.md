# workouts Specification (delta)

## ADDED Requirements

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

## MODIFIED Requirements

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
