# body-weight Specification

## Purpose

Define a persisted log of body-weight measurements plus a rolling-average trend endpoint that suppresses daily noise. Body weight is captured independently of meals, hydration, and workouts so that any writer — agent, manual REST call, future smart-scale importer — can record measurements without coupling to nutrition aggregation. The shape is forward-compatible with body-fat-percentage on the same row; further composition fields (lean mass, hydration %, bone mass) are explicit follow-ups when real smart-scale data shows up. The trend endpoint exists because daily weighing noise (hydration, glycogen, post-meal water retention) routinely swings 1–2 kg, and downstream tools (energy availability, race-day fuelling math, weight-loss trajectory) need a smoothed signal plus an honest data-quality indicator (`sample_count`) rather than a single day's reading.

## Requirements

### Requirement: Body-weight entries are stored in a dedicated table

The system SHALL persist body-weight measurements in a `body_weight_entries` table independent of meals, hydration, workouts, and products. Each row holds a positive `weight_kg`, a `logged_at` timestamp in UTC, an optional `body_fat_pct` (0–100), an optional free-text `note`, and audit timestamps. Multiple measurements per calendar date are allowed; the trend endpoint smooths within and across days.

#### Scenario: Table is created with the documented columns

- **WHEN** the migration set is applied to a clean database
- **THEN** `body_weight_entries` exists with columns:
  - `id` (UUID PRIMARY KEY)
  - `logged_at` (TIMESTAMPTZ NOT NULL)
  - `weight_kg` (NUMERIC(5, 2) NOT NULL, CHECK `weight_kg > 0`)
  - `body_fat_pct` (NUMERIC(4, 2) NULL, CHECK `body_fat_pct IS NULL OR (body_fat_pct >= 0 AND body_fat_pct <= 100)`)
  - `note` (TEXT NULL)
  - `created_at` (TIMESTAMPTZ NOT NULL DEFAULT now())
  - `updated_at` (TIMESTAMPTZ NOT NULL DEFAULT now())
- **AND** an index `body_weight_entries_logged_at_idx` exists on `(logged_at)`
- **AND** there is NO uniqueness constraint on `logged_at` or `(logged_at, weight_kg)` — multiple entries per day, even at the same instant, are allowed

### Requirement: POST /weight logs a single measurement

The system SHALL expose `POST /weight` that creates a body-weight entry from `{weight_kg, logged_at, body_fat_pct?, note?}` and accepts the standard `Idempotency-Key` header.

#### Scenario: Successful log

- **WHEN** the client posts `{"weight_kg": 72.5, "logged_at": "2026-06-07T07:00:00Z"}`
- **THEN** the system creates a row and returns `201 Created` with the new entry including its generated `id`

#### Scenario: Optional body_fat_pct is accepted

- **WHEN** the client also supplies `body_fat_pct: 14.2`
- **THEN** the system stores it on the row and echoes it back

#### Scenario: Optional note is accepted

- **WHEN** the client also supplies a `note` (free-text, ≤ 500 chars)
- **THEN** the system stores it and echoes it back

#### Scenario: Missing weight is rejected

- **WHEN** the client posts a body without `weight_kg` (or with a non-numeric value)
- **THEN** the system returns `400 Bad Request` with `{"error":"weight_kg_invalid"}`

#### Scenario: Non-positive weight is rejected

- **WHEN** the client posts `weight_kg` that is zero or negative
- **THEN** the system returns `400 Bad Request` with `{"error":"weight_kg_invalid"}`

#### Scenario: Body-fat % out of range is rejected

- **WHEN** the client posts `body_fat_pct` < 0 or > 100
- **THEN** the system returns `400 Bad Request` with `{"error":"body_fat_pct_invalid"}`

#### Scenario: Note longer than 500 characters is rejected

- **WHEN** the client posts a `note` whose length exceeds 500 characters
- **THEN** the system returns `400 Bad Request` with `{"error":"note_too_long"}`

#### Scenario: logged_at far in the future is rejected

- **WHEN** the client posts `logged_at` more than 24 hours in the future
- **THEN** the system returns `400 Bad Request` with `{"error":"logged_at_too_far_future"}`

#### Scenario: Idempotent retry returns the original entry

- **WHEN** two POST requests arrive with the same `Idempotency-Key` and byte-identical body
- **THEN** the second response replays the first response body and status `201`
- **AND** only one `body_weight_entries` row exists

### Requirement: GET /weight lists entries in a window

The system SHALL expose `GET /weight?from=<rfc3339>&to=<rfc3339>` that returns entries whose `logged_at` falls within the half-open window, ordered by `logged_at` ascending.

#### Scenario: Window filtering returns only entries in range

- **WHEN** the client calls `GET /weight?from=2026-06-01T00:00:00Z&to=2026-06-08T00:00:00Z`
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
- **THEN** the response body has the shape `{"entries": [Entry, ...]}` (consistent with `/hydration`)

### Requirement: PATCH /weight/{id} updates a subset of fields

The system SHALL expose `PATCH /weight/{id}` accepting partial updates of `weight_kg`, `body_fat_pct`, `logged_at`, and `note`. Validation rules match the POST endpoint.

#### Scenario: Partial update changes only supplied fields

- **WHEN** the client patches `{"body_fat_pct": 13.8}` on an existing entry
- **THEN** the response shows the new body-fat %
- **AND** other fields remain unchanged

