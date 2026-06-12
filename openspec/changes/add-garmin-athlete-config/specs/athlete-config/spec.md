# athlete-config Specification (delta)

## ADDED Requirements

### Requirement: Single-row athlete physiology configuration

The system SHALL maintain exactly one `athlete_config` row representing the active user's physiology configuration — FTP, threshold heart rate and paces, max HR, lactate-threshold HR, and HR-zone (and optional power-zone) boundaries. The row is a singleton (fixed sentinel primary key, upsert-in-place), created lazily on first write, mirroring the `nutrition_goals` singleton shape. Every field is nullable: a NULL means "not configured / not provided by Garmin", distinct from a real zero.

#### Scenario: Config is absent until first write

- **WHEN** the client calls `GET /athlete-config` before any config has been set
- **THEN** the system returns `200 OK` with `{"athlete_config": null}`

#### Scenario: First PUT creates the config row

- **WHEN** the client calls `PUT /athlete-config` with a body containing any config fields
- **THEN** the system creates the single `athlete_config` row
- **AND** returns `200 OK` with the stored config object

#### Scenario: Subsequent PUT overwrites the config row

- **WHEN** the client calls `PUT /athlete-config` and a row already exists
- **THEN** the system replaces all config fields with the values from the request body
- **AND** absent fields are stored as null (cleared), matching `PUT /goals` full-replace semantics
- **AND** returns `200 OK` with the stored config object

### Requirement: Config carries threshold, max-HR, and zone-boundary fields

The system SHALL accept the following optional fields on `PUT /athlete-config`, persist them on the singleton row, and return the populated ones (nulls omitted) on `GET /athlete-config`. All fields are nullable and independent — any subset MAY be supplied.

Supported fields:

- `ftp_watts` (integer, cycling functional threshold power; must be `> 0` when present)
- `threshold_hr` (integer, functional threshold heart rate in bpm; `> 0`)
- `lactate_threshold_hr` (integer, Garmin lactate-threshold HR in bpm; `> 0`) — kept distinct from `threshold_hr` because Garmin exposes both
- `max_hr` (integer, bpm; `> 0`)
- `threshold_pace_sec_per_km` (number, run threshold pace in seconds per kilometre; `> 0`)
- `threshold_swim_pace_sec_per_100m` (number, swim threshold pace in seconds per 100 m; `> 0`)
- `hr_zone_1_max` … `hr_zone_5_max` (five integers, the upper HR bound of each zone in bpm; each `> 0` when present)
- `power_zone_1_max` … `power_zone_5_max` (five integers, the upper power bound of each zone in watts; optional; each `> 0` when present)

Zone boundaries are stored as each zone's *maximum* (upper bound); zone 1's lower bound is resting/0 and each subsequent zone's lower bound is the previous zone's max, so five maxima fully describe the boundaries.

#### Scenario: Partial config is accepted

- **WHEN** the client calls `PUT /athlete-config` with only `{"ftp_watts": 265, "max_hr": 188}`
- **THEN** the system stores those two fields
- **AND** stores all other config columns as null
- **AND** the response includes only the populated fields (nulls omitted)

#### Scenario: HR-zone boundaries are stored and returned

- **WHEN** the client puts `{"hr_zone_1_max": 120, "hr_zone_2_max": 140, "hr_zone_3_max": 155, "hr_zone_4_max": 168, "hr_zone_5_max": 182}`
- **THEN** the system stores all five HR-zone maxima
- **AND** `GET /athlete-config` returns them on the config object

#### Scenario: Power zones are optional and omitted when absent

- **WHEN** the stored config has HR-zone boundaries set but no power-zone boundaries
- **AND** the client calls `GET /athlete-config`
- **THEN** the response includes the `hr_zone_*` fields
- **AND** the response omits every `power_zone_*` key (nulls omitted)

#### Scenario: Negative or non-numeric values are rejected

- **WHEN** the client supplies any field whose value is negative, NaN, or non-numeric (for example `{"ftp_watts": -10}`)
- **THEN** the system returns `400 Bad Request` with `{"error":"athlete_config_value_invalid","field":"<which>"}`
- **AND** the request is not partially applied

### Requirement: PUT /athlete-config rejects an idempotency key

The system SHALL reject `PUT /athlete-config` when the request carries an `Idempotency-Key` header, consistent with the PUT full-replace rule established by harden-write-paths (a replayed PUT would misrepresent intermediate state).

#### Scenario: Idempotency-Key on PUT is rejected

- **WHEN** the client supplies `Idempotency-Key` on `PUT /athlete-config`
- **THEN** the system returns `400 Bad Request` with `{"error":"idempotency_unsupported_for_put"}`
- **AND** no config row is created or modified

### Requirement: Config is the capture-only source of physiology; it consumes nothing in this change

The system SHALL treat `athlete-config` as a capture-only mirror in this change: it stores FTP, thresholds, and zone boundaries and exposes them for reading, but does NOT derive `intensity_factor` from `ftp_watts`, does NOT relate the workouts capability's stored `secs_in_zone_*` to these zone boundaries, and does NOT feed any value into the race-fueling/raceprep intensity or carb-load math. Those consumptions are explicit follow-ups outside this change.

#### Scenario: Storing FTP does not back-fill workout intensity_factor

- **WHEN** `athlete_config.ftp_watts` is set and a workout with `normalized_power_w` set but `intensity_factor` NULL exists
- **AND** the client calls `GET /workouts/{id}`
- **THEN** the workout's `intensity_factor` remains NULL (unchanged by this change)
- **AND** no computation of `normalized_power_w / ftp_watts` occurs

#### Scenario: Config is not merged into summary totals

- **WHEN** any config field is set
- **AND** the client calls `GET /summary/daily`
- **THEN** no `athlete_config` field appears in the summary `totals` (unit isolation preserved)

### Requirement: Config float values are rounded at the response boundary

The system SHALL round every numeric config value to one decimal place in HTTP responses, applying `numfmt.Round1` only at the response-building boundary; storage stays at full precision.

#### Scenario: Threshold pace rounds on read

- **WHEN** the stored `threshold_pace_sec_per_km` is `258.04999`
- **THEN** `GET /athlete-config` returns `"threshold_pace_sec_per_km": 258.0`
- **AND** the stored column is unchanged
