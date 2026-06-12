# garmin-bridge Specification (delta)

## ADDED Requirements

### Requirement: The bridge enriches activities with per-activity detail

The bridge SHALL, for each activity in a synced day, attach the richer detail Garmin exposes: scalar performance fields read from the activity summary already fetched (no extra Garmin call), plus HR-zone time, per-lap splits, strength sets, and ambient weather (humidity, wind) fetched per activity. Each per-activity detail fetch SHALL be individually guarded so that a failing, throttled, or account-unavailable Garmin endpoint yields absent detail for that activity — never an aborted day. The mapper SHALL attach whatever detail is present and omit what is absent, posting the result as the scalar/zone/weather fields plus nested `splits`/`sets` arrays on the `/workouts/bulk` items.

#### Scenario: Scalar fields are mapped from the existing activity summary

- **WHEN** the day's `get_activities_by_date` payload carries elevation gain, normalized power, cadence, stride, max HR, and training-effect fields for an activity
- **THEN** the bridge maps them into the `/workouts` item (`elevation_gain_m`, `normalized_power_w`, `avg_cadence`, etc.) with NO additional Garmin call
- **AND** a field absent from the summary is simply omitted from the item

#### Scenario: HR-zone, split, set, and weather detail are fetched per activity and guarded

- **WHEN** the bridge syncs a day with two activities
- **THEN** for each activity it fetches HR-zone time, splits, exercise sets, and weather (`get_activity_weather`) via the Garmin client
- **AND** if one of those endpoints raises or returns nothing for an activity, that activity's item omits the corresponding field/array (e.g. an indoor activity with no weather omits `humidity_pct`/`wind_speed_mps`)
- **AND** the rest of the day's activities and capabilities still sync (one bad detail fetch is not fatal)

#### Scenario: Detail is posted as nested arrays on the bulk upsert

- **WHEN** an endurance activity has per-lap splits and a strength activity has exercise sets
- **THEN** the endurance item carries a `splits` array and the strength item carries a `sets` array on the `/workouts/bulk` request
- **AND** re-running the sync for the same date re-posts the same items, and the backend's replace-on-resync upsert leaves no duplicate splits or sets

## MODIFIED Requirements

### Requirement: Headless daily sync maps Garmin data to the REST API

The bridge SHALL expose `POST /sync` (optionally for a specific date, default
today) that reads the stored token from the backend (`GET /garmin/token`),
obtains a fresh access token without any interactive step, fetches the day's
Garmin data, and writes it to the existing nutrition REST API under
`GARMIN_API_TOKEN`. The mapping SHALL be: sleep/HRV/RHR/stress →
`/recovery-metrics`; VO2max/training-load → `/fitness-metrics`; sweat loss →
`/hydration-balance`; weigh-ins → `/weight`; activities → `/workouts`
(`source = "garmin"`), where each activity additionally carries the scalar
performance and HR-zone fields plus nested `splits`/`sets` detail when Garmin
provides them. Sync SHALL require no MFA or human interaction.

#### Scenario: Daily sync writes a day's data

- **WHEN** `POST /sync` runs with a valid stored token
- **THEN** the bridge refreshes its access token without prompting for MFA
- **AND** posts the day's recovery, fitness, hydration-balance, weight, and
  activity data to their respective endpoints under the garmin identity
- **AND** each activity item carries the available scalar/zone/split/set detail

#### Scenario: Re-running a day is idempotent

- **WHEN** `POST /sync` is run twice for the same date
- **THEN** the date-keyed metrics are upserted (not duplicated)
- **AND** activities are deduped by `external_id = "garmin:<activity_id>"` via the
  existing `/workouts` UPSERT (no new field or migration)
- **AND** each activity's nested splits and sets are replaced (not duplicated) on the second run

#### Scenario: Sync with no stored token fails clearly

- **WHEN** `POST /sync` runs and the backend has no stored token (`404`)
- **THEN** the bridge returns an error indicating a login is required
- **AND** writes nothing
