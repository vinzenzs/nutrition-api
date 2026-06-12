# daily-summary Specification

## Purpose
TBD - created by archiving change add-garmin-daily-energy. Update Purpose after archive.
## Requirements
### Requirement: Daily summary is stored one snapshot per date

The system SHALL persist Garmin's whole-day energy/activity totals in a `daily_summary` table independent of every other capability. Each row is identified by a `date` (one snapshot per calendar day) and holds nullable expenditure/activity signals plus audit timestamps. The shape is source-agnostic: any writer (initially the garmin-bridge, later any activity source) targets it via the REST endpoints, "POSTing every day it sees" without tracking sync state. NULL on any metric means "the device did not report it for that day," a meaningful state, not a data-quality bug.

#### Scenario: Table is created with the documented columns

- **WHEN** the migration set is applied to a clean database
- **THEN** `daily_summary` exists with columns:
  - `date` (DATE PRIMARY KEY)
  - `active_kcal` (INTEGER NULL, CHECK `active_kcal IS NULL OR active_kcal >= 0`)
  - `resting_kcal` (INTEGER NULL, CHECK `resting_kcal IS NULL OR resting_kcal >= 0`)
  - `total_kcal` (INTEGER NULL, CHECK `total_kcal IS NULL OR total_kcal >= 0`)
  - `steps` (INTEGER NULL, CHECK `steps IS NULL OR steps >= 0`)
  - `floors` (INTEGER NULL, CHECK `floors IS NULL OR floors >= 0`)
  - `moderate_intensity_minutes` (INTEGER NULL, CHECK `moderate_intensity_minutes IS NULL OR moderate_intensity_minutes >= 0`)
  - `vigorous_intensity_minutes` (INTEGER NULL, CHECK `vigorous_intensity_minutes IS NULL OR vigorous_intensity_minutes >= 0`)
  - `distance_m` (NUMERIC(10, 1) NULL, CHECK `distance_m IS NULL OR distance_m >= 0`)
  - `created_at` (TIMESTAMPTZ NOT NULL DEFAULT now())
  - `updated_at` (TIMESTAMPTZ NOT NULL DEFAULT now())
- **AND** `date` is the primary key (one row per calendar day; no surrogate id)

#### Scenario: All metric columns are nullable

- **WHEN** the client posts a snapshot with only `date` and `total_kcal`
- **THEN** the row is created with every other metric column NULL
- **AND** the response omits the NULL fields (omitempty)

### Requirement: POST /daily-summary upserts a snapshot by date

The system SHALL expose `POST /daily-summary` that accepts a body carrying a `date` plus any subset of the metric fields and persists it. When a row already exists for that `date`, the system UPDATES it (full-replace of the metric columns); otherwise it INSERTS. This lets the importer "POST every day it sees" without tracking sync state. The endpoint accepts the standard `Idempotency-Key` header.

#### Scenario: First POST for a date inserts

- **WHEN** the client posts `{"date":"2026-06-11","active_kcal":820,"resting_kcal":1650,"total_kcal":2470,"steps":12400,"floors":14,"moderate_intensity_minutes":35,"vigorous_intensity_minutes":48,"distance_m":9320.5}`
- **THEN** the system creates a row for `2026-06-11` and returns `201 Created` echoing the stored fields

#### Scenario: Second POST for the same date updates in place

- **WHEN** a row for `2026-06-11` exists
- **AND** the client posts another body for `2026-06-11` with `steps: 13100`
- **THEN** the system UPDATES the existing row and returns `200 OK`
- **AND** no duplicate row for that date exists
- **AND** fields omitted from the second body are reset to NULL (full-replace upsert)

#### Scenario: Missing or invalid date is rejected

- **WHEN** the client posts a body without `date`, or with a `date` that is not a valid `YYYY-MM-DD`
- **THEN** the system returns `400 Bad Request` with `{"error":"date_invalid"}`

#### Scenario: Negative metric is rejected

- **WHEN** the client posts `total_kcal: -1` (or any negative metric such as `steps: -5`, `distance_m: -0.1`)
- **THEN** the system returns `400 Bad Request` with the matching error code (`total_kcal_invalid`, `steps_invalid`, `distance_m_invalid`)
- **AND** no row is written

