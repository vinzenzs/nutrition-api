## ADDED Requirements

### Requirement: Meal and hydration tools accept an optional workout_id

The MCP server SHALL extend the existing `log_meal`, `log_meal_freeform`, `patch_meal`, `log_hydration`, and `patch_hydration` tools with an optional `workout_id` argument. When supplied, the wrapper forwards it in the REST body verbatim. When omitted, the wrapper does not include the field. PATCH tools additionally honour the empty-string sentinel `""` to clear an existing link.

#### Scenario: log_meal forwards workout_id when supplied

- **WHEN** the agent calls `log_meal` with `{"product_id":"<p>","quantity_g":150,"logged_at":"…","workout_id":"<w>"}`
- **THEN** the wrapper issues `POST /meals` with `"workout_id":"<w>"` in the body
- **AND** the response (forwarded from the REST API) includes `workout_id`

#### Scenario: log_meal omits workout_id when not supplied

- **WHEN** the agent calls `log_meal` without `workout_id`
- **THEN** the wrapper's POST body does NOT contain a `workout_id` key
- **AND** existing behaviour is preserved

#### Scenario: log_meal_freeform and log_hydration forward workout_id the same way

- **WHEN** the agent calls `log_meal_freeform` or `log_hydration` with `workout_id`
- **THEN** the field is forwarded in the POST body verbatim

#### Scenario: patch_meal forwards empty string to clear

- **WHEN** the agent calls `patch_meal` with `{"meal_id":"<id>","workout_id":""}`
- **THEN** the wrapper issues `PATCH /meals/<id>` with body `{"workout_id":""}`
- **AND** the REST backend interprets the empty string as "clear the link"

#### Scenario: Tool descriptions explain the link semantics

- **WHEN** the agent reads the description of any of these tools
- **THEN** the description mentions that `workout_id` is an optional link to a `workouts` row
- **AND** the PATCH tools' descriptions explicitly document the empty-string clear semantic
- **AND** the descriptions note that the link is metadata — the `workout_fueling_summary` tool uses time-window matching, not tags

### Requirement: workout_fueling_summary tool wraps GET /workouts/{id}/fueling

The MCP server SHALL expose a new `workout_fueling_summary` tool that wraps `GET /workouts/{id}/fueling`. The tool takes `workout_id` (required string) plus optional `pre_window_min` and `post_window_min` integers. Read-only: no `Idempotency-Key`, no derived key.

#### Scenario: workout_fueling_summary calls the fueling endpoint

- **WHEN** the agent calls `workout_fueling_summary` with `{"workout_id":"<w>","pre_window_min":180,"post_window_min":90}`
- **THEN** the wrapper issues `GET /workouts/<w>/fueling?pre_window_min=180&post_window_min=90`
- **AND** does NOT include an `Idempotency-Key` header

#### Scenario: workout_fueling_summary omits unset optional params

- **WHEN** the agent calls `workout_fueling_summary` with only `{"workout_id":"<w>"}`
- **THEN** the wrapper issues `GET /workouts/<w>/fueling` (no query params)
- **AND** the REST server applies the default windows (240 pre, 60 post)

#### Scenario: workout_fueling_summary description spells out the semantics

- **WHEN** the agent reads the `workout_fueling_summary` tool description
- **THEN** the description states that aggregation is by time-window, not by the `workout_id` tag on intake entries
- **AND** lists the default windows (240 min pre, 60 min post) and the bounds (0..720)
- **AND** notes that `nutrition` and `hydration` are separate sub-objects per window so the agent does NOT mix units
- **AND** notes that when `workout_fuel_entries` ships (future), this tool's response will gain those contributions automatically with no contract change

#### Scenario: 404 from the endpoint is forwarded as isError

- **WHEN** the REST endpoint returns `404 {"error":"workout_not_found"}`
- **THEN** the wrapper forwards the body verbatim
- **AND** sets `isError = true` on the tool result
