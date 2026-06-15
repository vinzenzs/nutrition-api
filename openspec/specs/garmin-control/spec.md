# garmin-control Specification

## Purpose

Expose authenticated backend proxy endpoints that drive the garmin-bridge's
interactive multi-factor login from the nutrition API's own surface. The backend
forwards login and MFA requests to the bridge verbatim, carrying no credentials
of its own (the bridge holds them in its configuration) and surfacing nothing
sensitive — so the LLM coaching agent and mobile client can trigger a Garmin
re-authentication through the same authenticated API they use for everything
else, without ever touching the Garmin password or token blob.
## Requirements
### Requirement: Backend proxy endpoints drive the bridge's interactive login

The system SHALL expose `POST /garmin/login` and `POST /garmin/login/mfa` that
forward to the garmin-bridge at `GARMIN_BRIDGE_URL`, returning the bridge's
status code and body verbatim. The endpoints SHALL add no fields and parse
nothing. `POST /garmin/login` SHALL carry no credentials (the bridge reads them
from its own configuration); `POST /garmin/login/mfa` SHALL forward the supplied
6-digit code. When `GARMIN_BRIDGE_URL` is unset, the endpoints SHALL return
`503 garmin_disabled`. The endpoints SHALL require authentication.

#### Scenario: Start login forwards to the bridge

- **WHEN** an authenticated client `POST`s `/garmin/login`
- **THEN** the backend forwards the call to the bridge's `/login`
- **AND** returns the bridge's response verbatim (e.g. `{"needs_mfa": true}`)
- **AND** sends no credentials in the forwarded request

#### Scenario: Submit MFA forwards the code

- **WHEN** an authenticated client `POST`s `/garmin/login/mfa` with `{"code":"418923"}`
- **THEN** the backend forwards the code to the bridge's `/login/mfa`
- **AND** returns the bridge's success/error response verbatim

#### Scenario: Disabled when the bridge URL is unset

- **WHEN** `GARMIN_BRIDGE_URL` is unset and either endpoint is called
- **THEN** the response is `503 garmin_disabled`

#### Scenario: The password and token never appear on this path

- **WHEN** any login proxy request or response is logged
- **THEN** no Garmin password or token blob appears (only the bridge's
  non-sensitive status; the password lives solely in the bridge's secret)

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

### Requirement: Pushing a plan scope schedules every planned workout in it

The system SHALL expose `POST /garmin/schedule/plan` accepting a plan scope (a
plan-week or a date range, mirroring materialize) and SHALL push each planned
workout in that scope through the single-workout path. Per-workout failures SHALL
be collected and returned rather than aborting the batch. It SHALL require
authentication and return `503 garmin_disabled` when the bridge URL is unset.

#### Scenario: A plan week is scheduled in one call

- **WHEN** an authenticated client `POST`s `/garmin/schedule/plan` for a plan week
  containing several planned workouts
- **THEN** each planned workout in that week is scheduled on the watch
- **AND** the response reports per-workout results

#### Scenario: One bad item does not abort the batch

- **WHEN** one workout in the scope fails to compile or schedule
- **THEN** the others are still scheduled
- **AND** the response reports the failure alongside the successes

### Requirement: The backend reads the Garmin calendar through the bridge

The system SHALL expose `GET /garmin/calendar` accepting a date range and
returning the bridge's calendar response verbatim, for reconciliation. It SHALL
require authentication and return `503 garmin_disabled` when the bridge URL is
unset.

#### Scenario: Calendar read passes through

- **WHEN** an authenticated client `GET`s `/garmin/calendar` with a date range
- **THEN** the backend forwards to the bridge's `GET /calendar` and returns its response verbatim

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

### Requirement: Backend proxy triggers a history backfill on the bridge

The system SHALL expose `POST /garmin/backfill` that forwards a `{from, to}` body to the garmin-bridge's `POST /sync/backfill` at `GARMIN_BRIDGE_URL`, returning the bridge's status code and body verbatim. The endpoint SHALL add no fields and parse nothing beyond passing the body through. When `GARMIN_BRIDGE_URL` is unset, the endpoint SHALL return `503 garmin_disabled`. The endpoint SHALL require authentication. Because a paced backfill can run longer than an interactive call, the proxy SHALL apply a timeout sufficient to cover a capped backfill rather than the short interactive-login timeout.

#### Scenario: Backfill trigger forwards to the bridge

- **WHEN** an authenticated client `POST`s `/garmin/backfill` with `{"from":"2026-03-01","to":"2026-04-30"}`
- **THEN** the backend forwards the body to the bridge's `POST /sync/backfill`
- **AND** returns the bridge's response verbatim (including the per-day summary and `days_total`/`days_ok`/`days_failed` roll-up)

#### Scenario: Partial-failure status is passed through

- **WHEN** the bridge completes the range with one or more failed days and returns `207`
- **THEN** the backend returns `207` with the bridge's body unchanged
- **AND** adds no interpretation of its own

#### Scenario: Disabled when the bridge URL is unset

- **WHEN** `GARMIN_BRIDGE_URL` is unset and `POST /garmin/backfill` is called
- **THEN** the response is `503 garmin_disabled`

#### Scenario: Unauthenticated callers are rejected

- **WHEN** an unauthenticated client calls `POST /garmin/backfill`
- **THEN** the request is rejected by the auth middleware before any forward to the bridge

### Requirement: Scheduling a standalone template to a date creates and pushes an ad-hoc workout

The system SHALL expose `POST /garmin/schedule/template` accepting a `template_id` and a `date` (YYYY-MM-DD). The endpoint SHALL create an ad-hoc planned workout from the template — `source` = `manual`, `status` = `planned`, `template_id` set, `plan_slot_id` = NULL, `started_at` = the supplied date, and `ended_at` = `started_at` plus the sum of the template's timed step durations (falling back to 60 minutes when the program has no time-based durations) — and then compile, schedule, and track it on the Garmin watch through the same path used for planned workouts (the bridge compile of the template's steps, calendar scheduling on the date, and persistence of the returned `garmin_workout_id` and `garmin_schedule_id` onto the new workout row). The created workout SHALL be returned. Because the ad-hoc row carries no `plan_slot_id`, the compiled program is the raw template steps with no slot overrides. The endpoint SHALL be disabled (`503 garmin_disabled`) when the bridge URL is unset, and SHALL surface bridge failures as `502`. This is the server-side replacement for the vault's `garmin.py schedule-yoga`, and works for any sport the bridge accepts (notably `yoga` and `mobility`).

