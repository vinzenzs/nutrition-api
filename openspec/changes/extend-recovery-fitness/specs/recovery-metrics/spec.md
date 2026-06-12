# recovery-metrics Specification (delta)

## ADDED Requirements

### Requirement: Recovery snapshots carry SpO2, respiration, and sleep-stage detail

The system SHALL persist additional nullable recovery signals on the `recovery_metrics` snapshot: blood-oxygen (SpO2) average and lowest, overnight respiration average and lowest, and the per-stage sleep breakdown (deep / light / REM / awake seconds). These extend the existing recovery snapshot in place — same date-keyed row, same full-replace upsert, same source-agnostic contract — and remain unit-isolated on the recovery shape. NULL on any of them is a meaningful state ("the device did not report it for that day"), never a data-quality bug.

#### Scenario: Detail columns are added to the recovery_metrics table

- **WHEN** the migration set is applied to a clean database
- **THEN** `recovery_metrics` carries the additional nullable columns:
  - `spo2_avg` (INTEGER NULL, CHECK `spo2_avg IS NULL OR (spo2_avg BETWEEN 0 AND 100)`)
  - `spo2_lowest` (INTEGER NULL, CHECK `spo2_lowest IS NULL OR (spo2_lowest BETWEEN 0 AND 100)`)
  - `respiration_avg` (NUMERIC(4, 1) NULL, CHECK `respiration_avg IS NULL OR respiration_avg > 0`)
  - `respiration_lowest` (NUMERIC(4, 1) NULL, CHECK `respiration_lowest IS NULL OR respiration_lowest > 0`)
  - `deep_sleep_seconds` (INTEGER NULL, CHECK `deep_sleep_seconds IS NULL OR deep_sleep_seconds >= 0`)
  - `light_sleep_seconds` (INTEGER NULL, CHECK `light_sleep_seconds IS NULL OR light_sleep_seconds >= 0`)
  - `rem_sleep_seconds` (INTEGER NULL, CHECK `rem_sleep_seconds IS NULL OR rem_sleep_seconds >= 0`)
  - `awake_seconds` (INTEGER NULL, CHECK `awake_seconds IS NULL OR awake_seconds >= 0`)
- **AND** every existing row carries NULL for all of them
- **AND** the migration succeeds without back-filling any of them

#### Scenario: POST accepts and echoes the new recovery fields

- **WHEN** the client posts `{"date":"2026-06-09","sleep_seconds":27000,"spo2_avg":95,"spo2_lowest":89,"respiration_avg":13.4,"respiration_lowest":9.8,"deep_sleep_seconds":6000,"light_sleep_seconds":15000,"rem_sleep_seconds":5400,"awake_seconds":600}`
- **THEN** the system upserts the row for `2026-06-09` and echoes the stored new fields
- **AND** a subsequent `GET /recovery-metrics/2026-06-09` returns them

#### Scenario: New recovery fields are nullable and omitted when absent

- **WHEN** the client posts a snapshot with only `date` and `sleep_seconds` (no SpO2 / respiration / sleep-stage fields)
- **THEN** the row is created with every new column NULL
- **AND** the response omits the NULL new fields (omitempty)

#### Scenario: Out-of-range new metric is rejected

- **WHEN** the client posts `spo2_avg: 120` (or `spo2_lowest: -1`, `respiration_avg: 0`, `deep_sleep_seconds: -1`)
- **THEN** the system returns `400 Bad Request` with the matching error code (`spo2_avg_invalid`, `spo2_lowest_invalid`, `respiration_avg_invalid`, `deep_sleep_seconds_invalid`)
- **AND** no row is written

#### Scenario: Respiration floats are rounded at the response boundary

- **WHEN** a recovery snapshot with `respiration_avg` / `respiration_lowest` set is serialized to a response
- **THEN** those floats are rounded with `numfmt.Round1` at the boundary, consistent with the rest of the recovery shape
- **AND** the new fields are NEVER merged into `summary`'s Totals struct (unit isolation preserved)
