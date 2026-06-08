# workout-fuel Specification

## Purpose

Define a persisted log of in-session fueling events — gels, electrolyte drinks, salt tabs, caffeine pills, pre-race espresso — captured in their natural units (carbs in g, sodium/potassium/caffeine in mg, optional volume in ml) alongside a required free-text `name` so rehearsal data preserves WHAT was taken. Workout-fuel is the sister capability to `hydration` and `body-weight`: capture-only and deliberately unit-isolated. It is explicitly NOT an extension of `hydration_entries`, because mixing ml with grams and milligrams in a single Totals struct is the canonical footgun that the hydration capability was designed to avoid. Sodium targets during endurance work sit at 300–800 mg/hr and carb-per-hour rates dominate long-ride performance — both are invisible to a ml-only hydration log, and both belong in a structure whose schema makes its units obvious. Workout-fuel data feeds the workout-anchored fueling summary (composed in via `/workouts/{id}/fueling`), but it does NOT roll into the daily hydration or daily nutrition summaries: in-session fueling is its own protocol, distinct from food-choice macro adherence and from baseline hydration.

## Requirements

### Requirement: Workout-fuel entries are stored in a dedicated table

The system SHALL persist in-session fueling events in a `workout_fuel_entries` table independent of meals, hydration, and workouts. Each row carries a free-text `name` (required), an optional `quantity_ml`, and up to four optional measurable nutriment fields (`carbs_g`, `sodium_mg`, `potassium_mg`, `caffeine_mg`), plus an optional `workout_id` link, an optional `note`, and audit timestamps. At least one of `{quantity_ml, carbs_g, sodium_mg, potassium_mg, caffeine_mg}` MUST be set per row — entries with no measurable intake are rejected.

#### Scenario: Table is created with the documented columns

- **WHEN** the migration set is applied to a clean database
- **THEN** `workout_fuel_entries` exists with columns:
  - `id` (UUID PRIMARY KEY)
  - `logged_at` (TIMESTAMPTZ NOT NULL)
  - `name` (TEXT NOT NULL, CHECK `length(name) > 0`)
  - `quantity_ml` (NUMERIC(10, 1) NULL, CHECK `quantity_ml IS NULL OR quantity_ml > 0`)
  - `carbs_g` (NUMERIC(10, 1) NULL, CHECK `carbs_g IS NULL OR carbs_g >= 0`)
  - `sodium_mg` (NUMERIC(10, 1) NULL, CHECK `sodium_mg IS NULL OR sodium_mg >= 0`)
  - `potassium_mg` (NUMERIC(10, 1) NULL, CHECK `potassium_mg IS NULL OR potassium_mg >= 0`)
  - `caffeine_mg` (NUMERIC(10, 1) NULL, CHECK `caffeine_mg IS NULL OR caffeine_mg >= 0`)
  - `note` (TEXT NULL)
  - `workout_id` (UUID NULL REFERENCES workouts(id) ON DELETE SET NULL)
  - `created_at`, `updated_at` (TIMESTAMPTZ NOT NULL DEFAULT now())
- **AND** an index `workout_fuel_entries_logged_at_idx` exists on `(logged_at)`
- **AND** a partial index `workout_fuel_entries_workout_id_idx` exists on `(workout_id) WHERE workout_id IS NOT NULL`

### Requirement: POST /workout-fuel logs a single entry

The system SHALL expose `POST /workout-fuel` that creates a workout-fuel entry from `{name, logged_at, quantity_ml?, carbs_g?, sodium_mg?, potassium_mg?, caffeine_mg?, note?, workout_id?}` and accepts the standard `Idempotency-Key` header.

#### Scenario: Successful log with a gel

- **WHEN** the client posts `{"name":"Maurten Gel 100","logged_at":"2026-06-07T08:45:00Z","carbs_g":25,"sodium_mg":0,"caffeine_mg":100}`
- **THEN** the system creates a row and returns `201 Created` with the new entry including its generated `id`

#### Scenario: Successful log with an electrolyte drink

- **WHEN** the client posts `{"name":"Skratch","logged_at":"2026-06-07T08:30:00Z","quantity_ml":500,"carbs_g":20,"sodium_mg":380}`
- **THEN** the system creates the row and returns `201 Created`
- **AND** the response includes all supplied fields

