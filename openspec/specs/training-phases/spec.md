# training-phases Specification

## Purpose

Paired training phases and reusable goal templates that extend the effective-goals resolution chain with a resolver-time step between per-date overrides and the singleton default. Phases are named date ranges tagged with a phase type (`base`, `build`, `peak`, `recovery`, `race_week`, `off_season`, `other`) and may point at a goal template whose nutrient bounds drive adherence for every date the phase covers — unless a per-date override exists for that date, in which case the override still wins.
## Requirements
### Requirement: Goal templates are named reusable goal-sets

The system SHALL persist named goal templates via a `goal_templates` table whose nutrient-bound columns match the existing `nutrition_goals` and `daily_goal_overrides` projection (30 columns for the 15 supported nutrients × `{min, max}`). Templates SHALL be addressable by `name` in the REST surface (URL path) and by `id` (UUID) in foreign-key references from other tables. Template `name` is `UNIQUE NOT NULL` and chosen by the user; the validation rules for template goals SHALL be identical to those for `PUT /goals` (`goal_value_invalid` for negatives/NaN/empty Range, `goal_range_invalid` for inverted min/max, legacy `kcal_target` rejected).

#### Scenario: PUT /goal-templates/{name} upserts a template

- **WHEN** the client calls `PUT /goal-templates/weekday-easy-training` with body `{"kcal":{"min":2090,"max":2310},"protein_g":{"min":150,"max":190},"carbs_g":{"min":280,"max":340}}`
- **THEN** the system stores those three target bounds on a `goal_templates` row with `name = "weekday-easy-training"`
- **AND** returns `200 OK` with body `{"template": {"name":"weekday-easy-training","goals":<the stored Goals>,"id":"<uuid>","notes":null,"created_at":...,"updated_at":...}}`
- **AND** the response's nutrient bounds are rounded to 1dp per the existing rounding requirement

#### Scenario: PUT replaces all bounds wholesale (full-replace semantics)

- **WHEN** a template `weekday-easy-training` already exists with `kcal` and `protein_g` set
- **AND** the client PUTs `{"carbs_g":{"min":300}}`
- **THEN** the row now has `carbs_g_min = 300` and every other nutrient bound NULL (matches `PUT /goals` full-replace semantics)

#### Scenario: Idempotency-Key on PUT is rejected

