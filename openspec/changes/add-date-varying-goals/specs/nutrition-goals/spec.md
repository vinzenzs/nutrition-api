## ADDED Requirements

### Requirement: Daily goal overrides

The system SHALL persist per-date overrides of the default goals via a `daily_goal_overrides` table keyed on date, and expose CRUD endpoints for them. Override goal sets use the same `{min?, max?}` Range shape as the default singleton, and the same validation rules (legacy `kcal_target` rejected, empty `{}` rejected, inverted min/max rejected, negative/NaN rejected).

#### Scenario: PUT /goals/overrides/{date} creates or replaces an override

- **WHEN** the client puts `{"kcal":{"min":2280,"max":2520},"protein_g":{"min":160,"max":200}}` to `PUT /goals/overrides/2026-06-15`
- **THEN** the system stores those two targets on the `2026-06-15` row, clearing every other nutrient bound (full-replace semantics, matching `PUT /goals`)
- **AND** returns `200 OK` with the stored goals object

#### Scenario: Subsequent PUT overwrites the override row

- **WHEN** the client PUTs again on the same date with a different body
- **THEN** the row is replaced wholesale; absent fields are cleared

#### Scenario: Idempotency-Key on PUT is rejected

- **WHEN** the client supplies `Idempotency-Key` on `PUT /goals/overrides/{date}`
- **THEN** the system returns `400 idempotency_unsupported_for_put` (consistent with the auth capability's PUT rule established by harden-write-paths)

#### Scenario: Legacy kcal_target on the override body is rejected

- **WHEN** the client puts a body containing `kcal_target` (the pre-unify-adherence-shape field)
- **THEN** the system returns `400 goal_value_invalid` with `field: kcal_target`

#### Scenario: Invalid date format returns 400

- **WHEN** the date path segment does not match `YYYY-MM-DD`
- **THEN** the system returns `400 Bad Request` with `{"error":"date_invalid"}`

#### Scenario: GET /goals/overrides/{date} returns the override

- **WHEN** an override exists for `2026-06-15` and the client calls `GET /goals/overrides/2026-06-15`
- **THEN** the response is `200 OK` with `{"goals": <override>}` matching the Goals shape (rounded per the existing rounding requirement)

#### Scenario: GET on a date with no override returns 404

- **WHEN** no override exists for the requested date
- **THEN** the system returns `404 Not Found` with `{"error":"override_not_found"}`
- **AND** the response does NOT fall back to the default goals — the caller asked specifically for the override

#### Scenario: DELETE /goals/overrides/{date} removes the override

- **WHEN** the client deletes an existing override
- **THEN** the system returns `204 No Content`
- **AND** subsequent GETs for that date return `404 override_not_found`
- **AND** subsequent summary queries for that date use the default goals (override is gone)

#### Scenario: DELETE of unknown date returns 404

- **WHEN** the client deletes an override that doesn't exist
- **THEN** the system returns `404 Not Found` with `{"error":"override_not_found"}`

#### Scenario: GET /goals/overrides?from=&to= lists overrides in a range

- **WHEN** the client calls `GET /goals/overrides?from=2026-06-01&to=2026-06-30`
- **THEN** the response is `200 OK` with body `{"overrides": [{"date": "...", "goals": {...}}, ...]}` containing one entry per date that has an override (dates without an override are omitted, not echoed with the default)
- **AND** entries are ordered by date ascending

#### Scenario: Range query without dates is rejected

- **WHEN** the client omits `from` or `to`
- **THEN** the system returns `400 Bad Request` with `{"error":"range_required"}`

#### Scenario: Range query larger than 366 days is rejected

- **WHEN** the range spans more than 366 days inclusive
- **THEN** the system returns `400 Bad Request` with `{"error":"range_too_large","max_days":366}`

### Requirement: Effective goals resolve override-first, default-fallback

The system SHALL expose `EffectiveFor(date)` semantics inside the summary capability: when computing adherence for a given calendar date, the system first consults `daily_goal_overrides`; if a row exists for that date the override is used; otherwise the default singleton from `nutrition_goals` is used; if neither exists, no adherence is computed.

#### Scenario: Override drives adherence when present

- **WHEN** the user has both a default goal of `kcal: {min:2090, max:2310}` and an override on `2026-06-15` of `kcal: {min:2280, max:2520}`
- **AND** the client calls `GET /summary/daily?date=2026-06-15`
- **THEN** the adherence row for `kcal` uses the override bounds `{min:2280, max:2520}`
- **AND** the response includes `"goal_source": "override"`

#### Scenario: Default drives adherence when no override

- **WHEN** no override exists for `2026-06-14` and the default singleton is set
- **AND** the client calls `GET /summary/daily?date=2026-06-14`
- **THEN** the adherence row uses the default bounds
- **AND** the response includes `"goal_source": "default"`

#### Scenario: Neither override nor default means no adherence

- **WHEN** neither an override for the date nor the default singleton is set
- **AND** the client calls `GET /summary/daily?date=…`
- **THEN** the response does NOT include an `adherence` object
- **AND** the response includes `"goal_source": "none"`

#### Scenario: Range summary switches goal source day-by-day

- **WHEN** the client calls `GET /summary/range?from=2026-06-14&to=2026-06-16` with an override only on `2026-06-15`
- **THEN** the per-day `goal_source` values are `"default"`, `"override"`, `"default"` respectively
- **AND** each day's adherence rows reflect that day's effective goals

#### Scenario: Range summary fetches overrides in a single round-trip

- **WHEN** the client calls `GET /summary/range?from=…&to=…` spanning N days
- **THEN** the system issues at most one query to fetch every override in the window (not one per day)

#### Scenario: goal_source field is omitted when adherence is suppressed

- **WHEN** a daily summary request supplies `meal_type` (which suppresses adherence)
- **THEN** the response omits the `goal_source` field
- **AND** the response does NOT include an `adherence` object (existing behaviour preserved)

## MODIFIED Requirements

### Requirement: Daily and range summaries compute adherence against goals

The summary capability SHALL include an `adherence` object in `GET /summary/daily` and in each per-day entry of `GET /summary/range`, computed against the **effective goals for that calendar date** (override if present, else default singleton). For EVERY goal-targeted nutrient (i.e. every field where the effective goal is non-null), `adherence` includes exactly one entry with the shape `{actual: number|null, target: {min?, max?}, delta_pct?: number, status: "under" | "on" | "over" | "no_data"}`. The same code path produces adherence in both endpoints — daily and range MUST report the same shape and the same status for the same `(effective goals, daily totals)` pair. Each day's response also exposes a `goal_source` string identifying which set produced the adherence rows: `"override" | "default" | "none"`.

#### Scenario: Adherence row exists for every configured goal

- **WHEN** the effective goals row has 15 fields set and the day has logged meals
- **THEN** the daily summary's `adherence` object has 15 entries
- **AND** every entry has a non-empty `target` matching the effective goal's `{min?, max?}`

#### Scenario: Adherence row exists for every configured goal on empty days

- **WHEN** the effective goals row has 15 fields set and the day has zero logged meals
- **THEN** the daily summary's `adherence` object STILL has 15 entries
- **AND** every entry has `actual: null` and `status: "no_data"`

#### Scenario: Daily and range agree on shape

- **WHEN** the same effective goals and the same day's data are queried via `GET /summary/daily?date=D` and via `GET /summary/range?from=D&to=D` (single-day range)
- **THEN** the adherence object in the daily response is structurally identical (same keys, same values, same statuses) to the adherence object on the matching day in the range response
- **AND** `goal_source` is the same on both responses

#### Scenario: kcal Range status uses the user-set bounds

- **WHEN** kcal goal is `{"min": 2090, "max": 2310}` and daily kcal total is 2150
- **THEN** `adherence.kcal = {actual: 2150, target: {"min": 2090, "max": 2310}, delta_pct: -2.3, status: "on"}`

- **WHEN** the daily total is 1900
- **THEN** `adherence.kcal.status = "under"`

- **WHEN** the daily total is 2400
- **THEN** `adherence.kcal.status = "over"`

#### Scenario: Range min/max nutrients status from boundaries

- **WHEN** `protein_g` goal is `{"min": 150, "max": 190}` and the daily total is 160
- **THEN** `adherence.protein_g.status = "on"`
- **AND** at 140 the status is `"under"`
- **AND** at 200 the status is `"over"`

#### Scenario: Min-only nutrients never produce "over"

- **WHEN** the goal is `{"min": 30}` (no max) and the daily total is 60
- **THEN** `adherence.fiber_g.status = "on"`

#### Scenario: Max-only nutrients never produce "under"

- **WHEN** the goal is `{"max": 50}` (no min) and the daily total is 10
- **THEN** `adherence.sugar_g.status = "on"`

#### Scenario: Macro total of zero on a day with meals is not no_data

- **WHEN** the day has logged meals but none contributed to `protein_g`, so the total is 0 (macros are always numeric in totals)
- **THEN** the `adherence.protein_g.actual` is `0`
- **AND** the status reflects `under` (assuming `min > 0`)
- **AND** the status is NOT `no_data` (the day had logged meals; this is real progress info)

#### Scenario: Micro total null produces no_data

- **WHEN** a goal exists for `iron_mg` and no contributing meal on the day had a non-null `iron_mg` value
- **THEN** `adherence.iron_mg = {actual: null, target: <the goal>, status: "no_data"}`
- **AND** `delta_pct` is absent
- **AND** `totals.iron_mg` is also absent from totals (existing behaviour preserved)

#### Scenario: No goal set means no adherence entry

- **WHEN** `kcal` goal is null in the effective goals (the user has not set one on either the override or the default)
- **THEN** the `adherence` object has no `kcal` key
- **AND** the daily totals still include kcal as a raw number
