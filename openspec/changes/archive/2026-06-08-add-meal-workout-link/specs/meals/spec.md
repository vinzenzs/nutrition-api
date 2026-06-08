## ADDED Requirements

### Requirement: Meal entries may reference a workout

The system SHALL allow a meal entry to carry an optional `workout_id` referencing a row in `workouts`. The link is metadata; meal validation, nutriment computation, snapshot semantics, and CRUD behaviour are otherwise unchanged. When the referenced workout is deleted, the meal's `workout_id` is automatically cleared to NULL (the meal itself stays in the log).

#### Scenario: Meal entries table has a nullable workout_id column

- **WHEN** the migration set is applied
- **THEN** `meal_entries.workout_id` exists as `UUID NULL REFERENCES workouts(id) ON DELETE SET NULL`
- **AND** a partial index `meal_entries_workout_id_idx ON (workout_id) WHERE workout_id IS NOT NULL` exists

#### Scenario: POST /meals accepts workout_id

- **WHEN** the client posts a meal body that includes `"workout_id": "<uuid>"` referencing an existing workout
- **THEN** the system creates the meal with that link
- **AND** the response includes the `workout_id` field
- **AND** the standard `Idempotency-Key` semantics apply unchanged

#### Scenario: POST /meals/freeform accepts workout_id

- **WHEN** the client posts a freeform meal body that includes `workout_id`
- **THEN** the same persist + return behaviour as POST /meals applies

#### Scenario: workout_id omitted persists as null

- **WHEN** the client posts a meal without `workout_id`
- **THEN** the row is stored with `workout_id = NULL`
- **AND** the response omits the `workout_id` field (omitempty)
- **AND** existing-shape consumers see no change

#### Scenario: workout_id referencing an unknown workout is rejected

- **WHEN** the client posts a meal with a `workout_id` that does not exist in `workouts`
- **THEN** the system returns `400 Bad Request` with `{"error":"workout_not_found"}`
- **AND** no meal row is created

#### Scenario: PATCH sets workout_id to a new UUID

- **WHEN** the client patches an existing meal with `{"workout_id":"<uuid>"}` referencing an existing workout
- **THEN** the response shows the new link
- **AND** other fields are unchanged

#### Scenario: PATCH clears workout_id via empty string

- **WHEN** the client patches an existing meal with `{"workout_id":""}`
- **THEN** the response shows the meal with `workout_id` cleared (omitted from the JSON)
- **AND** the row's `workout_id` column is NULL

#### Scenario: PATCH omitting workout_id leaves the link unchanged

- **WHEN** the client patches an existing meal without the `workout_id` field in the body
- **THEN** the previously-stored `workout_id` value is preserved

#### Scenario: PATCH workout_id to a non-existent UUID is rejected

- **WHEN** the client patches with `{"workout_id":"<unknown-uuid>"}`
- **THEN** the system returns `400 Bad Request` with `{"error":"workout_not_found"}`
- **AND** the meal's existing `workout_id` is unchanged

#### Scenario: Deleting a referenced workout clears workout_id on its meals

- **WHEN** a meal exists with `workout_id = X` and the workout `X` is deleted via `DELETE /workouts/X`
- **THEN** the meal row's `workout_id` becomes NULL automatically (FK ON DELETE SET NULL)
- **AND** subsequent reads of the meal omit the `workout_id` field
- **AND** the meal's other fields are unchanged

#### Scenario: GET /meals/{id} returns workout_id when set

- **WHEN** a meal has `workout_id` populated and the client fetches it
- **THEN** the response includes the `workout_id` field

#### Scenario: GET /meals (list) returns workout_id when set per entry

- **WHEN** the client lists meals in a window
- **THEN** each meal in the response includes `workout_id` when set (omitted when null)
