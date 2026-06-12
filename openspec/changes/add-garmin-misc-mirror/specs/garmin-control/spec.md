# garmin-control Specification (delta)

## ADDED Requirements

### Requirement: Reading an activity's linked gear passes through the bridge

The system SHALL expose `GET /garmin/activity/{activity_id}/gear` that forwards to the bridge's `get_activity_gear` read and returns the bridge's response verbatim — the gear (shoes/bike) Garmin associates with that activity. It SHALL require authentication and return `503 garmin_disabled` when `GARMIN_BRIDGE_URL` is unset.

#### Scenario: Activity gear passes through

- **WHEN** an authenticated client `GET`s `/garmin/activity/{activity_id}/gear`
- **THEN** the backend forwards to the bridge's activity-gear read and returns its response verbatim

#### Scenario: Disabled when the bridge URL is unset

- **WHEN** `GARMIN_BRIDGE_URL` is unset and the endpoint is called
- **THEN** the response is `503 garmin_disabled`

### Requirement: Downloading a structured workout returns its FIT blob through the bridge

The system SHALL expose `GET /garmin/workout/{garmin_workout_id}/download` accepting an optional `format` (default `fit`) and forwarding to the bridge, which downloads the structured workout file from Garmin via `download_workout` and returns it as a base64-wrapped JSON envelope. The backend SHALL return the bridge's envelope verbatim. This is the structured-workout analogue of the activity export (`GET /garmin/activity/{id}/export`) shipped by the workout-library change. It SHALL require authentication and return `503 garmin_disabled` when the bridge URL is unset.

#### Scenario: Workout download returns the base64 blob envelope

- **WHEN** an authenticated client `GET`s `/garmin/workout/{garmin_workout_id}/download`
- **THEN** the backend forwards to the bridge's download and returns the `{garmin_workout_id, format, filename, content_base64}` envelope verbatim

#### Scenario: Format is forwarded when supplied

- **WHEN** the client supplies `?format=fit`
- **THEN** the backend forwards `fit` to the bridge and the envelope's `format` is `fit`

#### Scenario: Disabled when the bridge URL is unset

- **WHEN** `GARMIN_BRIDGE_URL` is unset and the endpoint is called
- **THEN** the response is `503 garmin_disabled`

### Requirement: Uploading a FIT activity is an opt-in write through the bridge

The system SHALL expose `POST /garmin/activity/upload` accepting a base64-wrapped FIT payload `{filename, content_base64}` and forwarding it to the bridge, which uploads the activity to Garmin via `upload_activity` and returns the created Garmin activity reference verbatim. This writes FROM the nutrition API TO Garmin and SHALL be invoked only on explicit request (no automatic push). It SHALL require authentication and return `503 garmin_disabled` when the bridge URL is unset.

#### Scenario: A FIT activity is uploaded

- **WHEN** an authenticated client `POST`s `/garmin/activity/upload` with `{"filename":"ride.fit","content_base64":"…"}`
- **THEN** the backend forwards the payload to the bridge's upload and returns the bridge's success response verbatim

#### Scenario: Disabled when the bridge URL is unset

- **WHEN** `GARMIN_BRIDGE_URL` is unset and the endpoint is called
- **THEN** the response is `503 garmin_disabled`

### Requirement: Renaming an activity is a write through the bridge

The system SHALL expose `PATCH /garmin/activity/{activity_id}` accepting `{name}` and forwarding it to the bridge, which renames the Garmin activity via `set_activity_name` and returns the bridge's response verbatim. It SHALL require authentication and return `503 garmin_disabled` when the bridge URL is unset.

#### Scenario: An activity is renamed

- **WHEN** an authenticated client `PATCH`es `/garmin/activity/{activity_id}` with `{"name":"Evening Z2 ride"}`
- **THEN** the backend forwards the new name to the bridge and returns its response verbatim

#### Scenario: Missing name is rejected

- **WHEN** the client `PATCH`es a body without `name`
- **THEN** the system returns `400 Bad Request` with `{"error":"name_required"}`

#### Scenario: Disabled when the bridge URL is unset

- **WHEN** `GARMIN_BRIDGE_URL` is unset and the endpoint is called
- **THEN** the response is `503 garmin_disabled`

### Requirement: Deleting an activity is an idempotent write through the bridge

The system SHALL expose `DELETE /garmin/activity/{activity_id}` that forwards to the bridge, which deletes the Garmin activity via `delete_activity`. Deleting an activity that Garmin reports as already absent SHALL succeed as a no-op (idempotent, 404-is-success). It SHALL require authentication and return `503 garmin_disabled` when the bridge URL is unset.

#### Scenario: An activity is deleted

- **WHEN** an authenticated client `DELETE`s `/garmin/activity/{activity_id}` for an existing activity
- **THEN** the backend calls the bridge to delete it and returns success

#### Scenario: Deleting an already-absent activity is a no-op success

- **WHEN** the target activity is already gone on Garmin's side (bridge reports it absent)
- **THEN** the response indicates success without error

#### Scenario: Disabled when the bridge URL is unset

- **WHEN** `GARMIN_BRIDGE_URL` is unset and the endpoint is called
- **THEN** the response is `503 garmin_disabled`
