# garmin-bridge Specification (delta)

## ADDED Requirements

### Requirement: The bridge deletes a structured workout object from the library

The bridge SHALL expose `DELETE /workouts/{garmin_workout_id}` that deletes the
named structured workout object from the athlete's Garmin workout library via
garminconnect. A workout id that Garmin reports as already absent (404 / not
found) SHALL be treated as a no-op success, so the backend's re-push and
unschedule paths can call it safely on retry. A genuine Garmin error SHALL
surface as `502 garmin_error`. This complements the existing create — the bridge
previously created library objects but never removed them, orphaning every
prior object on re-push.

#### Scenario: Deleting an existing object removes it

- **WHEN** `DELETE /workouts/{garmin_workout_id}` is called with a live object's id
- **THEN** the bridge deletes that object from the Garmin library
- **AND** responds indicating it was deleted

#### Scenario: Deleting an already-absent object is a no-op

- **WHEN** `DELETE /workouts/{garmin_workout_id}` is called with an id Garmin no longer has
- **THEN** the bridge treats the 404 as success and responds indicating the object was already absent

#### Scenario: A genuine Garmin error surfaces

- **WHEN** Garmin returns a non-404 error deleting the object
- **THEN** the bridge responds `502 garmin_error`

### Requirement: The bridge reads the Garmin workout library

The bridge SHALL expose `GET /workouts` returning the structured workouts in the
athlete's Garmin library (accepting optional `start`/`limit` pagination
passthrough) and `GET /workouts/{garmin_workout_id}` returning one library object
by id, for reconciliation by the backend. A token-less request SHALL return
`409 login_required`, matching the other token-backed bridge operations.

#### Scenario: Library list returns the stored workouts

- **WHEN** `GET /workouts` is called with a valid stored token
- **THEN** the response lists the athlete's library workouts with their Garmin ids

#### Scenario: By-id read returns one object

- **WHEN** `GET /workouts/{garmin_workout_id}` is called for an existing object
- **THEN** the response carries that workout object

#### Scenario: No stored token fails clearly

- **WHEN** either read is called and the backend has no stored token
- **THEN** the bridge returns `409 login_required` and reads nothing

### Requirement: The bridge pushes a hydration value back to Garmin

The bridge SHALL expose `POST /hydration` accepting `{value_ml, date}` and
recording that value against the date in Garmin via garminconnect
`add_hydration_data`. This is the only write FROM the nutrition system TO Garmin.
Because Garmin records a day's total (set/replace, not append), re-posting the
same date overwrites rather than accumulates. A token-less request SHALL return
`409 login_required`.

#### Scenario: Hydration value is recorded for a date

- **WHEN** `POST /hydration` is called with `{"value_ml":2400,"date":"2026-06-12"}` and a valid token
- **THEN** the bridge records 2400 ml against that date in Garmin
- **AND** responds with success

#### Scenario: Re-posting the same date overwrites

- **WHEN** `POST /hydration` is called twice for the same date with different values
- **THEN** Garmin holds the latter value for that date (set/replace, not summed)

#### Scenario: No stored token fails clearly

- **WHEN** `POST /hydration` is called and the backend has no stored token
- **THEN** the bridge returns `409 login_required` and writes nothing

### Requirement: The bridge exports an activity's FIT/GPX blob

The bridge SHALL expose `GET /activity/{activity_id}/export` accepting an optional
`format` (default `fit`; `gpx`/`tcx`/`csv` passthrough) that downloads the
activity file from Garmin via garminconnect `download_activity` and returns it as
a base64-wrapped JSON envelope `{activity_id, format, filename, content_base64}`
— never streaming raw binary, so the blob crosses the JSON control/MCP transport
intact. A token-less request SHALL return `409 login_required`.

#### Scenario: Export returns the base64 envelope

- **WHEN** `GET /activity/{activity_id}/export` is called with a valid token
- **THEN** the bridge downloads the FIT file and responds with
  `{activity_id, format, filename, content_base64}` where `content_base64` is the file's base64 encoding

#### Scenario: Format is honoured

- **WHEN** `GET /activity/{activity_id}/export?format=gpx` is called
- **THEN** the bridge downloads the GPX file and the envelope's `format` is `gpx`

#### Scenario: No stored token fails clearly

- **WHEN** `GET /activity/{activity_id}/export` is called and the backend has no stored token
- **THEN** the bridge returns `409 login_required` and exports nothing

## MODIFIED Requirements

### Requirement: The bridge schedules and unschedules workouts on the calendar

The bridge SHALL expose `POST /schedule` accepting a Garmin workout id and a
date, placing that workout on the Garmin calendar and returning the Garmin
schedule id; and `DELETE /schedule` accepting a Garmin schedule id, removing the
scheduled entry. Deleting an already-absent schedule id SHALL succeed as a no-op.
Removing the **calendar entry** does NOT delete the underlying structured
**workout object** from the library — that object is deleted via the separate
`DELETE /workouts/{garmin_workout_id}` operation, which the backend invokes on
the unschedule and re-push paths to avoid orphaning prior objects.

#### Scenario: Scheduling returns a schedule id

- **WHEN** `POST /schedule` is called with a Garmin workout id and a date
- **THEN** the workout is placed on that date and the response carries the Garmin schedule id

#### Scenario: Unscheduling is idempotent

- **WHEN** `DELETE /schedule` is called with a schedule id that is already gone
- **THEN** the response indicates success (no-op)

#### Scenario: Unscheduling leaves the library object for separate deletion

- **WHEN** `DELETE /schedule` removes a calendar entry
- **THEN** the underlying structured workout object remains in the library
- **AND** the backend removes it via `DELETE /workouts/{garmin_workout_id}`
