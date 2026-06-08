## ADDED Requirements

### Requirement: Hydration entries are stored in a dedicated table

The system SHALL persist hydration events in a `hydration_entries` table independent of `meal_entries`. Each entry holds a positive `quantity_ml`, a `logged_at` timestamp in UTC, an optional free-text `note`, and create/update timestamps.

#### Scenario: Table is created with the documented columns

- **WHEN** the migration set is applied to a clean database
- **THEN** `hydration_entries` exists with columns `id` (UUID PK), `logged_at` (TIMESTAMPTZ NOT NULL), `quantity_ml` (NUMERIC(10,1) NOT NULL with CHECK `quantity_ml > 0`), `note` (TEXT NULL), `created_at` and `updated_at` (TIMESTAMPTZ NOT NULL DEFAULT now())
- **AND** an index `hydration_entries_logged_at_idx` exists on `(logged_at)`

### Requirement: POST /hydration logs a single entry

The system SHALL expose `POST /hydration` that creates a hydration entry from `{quantity_ml, logged_at, note?}` and accepts the standard `Idempotency-Key` header.

#### Scenario: Successful log

- **WHEN** the client posts `{"quantity_ml": 500, "logged_at": "2026-06-07T08:00:00Z"}`
- **THEN** the system creates a row and returns `201 Created` with the new entry including its generated `id`

#### Scenario: Optional note is accepted

- **WHEN** the client also supplies a `note` (free-text, ≤ 500 chars)
- **THEN** the system stores it on the row and echoes it back in the response

#### Scenario: Non-positive quantity is rejected

- **WHEN** the client posts `quantity_ml` that is zero or negative
- **THEN** the system returns `400 Bad Request` with `{"error":"quantity_ml_invalid"}`

#### Scenario: Missing quantity is rejected

- **WHEN** the client posts a body without `quantity_ml` (or with a non-numeric value)
- **THEN** the system returns `400 Bad Request` with `{"error":"quantity_ml_invalid"}`

#### Scenario: Note longer than 500 characters is rejected

- **WHEN** the client posts a `note` whose length exceeds 500 characters
- **THEN** the system returns `400 Bad Request` with `{"error":"note_too_long"}`

#### Scenario: logged_at far in the future is rejected

- **WHEN** the client posts `logged_at` more than 24 hours in the future
- **THEN** the system returns `400 Bad Request` with `{"error":"logged_at_too_far_future"}`

#### Scenario: Idempotent retry returns the original entry

- **WHEN** two POST requests arrive with the same `Idempotency-Key` and byte-identical body
- **THEN** the second response replays the first response body and status `201`
- **AND** only one `hydration_entries` row exists

### Requirement: GET /hydration lists entries in a window

The system SHALL expose `GET /hydration?from=<rfc3339>&to=<rfc3339>` that returns entries whose `logged_at` falls within the half-open window, ordered by `logged_at` ascending.

#### Scenario: Window filtering returns only entries in range

- **WHEN** the client calls `GET /hydration?from=2026-06-07T00:00:00Z&to=2026-06-08T00:00:00Z`
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

### Requirement: PATCH /hydration/{id} updates a subset of fields

The system SHALL expose `PATCH /hydration/{id}` accepting partial updates of `quantity_ml`, `logged_at`, and `note`. Unknown fields are ignored. Validation rules match the POST endpoint.

#### Scenario: Partial update changes only supplied fields

- **WHEN** the client patches `{"note":"morning water"}` on an existing entry
- **THEN** the response shows the new note
- **AND** other fields remain unchanged

#### Scenario: Patching to an invalid quantity is rejected

- **WHEN** the client patches `{"quantity_ml": -1}`
- **THEN** the system returns `400 Bad Request` with `{"error":"quantity_ml_invalid"}`

#### Scenario: Patch on unknown id returns 404

- **WHEN** the client patches an id that does not exist
- **THEN** the system returns `404 Not Found` with `{"error":"hydration_not_found"}`

### Requirement: DELETE /hydration/{id} removes an entry

The system SHALL expose `DELETE /hydration/{id}` that permanently removes a hydration entry.

#### Scenario: Delete returns 204 on success

- **WHEN** the client deletes an existing entry
- **THEN** the system returns `204 No Content` with an empty body
- **AND** subsequent GETs for that id return `404 hydration_not_found`

#### Scenario: Delete of unknown id returns 404

- **WHEN** the client deletes an id that does not exist
- **THEN** the system returns `404 Not Found` with `{"error":"hydration_not_found"}`

### Requirement: GET /summary/hydration/daily returns a daily total + entries

The system SHALL expose `GET /summary/hydration/daily?date=YYYY-MM-DD&tz=<iana>` that returns the total volume and per-entry list for a single calendar date in the supplied timezone.

#### Scenario: Total sums the day's entries

- **WHEN** the client calls `GET /summary/hydration/daily?date=2026-06-07&tz=Europe/Berlin`
- **AND** three entries logged on that calendar date (in the supplied TZ) totalled `500 + 750 + 1000 = 2250 ml`
- **THEN** the response is `200 OK` with body containing `{"date":"2026-06-07","tz":"Europe/Berlin","total_ml":2250,"entry_count":3,"entries":[...]}`
- **AND** `entries` is ordered by `logged_at` ascending

#### Scenario: Empty day returns total 0 and empty entries

- **WHEN** no entries fall within the day window
- **THEN** the response is `200 OK` with `total_ml: 0`, `entry_count: 0`, `entries: []`

#### Scenario: Missing tz falls back to DEFAULT_USER_TZ and logs a warning

- **WHEN** the client omits `tz` and `DEFAULT_USER_TZ=Europe/Berlin` is set
- **THEN** the system uses `Europe/Berlin` for the day window
- **AND** the response `tz` field reflects `Europe/Berlin`
- **AND** a WARN-level log line records that the fallback was used

#### Scenario: Invalid tz is rejected

- **WHEN** the client supplies a `tz` that does not parse via `time.LoadLocation`
- **THEN** the system returns `400 Bad Request` with `{"error":"tz_invalid"}`

#### Scenario: Invalid date format is rejected

- **WHEN** the client supplies a `date` not matching `YYYY-MM-DD`
- **THEN** the system returns `400 Bad Request` with `{"error":"date_invalid"}`

#### Scenario: Numeric values are rounded to one decimal at serialisation

- **WHEN** the unrounded `total_ml` is `2249.999999`
- **THEN** the response body shows `"total_ml": 2250`
- **AND** the stored entry values are unchanged

### Requirement: Hydration is unit-isolated from nutrition summaries

The system SHALL NOT include hydration data in the nutrition daily summary (`GET /summary/daily`) or the nutrition range summary (`GET /summary/range`). Hydration responses SHALL NOT contain any nutriment fields.

#### Scenario: Nutrition summary does not include hydration

- **WHEN** the client calls `GET /summary/daily?date=…&tz=…`
- **THEN** the response body does not include `total_ml`, `hydration`, or any hydration-related field
- **AND** hydration entries do not contribute to any nutriment total

#### Scenario: Hydration summary does not include nutriment fields

- **WHEN** the client calls `GET /summary/hydration/daily?date=…&tz=…`
- **THEN** the response body does not include `kcal`, `protein_g`, or any other nutriment field
