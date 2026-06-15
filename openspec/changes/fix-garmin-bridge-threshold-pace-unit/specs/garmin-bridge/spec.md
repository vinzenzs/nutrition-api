## MODIFIED Requirements

### Requirement: The bridge refreshes the athlete physiology config each sync

The bridge SHALL, on each daily sync, fetch the athlete's Garmin physiology configuration and write it to the backend via `PUT /athlete-config`. The values SHALL be read from the endpoints that actually expose them: FTP from `get_cycling_ftp()` (`functionalThresholdPower` → `ftp_watts`); lactate-threshold HR and threshold paces from `get_user_profile()`'s `userData` (`lactateThresholdHeartRate` → `lactate_threshold_hr`; `lactateThresholdSpeed` → `threshold_pace_sec_per_km`; `lactateThresholdSwimSpeed` → `threshold_swim_pace_sec_per_100m`); and max HR and HR-zone maxima from the heart-rate-zones endpoint (`/biometric-service/heartRateZones`, fetched via raw `connectapi` as the Garmin client exposes no helper) using the `DEFAULT`-sport entry, where `max_hr` ← `maxHeartRateUsed` and each `hr_zone_N_max` is derived from the zone floors (zone N's max is zone N+1's floor for N in 1..4, and zone 5's max is `maxHeartRateUsed`). The `lactateThresholdSpeed` / `lactateThresholdSwimSpeed` fields are pace values in **seconds per metre** (not metres-per-second), so the conversions SHALL be `threshold_pace_sec_per_km = lactateThresholdSpeed × 1000` and `threshold_swim_pace_sec_per_100m = lactateThresholdSwimSpeed × 100`; a converted pace falling outside a plausible band SHALL be omitted rather than stored. The bridge SHALL NOT read these from `get_userprofile_settings()` (which carries only display preferences and no `userData` or zones) nor treat `userData.ftpAutoDetected` (a boolean flag) as a value. Power-zone boundaries SHALL be omitted while no Garmin endpoint exposes them. Because this configuration is slowly-changing physiology and NOT a date-keyed snapshot, the refresh is a single in-place singleton upsert (not one row per day): the same `PUT /athlete-config` is re-issued each sync and overwrites the prior config (Garmin is source-of-truth). Each config fetch SHALL be individually guarded by the existing `safe()` pattern so a failing, throttled, or account-unavailable Garmin endpoint yields absent config for that sync — never an aborted day. The mapper SHALL attach whatever fields are present and omit what is absent.

#### Scenario: Threshold pace is converted from seconds-per-metre

- **WHEN** `get_user_profile().userData` carries a `lactateThresholdSpeed` of `0.25` (seconds per metre)
- **THEN** the `PUT /athlete-config` body carries `threshold_pace_sec_per_km` of `250` (`0.25 × 1000`), not a reciprocal-derived value

#### Scenario: An out-of-band converted pace is omitted

- **WHEN** the converted threshold pace falls outside a plausible band (e.g. a garbage source yielding far below 1:30/km or far above 20:00/km)
- **THEN** the pace field is omitted from the `PUT /athlete-config` body rather than stored, and the rest of the config is still written

#### Scenario: Config is fetched from the real endpoints, mapped, and written via the singleton PUT

- **WHEN** a daily sync runs and `get_cycling_ftp()` returns a `functionalThresholdPower`, `get_user_profile().userData` carries `lactateThresholdHeartRate`, and `/biometric-service/heartRateZones` returns a `DEFAULT`-sport entry with zone floors and `maxHeartRateUsed`
- **THEN** the bridge issues `PUT /athlete-config` with `ftp_watts`, `lactate_threshold_hr`, `max_hr`, and `hr_zone_1_max..hr_zone_5_max` derived from the zone floors
- **AND** a field absent from Garmin's responses is simply omitted from the request body

#### Scenario: Preferences-only settings and the FTP flag are not used as config sources

- **WHEN** `get_userprofile_settings()` returns only display preferences (no `userData`, no zones) and `userData.ftpAutoDetected` is the boolean `true`
- **THEN** the bridge does not derive any config field from `get_userprofile_settings()` and does not treat `ftpAutoDetected` as `ftp_watts`

#### Scenario: Power zones are omitted while unavailable

- **WHEN** a sync runs and no Garmin endpoint exposes configured power-zone boundaries
- **THEN** the `PUT /athlete-config` body omits all `power_zone_*` fields and the rest of the config is still written

#### Scenario: Config refresh is a non-date-keyed singleton overwrite

- **WHEN** `POST /sync` is run on two different dates
- **THEN** each run re-issues `PUT /athlete-config` overwriting the single config row in place (no per-day config rows accumulate)
- **AND** the most recent Garmin values win (Garmin is source-of-truth for these fields)

#### Scenario: A failing config fetch does not abort the day

- **WHEN** `get_cycling_ftp`, `get_user_profile`, or the heart-rate-zones fetch raises or returns nothing during a sync
- **THEN** the bridge omits the corresponding config fields (or skips the `PUT` when nothing was obtained)
- **AND** the rest of the day's recovery, fitness, hydration-balance, weight, and activity sync still completes
