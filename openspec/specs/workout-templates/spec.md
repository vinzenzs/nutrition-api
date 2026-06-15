# workout-templates Specification

## Purpose

Define a catalogue of reusable, first-party-authored workout templates — the structured "what to do" of a session, independent of any date or athlete instance. A template carries a sport, a name, optional metadata, and an ordered, validated step program (executable steps and bounded repeat groups, each with an intent, exactly one duration, and a target). Templates are the building block that training plans reference per slot and that the Garmin bridge compiles into structured calendar workouts; they hold no scheduling, provenance, or completion state of their own. The capability exposes CRUD over REST (mirrored 1:1 by MCP tools) and validates the step structure at the service layer so malformed programs never reach the database.

## Requirements
### Requirement: Workout templates are stored in a dedicated table

The system SHALL persist reusable workout templates in a `workout_templates`
table independent of `workouts`. Each row holds a `sport` (reusing the
`workouts` sport vocabulary), a `name`, an optional `description`, an optional
author-supplied `estimated_duration_sec`, an ordered list of `steps` stored as
JSONB, and audit timestamps. Templates are first-party authored — there is no
`external_id` or `source` column and no uniqueness constraint on `name`.

#### Scenario: Table is created with the documented columns

- **WHEN** the migration set is applied to a clean database
- **THEN** `workout_templates` exists with columns:
  - `id` (UUID PRIMARY KEY)
  - `sport` (TEXT NOT NULL, CHECK IN `('run', 'bike', 'swim', 'strength', 'yoga', 'mobility', 'other')`)
  - `name` (TEXT NOT NULL)
  - `description` (TEXT NULL)
  - `estimated_duration_sec` (INTEGER NULL, CHECK `estimated_duration_sec IS NULL OR estimated_duration_sec > 0`)
  - `steps` (JSONB NOT NULL, CHECK `jsonb_typeof(steps) = 'array' AND jsonb_array_length(steps) > 0`)
  - `created_at` (TIMESTAMPTZ NOT NULL DEFAULT now())
  - `updated_at` (TIMESTAMPTZ NOT NULL DEFAULT now())
- **AND** an index exists on `(sport)` to support the list filter
- **AND** the `sport` CHECK reuses the `workouts` sport vocabulary, including `'yoga'` and `'mobility'`

#### Scenario: sport vocabulary admits yoga and mobility

- **WHEN** a template is created with `sport: "yoga"` or `sport: "mobility"`
- **THEN** the row persists with that sport and it is returned unchanged on read
- **AND** the value passes the same validation used for `workouts` sports

### Requirement: A template's steps are a validated structured program

A template's `steps` SHALL be an ordered, non-empty array of nodes. Each node is
either a single executable step or a repeat group. A single step SHALL carry an
`intent` (`warmup`, `active`, `interval`, `recovery`, `rest`, or `cooldown`),
exactly one `duration` (`{kind:"time",seconds}` with `seconds > 0`,
`{kind:"distance",meters}` with `meters > 0`, `{kind:"lap_button"}`, or
`{kind:"open"}`), and a `target` whose `kind` is one of `none`, `hr_zone`,
`power_zone`, `pace`, `hr_bpm`, `power_w`, or `rpe`; an optional free-text
`note` MAY be present. A repeat group SHALL carry a `count >= 2` and a non-empty
`steps` array of single steps only — repeat groups SHALL NOT nest. The system
SHALL validate this structure on every write at the service layer and reject
malformed steps with a sentinel error mapped to a 1:1 API error code.

#### Scenario: A valid structured template is accepted

- **WHEN** `POST /workout-templates` is called with a `run` template whose steps
  are `[warmup time 600s @ hr_zone 1–2, repeat ×5 of (interval time 180s @
  power_zone 4–4, recovery time 120s @ hr_zone 1), cooldown time 300s @ hr_zone 1]`
- **THEN** the template is persisted and returned with a generated `id` and the
  steps echoed verbatim

#### Scenario: Empty steps are rejected

- **WHEN** a create or patch supplies `steps: []`
- **THEN** the response is a validation error and nothing is persisted

#### Scenario: Nested repeat groups are rejected

- **WHEN** a write supplies a `repeat` node whose `steps` contain another
  `repeat` node
- **THEN** the response is a validation error identifying the nesting violation

#### Scenario: Out-of-range target zones are rejected

- **WHEN** a step supplies a `hr_zone` or `power_zone` target with a bound
  outside `1..5`, or a `target` whose `low` exceeds its `high`
- **THEN** the response is a validation error and nothing is persisted

#### Scenario: An unknown duration or target kind is rejected

- **WHEN** a step supplies a `duration` or `target` with an unrecognized `kind`
- **THEN** the response is a validation error and nothing is persisted

### Requirement: REST surface for template CRUD

The system SHALL expose `POST /workout-templates`, `GET /workout-templates`
(with an optional `?sport=` filter), `GET /workout-templates/{id}`,
`PATCH /workout-templates/{id}`, and `DELETE /workout-templates/{id}`, behind the
standard auth + idempotency middleware. `POST` honors `Idempotency-Key`;
`PATCH` is a partial update where a supplied `steps` array fully replaces the
prior steps. All responses are JSON template objects (or a list for the
collection GET).

#### Scenario: Create then fetch round-trips the template

- **WHEN** a client `POST`s a valid template and then `GET`s it by the returned `id`
- **THEN** the fetched body equals the created template including its steps

#### Scenario: List filters by sport

- **WHEN** templates of several sports exist and a client `GET`s
  `/workout-templates?sport=swim`
- **THEN** only `swim` templates are returned

#### Scenario: Patch replaces steps as a unit

- **WHEN** a client `PATCH`es a template with a new `steps` array and omits all
  other fields
- **THEN** the steps are replaced wholesale and every other field is unchanged

#### Scenario: Patch leaves omitted fields unchanged

- **WHEN** a client `PATCH`es only `name`
- **THEN** `name` is updated and `sport`, `description`, `estimated_duration_sec`,
  and `steps` are unchanged

#### Scenario: Delete removes the template

- **WHEN** a client `DELETE`s a template by `id`
- **THEN** a subsequent `GET` for that `id` returns 404

#### Scenario: Fetching a missing template returns 404

- **WHEN** a client `GET`s `/workout-templates/{id}` for an unknown `id`
- **THEN** the response is a 404 with the standard error shape

### Requirement: MCP tools mirror the template REST surface

The MCP server SHALL expose `create_workout_template`, `list_workout_templates`,
`get_workout_template`, `patch_workout_template`, and `delete_workout_template`,
each issuing exactly one HTTP call to the corresponding REST endpoint and
forwarding the response body verbatim. Write tools SHALL auto-derive an
idempotency key when the caller does not supply one, per the existing convention.
The MCP integration expected-tools list SHALL include all five.

#### Scenario: create_workout_template issues one POST

- **WHEN** the agent calls `create_workout_template` with a template body
- **THEN** the MCP server issues exactly one `POST /workout-templates`
- **AND** the tool result is the REST response verbatim

#### Scenario: Expected-tools list includes the template tools

- **WHEN** the MCP integration test enumerates registered tools
- **THEN** all five workout-template tools are present

