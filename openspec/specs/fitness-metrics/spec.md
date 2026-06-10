# fitness-metrics Specification

## Purpose

Define a persisted log of daily fitness snapshots — VO2max (running and cycling), race predictors, and acute/chronic training load — captured one row per calendar date, independent of every other capability. The shape is source-agnostic: any writer (initially `garmin.py`, later any fitness source) targets it via the REST endpoints, "POSTing every day it sees" without tracking sync state. NULL on any metric is a meaningful state ("not reported for that day"), not a data-quality bug. Race predictions are stored as whole seconds (the agent formats `h:mm:ss`) and the acute:chronic ratio is derived at read time rather than stored. Fitness metrics are kept in their own response shape and never merged into nutrition, recovery, or weight totals.

## Requirements

### Requirement: Fitness metrics are stored one snapshot per date

The system SHALL persist daily fitness metrics in a `fitness_metrics` table independent of every other capability. Each row is identified by a `date` (one snapshot per calendar day) and holds nullable fitness signals plus audit timestamps. The shape is source-agnostic. NULL on any metric means "not reported for that day."

#### Scenario: Table is created with the documented columns

- **WHEN** the migration set is applied to a clean database
- **THEN** `fitness_metrics` exists with columns:
  - `date` (DATE PRIMARY KEY)
  - `vo2max_running` (NUMERIC(4, 1) NULL, CHECK `vo2max_running IS NULL OR vo2max_running > 0`)
  - `vo2max_cycling` (NUMERIC(4, 1) NULL, CHECK `vo2max_cycling IS NULL OR vo2max_cycling > 0`)
  - `race_predictor_5k_seconds` (INTEGER NULL, CHECK `race_predictor_5k_seconds IS NULL OR race_predictor_5k_seconds > 0`)
  - `race_predictor_10k_seconds` (INTEGER NULL, CHECK `race_predictor_10k_seconds IS NULL OR race_predictor_10k_seconds > 0`)
  - `race_predictor_half_seconds` (INTEGER NULL, CHECK `race_predictor_half_seconds IS NULL OR race_predictor_half_seconds > 0`)
  - `race_predictor_full_seconds` (INTEGER NULL, CHECK `race_predictor_full_seconds IS NULL OR race_predictor_full_seconds > 0`)
  - `acute_load` (NUMERIC(6, 1) NULL, CHECK `acute_load IS NULL OR acute_load >= 0`)
  - `chronic_load` (NUMERIC(6, 1) NULL, CHECK `chronic_load IS NULL OR chronic_load >= 0`)
  - `created_at` (TIMESTAMPTZ NOT NULL DEFAULT now())
  - `updated_at` (TIMESTAMPTZ NOT NULL DEFAULT now())
- **AND** `date` is the primary key (one row per calendar day; no surrogate id)
- **AND** race predictions are stored as whole **seconds** (the agent formats `h:mm:ss`); there is NO formatted-string column

#### Scenario: All metric columns are nullable

- **WHEN** the client posts a snapshot with only `date` and `vo2max_running`
- **THEN** the row is created with every other metric column NULL
- **AND** the response omits the NULL fields (omitempty)

#### Scenario: Acute:chronic ratio is not stored

- **WHEN** the client inspects the stored or returned shape
- **THEN** there is NO `acwr` / `acute_chronic_ratio` column — it is `acute_load / chronic_load`, derived by the agent at read time

### Requirement: POST /fitness-metrics upserts a snapshot by date

The system SHALL expose `POST /fitness-metrics` that accepts a body carrying a `date` plus any subset of the metric fields and persists it via date-keyed upsert (full-replace of the metric columns on conflict). The endpoint accepts the standard `Idempotency-Key` header.

#### Scenario: First POST for a date inserts

- **WHEN** the client posts `{"date":"2026-06-09","vo2max_running":54.0,"race_predictor_5k_seconds":1230,"acute_load":420.5,"chronic_load":380.0}`
- **THEN** the system creates a row for `2026-06-09` and returns `201 Created` echoing the stored fields

#### Scenario: Second POST for the same date updates in place

- **WHEN** a row for `2026-06-09` exists
- **AND** the client posts another body for `2026-06-09` with `vo2max_cycling: 58.0`
- **THEN** the system UPDATES the existing row and returns `200 OK`
- **AND** no duplicate row for that date exists
- **AND** fields omitted from the second body are reset to NULL (full-replace upsert)

#### Scenario: Missing or invalid date is rejected

- **WHEN** the client posts a body without `date`, or with a malformed `date`
- **THEN** the system returns `400 Bad Request` with `{"error":"date_invalid"}`

#### Scenario: Non-positive metric is rejected

- **WHEN** the client posts `vo2max_running: 0` (or a negative race predictor, or a negative load)
- **THEN** the system returns `400 Bad Request` with the matching error code (`vo2max_running_invalid`, `race_predictor_5k_seconds_invalid`, `acute_load_invalid`, …)
- **AND** no row is written

### Requirement: GET /fitness-metrics lists snapshots in a date window

The system SHALL expose `GET /fitness-metrics?from=<YYYY-MM-DD>&to=<YYYY-MM-DD>` returning rows whose `date` is in the inclusive window, ordered by `date` ascending, with a 92-day cap.

#### Scenario: Window filtering returns only in-range snapshots

- **WHEN** the client calls `GET /fitness-metrics?from=2026-06-01&to=2026-06-30`
- **THEN** only rows with `from <= date <= to` are returned, ordered by `date` ascending
- **AND** the response shape is `{"fitness_metrics": [Snapshot, ...]}`

#### Scenario: Missing window is rejected

- **WHEN** the client omits `from` or `to`
- **THEN** the system returns `400 Bad Request` with `{"error":"window_required"}`

#### Scenario: Window larger than 92 days is rejected

- **WHEN** the supplied window spans more than 92 days
- **THEN** the system returns `400 Bad Request` with `{"error":"range_too_large","max_days":92}`

### Requirement: GET /fitness-metrics/{date} returns a single snapshot

The system SHALL expose `GET /fitness-metrics/{date}` returning the snapshot for that date.

#### Scenario: Existing date returns the snapshot

- **WHEN** a snapshot for `2026-06-09` exists
- **AND** the client calls `GET /fitness-metrics/2026-06-09`
- **THEN** the response is `200 OK` with the snapshot

#### Scenario: Unknown date returns 404

- **WHEN** no snapshot exists for the requested date
- **THEN** the system returns `404 Not Found` with `{"error":"fitness_metrics_not_found"}`

### Requirement: DELETE /fitness-metrics/{date} removes a snapshot

The system SHALL expose `DELETE /fitness-metrics/{date}` removing the snapshot for that date.

#### Scenario: Delete returns 204 on success

- **WHEN** the client deletes an existing snapshot
- **THEN** the system returns `204 No Content`
- **AND** a subsequent GET for that date returns `404 fitness_metrics_not_found`

#### Scenario: Delete of unknown date returns 404

- **WHEN** the client deletes a date with no snapshot
- **THEN** the system returns `404 Not Found` with `{"error":"fitness_metrics_not_found"}`

### Requirement: Fitness metrics are unit-isolated

The system SHALL keep fitness metrics in their own response shape, never merged into nutrition, recovery, or weight totals.

#### Scenario: Fitness shape carries no recovery or nutrition fields

- **WHEN** the client fetches any fitness snapshot
- **THEN** the response carries only fitness fields plus audit timestamps
- **AND** no `sleep_seconds`, `hrv_ms`, `kcal`, or `weight_kg` field appears
