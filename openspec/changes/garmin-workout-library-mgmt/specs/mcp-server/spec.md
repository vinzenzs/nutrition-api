# mcp-server Specification (delta)

## ADDED Requirements

### Requirement: Garmin workout-library and blob tools manage and export from the agent

The MCP server SHALL expose five tools, each issuing exactly one HTTP call to the
corresponding backend control endpoint and forwarding the response body verbatim
via `toToolResult`:

- `garmin_delete_workout` (a `workout_id` → `DELETE /garmin/workout/{workout_id}`)
  — reaps the orphaned Garmin workout object for one workout.
- `garmin_list_workouts` (optional `start`/`limit` → `GET /garmin/workouts`) —
  lists the Garmin workout library for plan↔watch reconciliation.
- `garmin_get_workout` (a `garmin_workout_id` →
  `GET /garmin/workout/{garmin_workout_id}`) — fetches one library object.
- `garmin_push_hydration` (a `value_ml` + `date` → `POST /garmin/hydration`) —
  pushes the user's logged hydration back to Garmin; the ONLY push from the
  nutrition system to Garmin, and opt-in.
- `garmin_export_activity` (an `activity_id` + optional `format` →
  `GET /garmin/activity/{activity_id}/export`) — returns the activity's FIT/GPX
  blob as a base64 envelope.

Write tools (`garmin_delete_workout`, `garmin_push_hydration`) SHALL auto-derive
an idempotency key when the caller supplies none; read tools
(`garmin_list_workouts`, `garmin_get_workout`, `garmin_export_activity`) SHALL NOT
send `Idempotency-Key`. When the backend returns `503 garmin_disabled`, the tool
result SHALL carry that body with `isError=true`. The MCP integration
expected-tools list SHALL include all five.

#### Scenario: garmin_delete_workout issues one DELETE

- **WHEN** the agent calls `garmin_delete_workout` with `{"workout_id":"<uuid>"}`
- **THEN** the MCP server issues exactly one `DELETE /garmin/workout/<uuid>`
- **AND** sets `Idempotency-Key` to the agent's explicit key (if any) or to the derived stable key
- **AND** the tool result is the backend response verbatim

#### Scenario: garmin_list_workouts issues one GET

- **WHEN** the agent calls `garmin_list_workouts` with `{}`
- **THEN** the MCP server issues exactly one `GET /garmin/workouts`
- **AND** does NOT include an `Idempotency-Key` header
- **AND** the tool result is the library list verbatim

#### Scenario: garmin_list_workouts forwards pagination

- **WHEN** the agent calls `garmin_list_workouts` with `{"start":0,"limit":50}`
- **THEN** the MCP server issues `GET /garmin/workouts?start=0&limit=50`

#### Scenario: garmin_get_workout issues one GET by id

- **WHEN** the agent calls `garmin_get_workout` with `{"garmin_workout_id":"123"}`
- **THEN** the MCP server issues exactly one `GET /garmin/workout/123`
- **AND** does NOT include an `Idempotency-Key` header

#### Scenario: garmin_push_hydration issues one POST

- **WHEN** the agent calls `garmin_push_hydration` with `{"value_ml":2400,"date":"2026-06-12"}`
- **THEN** the MCP server issues exactly one `POST /garmin/hydration` with that body
- **AND** sets `Idempotency-Key` to the agent's explicit key (if any) or to the derived stable key
- **AND** the tool result is the backend response verbatim

#### Scenario: garmin_push_hydration description names the write direction and opt-in nature

- **WHEN** the agent reads the `garmin_push_hydration` tool description
- **THEN** the description states this is the only push FROM the nutrition system TO Garmin
- **AND** notes it is opt-in (invoke only on explicit request) and overwrites the day's Garmin total (not append)

#### Scenario: garmin_export_activity issues one GET and returns the blob envelope

- **WHEN** the agent calls `garmin_export_activity` with `{"activity_id":"987","format":"gpx"}`
- **THEN** the MCP server issues exactly one `GET /garmin/activity/987/export?format=gpx`
- **AND** does NOT include an `Idempotency-Key` header
- **AND** the tool result carries the `{activity_id, format, filename, content_base64}` envelope verbatim

#### Scenario: garmin_export_activity omits format when not supplied

- **WHEN** the agent calls `garmin_export_activity` with only `{"activity_id":"987"}`
- **THEN** the request URL does NOT include a `format` query parameter
- **AND** the backend applies the default `fit` format

#### Scenario: Disabled bridge surfaces as a tool error

- **WHEN** `GARMIN_BRIDGE_URL` is unset and the agent calls any of these five tools
- **THEN** the tool result carries the `503 garmin_disabled` body with `isError=true`

#### Scenario: Expected-tools list includes the five new tools

- **WHEN** the MCP integration test enumerates registered tools
- **THEN** `garmin_delete_workout`, `garmin_list_workouts`, `garmin_get_workout`,
  `garmin_push_hydration`, and `garmin_export_activity` are all present

## MODIFIED Requirements

### Requirement: Garmin scheduling tools push the plan to the watch from the agent

The MCP server SHALL expose `garmin_schedule_workout` (a `workout_id` →
`POST /garmin/schedule/workout`), `garmin_unschedule_workout` (a `workout_id` →
`DELETE /garmin/schedule/workout/{id}`), `garmin_schedule_plan` (a plan scope →
`POST /garmin/schedule/plan`), and `garmin_list_scheduled` (a date range →
`GET /garmin/calendar`), each issuing exactly one HTTP call to the corresponding
backend endpoint and forwarding the response verbatim. Write tools SHALL
auto-derive an idempotency key when the caller does not supply one. When the
backend returns `503 garmin_disabled`, the tool result SHALL carry that body with
`isError=true`. The MCP integration expected-tools list SHALL include all four.
The `garmin_unschedule_workout` tool's underlying backend endpoint now ALSO
deletes the workout's Garmin library object (not just its calendar entry), so the
tool's description SHALL state that unscheduling removes both the calendar entry
AND the library object, leaving no orphan; and that `garmin_delete_workout` reaps
only the library object for a workout the agent finds orphaned during
reconciliation.

#### Scenario: garmin_schedule_workout issues one POST

- **WHEN** the agent calls `garmin_schedule_workout` with a planned workout's id
- **THEN** the MCP server issues exactly one `POST /garmin/schedule/workout`
- **AND** the tool result is the backend response verbatim

#### Scenario: garmin_schedule_plan pushes a week

- **WHEN** the agent calls `garmin_schedule_plan` with a plan-week scope
- **THEN** the MCP server issues exactly one `POST /garmin/schedule/plan`
- **AND** the tool result reports the per-workout results verbatim

#### Scenario: garmin_unschedule_workout description names the dual teardown

- **WHEN** the agent reads the `garmin_unschedule_workout` tool description
- **THEN** the description states that unscheduling removes both the Garmin
  calendar entry AND the underlying workout library object
- **AND** points at `garmin_delete_workout` for reaping a stray library object alone

#### Scenario: Disabled bridge surfaces as a tool error

- **WHEN** `GARMIN_BRIDGE_URL` is unset and the agent calls any scheduling tool
- **THEN** the tool result carries the `503 garmin_disabled` body with `isError=true`

#### Scenario: Expected-tools list includes the scheduling tools

- **WHEN** the MCP integration test enumerates registered tools
- **THEN** `garmin_schedule_workout`, `garmin_unschedule_workout`,
  `garmin_schedule_plan`, and `garmin_list_scheduled` are all present
