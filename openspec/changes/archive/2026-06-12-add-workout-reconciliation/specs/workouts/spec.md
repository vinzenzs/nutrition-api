# workouts — delta for add-workout-reconciliation

## ADDED Requirements

### Requirement: Garmin imports reconcile against open planned workouts

The system SHALL reconcile an ingested completed Garmin activity against an open
planned workout. When a completed activity is ingested via `POST /workouts` or
`POST /workouts/bulk` with `source='garmin'` and its `external_id` is not already
stored, the system SHALL attempt to match exactly one **open planned workout** —
a row with `status='planned'`, `external_id IS NULL`, the same sport, and the
same **local calendar day** as the activity's start. On exactly one match the
system SHALL **fulfill** that planned workout in place: set its `external_id`,
`source`, and actual metrics from the activity, flip `status` to `completed`, and
retain its `template_id` and `plan_slot_id`; no new row is created. On no match
the system SHALL insert a standalone completed row (the prior behavior). On more
than one candidate the system SHALL insert a standalone completed row and mark it
as needing a link rather than guess. The match SHALL run only on first sight; a
subsequent re-sync of the same activity follows the existing `external_id` UPSERT
path.

#### Scenario: A completed import fulfills the matching planned workout

- **WHEN** a `garmin` activity is ingested for a sport and local day on which
  exactly one open planned workout exists
- **THEN** that planned workout is updated to `status='completed'` with the
  activity's `external_id`, `source`, and actual metrics
- **AND** its `template_id` and `plan_slot_id` are retained
- **AND** no second row is created

#### Scenario: No matching planned workout creates a standalone row

- **WHEN** a `garmin` activity is ingested and no open planned workout matches its
  sport and local day
- **THEN** a standalone completed workout is created (the prior behavior)

#### Scenario: Ambiguous match is flagged, not guessed

- **WHEN** a `garmin` activity matches more than one open planned workout of the
  same sport on the same local day
- **THEN** a standalone completed workout is created and marked as needing a link
- **AND** no planned workout is auto-fulfilled

#### Scenario: Re-sync of a fulfilled activity is idempotent

- **WHEN** the daily sync re-sends an activity whose `external_id` is already
  stored (on a fulfilled planned row)
- **THEN** ingestion follows the existing `external_id` UPSERT path and updates
  that row in place
- **AND** reconciliation does not run again

#### Scenario: Matching uses local calendar day and exact sport

- **WHEN** an activity starts late in the local evening
- **THEN** it is matched against planned workouts on that local date (not the UTC
  date)
- **AND** only planned workouts of the same sport are considered

### Requirement: Explicit fulfill and unfulfill endpoints

The system SHALL expose `POST /workouts/{plannedId}/fulfill` accepting a
`completed_id`, which merges an existing completed activity into an existing
planned workout (copying `external_id`, `source`, and actuals onto the planned
row, flipping it to `completed`, removing the redundant standalone row, and
clearing any needs-link flag); and `POST /workouts/{id}/unfulfill`, which
reverses a merge (clearing `external_id` and actuals and restoring
`status='planned'`). The planned row is the surviving identity in a merge so its
`plan_slot_id` remains stable.

#### Scenario: Manual fulfill merges two existing rows

- **WHEN** a client `POST`s `/workouts/{plannedId}/fulfill` with the id of a
  standalone completed activity of the same session
- **THEN** the planned workout becomes `completed` with the activity's
  `external_id` and actuals
- **AND** the standalone completed row is removed
- **AND** the planned workout's `plan_slot_id` is unchanged

#### Scenario: Unfulfill restores the planned workout

- **WHEN** a client `POST`s `/workouts/{id}/unfulfill` on a fulfilled workout
- **THEN** its `external_id` and actual metrics are cleared and `status` returns
  to `planned`
- **AND** the row retains its `template_id` and `plan_slot_id`

#### Scenario: Fulfill clears the needs-link flag

- **WHEN** a flagged (needs-link) completed activity is merged via `fulfill`
- **THEN** the needs-link flag is cleared on the surviving row
