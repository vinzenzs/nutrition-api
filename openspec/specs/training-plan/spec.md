# training-plan Specification

## Purpose

Define a structured training plan as a three-level tree — plan → weeks → slots — that references reusable `workout-template`s and expands deterministically into dated `planned` workouts. A plan anchors to a `start_date` (the Monday of week 1) and optionally to a race; weeks carry an ordinal and an optional training phase; slots place a template on a weekday with an ordering within the day. The plan is authoring state only: materialization computes session dates purely from `start_date` and ordinals, upserts planned `workouts` keyed on `plan_slot_id` so re-runs are idempotent, and never touches sessions an athlete has already completed. The capability exposes per-resource CRUD plus a materialize endpoint over REST, mirrored 1:1 by MCP tools.
## Requirements
### Requirement: A training plan is stored as plan → weeks → slots

The system SHALL persist a training plan across three tables: `training_plans`
(name, optional `race_id`, `start_date` = the Monday of week 1, optional notes),
`plan_weeks` (an `ordinal >= 1` unique within the plan, an optional `phase_id`,
notes), and `plan_slots` (a `weekday` 0–6 where 0=Monday, an `ordinal` ordering
sessions within a day, a required `template_id`, and an optional `time_of_day`).
Weeks cascade-delete with their plan and slots with their week. A slot's
`template_id` SHALL be `ON DELETE RESTRICT` so a referenced template cannot be
deleted.

#### Scenario: Tables are created with the documented shape

- **WHEN** the migration set is applied to a clean database
- **THEN** `training_plans`, `plan_weeks`, and `plan_slots` exist with the
  documented columns and foreign keys
- **AND** `plan_weeks` has a UNIQUE constraint on `(plan_id, ordinal)`
- **AND** `plan_weeks.ordinal` and `plan_slots.weekday` carry CHECK constraints
  (`ordinal >= 1`, `weekday BETWEEN 0 AND 6`)
- **AND** `plan_slots.template_id` references `workout_templates(id)` ON DELETE RESTRICT

### Requirement: REST surface for plan, week, and slot management

The system SHALL expose plan CRUD (`POST /training-plans`, `GET /training-plans`,
`GET /training-plans/{id}`, `PATCH /training-plans/{id}`,
`DELETE /training-plans/{id}`), week sub-resources
(`POST /training-plans/{id}/weeks`, `PATCH /training-plans/{id}/weeks/{weekId}`,
`DELETE /training-plans/{id}/weeks/{weekId}`), and slot sub-resources
(`POST /training-plans/{id}/weeks/{weekId}/slots`,
`PATCH /training-plans/{id}/slots/{slotId}`,
`DELETE /training-plans/{id}/slots/{slotId}`), behind the standard auth +
idempotency middleware. `GET /training-plans/{id}` SHALL return the full nested
tree (weeks, each with its ordered slots). Writes are per-resource so slot ids
remain stable across edits.

#### Scenario: Get returns the nested plan tree

- **WHEN** a client creates a plan, adds a week, adds two slots to that week, and
  `GET`s the plan by id
- **THEN** the response contains the plan with its week, and the week with its
  two slots in `ordinal` order

#### Scenario: Slot ids are stable across edits

- **WHEN** a slot's `template_id` is changed via `PATCH …/slots/{slotId}`
- **THEN** the slot retains its `id`
- **AND** a subsequent materialize updates the same planned workout rather than
  creating a new one

#### Scenario: A referenced template cannot be deleted

- **WHEN** a client attempts to delete a `workout-template` still referenced by a
  slot
- **THEN** the delete is rejected (RESTRICT), leaving the slot intact

#### Scenario: Deleting a plan cascades weeks and slots

- **WHEN** a client `DELETE`s a plan
- **THEN** its weeks and slots are removed
- **AND** any planned workouts that referenced its slots have their `plan_slot_id`
  set null (the workout rows are preserved)

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

### Requirement: A plan may anchor to a race and weeks to phases

The system SHALL allow a `training_plans` row to reference a `race` via
`race_id` and a `plan_weeks` row to reference a `training-phase` via `phase_id`,
both `ON DELETE SET NULL`. These links are contextual (target race; per-week
nutrition/intent) and SHALL NOT affect materialization dates, which are computed
solely from `start_date` and ordinals.

#### Scenario: Deleting a referenced race detaches the plan

- **WHEN** a race referenced by a plan is deleted
- **THEN** the plan's `race_id` becomes null and the plan is otherwise unchanged

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

### Requirement: A plan carries optional plan-level methodology prose

The system SHALL allow a `training_plans` row to carry an optional `methodology`
free-text Markdown field, stored in a nullable `TEXT` column distinct from the
existing operational `notes`. It holds the cross-cutting, non-phase-specific
reference (e.g. Key Principles, the Rowing Strategy table) and is stored verbatim
(no server-side rendering). `PATCH /training-plans/{id}` SHALL accept
`methodology`, and `GET /training-plans/{id}` (and the nested plan tree) SHALL
return it. A null `methodology` means none is set and SHALL serialize as null. The
MCP `patch_training_plan` tool SHALL carry `methodology` in its payload; no new MCP
tool is added.

#### Scenario: A plan stores and returns plan-level methodology

- **WHEN** a client `PATCH`es a plan with a `methodology` Markdown string (Key
  Principles + Rowing Strategy)
- **THEN** the plan persists it and `GET /training-plans/{id}` returns it verbatim

#### Scenario: Plan methodology is independent of notes

- **WHEN** a plan has both `notes` and `methodology` and a `PATCH` supplies a new
  `methodology` without `notes`
- **THEN** `methodology` is replaced and `notes` is left unchanged

#### Scenario: patch_training_plan carries methodology

- **WHEN** the agent calls `patch_training_plan` with a `methodology` field
- **THEN** the plan is updated with that methodology and the read returns it

