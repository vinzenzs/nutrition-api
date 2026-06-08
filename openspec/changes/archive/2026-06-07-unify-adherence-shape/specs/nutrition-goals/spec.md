## MODIFIED Requirements

### Requirement: Goals support macro and micro targets with per-field ranges

The system SHALL accept the following optional fields on `PUT /goals`. Each target has the unified shape `{min?: number, max?: number}` — both bounds are optional but at least one MUST be present when the field is supplied. Fields not supplied at all are stored as null (cleared). Any null target means "no goal for this nutrient" and is reflected in adherence via the `no_data` status.

Supported fields:

- `kcal` (`{min?, max?}`)
- `protein_g` (`{min?, max?}`)
- `carbs_g` (`{min?, max?}`)
- `fat_g` (`{min?, max?}`)
- `fiber_g` (`{min?, max?}`)
- `sugar_g` (`{min?, max?}`)
- `salt_g` (`{min?, max?}`)
- `iron_mg` (`{min?, max?}`)
- `calcium_mg` (`{min?, max?}`)
- `vitamin_d_mcg` (`{min?, max?}`)
- `vitamin_b12_mcg` (`{min?, max?}`)
- `vitamin_c_mg` (`{min?, max?}`)
- `magnesium_mg` (`{min?, max?}`)
- `potassium_mg` (`{min?, max?}`)
- `zinc_mg` (`{min?, max?}`)

The legacy `kcal_target` field is no longer accepted; requests carrying it are rejected with `400 goal_value_invalid`.

#### Scenario: Partial goals are accepted

- **WHEN** the client calls `PUT /goals` with only `{"kcal": {"min": 2090, "max": 2310}, "protein_g": {"min": 150, "max": 190}}`
- **THEN** the system stores those two targets
- **AND** stores all other target columns as null
- **AND** the response includes only the populated targets (nulls omitted)

#### Scenario: Single-bound goals are accepted

- **WHEN** the client supplies `{"fiber_g": {"min": 30}}`
- **THEN** the system stores `fiber_g_min = 30, fiber_g_max = null`
- **AND** subsequent reads return `"fiber_g": {"min": 30}` (the max key is omitted)

- **WHEN** the client supplies `{"sugar_g": {"max": 50}}`
- **THEN** the system stores `sugar_g_min = null, sugar_g_max = 50`
- **AND** subsequent reads return `"sugar_g": {"max": 50}`

#### Scenario: Empty range object is rejected

- **WHEN** the client supplies a field with `{}` (neither `min` nor `max`)
- **THEN** the system returns `400 Bad Request` with `{"error":"goal_value_invalid","field":"<which>"}`

#### Scenario: Negative or non-numeric targets are rejected

- **WHEN** the client supplies any target value that is negative, NaN, or non-numeric
- **THEN** the system returns `400 Bad Request` with `{"error":"goal_value_invalid","field":"<which>"}`

#### Scenario: Inverted min/max is rejected

- **WHEN** the client supplies a range target with `min > max` (both present)
- **THEN** the system returns `400 Bad Request` with `{"error":"goal_range_invalid","field":"<which>"}`

#### Scenario: Legacy kcal_target field is rejected

- **WHEN** the client supplies `{"kcal_target": 2200}` (the pre-change shape)
- **THEN** the system returns `400 Bad Request` with `{"error":"goal_value_invalid","field":"kcal_target"}`
- **AND** the request is not partially applied

### Requirement: Daily and range summaries compute adherence against goals

The summary capability SHALL include an `adherence` object in `GET /summary/daily` and in each per-day entry of `GET /summary/range`. For EVERY goal-targeted nutrient (i.e. every field where the goal is non-null), `adherence` includes exactly one entry with the shape `{actual: number|null, target: {min?, max?}, delta_pct?: number, status: "under" | "on" | "over" | "no_data"}`. The same code path produces adherence in both endpoints — daily and range MUST report the same shape and the same status for the same `(goals, daily totals)` pair.

#### Scenario: Adherence row exists for every configured goal

- **WHEN** the goals row has 15 fields set and the day has logged meals
- **THEN** the daily summary's `adherence` object has 15 entries
- **AND** every entry has a non-empty `target` matching the goal's `{min?, max?}`

#### Scenario: Adherence row exists for every configured goal on empty days

- **WHEN** the goals row has 15 fields set and the day has zero logged meals
- **THEN** the daily summary's `adherence` object STILL has 15 entries
- **AND** every entry has `actual: null` and `status: "no_data"`

#### Scenario: Daily and range agree on shape

- **WHEN** the same goals and the same day's data are queried via `GET /summary/daily?date=D` and via `GET /summary/range?from=D&to=D` (single-day range)
- **THEN** the adherence object in the daily response is structurally identical (same keys, same values, same statuses) to the adherence object on the matching day in the range response

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

- **WHEN** `kcal` goal is null (the user has not set one)
- **THEN** the `adherence` object has no `kcal` key
- **AND** the daily totals still include kcal as a raw number

### Requirement: Nutrient values in responses are rounded to one decimal place

The system SHALL round every nutrient value to one decimal place in HTTP responses. Storage and internal computation use full precision; the rounding is applied only at the response-building boundary.

#### Scenario: Recipe-derived totals round to one decimal

- **WHEN** a daily summary's `totals.protein_g` is computed as `70.44969999999999` from float arithmetic
- **THEN** the response body shows `"protein_g": 70.4`
- **AND** the underlying meal entries' stored values are unchanged

#### Scenario: Goals row rounds on read

- **WHEN** a `nutrition_goals` row carries `kcal_min = 2089.99999`
- **THEN** `GET /goals` returns `"kcal": {"min": 2090, ...}`
- **AND** the stored column is unchanged

#### Scenario: Adherence values round on read

- **WHEN** `actual = 70.4496999...` and `target.min = 70.4`
- **THEN** the response shows `actual: 70.4`
- **AND** the `delta_pct` field is also rounded to one decimal place

#### Scenario: Status uses unrounded comparison

- **WHEN** the unrounded `actual = 70.04` and `target.min = 70.05`
- **THEN** the status is `"under"` (the comparison happens before rounding)
- **AND** the response shows `"actual": 70.0` (rounded for presentation only)
