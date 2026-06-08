## ADDED Requirements

### Requirement: Single-user nutrition goals row

The system SHALL maintain exactly one `nutrition_goals` row representing the active user's targets. The row holds per-day targets for macros and micros and is created lazily on first write.

#### Scenario: Goals are absent until first write

- **WHEN** the client calls `GET /goals` before any goals have been set
- **THEN** the system returns `200 OK` with `{"goals": null}`

#### Scenario: First PUT creates the goals row

- **WHEN** the client calls `PUT /goals` with a body containing any target fields
- **THEN** the system creates the goals row
- **AND** returns `200 OK` with the stored goals object

#### Scenario: Subsequent PUT overwrites the goals row

- **WHEN** the client calls `PUT /goals` and a row already exists
- **THEN** the system replaces all goal fields with the values from the request body
- **AND** absent fields are stored as null (cleared)
- **AND** returns `200 OK` with the stored goals object

### Requirement: Goals support macro and micro targets with per-field ranges

The system SHALL accept the following optional fields on `PUT /goals`. Each target is `{min?: number, max?: number}` except `kcal_target` which is a single number. Fields not supplied are stored as null. Any null target means "no goal for this nutrient" and is omitted from adherence output.

Supported fields:
- `kcal_target` (number)
- `protein_g` (`{min, max}`)
- `carbs_g` (`{min, max}`)
- `fat_g` (`{min, max}`)
- `fiber_g` (`{min}`)
- `sugar_g` (`{max}`)
- `salt_g` (`{max}`)
- `iron_mg` (`{min}`)
- `calcium_mg` (`{min}`)
- `vitamin_d_mcg` (`{min}`)
- `vitamin_b12_mcg` (`{min}`)
- `vitamin_c_mg` (`{min}`)
- `magnesium_mg` (`{min}`)
- `potassium_mg` (`{min}`)
- `zinc_mg` (`{min}`)

#### Scenario: Partial goals are accepted

- **WHEN** the client calls `PUT /goals` with only `{"kcal_target": 2200, "protein_g": {"min": 150, "max": 190}}`
- **THEN** the system stores those two targets
- **AND** stores all other target columns as null
- **AND** the response includes only the populated targets (nulls omitted)

#### Scenario: Negative or non-numeric targets are rejected

- **WHEN** the client supplies any target value that is negative, NaN, or non-numeric
- **THEN** the system returns `400 Bad Request` with `{"error":"goal_value_invalid","field":"<which>"}`

#### Scenario: Inverted min/max is rejected

- **WHEN** the client supplies a range target with `min > max`
- **THEN** the system returns `400 Bad Request` with `{"error":"goal_range_invalid","field":"<which>"}`

### Requirement: Daily and range summaries compute adherence against goals

The summary capability SHALL include an `adherence` object in `GET /summary/daily` and in each per-day entry of `GET /summary/range`. For each goal-targeted nutrient that has a non-null total for the day, adherence reports `{actual, target, delta_pct, status}` where `status` is `under`, `on`, or `over` per the rules below. Nutrients without a goal are omitted from `adherence`.

#### Scenario: kcal_target adherence uses ±5% as on-target

- **WHEN** kcal_target is 2200 and daily kcal total is 2150
- **THEN** `adherence.kcal = {actual: 2150, target: 2200, delta_pct: -2.3, status: "on"}`

#### Scenario: kcal more than 5% under target is "under"

- **WHEN** kcal_target is 2200 and daily kcal total is 1900
- **THEN** `adherence.kcal.status = "under"`

#### Scenario: kcal more than 5% over target is "over"

- **WHEN** kcal_target is 2200 and daily kcal total is 2400
- **THEN** `adherence.kcal.status = "over"`

#### Scenario: Range min/max nutrients status from min/max boundaries

- **WHEN** protein_g goal is `{min: 150, max: 190}` and the daily total is 160
- **THEN** `adherence.protein_g.status = "on"`
- **AND** when the total is 140 the status is `"under"`
- **AND** when the total is 200 the status is `"over"`

#### Scenario: Min-only nutrients never produce "over"

- **WHEN** the goal is min-only (e.g. `fiber_g.min = 30`) and the daily total is 60
- **THEN** `adherence.fiber_g.status = "on"` (more than the min is fine, never "over")

#### Scenario: Max-only nutrients never produce "under"

- **WHEN** the goal is max-only (e.g. `sugar_g.max = 50`) and the daily total is 10
- **THEN** `adherence.sugar_g.status = "on"` (less than the max is fine, never "under")

#### Scenario: No goal set means no adherence entry

- **WHEN** kcal_target is null
- **THEN** the `adherence` object has no `kcal` key
- **AND** the daily totals still include kcal as a raw number

#### Scenario: Goal set but no contributing entries omits the nutrient

- **WHEN** a goal exists for `iron_mg` but no logged meal on the day has a non-null iron value
- **THEN** `adherence.iron_mg` is omitted (no fake-zero adherence)
- **AND** `totals.iron_mg` is also omitted from totals
