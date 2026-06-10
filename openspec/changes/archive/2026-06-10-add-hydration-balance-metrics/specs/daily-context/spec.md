## ADDED Requirements

### Requirement: Daily context surfaces the hydration-balance snapshot

The system SHALL extend the `GET /context/daily` bundle with a `hydration_balance` block — the `hydration_balance_metrics` row for the requested `date`, or `null` when no snapshot exists for that day (same-day-or-null, no carryover: a stale sweat estimate misleads). The block is pure read composition over the `hydration-balance` repo (`GetByDate`); `daily-context` still defines no tables and performs no writes. It sits alongside the existing `hydration` block and is never merged with it: `hydration` carries the user's logged intake (`total_ml`, `entries_count`); `hydration_balance` carries Garmin's daily sweat/intake estimate.

#### Scenario: Bundle includes hydration_balance when a snapshot exists

- **WHEN** the client calls `GET /context/daily?date=2026-06-09` and a hydration-balance snapshot exists for that date
- **THEN** the response body includes a `hydration_balance` object (the snapshot for 2026-06-09)
- **AND** it sits alongside the existing `hydration` block (which keeps its `total_ml` / `entries_count` shape, unchanged)

#### Scenario: hydration_balance is null when absent (no carryover)

- **WHEN** no hydration-balance snapshot exists for the requested date, even if one exists for a prior day
- **THEN** `hydration_balance` is `null` (NOT the prior day's snapshot)

#### Scenario: The two hydration blocks stay distinct

- **WHEN** both a logged hydration entry and a hydration-balance snapshot exist for the date
- **THEN** `hydration` reflects only the logged intake (`total_ml`, `entries_count`)
- **AND** `hydration_balance` reflects only the Garmin estimate (`sweat_loss_ml`, `activity_intake_ml`, `goal_ml`)
- **AND** neither block's fields leak into the other
