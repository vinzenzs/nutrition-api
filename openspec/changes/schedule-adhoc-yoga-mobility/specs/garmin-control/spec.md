## ADDED Requirements

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