#### Scenario: Optional workout_id is accepted and validated

- **WHEN** the client posts an entry with `"workout_id": "<existing-uuid>"`
- **THEN** the entry is created with that link
- **AND** the response includes `workout_id`
- **WHEN** the client posts with `"workout_id": "<unknown-uuid>"`
- **THEN** the system returns `400 Bad Request` with `{"error":"workout_not_found"}`
- **AND** no row is created

#### Scenario: Name is required

- **WHEN** the client posts a body without `name` (or with empty string)
- **THEN** the system returns `400 Bad Request` with `{"error":"name_required"}`

#### Scenario: At least one quantitative field is required

- **WHEN** the client posts a body with `name` and `logged_at` only — all of `quantity_ml`, `carbs_g`, `sodium_mg`, `potassium_mg`, `caffeine_mg` are null/omitted
- **THEN** the system returns `400 Bad Request` with `{"error":"empty_entry"}`
- **AND** no row is created

#### Scenario: quantity_ml = 0 is rejected

- **WHEN** the client posts `quantity_ml: 0`
- **THEN** the system returns `400 Bad Request` with `{"error":"quantity_ml_invalid"}`

#### Scenario: quantity_ml < 0 is rejected

- **WHEN** the client posts `quantity_ml: -100`
- **THEN** the system returns `400 Bad Request` with `{"error":"quantity_ml_invalid"}`

#### Scenario: Nutriment fields = 0 are accepted (meaningful "explicitly zero")

- **WHEN** the client posts `caffeine_mg: 0` alongside other supplied fields
- **THEN** the entry is created and `caffeine_mg: 0` is echoed back
- **AND** this is distinct from omitting the field (which would be stored as NULL and not appear on read)

#### Scenario: Nutriment fields < 0 are rejected

- **WHEN** the client posts `sodium_mg: -5` (or any other negative nutriment)
- **THEN** the system returns `400 Bad Request` with `{"error":"<field>_invalid"}` (one of `carbs_g_invalid`, `sodium_mg_invalid`, `potassium_mg_invalid`, `caffeine_mg_invalid`)

#### Scenario: logged_at more than 24h in the future is rejected

- **WHEN** the client posts `logged_at` more than 24 hours in the future
- **THEN** the system returns `400 Bad Request` with `{"error":"logged_at_too_far_future"}`

#### Scenario: note longer than 500 characters is rejected

- **WHEN** `note` is longer than 500 characters
- **THEN** the system returns `400 Bad Request` with `{"error":"note_too_long"}`

#### Scenario: Idempotent retry returns the original entry

- **WHEN** two POST requests arrive with the same `Idempotency-Key` and byte-identical body
- **THEN** the second response replays the first response body and status `201`
- **AND** only one `workout_fuel_entries` row exists

### Requirement: GET /workout-fuel lists entries in a window

The system SHALL expose `GET /workout-fuel?from=<rfc3339>&to=<rfc3339>` that returns entries whose `logged_at` falls within the half-open window, ordered by `logged_at` ascending.

#### Scenario: Window filtering returns only entries in range

- **WHEN** the client calls `GET /workout-fuel?from=…&to=…`
- **THEN** only entries with `from <= logged_at < to` are returned
- **AND** entries outside the window are excluded

#### Scenario: Missing window parameters are rejected

- **WHEN** the client omits `from` or `to`
- **THEN** the system returns `400 Bad Request` with `{"error":"window_required"}`

#### Scenario: Inverted window is rejected

- **WHEN** `from >= to`
- **THEN** the system returns `400 Bad Request` with `{"error":"window_invalid"}`

#### Scenario: Window larger than 92 days is rejected

- **WHEN** the supplied window spans more than 92 days
- **THEN** the system returns `400 Bad Request` with `{"error":"range_too_large","max_days":92}`

#### Scenario: Response wraps the list

- **WHEN** the request is valid
- **THEN** the response body has the shape `{"entries": [Entry, ...]}` (consistent with `/hydration` and `/weight`)

### Requirement: PATCH /workout-fuel/{id} updates a subset of fields

