# race-prep Specification

## Purpose

Deterministic computation primitives for race-week nutrition planning. Starts with carb-load; can grow as other "agent should not hallucinate this" primitives surface (e.g. recovery-window macros, fuelling-rate during long efforts).

## Requirements

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

### Requirement: Carb-load apply endpoint persists schedule into goal overrides

The system SHALL expose `POST /race-prep/carb-load/apply` taking the same parameters as `GET /race-prep/carb-load` (`race_date`, `body_weight_kg`, optional `days_before`, `carbs_per_kg_per_day`, `race_day_carbs_per_kg`). The endpoint computes the carb-load schedule via the same primitive as the GET endpoint, then writes the per-day carbohydrate target into the corresponding per-date goal override row. All writes happen inside a single database transaction — if any per-date write fails, the whole apply rolls back and zero overrides are persisted. The endpoint returns the schedule alongside a per-date `applied` array reporting whether each target row was newly created or merged into an existing override.

The apply step writes ONLY the `carbs_g` bound (as `{min: target_carbs_g}`, min-only — matching the existing pattern for `fiber_g` / `iron_mg`). Non-carbohydrate fields on a pre-existing override row (`kcal`, `protein_g`, etc.) are preserved verbatim; the apply step never clears or overwrites them.

#### Scenario: Apply on empty overrides creates one row per schedule day

- **WHEN** the client calls `POST /race-prep/carb-load/apply` with `{"race_date":"2026-07-24","body_weight_kg":70}` and no pre-existing override rows on the target dates
- **THEN** the response is `200 OK` with body shape `{race_date, body_weight_kg, params, schedule, applied}`
- **AND** `schedule` matches what `GET /race-prep/carb-load` would return for the same inputs (4 entries, default protocol parameters)
- **AND** `applied` contains exactly 4 entries — one per schedule day
- **AND** each `applied` entry has `{date, carbs_g_min, created: true}`
- **AND** four new rows exist in `daily_goal_overrides`, each with only the `carbs_g_min` bound populated (every other column null)
- **AND** `applied[0].carbs_g_min == 700.0` for a 70kg athlete on the default 10 g/kg load-day protocol
- **AND** the race-day entry has `carbs_g_min == 140.0` (70 × 2)

#### Scenario: Apply merges into existing override, preserving non-carb fields

- **WHEN** an override already exists on `2026-07-22` with `{kcal: {min: 2090, max: 2310}, protein_g: {min: 150, max: 190}}` and no `carbs_g` bound
- **AND** the client calls `POST /race-prep/carb-load/apply` with `{"race_date":"2026-07-24","body_weight_kg":70}`
- **THEN** the `2026-07-22` override row now has `carbs_g: {min: 700}` AND the existing `kcal` and `protein_g` bounds unchanged
- **AND** the response's `applied` entry for `2026-07-22` has `{date: "2026-07-22", carbs_g_min: 700.0, created: false}`
- **AND** the `applied` entries for dates without prior overrides have `created: true`

#### Scenario: Apply replaces an existing carbs_g bound

- **WHEN** an override on `2026-07-22` already has `{carbs_g: {min: 500, max: 600}, kcal: {min: 2200}}`
- **AND** the client calls `POST /race-prep/carb-load/apply` with `{"race_date":"2026-07-24","body_weight_kg":70}`
- **THEN** the row's `carbs_g` is replaced with `{min: 700}` (the new target; no max — the apply step writes min-only)
- **AND** `kcal` is preserved verbatim
- **AND** the response's `applied` entry has `created: false`

#### Scenario: Apply is atomic — partial failure rolls back

- **WHEN** the schedule has 4 target dates and the third per-date write fails (e.g. a constraint violation forced for tests)
- **THEN** the transaction rolls back
- **AND** zero rows have been written or modified in `daily_goal_overrides`
- **AND** the response is `500 Internal Server Error` (or the propagated error code from the failing write)
- **AND** the response does NOT include an `applied` array (or includes an empty array; either is acceptable as long as no partial state is implied)

#### Scenario: Apply uses POST semantics, accepts Idempotency-Key

- **WHEN** the client calls `POST /race-prep/carb-load/apply` with an `Idempotency-Key` header
- **THEN** the existing idempotency middleware applies (deduping replays of the same key+body, consistent with every other POST write)
- **AND** the endpoint is NOT exempted from idempotency

#### Scenario: Apply rejects same param errors as GET

- **WHEN** the client calls apply with a `race_date` in the past
- **THEN** the system returns `400 Bad Request` with `{"error":"race_date_in_past"}` (same validation as the GET endpoint)
- **AND** no transaction is opened; no overrides are touched

- **WHEN** the client calls apply with `body_weight_kg=25`
- **THEN** the system returns `400 Bad Request` with `{"error":"body_weight_kg_invalid","range":{"min":30,"max":200}}`

- **WHEN** the client calls apply with `days_before=8`
- **THEN** the system returns `400 Bad Request` with `{"error":"days_before_invalid","range":{"min":0,"max":7}}`

#### Scenario: Apply requires authentication

- **WHEN** the request omits the `Authorization: Bearer <token>` header
- **THEN** the system returns `401 Unauthorized` (same auth posture as every other API endpoint)
- **AND** no overrides are touched

#### Scenario: Apply round-trip is visible to /summary/daily

- **WHEN** the client calls `POST /race-prep/carb-load/apply` for `race_date=2026-07-24` (3 load days)
- **AND** the client then calls `GET /summary/daily?date=2026-07-22`
- **THEN** the response includes an `adherence.carbs_g` entry with `target: {min: 700}`
- **AND** `goal_source: "override"` (the apply produced an override on that date)

#### Scenario: Apply response order matches schedule order

- **WHEN** the client calls apply with default params (4 entries)
- **THEN** `applied[i].date == schedule[i].date` for every index
- **AND** both arrays are ordered by date ascending
