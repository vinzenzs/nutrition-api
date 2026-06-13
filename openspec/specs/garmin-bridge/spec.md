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
(upsert by Garmin PR id); the athlete's physiology configuration (FTP,
thresholds, max HR, lactate-threshold HR, HR-zone and optional power-zone
boundaries) → `PUT /athlete-config` as a non-date-keyed singleton refresh
(in-place overwrite, Garmin source-of-truth); device inventory → `/devices`;
blood pressure / all-day HR / all-day stress → `/health-vitals`; and earned
badges / ad-hoc challenges → `/achievements`. Gear, personal records, devices,
and achievements are slowly-changing inventory refreshed via idempotent upsert
on each sync, not date-keyed snapshots; the device, health-vitals, and
achievement targets are reference/coaching context only and feed no nutrition
computation. Each per-capability fetch is guarded so its failure does not abort
the day. Sync SHALL require no MFA or human interaction.

#### Scenario: Daily sync writes a day's data

- **WHEN** `POST /sync` runs with a valid stored token
- **THEN** the bridge refreshes its access token without prompting for MFA
- **AND** posts the day's recovery, fitness, hydration-balance, daily-summary,
  weight, and activity data to their respective endpoints under the garmin
  identity
- **AND** each activity item carries the available scalar/zone/split/set detail
- **AND** upserts the current gear and personal-record inventory to `/gear` and
  `/personal-records`
- **AND** refreshes the athlete physiology config via `PUT /athlete-config` when
  Garmin provides it
- **AND** additionally upserts the day's device inventory, health-vitals snapshot,
  and earned achievements when Garmin provides them

#### Scenario: Re-running a day is idempotent

- **WHEN** `POST /sync` is run twice for the same date
- **THEN** the date-keyed metrics (including `/daily-summary` and `/health-vitals`) are upserted (not duplicated)
- **AND** activities are deduped by `external_id = "garmin:<activity_id>"` via the
  existing `/workouts` UPSERT (no new field or migration)
- **AND** each activity's nested splits and sets are replaced (not duplicated) on the second run
- **AND** gear and personal records are upserted by their Garmin external id, and
  the athlete config is re-written in place via the singleton `PUT`
- **AND** devices and achievements are deduped by `external_id`, and the
  health-vitals snapshot is upserted by `date` (no duplicates on the second run)

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
Removing the **calendar entry** does NOT delete the underlying structured
**workout object** from the library — that object is deleted via the separate
`DELETE /workouts/{garmin_workout_id}` operation, which the backend invokes on
the unschedule and re-push paths to avoid orphaning prior objects.

#### Scenario: Scheduling returns a schedule id

- **WHEN** `POST /schedule` is called with a Garmin workout id and a date
- **THEN** the workout is placed on that date and the response carries the Garmin schedule id

#### Scenario: Unscheduling is idempotent

- **WHEN** `DELETE /schedule` is called with a schedule id that is already gone
- **THEN** the response indicates success (no-op)

#### Scenario: Unscheduling leaves the library object for separate deletion

- **WHEN** `DELETE /schedule` removes a calendar entry
- **THEN** the underlying structured workout object remains in the library
- **AND** the backend removes it via `DELETE /workouts/{garmin_workout_id}`

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

### Requirement: The bridge refreshes the athlete physiology config each sync

The bridge SHALL, on each daily sync, fetch the athlete's Garmin physiology configuration — FTP, thresholds, max HR, and HR-zone (and any power-zone) boundaries — from the user-profile and user-settings payloads (`get_user_profile` / `get_userprofile_settings`, the source endpoints actually exposed by the Garmin client; the zone boundaries ride in the user-settings payload), map them to the `athlete-config` shape, and write them to the backend via `PUT /athlete-config`. Because this configuration is slowly-changing physiology and NOT a date-keyed snapshot, the refresh is a single in-place singleton upsert (not one row per day): the same `PUT /athlete-config` is re-issued each sync and overwrites the prior config (Garmin is source-of-truth). Each config fetch SHALL be individually guarded by the existing `safe()` pattern so a failing, throttled, or account-unavailable Garmin endpoint yields absent config for that sync — never an aborted day. The mapper SHALL attach whatever fields are present and omit what is absent.

#### Scenario: Config is fetched, mapped, and written via the singleton PUT

- **WHEN** a daily sync runs and Garmin's profile/settings payload carries FTP and threshold HR and five HR-zone boundaries
- **THEN** the bridge maps them and issues `PUT /athlete-config` with `ftp_watts`, `threshold_hr`, and `hr_zone_1_max..hr_zone_5_max`
- **AND** a field absent from Garmin's response is simply omitted from the request body

#### Scenario: Config refresh is a non-date-keyed singleton overwrite