#### Scenario: A template is scheduled to a date in one call

- **WHEN** an authenticated client `POST`s `/garmin/schedule/template` with `{"template_id":"<uuid>","date":"2026-06-20"}` for an existing template
- **THEN** the backend creates a planned workout with `source=manual`, `status=planned`, `template_id` set, `plan_slot_id=null`, `started_at` on `2026-06-20`, and `ended_at` after `started_at`
- **AND** it compiles and schedules the template via the bridge on that date
- **AND** it stores the returned `garmin_workout_id` and `garmin_schedule_id` on the new workout
- **AND** it returns the created workout

#### Scenario: ended_at follows the template's timed duration

- **WHEN** the scheduled template's steps sum to a positive timed duration
- **THEN** the created workout's `ended_at` equals `started_at` plus that summed duration
- **AND** when the template has no time-based durations (e.g. rep-based steps), `ended_at` equals `started_at` plus 60 minutes

#### Scenario: A yoga or mobility template keeps its real sport on the watch

- **WHEN** the scheduled template has `sport` of `yoga` or `mobility`
- **THEN** the created workout carries that sport (not coerced to `other`/`strength`)
- **AND** the sport is forwarded verbatim to the bridge's compile call

#### Scenario: An unknown template is rejected

- **WHEN** the client posts a `template_id` that does not exist
- **THEN** the response is a `404` and nothing is created or scheduled

#### Scenario: An invalid or missing date is rejected

- **WHEN** the client posts a body whose `date` is missing or not a valid `YYYY-MM-DD`
- **THEN** the response is a `400` validation error and nothing is created or scheduled

#### Scenario: Disabled when the bridge URL is unset

- **WHEN** the bridge URL is not configured
- **AND** an authenticated client posts to `/garmin/schedule/template`
- **THEN** the response is `503` with `{"error":"garmin_disabled"}`
- **AND** no workout is created

#### Scenario: The ad-hoc workout uses the existing unschedule and reconciliation paths

- **WHEN** an ad-hoc scheduled workout must be removed
- **THEN** `DELETE /garmin/schedule/workout/{id}` on the created workout id unschedules it and clears its Garmin ids (no new unschedule surface is added)
- **AND** when the completed Garmin activity for that sport and date is later ingested, it reconciles into the ad-hoc planned row via the existing open-planned sport+date match

