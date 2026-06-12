# training-plan — delta for add-plan-slot-targets

## ADDED Requirements

### Requirement: Plan slots carry optional per-intent target overrides

The system SHALL allow a `plan_slot` to carry an optional `target_overrides` list,
each entry a `{intent, target}` pair, stored as a JSONB column. The `intent` SHALL
be one of the workout-template step intents (`warmup`, `active`, `interval`,
`recovery`, `rest`, `cooldown`) and the `target` SHALL use the workout-templates
Target shape and be validated by the same validator (pace bounds positive, zones
within `1..5`, `low <= high`). The list SHALL contain at most one entry per
intent; a null or empty list means no overrides. Slot create and patch SHALL
accept `target_overrides`, the nested plan `GET` SHALL return it, and a `PATCH`
that supplies the list SHALL replace it wholesale (supplying `[]` clears it,
omitting it leaves it unchanged).

#### Scenario: A slot stores a pace override for its work intervals

- **WHEN** a slot referencing an interval template is created with
  `target_overrides: [{intent:"interval", target:{kind:"pace", low_sec_per_km:435, high_sec_per_km:435}}]`
- **THEN** the slot persists the override and the nested plan `GET` returns it

#### Scenario: Duplicate intent in the override list is rejected

- **WHEN** a slot write supplies two override entries with the same `intent`
- **THEN** the response is a validation error and nothing is persisted

#### Scenario: An invalid override target is rejected

- **WHEN** an override supplies a `pace` target with `low_sec_per_km` greater than
  `high_sec_per_km`, or a zone outside `1..5`, or an unknown target kind
- **THEN** the response is a validation error (the workout-templates Target
  validator) and nothing is persisted

#### Scenario: Patch replaces the override list wholesale

- **WHEN** a client `PATCH`es a slot with a new `target_overrides` list
- **THEN** the prior list is replaced entirely
- **AND** supplying `[]` clears all overrides while omitting the field leaves them unchanged

### Requirement: A planned workout's effective program applies slot overrides to template steps

The system SHALL define a planned workout's **effective program** as its
template's steps with each step's `target` replaced when that step's `intent`
matches an entry in the workout's slot `target_overrides`; steps whose intent has
no matching override SHALL be unchanged, and overrides SHALL affect targets only,
never durations or step structure. This effective program SHALL be the single
representation that downstream consumers use — both display and the Garmin
compile path (`add-garmin-scheduling`) SHALL build from effective steps, not raw
template steps. The effective program SHALL be resolved on read from the
template and slot, not snapshotted onto the workout row.

#### Scenario: Override replaces only the matching intent's target

- **WHEN** a template is `[warmup @hr_zone 1, repeat ×5 of (interval @power_zone 4, recovery @hr_zone 1), cooldown @hr_zone 1]`
  and the slot overrides `interval` with `pace 7:15`
- **THEN** the effective program has the interval steps targeting `pace 7:15`
- **AND** the warmup, recovery, and cooldown targets are unchanged
- **AND** all durations and the repeat structure are unchanged

#### Scenario: No overrides yields the template program verbatim

- **WHEN** a planned workout's slot has no `target_overrides`
- **THEN** its effective program equals the template's steps unchanged

### Requirement: A read endpoint exposes a planned workout's effective program

The system SHALL expose `GET /workouts/{id}/program` returning the effective steps
of a planned workout (resolved from its `template_id` and its slot's
`target_overrides`) together with enough metadata to render it (sport, name). When
the workout has no `template_id`, the endpoint SHALL return its bare metadata with
no steps rather than an error. The endpoint SHALL require authentication.

#### Scenario: Program reflects the slot override

- **WHEN** a client `GET`s `/workouts/{id}/program` for a planned workout whose
  slot overrides the interval pace to `7:15`
- **THEN** the returned steps show the interval target as `pace 7:15`

#### Scenario: A template-less planned workout returns metadata without steps

- **WHEN** a planned workout has no `template_id`
- **THEN** `GET /workouts/{id}/program` returns its sport and name with an empty
  step list and no error

### Requirement: MCP exposes slot target overrides and the planned program

The MCP server's `add_plan_slot` and `patch_plan_slot` tools SHALL accept
`target_overrides`, and a new `get_workout_program` tool SHALL issue exactly one
`GET /workouts/{id}/program` and forward the response verbatim. The MCP
integration expected-tools list SHALL include `get_workout_program`.

#### Scenario: get_workout_program issues one GET

- **WHEN** the agent calls `get_workout_program` with a workout id
- **THEN** the MCP server issues exactly one `GET /workouts/{id}/program`
- **AND** the tool result is the REST response verbatim

#### Scenario: add_plan_slot carries target overrides

- **WHEN** the agent calls `add_plan_slot` with a `target_overrides` list
- **THEN** the slot is created with those overrides

#### Scenario: Expected-tools list includes get_workout_program

- **WHEN** the MCP integration test enumerates registered tools
- **THEN** `get_workout_program` is present
