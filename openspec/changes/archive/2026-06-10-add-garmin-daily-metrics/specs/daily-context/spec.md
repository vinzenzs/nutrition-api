## ADDED Requirements

### Requirement: Daily context surfaces recovery and fitness snapshots

The system SHALL extend the `GET /context/daily` bundle with a `recovery` block and a `fitness` block, each being the snapshot row for the requested `date` or `null` when no snapshot exists for that day. Unlike the `weight` block, recovery and fitness do NOT carry over a prior day's value — a stale recovery/fitness reading is misleading, so the block is same-day-or-null. The blocks are pure read composition over the `recovery_metrics` and `fitness_metrics` repos (`GetByDate`); `daily-context` still defines no tables and performs no writes. The existing `weight` block additionally echoes the new biometric fields (`muscle_mass_kg`, `body_water_pct`, `bone_mass_kg`, `bmi`) when present on the entry.

#### Scenario: Bundle includes recovery and fitness when snapshots exist

- **WHEN** the client calls `GET /context/daily?date=2026-06-09` and recovery + fitness snapshots exist for that date
- **THEN** the response body includes a `recovery` object (the recovery snapshot for 2026-06-09) and a `fitness` object (the fitness snapshot for 2026-06-09)
- **AND** both sit alongside the existing `adherence`, `nutrition`, `hydration`, `workouts`, `workout_fuel`, `weight`, `phase`, `goal_override` keys

#### Scenario: Recovery and fitness are null when absent (no carryover)

- **WHEN** no recovery snapshot exists for the requested date, even if one exists for a prior day
- **THEN** `recovery` is `null` (NOT the prior day's snapshot)
- **AND** the same same-day-or-null rule applies to `fitness`

#### Scenario: Weight block echoes the new biometrics when present

- **WHEN** the day's body-weight entry carries `muscle_mass_kg` and `bmi`
- **THEN** the `weight` block includes those fields
- **AND** fields absent on the entry are omitted from the block (omitempty)
