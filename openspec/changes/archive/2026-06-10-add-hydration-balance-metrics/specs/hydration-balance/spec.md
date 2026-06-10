## ADDED Requirements

### Requirement: Hydration-balance metrics are stored one snapshot per date

The system SHALL persist daily hydration-balance metrics in a `hydration_balance_metrics` table independent of every other capability. Each row is identified by a `date` (one snapshot per calendar day) and holds nullable water-balance signals plus audit timestamps. The shape is source-agnostic (initially Garmin's daily hydration response). NULL on any metric means "not reported for that day." This capability is distinct from `hydration` (per-entry logged intake): it stores a device's daily sweat/intake estimate, a different grain and source.

#### Scenario: Table is created with the documented columns

- **WHEN** the migration set is applied to a clean database
- **THEN** `hydration_balance_metrics` exists with columns:
  - `date` (DATE PRIMARY KEY)
  - `sweat_loss_ml` (NUMERIC(10, 1) NULL, CHECK `sweat_loss_ml IS NULL OR sweat_loss_ml > 0`)
  - `activity_intake_ml` (NUMERIC(10, 1) NULL, CHECK `activity_intake_ml IS NULL OR activity_intake_ml >= 0`)
  - `goal_ml` (NUMERIC(10, 1) NULL, CHECK `goal_ml IS NULL OR goal_ml > 0`)
  - `created_at` (TIMESTAMPTZ NOT NULL DEFAULT now())
  - `updated_at` (TIMESTAMPTZ NOT NULL DEFAULT now())
- **AND** `date` is the primary key (one row per calendar day; no surrogate id)
- **AND** there is NO `total_intake_ml` column — the daily intake total lives in the `hydration` capability, not here

#### Scenario: All metric columns are nullable; activity_intake_ml allows zero

- **WHEN** the client posts a snapshot with only `date` and `activity_intake_ml: 0`
- **THEN** the row is created with `activity_intake_ml = 0` and the other metrics NULL
- **AND** the response includes `activity_intake_ml` (a real zero — drank nothing during activity) and omits the NULL fields (omitempty)

### Requirement: POST /hydration-balance upserts a snapshot by date

The system SHALL expose `POST /hydration-balance` that accepts a body carrying a `date` plus any subset of the metric fields and persists it via date-keyed upsert (full-replace of the metric columns on conflict). The endpoint accepts the standard `Idempotency-Key` header.

#### Scenario: First POST for a date inserts

- **WHEN** the client posts `{"date":"2026-06-09","sweat_loss_ml":2400,"activity_intake_ml":1800,"goal_ml":3000}`
- **THEN** the system creates a row for `2026-06-09` and returns `201 Created` echoing the stored fields

#### Scenario: Second POST for the same date updates in place

- **WHEN** a row for `2026-06-09` exists
- **AND** the client posts another body for `2026-06-09` with `sweat_loss_ml: 2600`
- **THEN** the system UPDATES the existing row and returns `200 OK`
- **AND** no duplicate row for that date exists
- **AND** fields omitted from the second body are reset to NULL (full-replace upsert)

#### Scenario: Missing or invalid date is rejected

- **WHEN** the client posts a body without `date`, or with a `date` that is not a valid `YYYY-MM-DD`
- **THEN** the system returns `400 Bad Request` with `{"error":"date_invalid"}`

#### Scenario: Out-of-range metric is rejected

- **WHEN** the client posts `sweat_loss_ml: 0` (or a negative `activity_intake_ml`, or `goal_ml: 0`)
- **THEN** the system returns `400 Bad Request` with the matching error code (`sweat_loss_ml_invalid`, `activity_intake_ml_invalid`, `goal_ml_invalid`)
- **AND** no row is written

### Requirement: GET /hydration-balance lists snapshots in a date window

The system SHALL expose `GET /hydration-balance?from=<YYYY-MM-DD>&to=<YYYY-MM-DD>` returning rows whose `date` falls in the inclusive window, ordered by `date` ascending, with a 92-day cap.

#### Scenario: Window filtering returns only in-range snapshots

- **WHEN** the client calls `GET /hydration-balance?from=2026-06-01&to=2026-06-30`
- **THEN** only rows with `from <= date <= to` are returned, ordered by `date` ascending
- **AND** the response shape is `{"hydration_balance": [Snapshot, ...]}`

#### Scenario: Missing window is rejected

- **WHEN** the client omits `from` or `to`
- **THEN** the system returns `400 Bad Request` with `{"error":"window_required"}`

#### Scenario: Window larger than 92 days is rejected

- **WHEN** the supplied window spans more than 92 days
- **THEN** the system returns `400 Bad Request` with `{"error":"range_too_large","max_days":92}`

### Requirement: GET /hydration-balance/{date} returns a single snapshot

The system SHALL expose `GET /hydration-balance/{date}` returning the snapshot for that date.

#### Scenario: Existing date returns the snapshot

- **WHEN** a snapshot for `2026-06-09` exists
- **AND** the client calls `GET /hydration-balance/2026-06-09`
- **THEN** the response is `200 OK` with the snapshot

#### Scenario: Unknown date returns 404

- **WHEN** no snapshot exists for the requested date
- **THEN** the system returns `404 Not Found` with `{"error":"hydration_balance_not_found"}`

### Requirement: DELETE /hydration-balance/{date} removes a snapshot

The system SHALL expose `DELETE /hydration-balance/{date}` removing the snapshot for that date.

#### Scenario: Delete returns 204 on success

- **WHEN** the client deletes an existing snapshot
- **THEN** the system returns `204 No Content`
- **AND** a subsequent GET for that date returns `404 hydration_balance_not_found`

#### Scenario: Delete of unknown date returns 404

- **WHEN** the client deletes a date with no snapshot
- **THEN** the system returns `404 Not Found` with `{"error":"hydration_balance_not_found"}`

### Requirement: Hydration-balance is unit-isolated and distinct from hydration

The system SHALL keep hydration-balance metrics in their own response shape, never merged into the `hydration` per-entry intake totals, nutrition, recovery, or fitness shapes. All three balance fields are millilitres; they never appear inside a shared Totals struct or the hydration summary.

#### Scenario: Hydration-balance shape carries no foreign fields

- **WHEN** the client fetches any hydration-balance snapshot
- **THEN** the response carries only `date`, `sweat_loss_ml`, `activity_intake_ml`, `goal_ml`, and audit timestamps
- **AND** no `total_ml`, `entries_count`, `kcal`, or `weight_kg` field appears
