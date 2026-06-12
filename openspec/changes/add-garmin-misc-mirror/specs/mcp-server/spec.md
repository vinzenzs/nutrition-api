# mcp-server Specification (delta)

## ADDED Requirements

### Requirement: Device, health-vitals, and achievement read tools mirror the new REST endpoints

The MCP server SHALL expose three read tools, each issuing exactly one HTTP call to the corresponding backend list endpoint and forwarding the response body verbatim via `toToolResult`:

- `devices_list` (no required args → `GET /devices`) — lists the mirrored Garmin device inventory (name, model, last sync, battery, firmware).
- `health_vitals_list` (a `from`/`to` date window → `GET /health-vitals`) — lists the date-keyed blood-pressure / all-day-HR / all-day-stress snapshots.
- `achievements_list` (optional `kind` → `GET /achievements`) — lists earned badges and ad-hoc challenges.

These are read tools and SHALL NOT send an `Idempotency-Key` header. When the backend returns an error, the tool result SHALL carry that body with `isError=true`. The MCP integration expected-tools list SHALL include all three.

#### Scenario: devices_list issues one GET

- **WHEN** the agent calls `devices_list` with `{}`
- **THEN** the MCP server issues exactly one `GET /devices`
- **AND** does NOT include an `Idempotency-Key` header
- **AND** the tool result is the device list verbatim

#### Scenario: health_vitals_list forwards the window

- **WHEN** the agent calls `health_vitals_list` with `{"from":"2026-06-01","to":"2026-06-30"}`
- **THEN** the MCP server issues `GET /health-vitals?from=2026-06-01&to=2026-06-30`
- **AND** the tool result is the snapshot list verbatim

#### Scenario: achievements_list forwards the kind filter

- **WHEN** the agent calls `achievements_list` with `{"kind":"challenge"}`
- **THEN** the MCP server issues `GET /achievements?kind=challenge`
- **AND** the tool result is the achievement list verbatim

#### Scenario: Expected-tools list includes the three read tools

- **WHEN** the MCP integration test enumerates registered tools
- **THEN** `devices_list`, `health_vitals_list`, and `achievements_list` are all present

### Requirement: Garmin activity control tools manage activities from the agent

The MCP server SHALL expose five activity-level control tools, each issuing exactly one HTTP call to the corresponding backend control endpoint and forwarding the response body verbatim via `toToolResult`:

- `garmin_get_activity_gear` (an `activity_id` → `GET /garmin/activity/{activity_id}/gear`) — reads the gear linked to an activity.
- `garmin_download_workout` (a `garmin_workout_id` + optional `format` → `GET /garmin/workout/{garmin_workout_id}/download`) — exports a structured workout as a base64 blob envelope.
- `garmin_upload_activity` (a `filename` + `content_base64` → `POST /garmin/activity/upload`) — uploads a FIT activity to Garmin.
- `garmin_rename_activity` (an `activity_id` + `name` → `PATCH /garmin/activity/{activity_id}`) — renames an activity.
- `garmin_delete_activity` (an `activity_id` → `DELETE /garmin/activity/{activity_id}`) — deletes an activity; an already-absent activity is a no-op success.

Write tools (`garmin_upload_activity`, `garmin_rename_activity`, `garmin_delete_activity`) SHALL auto-derive an idempotency key when the caller supplies none; read tools (`garmin_get_activity_gear`, `garmin_download_workout`) SHALL NOT send `Idempotency-Key`. When the backend returns `503 garmin_disabled`, the tool result SHALL carry that body with `isError=true`. The MCP integration expected-tools list SHALL include all five.

#### Scenario: garmin_get_activity_gear issues one GET

- **WHEN** the agent calls `garmin_get_activity_gear` with `{"activity_id":"987"}`
- **THEN** the MCP server issues exactly one `GET /garmin/activity/987/gear`
- **AND** does NOT include an `Idempotency-Key` header
- **AND** the tool result is the backend response verbatim

#### Scenario: garmin_download_workout issues one GET and returns the blob envelope

- **WHEN** the agent calls `garmin_download_workout` with `{"garmin_workout_id":"123","format":"fit"}`
- **THEN** the MCP server issues exactly one `GET /garmin/workout/123/download?format=fit`
- **AND** does NOT include an `Idempotency-Key` header
- **AND** the tool result carries the `{garmin_workout_id, format, filename, content_base64}` envelope verbatim

#### Scenario: garmin_download_workout omits format when not supplied

- **WHEN** the agent calls `garmin_download_workout` with only `{"garmin_workout_id":"123"}`
- **THEN** the request URL does NOT include a `format` query parameter
- **AND** the backend applies the default `fit` format

#### Scenario: garmin_upload_activity issues one POST

- **WHEN** the agent calls `garmin_upload_activity` with `{"filename":"ride.fit","content_base64":"…"}`
- **THEN** the MCP server issues exactly one `POST /garmin/activity/upload` with that body
- **AND** sets `Idempotency-Key` to the agent's explicit key (if any) or to the derived stable key
- **AND** the tool result is the backend response verbatim

#### Scenario: garmin_rename_activity issues one PATCH

- **WHEN** the agent calls `garmin_rename_activity` with `{"activity_id":"987","name":"Evening Z2 ride"}`
- **THEN** the MCP server issues exactly one `PATCH /garmin/activity/987` with `{"name":"Evening Z2 ride"}`
- **AND** sets `Idempotency-Key` to the agent's explicit key (if any) or to the derived stable key

#### Scenario: garmin_delete_activity issues one DELETE

- **WHEN** the agent calls `garmin_delete_activity` with `{"activity_id":"987"}`
- **THEN** the MCP server issues exactly one `DELETE /garmin/activity/987`
- **AND** sets `Idempotency-Key` to the agent's explicit key (if any) or to the derived stable key
- **AND** an already-absent activity surfaces as a no-op success

#### Scenario: Disabled bridge surfaces as a tool error

- **WHEN** `GARMIN_BRIDGE_URL` is unset and the agent calls any of these five tools
- **THEN** the tool result carries the `503 garmin_disabled` body with `isError=true`

#### Scenario: Expected-tools list includes the five activity control tools

- **WHEN** the MCP integration test enumerates registered tools
- **THEN** `garmin_get_activity_gear`, `garmin_download_workout`, `garmin_upload_activity`, `garmin_rename_activity`, and `garmin_delete_activity` are all present
