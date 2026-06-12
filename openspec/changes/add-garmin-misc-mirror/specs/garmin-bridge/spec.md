# garmin-bridge Specification (delta)

## ADDED Requirements

### Requirement: The daily sync refreshes device, health-vitals, and achievement data

The bridge SHALL, as part of the daily sync, additionally fetch and post the Garmin tail captured by this change: device inventory (`get_devices` + `get_device_last_used`), daily health vitals (`get_blood_pressure` + `get_heart_rates` + `get_all_day_stress`), and earned badges + ad-hoc challenges (`get_earned_badges` + `get_adhoc_challenges`). Each fetch SHALL be individually guarded so that a failing, throttled, or account-unavailable Garmin endpoint yields absent data for that source — never an aborted day. Devices and achievements SHALL be posted as per-item upserts (`POST /devices`, `POST /achievements`, dedup by `external_id`); health vitals SHALL be posted as a date-keyed snapshot (`POST /health-vitals`, upsert by date). The mapper SHALL attach whatever fields are present and omit what is absent. None of this data feeds any nutrition, fueling, energy, or hydration computation — it is reference/coaching context only.

#### Scenario: Devices are mapped and upserted

- **WHEN** the day's `get_devices`/`get_device_last_used` payloads carry one or more paired devices
- **THEN** the bridge maps each into a `POST /devices` body (`external_id`, `display_name`, plus `model`/`last_sync_at`/`battery_pct`/`firmware_version` when present) and upserts it
- **AND** a field absent from the Garmin payload is omitted from the body

#### Scenario: Health vitals are mapped to a date-keyed snapshot

- **WHEN** the day's blood-pressure, all-day-HR, and all-day-stress payloads carry readings
- **THEN** the bridge maps them into one `POST /health-vitals` body for that `date` and upserts it
- **AND** a day with no vital readings is skipped (no empty snapshot posted)

#### Scenario: Badges and challenges are mapped and upserted

- **WHEN** the day's `get_earned_badges`/`get_adhoc_challenges` payloads carry achievements
- **THEN** the bridge maps each into a `POST /achievements` body (`external_id`, `kind`, `name`, plus `earned_at`/`progress_pct` when present) and upserts it
- **AND** the `external_id` is namespaced by `kind` so a badge id and a challenge id never collide on the backend's UNIQUE constraint

#### Scenario: One bad tail fetch does not abort the day

- **WHEN** any one of the device / health-vitals / achievement fetches raises or returns nothing
- **THEN** that source is omitted from the sync
- **AND** the rest of the day's capabilities (recovery, fitness, workouts, and the other tail sources) still sync

### Requirement: The bridge performs activity-level control operations

The bridge SHALL expose, alongside its workout-library operations, five activity-level operations backing the backend control endpoints: read an activity's linked gear (`get_activity_gear`); download a structured workout file (`download_workout`, returning a base64-wrapped envelope); upload a FIT activity (`upload_activity`); rename an activity (`set_activity_name`); and delete an activity (`delete_activity`). These are on-demand operations driven by the control endpoints, NOT part of the daily sync. Deleting an activity Garmin reports as already absent SHALL succeed as a no-op (idempotent).

#### Scenario: Activity gear is read

- **WHEN** the bridge is asked for an activity's gear
- **THEN** it calls garminconnect `get_activity_gear` and returns the gear list

#### Scenario: A structured workout downloads as a base64 envelope

- **WHEN** the bridge is asked to download a structured workout (default `fit` format)
- **THEN** it calls garminconnect `download_workout` and returns a `{garmin_workout_id, format, filename, content_base64}` envelope

#### Scenario: A FIT activity uploads

- **WHEN** the bridge is asked to upload a FIT payload
- **THEN** it calls garminconnect `upload_activity` and returns the created activity reference

#### Scenario: An activity is renamed

- **WHEN** the bridge is asked to rename an activity with a new name
- **THEN** it calls garminconnect `set_activity_name` and returns success

#### Scenario: Deleting an already-absent activity is a no-op

- **WHEN** the bridge is asked to delete an activity that Garmin reports as already gone
- **THEN** the operation succeeds as a no-op (idempotent), not an error

## MODIFIED Requirements

### Requirement: Headless daily sync maps Garmin data to the REST API

The bridge SHALL expose `POST /sync` (optionally for a specific date, default
today) that reads the stored token from the backend (`GET /garmin/token`),
obtains a fresh access token without any interactive step, fetches the day's
Garmin data, and writes it to the existing nutrition REST API under
`GARMIN_API_TOKEN`. The mapping SHALL be: sleep/HRV/RHR/stress →
`/recovery-metrics`; VO2max/training-load → `/fitness-metrics`; sweat loss →
`/hydration-balance`; weigh-ins → `/weight`; activities → `/workouts`
(`source = "garmin"`); device inventory → `/devices`; blood pressure / all-day
HR / all-day stress → `/health-vitals`; earned badges / ad-hoc challenges →
`/achievements`. The device, health-vitals, and achievement targets are
reference/coaching context only and feed no nutrition computation. Sync SHALL
require no MFA or human interaction.

#### Scenario: Daily sync writes a day's data

- **WHEN** `POST /sync` runs with a valid stored token
- **THEN** the bridge refreshes its access token without prompting for MFA
- **AND** posts the day's recovery, fitness, hydration-balance, weight, and
  activity data to their respective endpoints under the garmin identity
- **AND** additionally upserts the day's device inventory, health-vitals snapshot,
  and earned achievements when Garmin provides them

#### Scenario: Re-running a day is idempotent

- **WHEN** `POST /sync` is run twice for the same date
- **THEN** the date-keyed metrics are upserted (not duplicated)
- **AND** activities are deduped by `external_id = "garmin:<activity_id>"` via the
  existing `/workouts` UPSERT (no new field or migration)
- **AND** devices and achievements are deduped by `external_id`, and the
  health-vitals snapshot is upserted by `date` (no duplicates on the second run)

#### Scenario: Sync with no stored token fails clearly

- **WHEN** `POST /sync` runs and the backend has no stored token (`404`)
- **THEN** the bridge returns an error indicating a login is required
- **AND** writes nothing
