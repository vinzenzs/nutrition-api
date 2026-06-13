# health-vitals Specification

## Purpose
TBD - created by archiving change add-garmin-misc-mirror. Update Purpose after archive.
## Requirements
### Requirement: Health vitals are stored one snapshot per date

The system SHALL persist daily health vitals in a `health_vitals` table independent of every other capability, captured one row per calendar date (`date` is the primary key). Each row holds nullable cardiovascular readings — blood pressure (systolic / diastolic / pulse), all-day resting / min / max heart rate, and all-day average / max stress — plus audit timestamps. The shape is distinct from `recovery-metrics` (which carries sleep / HRV / readiness / body-battery): health-vitals is the raw blood-pressure and all-day-HR/stress detail that the recovery snapshot does not cover. NULL on any metric means "the device did not report it for that day," a meaningful state, not a data-quality bug. Health vitals are reference/coaching context and SHALL NOT feed any nutrition, fueling, energy, or hydration computation.

#### Scenario: Table is created with the documented columns

- **WHEN** the migration set is applied to a clean database
- **THEN** `health_vitals` exists with columns:
  - `date` (DATE PRIMARY KEY)
  - `bp_systolic` (INTEGER NULL, CHECK `bp_systolic IS NULL OR bp_systolic > 0`)
  - `bp_diastolic` (INTEGER NULL, CHECK `bp_diastolic IS NULL OR bp_diastolic > 0`)
  - `bp_pulse` (INTEGER NULL, CHECK `bp_pulse IS NULL OR bp_pulse > 0`)
  - `resting_hr` (INTEGER NULL, CHECK `resting_hr IS NULL OR resting_hr > 0`)
  - `min_hr` (INTEGER NULL, CHECK `min_hr IS NULL OR min_hr > 0`)
  - `max_hr` (INTEGER NULL, CHECK `max_hr IS NULL OR max_hr > 0`)
  - `stress_avg` (INTEGER NULL, CHECK `stress_avg IS NULL OR (stress_avg BETWEEN 0 AND 100)`)
  - `stress_max` (INTEGER NULL, CHECK `stress_max IS NULL OR (stress_max BETWEEN 0 AND 100)`)
  - `created_at` (TIMESTAMPTZ NOT NULL DEFAULT now())
  - `updated_at` (TIMESTAMPTZ NOT NULL DEFAULT now())
- **AND** `date` is the primary key (one row per calendar day; no surrogate id)

#### Scenario: All metric columns are nullable

- **WHEN** the client posts a snapshot with only `date` and `bp_systolic`/`bp_diastolic`
- **THEN** the row is created with every other metric column NULL
- **AND** the response omits the NULL fields (omitempty)

### Requirement: POST /health-vitals upserts a snapshot by date

The system SHALL expose `POST /health-vitals` that accepts a body carrying a `date` plus any subset of the vital fields and persists it. When a row already exists for that `date`, the system UPDATES it (full-replace of the metric columns); otherwise it INSERTS. This lets the importer "POST every day it sees" without tracking sync state. The endpoint accepts the standard `Idempotency-Key` header.

#### Scenario: First POST for a date inserts

- **WHEN** the client posts `{"date":"2026-06-09","bp_systolic":118,"bp_diastolic":74,"bp_pulse":52,"resting_hr":48,"max_hr":171,"stress_avg":26}`
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

- **WHEN** the client posts `bp_systolic: 0` (or `stress_avg: 120`, `max_hr: -1`)
- **THEN** the system returns `400 Bad Request` with the matching error code (`bp_systolic_invalid`, `stress_avg_invalid`, `max_hr_invalid`)
- **AND** no row is written

### Requirement: GET /health-vitals lists snapshots in a date window

The system SHALL expose `GET /health-vitals?from=<YYYY-MM-DD>&to=<YYYY-MM-DD>` returning rows whose `date` falls in the inclusive window, ordered by `date` ascending. The window is capped at 92 days, matching the other list endpoints.

#### Scenario: Window filtering returns only in-range snapshots

- **WHEN** the client calls `GET /health-vitals?from=2026-06-01&to=2026-06-30`
- **THEN** only rows with `from <= date <= to` are returned, ordered by `date` ascending
- **AND** the response shape is `{"health_vitals": [Snapshot, ...]}`

#### Scenario: Missing window is rejected

- **WHEN** the client omits `from` or `to`
- **THEN** the system returns `400 Bad Request` with `{"error":"window_required"}`

#### Scenario: Window larger than 92 days is rejected

- **WHEN** the supplied window spans more than 92 days
- **THEN** the system returns `400 Bad Request` with `{"error":"range_too_large","max_days":92}`

### Requirement: GET /health-vitals/{date} returns a single snapshot

The system SHALL expose `GET /health-vitals/{date}` returning the snapshot for that date.

#### Scenario: Existing date returns the snapshot

- **WHEN** a snapshot for `2026-06-09` exists
- **AND** the client calls `GET /health-vitals/2026-06-09`
- **THEN** the response is `200 OK` with the snapshot

#### Scenario: Unknown date returns 404

- **WHEN** no snapshot exists for the requested date
- **THEN** the system returns `404 Not Found` with `{"error":"health_vitals_not_found"}`

### Requirement: Health vitals are unit-isolated

The system SHALL keep health vitals in their own response shape, never merged into nutrition, hydration, fitness, or recovery totals. Blood pressure is mmHg, heart rates are bpm, stress is a unitless 0–100 score — these never appear inside a shared Totals struct, and the snapshot SHALL NOT be merged into the `recovery-metrics` shape.

#### Scenario: Health-vitals shape carries no nutrition or recovery fields

- **WHEN** the client fetches any health-vitals snapshot
- **THEN** the response carries only vital fields (`date`, `bp_systolic`, `bp_diastolic`, `bp_pulse`, `resting_hr`, `min_hr`, `max_hr`, `stress_avg`, `stress_max`) plus audit timestamps
- **AND** no `kcal`, `sleep_seconds`, `hrv_ms`, `vo2max_*`, or `weight_kg` field appears

#### Scenario: Nutrition summary does not include health vitals

- **WHEN** the client calls `GET /summary/daily?date=…&tz=…`
- **THEN** the response body does not include `bp_systolic`, `stress_max`, or any health-vitals field

