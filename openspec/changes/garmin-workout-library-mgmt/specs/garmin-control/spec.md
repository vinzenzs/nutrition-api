# garmin-control Specification (delta)

## ADDED Requirements

### Requirement: Deleting a workout object reaps the orphaned Garmin library entry

The system SHALL expose `DELETE /garmin/workout/{workout_id}` that loads the
workout row, reads its stored `garmin_workout_id`, calls the bridge to delete
that Garmin **workout object** from the library, and clears `garmin_workout_id`
on the row. Deleting an object that Garmin reports as already absent SHALL
succeed as a no-op (idempotent). The endpoint deletes only the library object,
not any calendar entry — `garmin_schedule_id` is left untouched and the
description SHALL point callers at unschedule for the full teardown. It SHALL
require authentication and return `503 garmin_disabled` when `GARMIN_BRIDGE_URL`
is unset.

#### Scenario: Delete reaps the stored workout object

- **WHEN** an authenticated client `DELETE`s `/garmin/workout/{id}` for a workout
  that carries a `garmin_workout_id`
- **THEN** the backend calls the bridge to delete that Garmin workout object
- **AND** clears `garmin_workout_id` on the workout row
- **AND** returns the updated workout

#### Scenario: Deleting an already-absent object is a no-op success

- **WHEN** the target object is already gone on Garmin's side (bridge reports it absent)
- **THEN** the response indicates success without error
- **AND** `garmin_workout_id` is cleared on the row

#### Scenario: Deleting with no stored workout id is a no-op success

- **WHEN** the workout has no `garmin_workout_id`
- **THEN** the response indicates nothing was deleted, without error

#### Scenario: Disabled when the bridge URL is unset

- **WHEN** `GARMIN_BRIDGE_URL` is unset and the endpoint is called
- **THEN** the response is `503 garmin_disabled`

### Requirement: The backend reads the Garmin workout library through the bridge

The system SHALL expose `GET /garmin/workouts` (the library list, accepting
optional `start`/`limit` passthrough pagination params) and
`GET /garmin/workout/{garmin_workout_id}` (one library object by its Garmin id),
each forwarding to the bridge's corresponding read and returning the bridge's
response verbatim, for reconciling the watch's library against the plan. They
SHALL require authentication and return `503 garmin_disabled` when the bridge URL
is unset.

#### Scenario: Library list passes through

- **WHEN** an authenticated client `GET`s `/garmin/workouts`
- **THEN** the backend forwards to the bridge's library list and returns its response verbatim

#### Scenario: Pagination params are forwarded

- **WHEN** the client supplies `start` and `limit`
- **THEN** the backend forwards them to the bridge unchanged

#### Scenario: Single library object passes through

- **WHEN** an authenticated client `GET`s `/garmin/workout/{garmin_workout_id}`
- **THEN** the backend forwards to the bridge's by-id read and returns its response verbatim

#### Scenario: Disabled when the bridge URL is unset

- **WHEN** `GARMIN_BRIDGE_URL` is unset and either endpoint is called
- **THEN** the response is `503 garmin_disabled`

### Requirement: Pushing logged hydration back to Garmin is an opt-in write

The system SHALL expose `POST /garmin/hydration` accepting `{value_ml, date}` and
forwarding it to the bridge, which records the value against that date in Garmin.
This is the only data path that writes FROM the nutrition API TO Garmin and SHALL
be invoked only on explicit request (no automatic push). It SHALL require
authentication and return `503 garmin_disabled` when the bridge URL is unset.

#### Scenario: Hydration value is pushed for a date

- **WHEN** an authenticated client `POST`s `/garmin/hydration` with `{"value_ml":2400,"date":"2026-06-12"}`
- **THEN** the backend forwards the value and date to the bridge
- **AND** returns the bridge's success response verbatim

#### Scenario: Disabled when the bridge URL is unset

- **WHEN** `GARMIN_BRIDGE_URL` is unset and the endpoint is called
- **THEN** the response is `503 garmin_disabled`

### Requirement: Exporting an activity returns its FIT/GPX blob through the bridge

The system SHALL expose `GET /garmin/activity/{activity_id}/export` accepting an
optional `format` (default `fit`; `gpx`/`tcx`/`csv` passthrough) and forwarding to
the bridge, which downloads the activity file from Garmin and returns it as a
base64-wrapped JSON envelope. The backend SHALL return the bridge's envelope
verbatim. It SHALL require authentication and return `503 garmin_disabled` when
the bridge URL is unset.

