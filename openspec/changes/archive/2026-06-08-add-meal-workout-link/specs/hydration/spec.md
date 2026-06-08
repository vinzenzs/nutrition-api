## ADDED Requirements

### Requirement: Hydration entries may reference a workout

The system SHALL allow a hydration entry to carry an optional `workout_id` referencing a row in `workouts`. The link is metadata; hydration validation, daily-summary aggregation, and CRUD behaviour are otherwise unchanged. When the referenced workout is deleted, the entry's `workout_id` is automatically cleared to NULL.

#### Scenario: Hydration entries table has a nullable workout_id column

- **WHEN** the migration set is applied
- **THEN** `hydration_entries.workout_id` exists as `UUID NULL REFERENCES workouts(id) ON DELETE SET NULL`
- **AND** a partial index `hydration_entries_workout_id_idx ON (workout_id) WHERE workout_id IS NOT NULL` exists

#### Scenario: POST /hydration accepts workout_id

- **WHEN** the client posts a hydration body that includes `"workout_id": "<uuid>"` referencing an existing workout
- **THEN** the system creates the entry with that link
- **AND** the response includes the `workout_id` field
- **AND** the standard `Idempotency-Key` semantics apply unchanged

#### Scenario: workout_id omitted persists as null

- **WHEN** the client posts a hydration entry without `workout_id`
- **THEN** the row is stored with `workout_id = NULL`
- **AND** the response omits the `workout_id` field (omitempty)

#### Scenario: workout_id referencing an unknown workout is rejected

- **WHEN** the client posts a hydration entry with a `workout_id` that does not exist in `workouts`
- **THEN** the system returns `400 Bad Request` with `{"error":"workout_not_found"}`
- **AND** no hydration row is created

#### Scenario: PATCH supports set / clear / no-touch for workout_id

- **WHEN** the client patches a hydration entry with `{"workout_id":"<uuid>"}` for an existing workout
- **THEN** the link is set and the response shows it
- **WHEN** the client patches with `{"workout_id":""}`
- **THEN** the link is cleared and `workout_id` is omitted from the response
- **WHEN** the client patches without the `workout_id` field
- **THEN** the previously-stored value is preserved

#### Scenario: PATCH workout_id to a non-existent UUID is rejected

- **WHEN** the client patches with `{"workout_id":"<unknown-uuid>"}`
- **THEN** the system returns `400 Bad Request` with `{"error":"workout_not_found"}`
- **AND** the entry's existing `workout_id` is unchanged

#### Scenario: Deleting a referenced workout clears workout_id on its hydration entries

- **WHEN** a hydration entry has `workout_id = X` and the workout `X` is deleted
- **THEN** the entry's `workout_id` becomes NULL automatically (FK ON DELETE SET NULL)
- **AND** subsequent reads of the entry omit the `workout_id` field

#### Scenario: GET /hydration returns workout_id when set per entry

- **WHEN** the client lists hydration entries in a window
- **THEN** each entry in the response includes `workout_id` when set (omitted when null)

#### Scenario: Daily hydration summary does NOT filter by workout_id

- **WHEN** the client calls `GET /summary/hydration/daily?date=…&tz=…`
- **THEN** every hydration entry on that date is included in the totals regardless of `workout_id` value
- **AND** the existing daily-summary shape is unchanged (no new fields)