- **WHEN** `POST /sync` is run on two different dates
- **THEN** each run re-issues `PUT /athlete-config` overwriting the single config row in place (no per-day config rows accumulate)
- **AND** the most recent Garmin values win (Garmin is source-of-truth for these fields)

#### Scenario: A failing profile or zone fetch does not abort the day

- **WHEN** `get_user_profile` or `get_heart_rate_zones` raises or returns nothing during a sync
- **THEN** the bridge omits the corresponding config fields (or skips the `PUT` when nothing was obtained)
- **AND** the rest of the day's recovery, fitness, hydration-balance, weight, and activity sync still completes

### Requirement: The bridge deletes a structured workout object from the library

The bridge SHALL expose `DELETE /workouts/{garmin_workout_id}` that deletes the
named structured workout object from the athlete's Garmin workout library via
garminconnect. A workout id that Garmin reports as already absent (404 / not
found) SHALL be treated as a no-op success, so the backend's re-push and
unschedule paths can call it safely on retry. A genuine Garmin error SHALL
surface as `502 garmin_error`. This complements the existing create — the bridge
previously created library objects but never removed them, orphaning every
prior object on re-push.

#### Scenario: Deleting an existing object removes it

- **WHEN** `DELETE /workouts/{garmin_workout_id}` is called with a live object's id
- **THEN** the bridge deletes that object from the Garmin library
- **AND** responds indicating it was deleted

#### Scenario: Deleting an already-absent object is a no-op

- **WHEN** `DELETE /workouts/{garmin_workout_id}` is called with an id Garmin no longer has
- **THEN** the bridge treats the 404 as success and responds indicating the object was already absent

#### Scenario: A genuine Garmin error surfaces

- **WHEN** Garmin returns a non-404 error deleting the object
- **THEN** the bridge responds `502 garmin_error`

### Requirement: The bridge reads the Garmin workout library

The bridge SHALL expose `GET /workouts` returning the structured workouts in the
athlete's Garmin library (accepting optional `start`/`limit` pagination
passthrough) and `GET /workouts/{garmin_workout_id}` returning one library object
by id, for reconciliation by the backend. A token-less request SHALL return
`409 login_required`, matching the other token-backed bridge operations.

#### Scenario: Library list returns the stored workouts

- **WHEN** `GET /workouts` is called with a valid stored token
- **THEN** the response lists the athlete's library workouts with their Garmin ids

#### Scenario: By-id read returns one object

- **WHEN** `GET /workouts/{garmin_workout_id}` is called for an existing object
- **THEN** the response carries that workout object

#### Scenario: No stored token fails clearly

- **WHEN** either read is called and the backend has no stored token
- **THEN** the bridge returns `409 login_required` and reads nothing

### Requirement: The bridge pushes a hydration value back to Garmin

The bridge SHALL expose `POST /hydration` accepting `{value_ml, date}` and
recording that value against the date in Garmin via garminconnect
`add_hydration_data`. This is the only write FROM the nutrition system TO Garmin.
Because Garmin records a day's total (set/replace, not append), re-posting the
same date overwrites rather than accumulates. A token-less request SHALL return
`409 login_required`.

#### Scenario: Hydration value is recorded for a date

- **WHEN** `POST /hydration` is called with `{"value_ml":2400,"date":"2026-06-12"}` and a valid token
- **THEN** the bridge records 2400 ml against that date in Garmin
- **AND** responds with success

#### Scenario: Re-posting the same date overwrites

- **WHEN** `POST /hydration` is called twice for the same date with different values
- **THEN** Garmin holds the latter value for that date (set/replace, not summed)

#### Scenario: No stored token fails clearly

- **WHEN** `POST /hydration` is called and the backend has no stored token
- **THEN** the bridge returns `409 login_required` and writes nothing

### Requirement: The bridge exports an activity's FIT/GPX blob

The bridge SHALL expose `GET /activity/{activity_id}/export` accepting an optional
`format` (default `fit`; `gpx`/`tcx`/`csv` passthrough) that downloads the
activity file from Garmin via garminconnect `download_activity` and returns it as
a base64-wrapped JSON envelope `{activity_id, format, filename, content_base64}`
— never streaming raw binary, so the blob crosses the JSON control/MCP transport
intact. A token-less request SHALL return `409 login_required`.

#### Scenario: Export returns the base64 envelope

- **WHEN** `GET /activity/{activity_id}/export` is called with a valid token
- **THEN** the bridge downloads the FIT file and responds with
  `{activity_id, format, filename, content_base64}` where `content_base64` is the file's base64 encoding

#### Scenario: Format is honoured

- **WHEN** `GET /activity/{activity_id}/export?format=gpx` is called
- **THEN** the bridge downloads the GPX file and the envelope's `format` is `gpx`

#### Scenario: No stored token fails clearly

- **WHEN** `GET /activity/{activity_id}/export` is called and the backend has no stored token
- **THEN** the bridge returns `409 login_required` and exports nothing

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