The system SHALL expose `PATCH /workout-fuel/{id}` accepting partial updates of any column except `id`, `created_at`, `updated_at`. `workout_id` supports the empty-string clear semantic established by `add-meal-workout-link`. Validation rules match the POST endpoint, including the at-least-one-quantitative-field requirement if the patch would result in an empty row.

#### Scenario: Partial update changes only supplied fields

- **WHEN** the client patches `{"sodium_mg": 420}` on an existing entry
- **THEN** the response shows the new sodium value
- **AND** all other fields remain unchanged

#### Scenario: PATCH workout_id supports set / clear / no-touch

- **WHEN** the client patches `{"workout_id":"<uuid>"}` for an existing workout
- **THEN** the link is set
- **WHEN** the client patches `{"workout_id":""}`
- **THEN** the link is cleared and `workout_id` is omitted from the response
- **WHEN** the client patches without the `workout_id` field
- **THEN** the previously-stored value is preserved

#### Scenario: PATCH workout_id to a non-existent UUID is rejected

- **WHEN** the client patches `{"workout_id":"<unknown-uuid>"}`
- **THEN** the system returns `400 Bad Request` with `{"error":"workout_not_found"}`

#### Scenario: PATCH that would leave the row with no quantitative field is rejected

- **WHEN** the client patches `{"carbs_g": null, "quantity_ml": null}` such that the resulting row has all of `quantity_ml`, `carbs_g`, `sodium_mg`, `potassium_mg`, `caffeine_mg` null
- **THEN** the system returns `400 Bad Request` with `{"error":"empty_entry"}`
- **AND** the row is NOT updated

#### Scenario: PATCH to an invalid value is rejected

- **WHEN** the client patches `{"carbs_g": -1}`
- **THEN** the system returns `400 Bad Request` with `{"error":"carbs_g_invalid"}`

#### Scenario: PATCH on unknown id returns 404

- **WHEN** the client patches an id that does not exist
- **THEN** the system returns `404 Not Found` with `{"error":"workout_fuel_not_found"}`

### Requirement: DELETE /workout-fuel/{id} removes an entry

The system SHALL expose `DELETE /workout-fuel/{id}` that permanently removes a workout-fuel entry.

#### Scenario: Delete returns 204 on success

- **WHEN** the client deletes an existing entry
- **THEN** the system returns `204 No Content` with an empty body
- **AND** subsequent reads via the list endpoint do not return it

#### Scenario: Delete of unknown id returns 404

- **WHEN** the client deletes an id that does not exist
- **THEN** the system returns `404 Not Found` with `{"error":"workout_fuel_not_found"}`

### Requirement: Deleting a referenced workout clears workout_id on its fuel entries

The system SHALL automatically clear `workout_id` to NULL on workout-fuel rows when the referenced workout is deleted, via the `ON DELETE SET NULL` foreign key. The fuel entry itself is preserved.

#### Scenario: Workout deletion cascades to NULL on workout-fuel rows

- **WHEN** a workout-fuel entry has `workout_id = X` and the workout `X` is deleted via `DELETE /workouts/X`
- **THEN** the entry's `workout_id` becomes NULL automatically
- **AND** subsequent reads of the entry omit the `workout_id` field
- **AND** all other fields of the entry are unchanged

### Requirement: Workout-fuel is unit-isolated from hydration and nutrition summaries

The system SHALL NOT include workout-fuel data in `GET /summary/hydration/daily` or in nutrition `GET /summary/daily` / `GET /summary/range`. Workout-fuel responses SHALL NOT contain nutriment fields that don't belong to the workout-fuel shape (no kcal, no per-100g nutriments).

#### Scenario: Daily hydration summary does not include workout_fuel ml

- **WHEN** a workout-fuel entry exists for date D with `quantity_ml: 500`
- **AND** the client calls `GET /summary/hydration/daily?date=D&tz=…`
- **THEN** the response's `total_ml` does NOT include the 500
- **AND** the response body does not include any workout-fuel fields

#### Scenario: Nutrition daily summary does not include workout_fuel carbs

- **WHEN** a workout-fuel entry exists for date D with `carbs_g: 80`
- **AND** the client calls `GET /summary/daily?date=D&tz=…`
- **THEN** the response's `totals.carbs_g` does NOT include the 80
- **AND** macro adherence is computed without workout-fuel contributions
