# garmin-bridge Specification

## Purpose

Provide a small, stateless bridge between Garmin Connect and the nutrition REST
API. It handles the interactive multi-factor login needed to mint a Garmin auth
token, persists that token to the backend (it holds no durable local state
itself), and runs a headless daily sync that fetches a day's Garmin data and
maps it onto the existing REST endpoints under the garmin identity.
## Requirements
### Requirement: Interactive MFA login mints and persists a Garmin token

The bridge SHALL expose a two-step login that performs Garmin SSO with
credentials it reads from its own configuration (never from the request body),
and SHALL handle multi-factor auth: `POST /login` begins the flow and, when MFA
is required, responds indicating a code is needed; `POST /login/mfa` completes
the flow with the supplied code. On success the bridge SHALL persist the minted
token blob to the backend via `PUT /garmin/token` and SHALL NOT return the blob
to the caller. The Garmin password SHALL NOT appear in any response or log.

#### Scenario: Login requiring MFA

- **WHEN** `POST /login` is called and Garmin requires MFA
- **THEN** the response indicates an MFA code is needed (e.g. `{"needs_mfa": true}`)
- **AND** the bridge retains the in-progress SSO state to resume with the code

#### Scenario: Completing MFA persists the token

- **WHEN** `POST /login/mfa` is called with the correct 6-digit code
- **THEN** login completes and the minted token blob is sent to the backend via `PUT /garmin/token`
- **AND** the response confirms success without returning the token

#### Scenario: Credentials never transit the request or logs

- **WHEN** any login request is processed
- **THEN** the Garmin password is taken from configuration, not the request
- **AND** neither the password nor the token blob appears in logs or responses

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
plus nested `splits`/`sets` detail when Garmin provides them; gear inventory →
`/gear` (upsert by Garmin gear id); personal records → `/personal-records`
(upsert by Garmin PR id). Gear and personal records are slowly-changing
inventory refreshed via idempotent upsert on each sync, not date-keyed
snapshots. Sync SHALL require no MFA or human interaction.

#### Scenario: Daily sync writes a day's data

- **WHEN** `POST /sync` runs with a valid stored token
- **THEN** the bridge refreshes its access token without prompting for MFA
- **AND** posts the day's recovery, fitness, hydration-balance, daily-summary,
  weight, and activity data to their respective endpoints under the garmin
  identity
- **AND** each activity item carries the available scalar/zone/split/set detail
- **AND** upserts the current gear and personal-record inventory to `/gear` and
  `/personal-records`

#### Scenario: Re-running a day is idempotent

- **WHEN** `POST /sync` is run twice for the same date
- **THEN** the date-keyed metrics (including `/daily-summary`) are upserted (not duplicated)
- **AND** activities are deduped by `external_id = "garmin:<activity_id>"` via the
  existing `/workouts` UPSERT (no new field or migration)
- **AND** each activity's nested splits and sets are replaced (not duplicated) on the second run
- **AND** gear and personal records are upserted by their Garmin external id
  (re-observing the same item updates it in place, no duplicate)

#### Scenario: Sync with no stored token fails clearly

- **WHEN** `POST /sync` runs and the backend has no stored token (`404`)
- **THEN** the bridge returns an error indicating a login is required
- **AND** writes nothing

### Requirement: The bridge is stateless except during the login window

The bridge SHALL hold no durable local state: the auth token lives in the
backend, so a restarted or rescheduled bridge resumes by reading it. The only
in-memory state is the transient SSO context between `POST /login` and
`POST /login/mfa`; because of it the bridge SHALL run as a single replica.

#### Scenario: Restart between syncs loses nothing

- **WHEN** the bridge process restarts between two daily syncs
- **THEN** the next `POST /sync` reads the token from the backend and proceeds
- **AND** no re-login is required

#### Scenario: Single replica for the login handshake

- **WHEN** the bridge is deployed
- **THEN** it runs with a single replica so the MFA resume reaches the pod that
  began the login

### Requirement: The bridge compiles and creates structured workouts in Garmin

The bridge SHALL expose `POST /workouts` accepting a sport, a name, and the
backend's structured step model (intents, durations by time/distance/lap-button/
open, and targets by HR/power zone, pace, RPE, or absolute HR/power), and SHALL
translate it into a garminconnect structured-workout payload
(`executableStepDTO` end conditions and targets; `repeatGroupDTO` for repeat
groups), create it in the athlete's Garmin workout library, and return the
created Garmin workout id. The garminconnect payload shape SHALL exist only in
the bridge and SHALL NOT be returned to or required from the backend.