#### Scenario: Patching to an invalid weight is rejected

- **WHEN** the client patches `{"weight_kg": -1}`
- **THEN** the system returns `400 Bad Request` with `{"error":"weight_kg_invalid"}`

#### Scenario: Patch on unknown id returns 404

- **WHEN** the client patches an id that does not exist
- **THEN** the system returns `404 Not Found` with `{"error":"weight_not_found"}`

### Requirement: DELETE /weight/{id} removes an entry

The system SHALL expose `DELETE /weight/{id}` that permanently removes a body-weight entry.

#### Scenario: Delete returns 204 on success

- **WHEN** the client deletes an existing entry
- **THEN** the system returns `204 No Content` with an empty body
- **AND** subsequent reads via the list endpoint do not return it

#### Scenario: Delete of unknown id returns 404

- **WHEN** the client deletes an id that does not exist
- **THEN** the system returns `404 Not Found` with `{"error":"weight_not_found"}`

### Requirement: GET /weight/trend returns a rolling-average curve

The system SHALL expose `GET /weight/trend?from=YYYY-MM-DD&to=YYYY-MM-DD&window_days=<int>&tz=<iana>` that returns, for each calendar date in `[from, to]` (in the supplied timezone), the trailing rolling average of `weight_kg` over the previous `window_days` days (inclusive of the current date). Each point carries the `sample_count` that fed the average, so callers can distinguish a real trend from a sparse-data mirage.

#### Scenario: Trend smooths daily noise

- **WHEN** the client logs three weights on three consecutive days (e.g. 73.1, 72.4, 73.6)
- **AND** calls `GET /weight/trend?from=<day3>&to=<day3>&window_days=3&tz=UTC`
- **THEN** the response has one `points` entry for day 3
- **AND** `rolling_avg_kg` equals the mean of the three (`73.0`, rounded to 1 dp)
- **AND** `sample_count` is `3`

#### Scenario: Sparse window reports honest sample_count

- **WHEN** only one entry exists in a 7-day trailing window
- **THEN** that date's point has `sample_count: 1`
- **AND** `rolling_avg_kg` equals that single sample

#### Scenario: Empty window returns null rolling_avg with sample_count 0

- **WHEN** no entries fall within a date's trailing window
- **THEN** the point has `rolling_avg_kg: null` and `sample_count: 0`
- **AND** the response point is still present (one per date in `[from, to]`)

#### Scenario: window_days defaults to 7

- **WHEN** the client omits `window_days`
- **THEN** the system uses `window_days = 7`

#### Scenario: window_days out of range is rejected

- **WHEN** `window_days` is below `1` or above `30`
- **THEN** the system returns `400 Bad Request` with `{"error":"window_days_invalid","range":{"min":1,"max":30}}`

#### Scenario: Multiple measurements on the same day both contribute

- **WHEN** two weights are logged on the same calendar date (e.g. morning 72.0 and evening 72.6)
- **AND** the trend is computed with `window_days=1` on that date
- **THEN** that date's `rolling_avg_kg` is the mean of both (`72.3`)
- **AND** `sample_count` is `2`

#### Scenario: Missing range is rejected

- **WHEN** the client omits `from` or `to`
- **THEN** the system returns `400 Bad Request` with `{"error":"range_required"}`

#### Scenario: Invalid date format is rejected

- **WHEN** `from` or `to` does not match `YYYY-MM-DD`
- **THEN** the system returns `400 Bad Request` with `{"error":"date_invalid"}`

#### Scenario: Inverted range is rejected

- **WHEN** `from > to`
- **THEN** the system returns `400 Bad Request` with `{"error":"range_invalid"}`

#### Scenario: Range larger than 366 days is rejected

- **WHEN** the range spans more than 366 days
- **THEN** the system returns `400 Bad Request` with `{"error":"range_too_large","max_days":366}`

#### Scenario: Default-tz fallback warns

- **WHEN** the client omits `tz` and `DEFAULT_USER_TZ=Europe/Berlin` is set
- **THEN** the system uses `Europe/Berlin` for date boundaries
- **AND** a WARN-level log line records that the fallback was used
- **AND** the response `tz` field reflects `Europe/Berlin`

#### Scenario: Invalid tz is rejected

- **WHEN** the client supplies a `tz` that does not parse via `time.LoadLocation`
- **THEN** the system returns `400 Bad Request` with `{"error":"tz_invalid"}`

#### Scenario: rolling_avg_kg is rounded to 1 decimal

- **WHEN** the computed mean is `73.4666…`
- **THEN** the response shows `"rolling_avg_kg": 73.5`

### Requirement: Body weight is unit-isolated from nutrition summaries

The system SHALL NOT include body-weight data in the nutrition daily summary (`GET /summary/daily`) or the nutrition range summary (`GET /summary/range`). Body-weight responses SHALL NOT contain any nutriment or hydration fields.

#### Scenario: Nutrition summary does not include weight

- **WHEN** the client calls `GET /summary/daily?date=…&tz=…`
- **THEN** the response body does not include `weight_kg`, `body_fat_pct`, or any body-weight-related field
- **AND** body-weight entries do not contribute to any nutriment total

#### Scenario: Body-weight responses do not include nutriment fields

- **WHEN** the client calls `GET /weight` or `GET /weight/trend`
- **THEN** the response body does not include `kcal`, `protein_g`, `total_ml`, or any other nutriment / hydration field