#### Scenario: Activity export returns the base64 blob envelope

- **WHEN** an authenticated client `GET`s `/garmin/activity/{activity_id}/export`
- **THEN** the backend forwards to the bridge's download and returns the
  `{activity_id, format, filename, content_base64}` envelope verbatim

#### Scenario: Format is forwarded when supplied

- **WHEN** the client supplies `?format=gpx`
- **THEN** the backend forwards `gpx` to the bridge and the envelope's `format` is `gpx`

#### Scenario: Disabled when the bridge URL is unset

- **WHEN** `GARMIN_BRIDGE_URL` is unset and the endpoint is called
- **THEN** the response is `503 garmin_disabled`

## MODIFIED Requirements

### Requirement: Pushing a planned workout compiles, schedules, and tracks Garmin ids

The system SHALL expose `POST /garmin/schedule/workout` accepting a `workout_id`
that refers to a planned workout (`status='planned'`) with a `template_id`. It
SHALL load the template's steps, call the bridge to create the structured Garmin
workout and schedule it on the workout's date, and persist the returned
`garmin_workout_id` and `garmin_schedule_id` onto the workout row. When the
workout already carries a `garmin_schedule_id`, the system SHALL unschedule the
prior calendar entry before scheduling the new one; and when it already carries a
`garmin_workout_id`, the system SHALL delete the prior Garmin **workout object**
from the library before creating the new one — so a re-push leaves no orphan
either on the calendar OR in the workout library. A prior object that Garmin
reports as already absent SHALL be treated as a no-op (the re-push still
proceeds). The endpoint SHALL require authentication and SHALL return
`503 garmin_disabled` when `GARMIN_BRIDGE_URL` is unset.

#### Scenario: A planned workout is pushed to the watch

- **WHEN** an authenticated client `POST`s `/garmin/schedule/workout` with a
  planned workout's id
- **THEN** the backend compiles and schedules it via the bridge
- **AND** stores the returned `garmin_workout_id` and `garmin_schedule_id` on the workout
- **AND** returns the updated workout

#### Scenario: Re-pushing replaces the prior calendar entry and the prior object

- **WHEN** a workout that already has a `garmin_schedule_id` and a
  `garmin_workout_id` is pushed again
- **THEN** the prior scheduled entry is unscheduled
- **AND** the prior Garmin workout object is deleted from the library before the new one is created
- **AND** the workout's ids are updated to the new entry's ids (no orphan left behind)

#### Scenario: Pushing a non-planned workout is rejected

- **WHEN** the target workout is not `status='planned'` or has no `template_id`
- **THEN** the response is a validation error and nothing is scheduled

#### Scenario: Disabled when the bridge URL is unset

- **WHEN** `GARMIN_BRIDGE_URL` is unset and the endpoint is called
- **THEN** the response is `503 garmin_disabled`

### Requirement: Unscheduling a workout clears its Garmin link

The system SHALL expose `DELETE /garmin/schedule/workout/{workout_id}` that
requires a stored `garmin_schedule_id`, calls the bridge to remove the scheduled
calendar entry, ALSO deletes the workout's Garmin **workout object** from the
library when a `garmin_workout_id` is stored, and clears both
`garmin_schedule_id` and `garmin_workout_id` on the workout row — so unscheduling
leaves no orphan in the library. A workout object that Garmin reports as already
absent SHALL be treated as a no-op (unschedule still succeeds and clears the
ids). It SHALL require authentication and return `503 garmin_disabled` when the
bridge URL is unset.

#### Scenario: Unschedule removes the entry, reaps the object, and clears the ids

- **WHEN** an authenticated client `DELETE`s `/garmin/schedule/workout/{id}` for a
  scheduled workout that carries both ids
- **THEN** the bridge removes the calendar entry
- **AND** the bridge deletes the Garmin workout object from the library
- **AND** the workout's `garmin_schedule_id` and `garmin_workout_id` are cleared

#### Scenario: Unscheduling an unscheduled workout is a no-op success

- **WHEN** the workout has no `garmin_schedule_id`
- **THEN** the response indicates nothing was scheduled, without error

#### Scenario: An already-absent object does not fail the unschedule

- **WHEN** the calendar entry is removed but the workout object is already gone on Garmin's side
- **THEN** the unschedule succeeds and both ids are cleared
