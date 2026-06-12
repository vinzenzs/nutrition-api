# fitness-metrics Specification (delta)

## ADDED Requirements

### Requirement: Fitness snapshots carry endurance, hill, fitness-age, and training-status

The system SHALL persist additional nullable fitness signals on the `fitness_metrics` snapshot: an endurance score, a hill score, a fitness age, and a free-text `training_status` label. These extend the existing fitness snapshot in place — same date-keyed row, same full-replace upsert, same source-agnostic contract — and remain unit-isolated on the fitness shape. The `training_status` label (a phrase such as "productive", "maintaining", "unproductive", "recovery") complements the already-stored numeric `acute_load` / `chronic_load`; it is stored verbatim as text, NOT as a constrained enum, so a future Garmin vocabulary is preserved rather than dropped. NULL on any of them is a meaningful "not reported for that day."

#### Scenario: Detail columns are added to the fitness_metrics table

- **WHEN** the migration set is applied to a clean database
- **THEN** `fitness_metrics` carries the additional nullable columns:
  - `endurance_score` (INTEGER NULL, CHECK `endurance_score IS NULL OR endurance_score > 0`)
  - `hill_score` (INTEGER NULL, CHECK `hill_score IS NULL OR hill_score > 0`)
  - `fitness_age` (NUMERIC(4, 1) NULL, CHECK `fitness_age IS NULL OR fitness_age > 0`)
  - `training_status` (TEXT NULL, CHECK `training_status IS NULL OR length(training_status) BETWEEN 1 AND 64`)
- **AND** every existing row carries NULL for all of them
- **AND** the migration succeeds without back-filling any of them

#### Scenario: POST accepts and echoes the new fitness fields

- **WHEN** the client posts `{"date":"2026-06-09","vo2max_running":54.0,"endurance_score":7200,"hill_score":61,"fitness_age":34.0,"training_status":"productive"}`
- **THEN** the system upserts the row for `2026-06-09` and echoes the stored new fields
- **AND** a subsequent `GET /fitness-metrics/2026-06-09` returns them

#### Scenario: New fitness fields are nullable and omitted when absent

- **WHEN** the client posts a snapshot with only `date` and `vo2max_running` (no endurance / hill / fitness-age / training-status)
- **THEN** the row is created with every new column NULL
- **AND** the response omits the NULL new fields (omitempty)

#### Scenario: Out-of-range numeric metric is rejected

- **WHEN** the client posts `endurance_score: 0` (or `hill_score: -1`, `fitness_age: 0`)
- **THEN** the system returns `400 Bad Request` with the matching error code (`endurance_score_invalid`, `hill_score_invalid`, `fitness_age_invalid`)
- **AND** no row is written

#### Scenario: training_status is stored verbatim and validated as a sane string

- **WHEN** the client posts `training_status: "maintaining"`
- **THEN** the value is stored verbatim (trimmed) and echoed on read
- **AND** an empty or whitespace-only `training_status` is rejected with `400 Bad Request` `{"error":"training_status_invalid"}`
- **AND** an unrecognised phrase (e.g. a future Garmin status word) is accepted, not gated against a fixed enum

#### Scenario: New numeric fields are rounded at the response boundary

- **WHEN** a fitness snapshot with `fitness_age` set is serialized to a response
- **THEN** that float is rounded with `numfmt.Round1` at the boundary, consistent with the rest of the fitness shape
- **AND** the new fields are NEVER merged into `summary`'s Totals struct (unit isolation preserved)