### Requirement: GET /daily-summary lists snapshots in a date window

The system SHALL expose `GET /daily-summary?from=<YYYY-MM-DD>&to=<YYYY-MM-DD>` returning rows whose `date` falls in the inclusive window, ordered by `date` ascending. The window is capped at 92 days, matching the other snapshot list endpoints.

#### Scenario: Window filtering returns only in-range snapshots

- **WHEN** the client calls `GET /daily-summary?from=2026-06-01&to=2026-06-30`
- **THEN** only rows with `from <= date <= to` are returned, ordered by `date` ascending
- **AND** the response shape is `{"daily_summary": [Snapshot, ...]}`

#### Scenario: Missing window is rejected

- **WHEN** the client omits `from` or `to`
- **THEN** the system returns `400 Bad Request` with `{"error":"window_required"}`

#### Scenario: Window larger than 92 days is rejected

- **WHEN** the supplied window spans more than 92 days
- **THEN** the system returns `400 Bad Request` with `{"error":"range_too_large","max_days":92}`

### Requirement: GET /daily-summary/{date} returns a single snapshot

The system SHALL expose `GET /daily-summary/{date}` returning the snapshot for that date.

#### Scenario: Existing date returns the snapshot

- **WHEN** a snapshot for `2026-06-11` exists
- **AND** the client calls `GET /daily-summary/2026-06-11`
- **THEN** the response is `200 OK` with the snapshot

#### Scenario: Unknown date returns 404

- **WHEN** no snapshot exists for the requested date
- **THEN** the system returns `404 Not Found` with `{"error":"daily_summary_not_found"}`

### Requirement: DELETE /daily-summary/{date} removes a snapshot

The system SHALL expose `DELETE /daily-summary/{date}` that removes the snapshot for that date.

#### Scenario: Delete returns 204 on success

- **WHEN** the client deletes an existing snapshot
- **THEN** the system returns `204 No Content`
- **AND** a subsequent GET for that date returns `404 daily_summary_not_found`

#### Scenario: Delete of unknown date returns 404

- **WHEN** the client deletes a date with no snapshot
- **THEN** the system returns `404 Not Found` with `{"error":"daily_summary_not_found"}`

### Requirement: Daily summary floats are rounded at the response boundary

The system SHALL round every measurement float in the daily-summary response with `numfmt.Round1` at the serialization boundary, consistent with the rest of the API. Integer metrics are returned as-is.

#### Scenario: distance_m is rounded to one decimal place

- **WHEN** a snapshot has `distance_m` stored at full precision (e.g. `9320.4789`)
- **AND** the client fetches the snapshot
- **THEN** the response carries `distance_m` rounded to one decimal place (`9320.5`)

### Requirement: Daily summary is unit-isolated

The system SHALL keep daily-summary totals in their own response shape, never merged into nutrition or hydration totals. Active/resting/total kcal here are *expenditure*, distinct from the *intake* kcal in `summary`; steps, floors, intensity minutes, and distance are activity counters. None of these SHALL appear inside `summary`'s Totals struct, and none SHALL replace the exercise-burn term in the Energy Availability computation.

#### Scenario: Daily-summary shape carries no nutrition fields

- **WHEN** the client fetches any daily-summary snapshot
- **THEN** the response carries only daily-summary fields (`date`, `active_kcal`, `resting_kcal`, `total_kcal`, `steps`, `floors`, `moderate_intensity_minutes`, `vigorous_intensity_minutes`, `distance_m`) plus audit timestamps
- **AND** no macro/micronutrient gram or milligram field appears

#### Scenario: Energy Availability denominator is unaffected by daily-summary expenditure

- **WHEN** a daily-summary snapshot exists with `total_kcal` for a date
- **AND** the client computes Energy Availability for that date
- **THEN** the EA subtrahend remains `Σ workouts.kcal_burned` (exercise burn), NOT the daily-summary `total_kcal`
- **AND** the EA value and its Loucks band classification are identical to what they would be with no daily-summary row present

