## ADDED Requirements

### Requirement: Recovery metrics are stored one snapshot per date

The system SHALL persist daily recovery metrics in a `recovery_metrics` table independent of every other capability. Each row is identified by a `date` (one snapshot per calendar day) and holds nullable wellness signals plus audit timestamps. The shape is source-agnostic: any writer (initially `garmin.py`, later any wellness source) targets it. NULL on any metric means "the device did not report it for that day," a meaningful state.

#### Scenario: Table is created with the documented columns

- **WHEN** the migration set is applied to a clean database
- **THEN** `recovery_metrics` exists with columns:
  - `date` (DATE PRIMARY KEY)
  - `sleep_seconds` (INTEGER NULL, CHECK `sleep_seconds IS NULL OR sleep_seconds > 0`)
  - `sleep_score` (INTEGER NULL, CHECK `sleep_score IS NULL OR (sleep_score BETWEEN 0 AND 100)`)
  - `hrv_ms` (NUMERIC(6, 1) NULL, CHECK `hrv_ms IS NULL OR hrv_ms > 0`)
  - `resting_hr` (INTEGER NULL, CHECK `resting_hr IS NULL OR resting_hr > 0`)
  - `stress_avg` (INTEGER NULL, CHECK `stress_avg IS NULL OR (stress_avg BETWEEN 0 AND 100)`)
  - `body_battery_charged` (INTEGER NULL, CHECK `body_battery_charged IS NULL OR (body_battery_charged BETWEEN 0 AND 100)`)
  - `body_battery_drained` (INTEGER NULL, CHECK `body_battery_drained IS NULL OR (body_battery_drained BETWEEN 0 AND 100)`)
  - `training_readiness` (INTEGER NULL, CHECK `training_readiness IS NULL OR (training_readiness BETWEEN 0 AND 100)`)
  - `created_at` (TIMESTAMPTZ NOT NULL DEFAULT now())
  - `updated_at` (TIMESTAMPTZ NOT NULL DEFAULT now())
- **AND** `date` is the primary key (one row per calendar day; no surrogate id)

#### Scenario: All metric columns are nullable

- **WHEN** the client posts a snapshot with only `date` and `sleep_seconds`
- **THEN** the row is created with every other metric column NULL
- **AND** the response omits the NULL fields (omitempty)

### Requirement: POST /recovery-metrics upserts a snapshot by date

The system SHALL expose `POST /recovery-metrics` that accepts a body carrying a `date` plus any subset of the metric fields and persists it. When a row already exists for that `date`, the system UPDATES it (full-replace of the metric columns); otherwise it INSERTS. This lets the importer "POST every day it sees" without tracking sync state. The endpoint accepts the standard `Idempotency-Key` header.

#### Scenario: First POST for a date inserts

- **WHEN** the client posts `{"date":"2026-06-09","sleep_seconds":27000,"sleep_score":82,"hrv_ms":61.0,"resting_hr":48,"stress_avg":28,"training_readiness":74}`
- **THEN** the system creates a row for `2026-06-09` and returns `201 Created` echoing the stored fields

#### Scenario: Second POST for the same date updates in place

- **WHEN** a row for `2026-06-09` exists
- **AND** the client posts another body for `2026-06-09` with `resting_hr: 46`
- **THEN** the system UPDATES the existing row and returns `200 OK`
- **AND** no duplicate row for that date exists
- **AND** fields omitted from the second body are reset to NULL (full-replace upsert)

#### Scenario: Missing or invalid date is rejected

- **WHEN** the client posts a body without `date`, or with a `date` that is not a valid `YYYY-MM-DD`
- **THEN** the system returns `400 Bad Request` with `{"error":"date_invalid"}`

#### Scenario: Out-of-range metric is rejected

- **WHEN** the client posts `sleep_score: 120` (or `stress_avg: -1`, `resting_hr: 0`, `training_readiness: 101`)
- **THEN** the system returns `400 Bad Request` with the matching error code (`sleep_score_invalid`, `stress_avg_invalid`, `resting_hr_invalid`, `training_readiness_invalid`)
- **AND** no row is written

### Requirement: GET /recovery-metrics lists snapshots in a date window

The system SHALL expose `GET /recovery-metrics?from=<YYYY-MM-DD>&to=<YYYY-MM-DD>` returning rows whose `date` falls in the inclusive window, ordered by `date` ascending. The window is capped at 92 days, matching the other list endpoints.

#### Scenario: Window filtering returns only in-range snapshots

- **WHEN** the client calls `GET /recovery-metrics?from=2026-06-01&to=2026-06-30`
- **THEN** only rows with `from <= date <= to` are returned, ordered by `date` ascending
- **AND** the response shape is `{"recovery_metrics": [Snapshot, ...]}`

#### Scenario: Missing window is rejected

- **WHEN** the client omits `from` or `to`
- **THEN** the system returns `400 Bad Request` with `{"error":"window_required"}`

#### Scenario: Window larger than 92 days is rejected

- **WHEN** the supplied window spans more than 92 days
- **THEN** the system returns `400 Bad Request` with `{"error":"range_too_large","max_days":92}`

### Requirement: GET /recovery-metrics/{date} returns a single snapshot

The system SHALL expose `GET /recovery-metrics/{date}` returning the snapshot for that date.

#### Scenario: Existing date returns the snapshot

- **WHEN** a snapshot for `2026-06-09` exists
- **AND** the client calls `GET /recovery-metrics/2026-06-09`
- **THEN** the response is `200 OK` with the snapshot

#### Scenario: Unknown date returns 404

- **WHEN** no snapshot exists for the requested date
- **THEN** the system returns `404 Not Found` with `{"error":"recovery_metrics_not_found"}`

### Requirement: DELETE /recovery-metrics/{date} removes a snapshot

The system SHALL expose `DELETE /recovery-metrics/{date}` that removes the snapshot for that date.

#### Scenario: Delete returns 204 on success

- **WHEN** the client deletes an existing snapshot
- **THEN** the system returns `204 No Content`
- **AND** a subsequent GET for that date returns `404 recovery_metrics_not_found`

#### Scenario: Delete of unknown date returns 404

- **WHEN** the client deletes a date with no snapshot
- **THEN** the system returns `404 Not Found` with `{"error":"recovery_metrics_not_found"}`

### Requirement: Recovery metrics are unit-isolated

The system SHALL keep recovery metrics in their own response shape, never merged into nutrition, hydration, or fitness totals. Sleep is seconds, HRV is milliseconds, body battery and stress and readiness are unitless 0–100 scores — these never appear inside a shared Totals struct.

#### Scenario: Recovery shape carries no nutrition or fitness fields

- **WHEN** the client fetches any recovery snapshot
- **THEN** the response carries only recovery fields (`date`, `sleep_seconds`, `sleep_score`, `hrv_ms`, `resting_hr`, `stress_avg`, `body_battery_charged`, `body_battery_drained`, `training_readiness`) plus audit timestamps
- **AND** no `kcal`, `vo2max_*`, or `weight_kg` field appears
