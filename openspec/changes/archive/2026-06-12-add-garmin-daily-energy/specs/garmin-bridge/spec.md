# garmin-bridge Specification (delta)

## ADDED Requirements

### Requirement: The bridge fetches and maps the whole-day user summary

The bridge SHALL, for each synced day, additionally fetch Garmin's whole-day energy/activity summary (`get_user_summary(date)`) and map it onto the backend's `/daily-summary` endpoint. The fetch SHALL be individually guarded by the existing `safe()` pattern so that a failing, throttled, or unavailable Garmin endpoint yields an absent daily-summary snapshot for that day — never an aborted day. The mapper (`map_daily_summary`) SHALL extract the documented fields defensively, attaching whatever is present and omitting what is absent.

#### Scenario: User summary is fetched under the safe() guard

- **WHEN** the bridge syncs a day
- **THEN** it fetches `get_user_summary(date)` via the Garmin client wrapped in `safe()`
- **AND** if that endpoint raises or returns nothing, the day's other capabilities still sync and no daily-summary snapshot is posted

#### Scenario: User summary fields are mapped to the daily-summary body

- **WHEN** `get_user_summary(date)` returns `activeKilocalories`, `bmrKilocalories`, `totalKilocalories`, `totalSteps`, `floorsAscended`, `moderateIntensityMinutes`, `vigorousIntensityMinutes`, and `totalDistanceMeters`
- **THEN** `map_daily_summary` produces a body with `active_kcal`, `resting_kcal`, `total_kcal`, `steps`, `floors`, `moderate_intensity_minutes`, `vigorous_intensity_minutes`, and `distance_m` respectively
- **AND** a field absent from the Garmin payload is omitted from the body (stored NULL by the backend)

#### Scenario: Daily summary is posted as a date-keyed snapshot, idempotently

- **WHEN** the bridge syncs a day with a non-empty user summary
- **THEN** it POSTs the mapped body to `/daily-summary` under the garmin identity in the same date-keyed snapshot flow as the other daily metrics
- **AND** re-running the sync for the same date upserts the snapshot in place (no duplicate row)

## MODIFIED Requirements

### Requirement: Headless daily sync maps Garmin data to the REST API

The bridge SHALL expose `POST /sync` (optionally for a specific date, default
today) that reads the stored token from the backend (`GET /garmin/token`),
obtains a fresh access token without any interactive step, fetches the day's
Garmin data, and writes it to the existing nutrition REST API under
`GARMIN_API_TOKEN`. The mapping SHALL be: sleep/HRV/RHR/stress →
`/recovery-metrics`; VO2max/training-load → `/fitness-metrics`; sweat loss →
`/hydration-balance`; whole-day energy/activity totals → `/daily-summary`;
weigh-ins → `/weight`; activities → `/workouts` (`source = "garmin"`), where
each activity additionally carries the scalar performance and HR-zone fields
plus nested `splits`/`sets` detail when Garmin provides them. Sync SHALL require
no MFA or human interaction.

#### Scenario: Daily sync writes a day's data

- **WHEN** `POST /sync` runs with a valid stored token
- **THEN** the bridge refreshes its access token without prompting for MFA
- **AND** posts the day's recovery, fitness, hydration-balance, daily-summary,
  weight, and activity data to their respective endpoints under the garmin
  identity
- **AND** each activity item carries the available scalar/zone/split/set detail

#### Scenario: Re-running a day is idempotent

- **WHEN** `POST /sync` is run twice for the same date
- **THEN** the date-keyed metrics (including `/daily-summary`) are upserted (not duplicated)
- **AND** activities are deduped by `external_id = "garmin:<activity_id>"` via the
  existing `/workouts` UPSERT (no new field or migration)
- **AND** each activity's nested splits and sets are replaced (not duplicated) on the second run

#### Scenario: Sync with no stored token fails clearly

- **WHEN** `POST /sync` runs and the backend has no stored token (`404`)
- **THEN** the bridge returns an error indicating a login is required
- **AND** writes nothing
