# training-plan — delta for add-slot-duration-override

## ADDED Requirements

### Requirement: Plan slots carry optional per-intent duration overrides

The system SHALL allow a `plan_slot` to carry an optional `duration_overrides`
list, each entry a `{intent, duration}` pair, stored as a JSONB column. The
`intent` SHALL be one of the workout-template step intents (`warmup`, `active`,
`interval`, `recovery`, `rest`, `cooldown`). The `duration` SHALL use the
workout-templates Duration shape restricted to its two bounded kinds
(`{kind:"time",seconds}` with `seconds > 0` or `{kind:"distance",meters}` with
`meters > 0`) and be validated by the same Duration validator; the unbounded kinds
(`lap_button`, `open`) SHALL be rejected as overrides. The list SHALL contain at
most one entry per intent; a null or empty list means no overrides. Slot create
and patch SHALL accept `duration_overrides`, the nested plan `GET` SHALL return
it, and a `PATCH` that supplies the list SHALL replace it wholesale (supplying
`[]` clears it, omitting it leaves it unchanged).

#### Scenario: A slot stores a duration override for its work block

- **WHEN** a slot referencing a tempo template is created with
  `duration_overrides: [{intent:"active", duration:{kind:"time", seconds:3600}}]`
- **THEN** the slot persists the override and the nested plan `GET` returns it

#### Scenario: Duplicate intent in the override list is rejected

- **WHEN** a slot write supplies two duration override entries with the same
  `intent`
- **THEN** the response is a validation error and nothing is persisted

#### Scenario: An unbounded or invalid override duration is rejected

- **WHEN** an override supplies a `duration` of kind `open` or `lap_button`, or a
  `time` duration with `seconds <= 0`, or a `distance` duration with `meters <= 0`,
  or an unknown duration kind
- **THEN** the response is a validation error (the workout-templates Duration
  validator plus the bounded-kind rule) and nothing is persisted

#### Scenario: Patch replaces the duration override list wholesale

- **WHEN** a client `PATCH`es a slot with a new `duration_overrides` list
- **THEN** the prior list is replaced entirely
- **AND** supplying `[]` clears all overrides while omitting the field leaves them
  unchanged

## MODIFIED Requirements

### Requirement: A planned workout's effective program applies slot overrides to template steps

The system SHALL define a planned workout's **effective program** as its
template's steps with each step's `target` replaced when that step's `intent`
matches an entry in the workout's slot `target_overrides`, **and** each step's
`duration` replaced when that step's `intent` matches an entry in the workout's
slot `duration_overrides`. The two override lists SHALL be independent and SHALL
compose (a step whose intent matches both gets both its target and its duration
replaced). Steps whose intent matches no override SHALL be unchanged, and
overrides SHALL affect targets and durations of existing steps only, never step
structure (step count, order, or repeat-group counts). This effective program
SHALL be the single representation that downstream consumers use — both display
and the Garmin compile path (`add-garmin-scheduling`) SHALL build from effective
steps, not raw template steps. The effective program SHALL be resolved on read
from the template and slot, not snapshotted onto the workout row.

#### Scenario: Override replaces only the matching intent's duration

- **WHEN** a template is `[warmup 10min @hr_zone 1, active 55min @tempo, cooldown 10min @hr_zone 1]`
  and the slot overrides `active` with `duration {kind:"time", seconds:3600}`
- **THEN** the effective program has the `active` step lasting 60min
- **AND** the warmup and cooldown durations are unchanged at 10min
- **AND** the step structure and all targets are unchanged

#### Scenario: Duration and target overrides compose on the same intent

- **WHEN** a slot carries both a `target_overrides` entry and a
  `duration_overrides` entry for intent `interval`
- **THEN** the effective program's interval steps show both the overridden target
  and the overridden duration

#### Scenario: No overrides yields the template program verbatim

- **WHEN** a planned workout's slot has neither `target_overrides` nor
  `duration_overrides`
- **THEN** its effective program equals the template's steps unchanged

### Requirement: Materialize expands the plan into planned workouts idempotently

The system SHALL expose `POST /training-plans/{id}/materialize` accepting a scope
of a single week (`{scope:"week",week:N}`), a date range
(`{scope:"range",from,to}`), or the whole plan (`{scope:"all"}`). For each
in-scope slot it SHALL compute the date as
`plan.start_date + (week.ordinal-1) weeks + slot.weekday`, derive a time window
from `slot.time_of_day` (or a default stacked by `slot.ordinal`) and the
**session length** defined below, and UPSERT a `workouts` row with
`status='planned'`, the slot's template's sport and name, `template_id`, and
`plan_slot_id`. The session length SHALL be derived in order: (1) the sum of the
slot's **effective program** step durations when every step is bounded by time;
(2) otherwise the template's `estimated_duration_sec`; (3) otherwise a one-hour
default — so a slot's `duration_overrides` move the materialized window in lockstep
with the watch workout. The upsert SHALL be keyed on `plan_slot_id` so re-running
updates the same rows, and its update SHALL apply only where the existing row's
`status` is `planned`. When a (week, weekday) has more than one slot, the
materialized workouts SHALL share a generated `session_group`. The response SHALL
return the planned workouts created or updated.

#### Scenario: A duration override moves the materialized session length

- **WHEN** a slot's effective program (template steps + `duration_overrides`) sums
  to 80min by time and the week is materialized
- **THEN** the planned workout's time window spans 80min, not the template's
  original `estimated_duration_sec`

#### Scenario: Materializing a week creates planned workouts on the right dates

- **WHEN** a plan with `start_date` = a Monday has a week 1 with a slot on weekday 2
  (Wednesday) and the client materializes week 1
- **THEN** a planned `workouts` row exists dated that Wednesday with the template's
  sport and name and `status='planned'`

#### Scenario: Re-materializing is idempotent

- **WHEN** the same week is materialized twice
- **THEN** no duplicate planned workouts are created (the slot-keyed rows are
  updated in place)

#### Scenario: Materialize never reverts a fulfilled planned workout

- **WHEN** a planned workout has been marked `completed` (carrying its
  `plan_slot_id`) and its plan is re-materialized
- **THEN** the slot-keyed update is skipped for that row (guarded by
  `status='planned'`) and the completed workout and its actuals are unchanged

### Requirement: MCP tools mirror the training-plan REST surface

The MCP server SHALL expose `create_training_plan`, `list_training_plans`,
`get_training_plan`, `patch_training_plan`, `delete_training_plan`,
`add_plan_week`, `patch_plan_week`, `delete_plan_week`, `add_plan_slot`,
`patch_plan_slot`, `delete_plan_slot`, and `materialize_training_plan`, each
issuing exactly one HTTP call to the corresponding endpoint and forwarding the
response verbatim. The `add_plan_slot` and `patch_plan_slot` tools SHALL accept
`duration_overrides` (alongside the existing `target_overrides`). Write tools
SHALL auto-derive an idempotency key when none is supplied. No new tools are added
by this change and the MCP integration expected-tools list is unchanged.

#### Scenario: add_plan_slot carries duration overrides

- **WHEN** the agent calls `add_plan_slot` with a `duration_overrides` list
- **THEN** the slot is created with those overrides and the nested plan `GET`
  returns them

#### Scenario: patch_plan_slot replaces duration overrides

- **WHEN** the agent calls `patch_plan_slot` with a new `duration_overrides` list
- **THEN** the slot's prior duration overrides are replaced wholesale
