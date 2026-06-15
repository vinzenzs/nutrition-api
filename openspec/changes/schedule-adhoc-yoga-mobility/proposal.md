## Why

The vault's `/yoga schedule` still shells out to `garmin.py schedule-yoga` because the API has no server-side path for it: the sport vocabulary lacks `yoga`/`mobility`, so a yoga session lands as `other`/`strength` on the watch, and the only way to put a workout on the Garmin calendar (`garmin_schedule_workout`) requires a plan-materialized workout row — ad-hoc mobility/yoga is never plan-bound. Moving this into the API lets the coach create and schedule standalone yoga/mobility sessions over MCP, with the same tracking, re-push, and reconciliation the planned path already has.

## What Changes

- Add `yoga` and `mobility` to the workout **sport** vocabulary, shared by both `workouts` and `workout-templates` (CHECK constraints, validation, swag docs). A yoga template now keeps its real sport end-to-end.
  - **BREAKING (spec scenarios only):** the `workouts` bulk-upsert scenario currently uses `sport: "yoga"` as its *invalid-sport* example; it must switch to a still-invalid value. No runtime API contract breaks — this only widens an enum.
- Add a one-shot **`POST /garmin/schedule/template`** endpoint + **`garmin_schedule_template`** MCP tool (`TierWriteAuto`, idempotency-key aware) taking `template_id` + `date`. It creates an ad-hoc planned workout (source `manual`, status `planned`, `template_id` set, `plan_slot_id` NULL, `started_at` = the date, `ended_at` = date + the sum of the template's timed step durations, fallback +60min), then runs the existing compile/schedule/track path and returns the created workout with its Garmin ids.
- Ad-hoc scheduled sessions reuse the existing surfaces unchanged: unschedule via `DELETE /garmin/schedule/workout/{id}` (`garmin_unschedule_workout`), and they reconcile with the completed Garmin activity through the existing `FindOpenPlanned` sport+date match.

## Capabilities

### New Capabilities
<!-- none -->

### Modified Capabilities
- `workouts`: the `sport` enum gains `yoga` and `mobility` (table CHECK + POST/bulk validation); the bulk-upsert invalid-sport scenario is re-pointed off `yoga`.
- `workout-templates`: the `sport` enum (reusing the workouts vocabulary) gains `yoga` and `mobility`.
- `garmin-control`: new requirement for scheduling a standalone template to a date via a one-shot endpoint + MCP tool that creates an ad-hoc planned workout and pushes it through the existing compile/schedule/track machinery.

## Impact

- **Migration:** new sequential migration altering the `sport` CHECK on `workouts` and `workout_templates` to add `yoga`, `mobility` (up + down).
- **Code:** `internal/workouts/types.go` and `internal/workouttemplates/types.go` (constants + `ValidSport`/`validSport`); `internal/workouts/repo.go` (new method to create an ad-hoc planned workout from a template); `internal/garmincontrol` (new handler + orchestration reusing `pushOne`); `internal/agenttools/registry_garmin.go` (new tool); route wiring in `internal/httpserver/server.go`.
- **MCP:** `internal/mcpserver` expected-tools list bumped; `internal/mcpserver/testdata/announced_schemas.json` regenerated for the new tool.
- **Docs:** `task swag` after handler changes.
- **External dependency (out of scope):** the backend forwards `sport` verbatim to the Garmin bridge's `/workouts`; the bridge (vault-side `garmin.py`) must accept `yoga`/`mobility` and map them to the right Garmin sport (`yoga` is native on Garmin; `mobility` likely maps to yoga/other bridge-side). Tracked as a follow-up on the bridge, not this change.
