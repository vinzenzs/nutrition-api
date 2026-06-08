## ADDED Requirements

### Requirement: Carb-load planning endpoint

The system SHALL expose `GET /race-prep/carb-load` returning a deterministic, stateless carb-loading schedule for a single race. The endpoint takes a race date, a body weight, and optional protocol parameters; it returns one schedule entry per day in `[race_date - days_before, race_date]` containing the target carbohydrate grams for that day. The endpoint performs no persistence and reads no user state — given the same inputs it always returns the same output.

#### Scenario: Default parameters produce a 4-entry schedule

- **WHEN** the client calls `GET /race-prep/carb-load?race_date=2026-07-24&body_weight_kg=70`
- **THEN** the response is `200 OK` with body shape `{race_date, body_weight_kg, params, schedule}`
- **AND** `schedule` contains exactly 4 entries: 3 carb-load days (`2026-07-21`, `2026-07-22`, `2026-07-23`) and race day (`2026-07-24`)
- **AND** entries are ordered by date ascending
- **AND** `params` echoes the effective inputs `{days_before: 3, carbs_per_kg_per_day: 10, race_day_carbs_per_kg: 2}`

#### Scenario: Each entry has the documented shape

- **WHEN** the client inspects any schedule entry
- **THEN** the entry has fields `{date, days_before, target_carbs_g, rationale}`
- **AND** `date` is a `YYYY-MM-DD` string
- **AND** `days_before` is an integer counting back from race day (`0` for race day, `3` for three days before)
- **AND** `target_carbs_g` is a positive number, rounded to 1 decimal place
- **AND** `rationale` is a human-readable label (e.g. `"carb-load day 1"`, `"race morning, pre-race meal ~3-4h before start"`)

#### Scenario: Load-day target is body_weight × carbs_per_kg_per_day

- **WHEN** the client calls with `body_weight_kg=70` and the defaults (`carbs_per_kg_per_day=10`)
- **THEN** every load-day entry's `target_carbs_g` is `700.0` (70 × 10, rounded to 1dp)

#### Scenario: Race-day target is body_weight × race_day_carbs_per_kg

- **WHEN** the client calls with `body_weight_kg=70` and the defaults (`race_day_carbs_per_kg=2`)
- **THEN** the entry with `days_before: 0` has `target_carbs_g: 140.0` (70 × 2)

#### Scenario: Custom parameters override defaults

- **WHEN** the client calls `GET /race-prep/carb-load?race_date=2026-07-24&body_weight_kg=80&days_before=4&carbs_per_kg_per_day=12&race_day_carbs_per_kg=3`
- **THEN** the schedule has 5 entries (4 load days + race day)
- **AND** every load-day `target_carbs_g` is `960.0` (80 × 12)
- **AND** the race-day `target_carbs_g` is `240.0` (80 × 3)
- **AND** `params` echoes `{days_before: 4, carbs_per_kg_per_day: 12, race_day_carbs_per_kg: 3}`

#### Scenario: days_before=0 returns race day only

- **WHEN** the client calls with `days_before=0`
- **THEN** the schedule contains exactly 1 entry — the race-day entry

#### Scenario: days_before=7 is accepted (upper bound inclusive)

- **WHEN** the client calls with `days_before=7`
- **THEN** the schedule contains exactly 8 entries (7 load days + race day)

#### Scenario: race_day_carbs_per_kg=0 produces a race-day entry with target 0

- **WHEN** the client calls with `race_day_carbs_per_kg=0`
- **THEN** the race-day entry exists in the schedule
- **AND** its `target_carbs_g` is `0.0`
- **AND** its `rationale` reflects "race morning"

#### Scenario: Missing race_date is rejected

- **WHEN** the client omits `race_date`
- **THEN** the system returns `400 Bad Request` with `{"error":"race_date_required"}`

#### Scenario: Missing body_weight_kg is rejected

- **WHEN** the client omits `body_weight_kg`
- **THEN** the system returns `400 Bad Request` with `{"error":"body_weight_kg_required"}`

#### Scenario: race_date in the past is rejected

- **WHEN** the request supplies a `race_date` strictly before today (in the configured user timezone)
- **THEN** the system returns `400 Bad Request` with `{"error":"race_date_in_past"}`

#### Scenario: race_date today is accepted

- **WHEN** the request supplies a `race_date` equal to today (in the configured user timezone)
- **THEN** the response is `200 OK`
- **AND** the schedule's race-day entry uses today's date

#### Scenario: Malformed race_date is rejected

- **WHEN** `race_date` is not in `YYYY-MM-DD` format
- **THEN** the system returns `400 Bad Request` with `{"error":"race_date_invalid"}`

#### Scenario: body_weight_kg out of range is rejected

- **WHEN** `body_weight_kg` is outside `[30, 200]` (e.g. `25` or `250`)
- **THEN** the system returns `400 Bad Request` with `{"error":"body_weight_kg_invalid","range":{"min":30,"max":200}}`

#### Scenario: days_before out of range is rejected

- **WHEN** `days_before` is outside `[0, 7]` (e.g. `-1` or `8`)
- **THEN** the system returns `400 Bad Request` with `{"error":"days_before_invalid","range":{"min":0,"max":7}}`

#### Scenario: carbs_per_kg_per_day out of range is rejected

- **WHEN** `carbs_per_kg_per_day` is outside `[1, 20]` (e.g. `0.5` or `25`)
- **THEN** the system returns `400 Bad Request` with `{"error":"carbs_per_kg_per_day_invalid","range":{"min":1,"max":20}}`

#### Scenario: race_day_carbs_per_kg out of range is rejected

- **WHEN** `race_day_carbs_per_kg` is outside `[0, 10]`
- **THEN** the system returns `400 Bad Request` with `{"error":"race_day_carbs_per_kg_invalid","range":{"min":0,"max":10}}`

#### Scenario: Non-numeric numeric params are rejected

- **WHEN** any numeric param (e.g. `body_weight_kg=heavy`) cannot be parsed as a number
- **THEN** the system returns `400 Bad Request` with `{"error":"<param>_invalid"}` for the offending param

#### Scenario: Endpoint requires authentication

- **WHEN** the request omits the `Authorization: Bearer <token>` header
- **THEN** the system returns `401 Unauthorized` (same auth posture as every other API endpoint)

#### Scenario: Endpoint is stateless and idempotent

- **WHEN** the client makes two identical `GET /race-prep/carb-load` requests
- **THEN** both responses are byte-for-byte identical
- **AND** no row is inserted into any database table
- **AND** the endpoint does NOT require an `Idempotency-Key` header (read-only)
