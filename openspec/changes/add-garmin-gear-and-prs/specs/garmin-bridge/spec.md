# garmin-bridge Specification (delta)

## ADDED Requirements

### Requirement: The bridge refreshes gear and personal-record inventory on each sync

The bridge SHALL, as part of the daily sync, fetch Garmin's gear inventory (via `get_gear` and `get_gear_stats`) and personal records (via `get_personal_records`), map them, and upsert each item to the backend (`POST /gear`, `POST /personal-records`). Because gear and PRs are slowly-changing inventory rather than date-keyed data, the refresh is an idempotent upsert by external id — re-observing the same gear or PR updates the row in place. Each inventory fetch SHALL be individually guarded with the existing `safe()` pattern so that a failing, throttled, or account-unavailable Garmin endpoint yields absent inventory for that sync — never an aborted day. The mapper SHALL join `get_gear` with `get_gear_stats` by gear id and SHALL omit any field Garmin does not supply.

#### Scenario: Gear and PRs are upserted on a normal sync

- **WHEN** `POST /sync` runs and Garmin returns two pieces of gear and three personal records
- **THEN** the bridge upserts two `/gear` items (joining gear with gear-stats mileage) and three `/personal-records` items
- **AND** re-running the sync upserts the same items in place, leaving no duplicates (dedup by external id)

#### Scenario: A failing inventory endpoint does not abort the day

- **WHEN** `get_personal_records` raises or returns nothing during a sync
- **THEN** the personal-record refresh is skipped for that sync
- **AND** gear, the date-keyed metrics, weigh-ins, and activities still sync (one bad inventory fetch is not fatal)

#### Scenario: Gear without stats still syncs

- **WHEN** `get_gear` returns gear but `get_gear_stats` is absent for an item
- **THEN** the bridge posts the gear item with its type and display name and omits `total_distance_m` / `total_activities`
- **AND** the backend stores the gear row with NULL distance and activity count

## MODIFIED Requirements

### Requirement: Headless daily sync maps Garmin data to the REST API

The bridge SHALL expose `POST /sync` (optionally for a specific date, default
today) that reads the stored token from the backend (`GET /garmin/token`),
obtains a fresh access token without any interactive step, fetches the day's
Garmin data, and writes it to the existing nutrition REST API under
`GARMIN_API_TOKEN`. The mapping SHALL be: sleep/HRV/RHR/stress →
`/recovery-metrics`; VO2max/training-load → `/fitness-metrics`; sweat loss →
`/hydration-balance`; weigh-ins → `/weight`; activities → `/workouts`
(`source = "garmin"`); gear inventory → `/gear` (upsert by Garmin gear id);
personal records → `/personal-records` (upsert by Garmin PR id). Gear and
personal records are slowly-changing inventory refreshed via idempotent upsert
on each sync, not date-keyed snapshots. Sync SHALL require no MFA or human
interaction.

#### Scenario: Daily sync writes a day's data

- **WHEN** `POST /sync` runs with a valid stored token
- **THEN** the bridge refreshes its access token without prompting for MFA
- **AND** posts the day's recovery, fitness, hydration-balance, weight, and
  activity data to their respective endpoints under the garmin identity
- **AND** upserts the current gear and personal-record inventory to `/gear` and
  `/personal-records`

#### Scenario: Re-running a day is idempotent

- **WHEN** `POST /sync` is run twice for the same date
- **THEN** the date-keyed metrics are upserted (not duplicated)
- **AND** activities are deduped by `external_id = "garmin:<activity_id>"` via the
  existing `/workouts` UPSERT (no new field or migration)
- **AND** gear and personal records are upserted by their Garmin external id
  (re-observing the same item updates it in place, no duplicate)

#### Scenario: Sync with no stored token fails clearly

- **WHEN** `POST /sync` runs and the backend has no stored token (`404`)
- **THEN** the bridge returns an error indicating a login is required
- **AND** writes nothing
