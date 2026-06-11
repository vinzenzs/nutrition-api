## ADDED Requirements

### Requirement: Persistent race with ordered legs

The system SHALL persist a race as `{name, race_date, race_type?, location?,
notes?}` owning an ordered list of legs, each
`{ordinal, discipline, distance_m?, expected_duration_min?, intensity?}`.
`discipline` MUST be one of `swim`, `bike`, `run`, `transition`, `other`.
Legs MUST be uniquely ordered within a race (`ordinal` unique per race) and MUST
be deleted when their race is deleted. The system SHALL expose create, read,
list, update, and delete over races.

#### Scenario: Create a race with legs

- **WHEN** the client POSTs a race with `name`, `race_date`, and a `legs` array
  of `{ordinal, discipline, expected_duration_min}`
- **THEN** the race and its legs are persisted and returned with a generated
  `id` and the legs ordered by `ordinal` ascending

#### Scenario: Deleting a race removes its legs

- **WHEN** a race with legs is deleted
- **THEN** the race and all its `race_legs` rows are removed
- **AND** a subsequent `GET /races/{id}` returns `404 race_not_found`

#### Scenario: Duplicate leg ordinal is rejected

- **WHEN** a race is created or updated with two legs sharing the same `ordinal`
- **THEN** the request is rejected with `400 leg_ordinal_duplicate`
- **AND** nothing is persisted

#### Scenario: Invalid discipline is rejected

- **WHEN** a leg is supplied with a `discipline` outside the allowed set
- **THEN** the request is rejected with `400 leg_discipline_invalid`

### Requirement: Per-leg fuelling plan computed on read

The system SHALL compute a per-leg in-event fuelling plan over a stored race at
read time from athlete parameters, and SHALL NOT persist the result.
`GET /races/{id}/fueling-plan` SHALL require `body_weight_kg` and accept an
optional `sweat_rate_ml_per_hr`. The response SHALL carry, per leg,
`carbs_g_per_hr`, `carbs_g_total`, `sodium_mg_per_hr`, `sodium_mg_total`,
`fluid_ml_per_hr`, `fluid_ml_total`, and a `rationale`, plus a race-level total
and `total_duration_min`.

#### Scenario: Carbs band by total race duration

- **WHEN** a fuelling plan is requested for a race whose summed leg duration is at
  least 150 minutes
- **THEN** the baseline carbohydrate target is `90 g/hr`
- **AND** for a summed duration in `[75, 150)` minutes the baseline is `60 g/hr`
- **AND** for a summed duration below 75 minutes the baseline is `0 g/hr` with a
  rationale noting fuel is not required

#### Scenario: Per-leg carbs scale by discipline intake capacity

- **WHEN** the carbohydrate baseline is `90 g/hr` for a race with a bike leg and a
  run leg
- **THEN** the bike leg's `carbs_g_per_hr` is `90` (factor 1.0)
- **AND** the run leg's `carbs_g_per_hr` is `63` (factor 0.7, rounded)

#### Scenario: Plan is not persisted

- **WHEN** a fuelling plan is requested twice with different `body_weight_kg`
  values
- **THEN** each response reflects its own parameters
- **AND** no plan is stored on the race between calls

### Requirement: Swim and transition legs carry zero intake

The fuelling plan SHALL assign zero carbohydrate, sodium, and fluid per hour and
in total to `swim` and `transition` legs, because intake is not feasible during
them.

#### Scenario: Swim leg gets zero fuelling

- **WHEN** a fuelling plan is computed for a race containing a `swim` leg
- **THEN** that leg's `carbs_g_per_hr`, `sodium_mg_per_hr`, and `fluid_ml_per_hr`
  are all `0`
- **AND** its `_total` fields are `0`

#### Scenario: Transition leg gets zero fuelling

- **WHEN** a fuelling plan is computed for a race containing a `transition` leg
- **THEN** that leg's per-hour and total carbs, sodium, and fluid are all `0`

### Requirement: Fuelling baseline is honest about defaulted inputs

When `sweat_rate_ml_per_hr` is supplied, fluid and sodium SHALL derive from it
(fluid capped at `1000 ml/hr`; sodium = `sweat_rate_ml_per_hr / 1000 × 800 mg/L`).
When it is omitted, the plan SHALL use a default fluid of `600 ml/hr` and sodium of
`600 mg/hr` and SHALL state in the leg `rationale` that a default sweat rate was
assumed.

#### Scenario: Sodium and fluid derive from a supplied sweat rate

- **WHEN** a plan is requested with `sweat_rate_ml_per_hr = 1000`
- **THEN** a non-swim, non-transition leg's `fluid_ml_per_hr` is `1000`
- **AND** its `sodium_mg_per_hr` is `800`

#### Scenario: Omitted sweat rate is flagged

- **WHEN** a plan is requested without `sweat_rate_ml_per_hr`
- **THEN** non-swim, non-transition legs use `600 ml/hr` fluid and `600 mg/hr`
  sodium
- **AND** each such leg's `rationale` states that a default sweat rate was assumed

### Requirement: Unit isolation across carbs, sodium, and fluid

The fuelling-plan response SHALL keep carbohydrate (`_g`), sodium (`_mg`), and
fluid (`_ml`) as distinct named fields and SHALL NOT merge them into a shared
totals structure.

#### Scenario: Distinct unit fields

- **WHEN** a fuelling plan is returned
- **THEN** carbohydrate values appear only under `*_g` fields, sodium only under
  `*_mg` fields, and fluid only under `*_ml` fields
- **AND** there is no combined nutrient/volume total field mixing the units

### Requirement: Fuelling-plan inputs are validated

The system SHALL validate fuelling-plan inputs and reject out-of-range values with
documented error codes. `body_weight_kg` MUST be present and within `30–200`;
`sweat_rate_ml_per_hr`, when present, MUST be within a sane positive range.

#### Scenario: Missing body weight is rejected

- **WHEN** `GET /races/{id}/fueling-plan` is called without `body_weight_kg`
- **THEN** the request is rejected with `400 body_weight_kg_required`

#### Scenario: Out-of-range body weight is rejected

- **WHEN** `body_weight_kg` is `15`
- **THEN** the request is rejected with `400 body_weight_kg_out_of_range`

#### Scenario: Fuelling plan for an unknown race

- **WHEN** a fuelling plan is requested for a race id that does not exist
- **THEN** the response is `404 race_not_found`