#### Scenario: A structured workout is created and its id returned

- **WHEN** `POST /workouts` is called with a run workout whose steps include a
  warmup, a repeat group of intervals with a power-zone target, and a cooldown
- **THEN** the bridge builds the garminconnect payload, creates the workout in
  the Garmin library, and responds with the Garmin workout id

#### Scenario: The Garmin payload shape stays inside the bridge

- **WHEN** the backend calls `POST /workouts`
- **THEN** it sends only the sport, name, and step model
- **AND** the response carries the opaque Garmin workout id, not the garminconnect payload

### Requirement: The bridge schedules and unschedules workouts on the calendar

The bridge SHALL expose `POST /schedule` accepting a Garmin workout id and a
date, placing that workout on the Garmin calendar and returning the Garmin
schedule id; and `DELETE /schedule` accepting a Garmin schedule id, removing the
scheduled entry. Deleting an already-absent schedule id SHALL succeed as a no-op.

#### Scenario: Scheduling returns a schedule id

- **WHEN** `POST /schedule` is called with a Garmin workout id and a date
- **THEN** the workout is placed on that date and the response carries the Garmin schedule id

#### Scenario: Unscheduling is idempotent

- **WHEN** `DELETE /schedule` is called with a schedule id that is already gone
- **THEN** the response indicates success (no-op)

### Requirement: The bridge reads the Garmin calendar for a date range

The bridge SHALL expose `GET /calendar` accepting a date range and returning the
scheduled workouts in that range, for reconciliation by the backend.

#### Scenario: Calendar read returns scheduled items

- **WHEN** `GET /calendar` is called with a from/to range that contains scheduled workouts
- **THEN** the response lists those scheduled items with their Garmin schedule ids

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

### Requirement: The bridge enriches recovery and fitness snapshots with additional daily signals

The bridge SHALL attach to each synced day's recovery and fitness snapshots the additional daily signals Garmin exposes: blood-oxygen (SpO2), respiration, and the per-stage sleep breakdown on the recovery snapshot; endurance score, hill score, fitness age, and the training-status label on the fitness snapshot. The sleep-stage seconds and the training-status label SHALL be read from payloads the bridge already fetches (the sleep DTO and `get_training_status`) with no extra Garmin call; SpO2, respiration, endurance, hill, and fitness-age SHALL each be fetched per day via an individually `safe()`-guarded call so that a failing, throttled, or account-unavailable Garmin endpoint yields absent detail for that field — never an aborted day. The mapper SHALL attach whatever detail is present and omit what is absent, posting the result onto the EXISTING `/recovery-metrics` and `/fitness-metrics` targets (no new sync target endpoint).

#### Scenario: Sleep-stage and training-status come free from already-fetched payloads

- **WHEN** the day's sleep DTO carries deep / light / REM / awake stage seconds and the already-fetched training-status payload carries a status phrase
- **THEN** the bridge maps the four sleep-stage seconds into the `/recovery-metrics` body and the `training_status` label into the `/fitness-metrics` body with NO additional Garmin call
- **AND** a field absent from those payloads is simply omitted from the snapshot

#### Scenario: SpO2, respiration, endurance, hill, and fitness-age are fetched per day and guarded

- **WHEN** the bridge syncs a day
- **THEN** it fetches SpO2, respiration, endurance score, hill score, and fitness age via the Garmin client, each through the existing `safe()` wrapper
- **AND** if one of those endpoints raises or returns nothing, the corresponding field is omitted from the recovery or fitness snapshot
- **AND** the rest of the day's snapshots and capabilities still sync (one bad detail fetch is not fatal)

#### Scenario: Enriched snapshots are posted to the existing snapshot endpoints

- **WHEN** a synced day yields SpO2 / respiration / sleep-stage detail and endurance / hill / fitness-age / training-status detail
- **THEN** the recovery detail is posted on the `/recovery-metrics` upsert and the fitness detail on the `/fitness-metrics` upsert
- **AND** re-running the sync for the same date re-posts the same snapshots, and the date-keyed full-replace upsert leaves no duplicate rows

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