- **WHEN** the client supplies `Idempotency-Key` on `PUT /goal-templates/{name}`
- **THEN** the system returns `400 idempotency_unsupported_for_put` (matches the auth capability's PUT rule)

#### Scenario: GET /goal-templates/{name} returns a template

- **WHEN** the template `weekday-easy-training` exists
- **AND** the client calls `GET /goal-templates/weekday-easy-training`
- **THEN** the system returns `200 OK` with `{"template": {...}}` matching the stored row (nutrient bounds rounded to 1dp)

#### Scenario: GET on a missing template returns 404

- **WHEN** no template with that name exists
- **THEN** the system returns `404 Not Found` with `{"error":"template_not_found"}`

#### Scenario: GET /goal-templates lists all templates

- **WHEN** the client calls `GET /goal-templates`
- **THEN** the system returns `200 OK` with body `{"templates": [<template>, …]}` (an array of every template row)
- **AND** entries are ordered by `name` ascending

#### Scenario: DELETE /goal-templates/{name} removes the template

- **WHEN** the template exists AND no phase references it
- **AND** the client deletes it
- **THEN** the system returns `204 No Content`
- **AND** subsequent GET returns `404 template_not_found`

#### Scenario: DELETE refuses when a phase references the template

- **WHEN** the template is referenced by one or more phases' `default_template_id`
- **AND** the client deletes it
- **THEN** the system returns `409 Conflict` with body `{"error":"template_in_use","referencing_phases":[{"id":"<uuid>","name":"<phase-name>"}, …]}`
- **AND** the template is NOT deleted

#### Scenario: DELETE of unknown template returns 404

- **WHEN** no template with that name exists
- **THEN** the system returns `404 Not Found` with `{"error":"template_not_found"}`

#### Scenario: Template name validation

- **WHEN** the client attempts to PUT a template with an empty or whitespace-only name in the URL
- **THEN** the system returns `400 Bad Request` with `{"error":"template_name_invalid"}`

- **WHEN** the name exceeds a documented maximum length (`128`)
- **THEN** the system returns `400 Bad Request` with `{"error":"template_name_too_long","max_length":128}`

### Requirement: Training phases are named date ranges tagged with type

The system SHALL persist training phases via a `training_phases` table with columns `(id UUID PK, name TEXT NOT NULL, type TEXT NOT NULL CHECK type IN <enum>, start_date DATE NOT NULL, end_date DATE NOT NULL, default_template_id UUID NULL REFERENCES goal_templates(id) ON DELETE RESTRICT, notes TEXT NULL, created_at, updated_at)`. The enum of phase types is `('base', 'build', 'peak', 'recovery', 'race_week', 'off_season', 'other')`. Date ranges are inclusive on both ends. Overlaps between phases SHALL be allowed; at adherence-resolution time the most-recently-updated overlapping phase wins.

#### Scenario: POST /phases creates a phase

- **WHEN** the client calls `POST /phases` with body `{"name":"build-block-2","type":"build","start_date":"2026-07-01","end_date":"2026-07-28","default_template_id":"<template-uuid>","notes":"weeks 5-8 of 16-week plan"}`
- **THEN** the system creates the row and returns `201 Created` with body `{"phase": {<the stored row>, "default_template_name":"<resolved name>"}}`
- **AND** the response includes `default_template_name` (the resolved name of the template referenced by `default_template_id`) as a convenience sibling

#### Scenario: POST validates date range

- **WHEN** the client POSTs with `start_date > end_date`
- **THEN** the system returns `400 Bad Request` with `{"error":"date_range_invalid"}`

#### Scenario: POST validates phase type

- **WHEN** the client POSTs with `type: "kettlebell"` (not in the enum)
- **THEN** the system returns `400 Bad Request` with `{"error":"phase_type_invalid","allowed":["base","build","peak","recovery","race_week","off_season","other"]}`

#### Scenario: POST validates name

- **WHEN** the client POSTs with empty or whitespace-only `name`
- **THEN** the system returns `400 Bad Request` with `{"error":"phase_name_invalid"}`

- **WHEN** `name` exceeds 128 characters
- **THEN** the system returns `400 Bad Request` with `{"error":"phase_name_too_long","max_length":128}`

#### Scenario: POST with a non-existent default_template_id is rejected

- **WHEN** the client POSTs with a `default_template_id` that doesn't match any existing template
- **THEN** the system returns `400 Bad Request` with `{"error":"template_not_found"}`
- **AND** no phase row is inserted

#### Scenario: POST with default_template_id omitted creates a template-less phase

- **WHEN** the client POSTs without `default_template_id`
- **THEN** the phase is created with `default_template_id = NULL`
- **AND** adherence on dates within the phase falls through to the singleton default (no `goal_source: "phase_template"`)

#### Scenario: GET /phases/{id} returns the phase

- **WHEN** a phase exists
- **AND** the client calls `GET /phases/<phase-id>`
- **THEN** the response is `200 OK` with `{"phase": {...}, "default_template_name": "<name or null>"}`

#### Scenario: GET on unknown id returns 404

- **WHEN** no phase with that id exists
- **THEN** the system returns `404 Not Found` with `{"error":"phase_not_found"}`

#### Scenario: GET /phases?from=&to= lists intersecting phases

- **WHEN** the client calls `GET /phases?from=2026-07-01&to=2026-07-31`
- **THEN** the response is `200 OK` with body `{"phases": [<phase>, …]}` containing every phase whose `[start_date, end_date]` intersects `[from, to]`
- **AND** entries are ordered by `start_date` ascending, ties broken by `updated_at` descending

#### Scenario: List range query without dates is rejected

- **WHEN** the client omits `from` or `to`
- **THEN** the system returns `400 Bad Request` with `{"error":"range_required"}`

#### Scenario: List range query larger than 730 days is rejected

- **WHEN** the range spans more than 730 days inclusive
- **THEN** the system returns `400 Bad Request` with `{"error":"range_too_large","max_days":730}`

#### Scenario: PATCH /phases/{id} updates a subset of fields

- **WHEN** a phase exists
- **AND** the client calls `PATCH /phases/<id>` with body `{"default_template_id":"<new-template-uuid>"}`
- **THEN** the system updates only `default_template_id` (and bumps `updated_at`)
- **AND** every other field is preserved
- **AND** the response is `200 OK` with the updated phase

#### Scenario: PATCH with name / type / dates also works

- **WHEN** the client PATCHes `{"name":"build-block-2-revised","type":"peak","start_date":"2026-07-08"}`
- **THEN** the system updates those three fields
- **AND** the dates are validated together as if POST'd (the resulting `start_date > end_date` after a partial update returns `400 date_range_invalid`)

#### Scenario: PATCH with empty body returns 400

- **WHEN** the client PATCHes with `{}`
- **THEN** the system returns `400 Bad Request` with `{"error":"patch_empty"}`

#### Scenario: DELETE /phases/{id} removes the phase

- **WHEN** the phase exists
- **AND** the client deletes it
- **THEN** the system returns `204 No Content`
- **AND** subsequent adherence resolution for dates previously in the phase falls through to the singleton default (or to per-date overrides if any exist on those dates)
- **AND** any template the phase referenced is unaffected

#### Scenario: DELETE on unknown id returns 404

- **WHEN** no phase with that id exists
- **THEN** the system returns `404 Not Found` with `{"error":"phase_not_found"}`

### Requirement: Phase resolution participates in adherence at read time

The system SHALL extend the effective-goals resolution chain in the nutrition-goals capability to consult `training_phases` between the per-date override step and the singleton default step. For a date `D`, the resolver SHALL find any phase covering `D` (where `D` is in `[start_date, end_date]` inclusive), and if exactly one matches, resolve to its `default_template_id`'s goal-set. If multiple phases overlap `D`, the resolver SHALL pick the most-recently-updated phase (`ORDER BY updated_at DESC LIMIT 1`). If the matched phase's `default_template_id` is NULL, the resolver continues to the singleton default — that is, a phase without a template is invisible to adherence.

#### Scenario: Phase with template resolves to phase_template

- **WHEN** a phase covers `2026-07-15` and has `default_template_id` pointing at a template with `kcal: {min: 2090, max: 2310}`
- **AND** no per-date override exists for `2026-07-15`
- **AND** the client calls `GET /summary/daily?date=2026-07-15`
- **THEN** the adherence rows use the template's bounds
- **AND** the response includes `"goal_source": "phase_template"`
- **AND** the response includes `"phase_name": "<the phase's name>"`

#### Scenario: Per-date override wins over phase

- **WHEN** a phase covers `2026-07-15` AND a per-date override also exists for `2026-07-15`
- **THEN** the adherence row uses the override's bounds
- **AND** `goal_source` is `"override"`
- **AND** `phase_name` is absent (or null)

#### Scenario: Phase without template falls through to default

- **WHEN** a phase covers `2026-07-15` with `default_template_id = NULL`
- **AND** the singleton default has `kcal: {min: 2000}`
- **AND** no per-date override exists
- **THEN** the adherence row uses the singleton default's bounds
- **AND** `goal_source` is `"default"`
- **AND** `phase_name` is absent

#### Scenario: Most-recently-updated overlapping phase wins

- **WHEN** two phases both cover `2026-07-15` — phase A (`recovery`) updated at T1, phase B (`race_week`) updated at T2 > T1
- **AND** no per-date override exists
- **THEN** the resolver picks phase B
- **AND** `phase_name` is phase B's name

#### Scenario: Phase deletion makes the resolver fall through

- **WHEN** a phase covered `2026-07-15` and was driving its adherence
- **AND** the phase is deleted
- **AND** the client calls `GET /summary/daily?date=2026-07-15` again
- **THEN** the response no longer shows `goal_source: "phase_template"`
- **AND** the resolver falls through to the next step (override > singleton default > none) per the existing chain

#### Scenario: Range summary resolves phases per-day in one batch

- **WHEN** the client calls `GET /summary/range?from=2026-07-01&to=2026-07-31`
- **AND** the window spans 31 days with one phase covering days 5-15
- **THEN** the system issues at most one batched query to find every phase intersecting `[2026-07-01, 2026-07-31]` (not one per day)
- **AND** per-day goal_source values reflect override > phase_template > default > none as appropriate
- **AND** `phase_name` is set per-day where goal_source is `phase_template`

#### Scenario: Range summary mixes goal sources across days

- **WHEN** a 7-day range has: 2 days with overrides, 3 days inside a phase with a template, 1 day inside a phase without a template (falls through), 1 day with no phase (falls through to default)
- **THEN** the per-day `goal_source` array is `["override","override","phase_template","phase_template","phase_template","default","default"]`
- **AND** the `phase_name` values are `[null,null,"<phase>","<phase>","<phase>",null,null]`

### Requirement: Phase response includes resolved template name

The system SHALL include a `default_template_name` field on every phase response (single-phase or list entry) carrying the human-readable name of the template referenced by `default_template_id`. When `default_template_id` is NULL, `default_template_name` SHALL be `null` (or absent — implementation choice consistent with codebase JSON conventions). This is a convenience for the MCP agent: it can present "your build-block-2 phase uses the weekday-easy-training template" without a follow-up `get_goal_template` call.

#### Scenario: Phase response includes template name

- **WHEN** a phase has `default_template_id` set
- **AND** the client calls `GET /phases/<id>`
- **THEN** the response includes `"default_template_name": "<the template's name>"` alongside `"default_template_id"`

#### Scenario: Phase response omits/nulls template name when unset

- **WHEN** a phase has `default_template_id = NULL`
- **AND** the client calls `GET /phases/<id>`
- **THEN** the response has `default_template_id: null` AND `default_template_name: null` (or both absent)

### Requirement: A phase carries optional methodology prose

The system SHALL allow a `training_phases` row to carry an optional `methodology`
free-text Markdown field, stored in a nullable `TEXT` column distinct from the
existing operational `notes`. It holds the curated, cited "why this phase"
narrative and is stored verbatim (no server-side rendering or transformation). The
phase create and update paths SHALL accept `methodology`, and the phase read paths
SHALL return it. A null `methodology` means none is set and SHALL serialize as
null, not an error. The MCP phase-write tool SHALL carry `methodology` in its
payload; no new MCP tool is added.

#### Scenario: A phase stores and returns methodology

- **WHEN** a phase is created or updated with a `methodology` Markdown string
  (e.g. a Base-phase "Why" block citing Seiler)
- **THEN** the phase persists it in the `methodology` column and the phase read
  returns it verbatim

#### Scenario: Methodology is independent of notes

- **WHEN** a phase has both `notes` and `methodology` set and a write supplies a new
  `methodology` without `notes`
- **THEN** `methodology` is replaced and `notes` is left unchanged

#### Scenario: Absent methodology serializes as null

- **WHEN** a phase has no `methodology`
- **THEN** the read returns `methodology` as null and no error

#### Scenario: The phase-write MCP tool carries methodology

- **WHEN** the agent writes a phase with a `methodology` field
- **THEN** the phase is persisted with that methodology and the read returns it

