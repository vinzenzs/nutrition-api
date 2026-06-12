## Why

Pushing the plan to the watch leaks Garmin workout objects. Every
`garmin_schedule_workout` calls the bridge's `create_workout` to build a fresh
structured workout in the Garmin library, and NOTHING ever deletes it.
Re-pushing a planned workout (an edited target, a moved date) creates a *new*
library object and unschedules only the old *calendar entry* — the prior
**workout object** is orphaned in the library, and unscheduling a workout
removes the calendar entry but leaves the object behind entirely. Over an
18-week plan with normal edits, the athlete's Garmin workout library fills with
dozens of dead duplicates. The stored `garmin_workout_id` (migration 032,
`workouts.garmin_workout_id`) is the handle that lets us delete the right one —
we just never use it on the delete side. **Fixing this orphan bug is the
highest-value part of this change and we lead with it.**

Alongside the fix, the "mirror everything" Garmin arc set out to bring Garmin's
whole surface under our control plane. Two of those capabilities do not fit the
sync-into-tables model the other arc changes (A/B/C/D) use — they are **write
and blob operations, not periodic reads**, so we expose them deliberately as
**MCP tools** rather than as sync mappings:

- **Hydration write-back** (`add_hydration_data`) pushes the user's *logged*
  hydration FROM us TO Garmin. It is the only write-from-us-to-Garmin data path
  in the entire integration and is explicitly **opt-in** (the agent or user
  triggers it; nothing pushes automatically).
- **FIT/GPX export** (`download_activity`) pulls an activity's binary file out of
  Garmin. It is a blob, not a row — there is no table to sync it into — so it
  lives as an on-demand export tool.

To reconcile "what's actually on the watch" against "what our plan thinks is
there," we also add a **read of the Garmin workout library** (`get_workouts` /
`get_workout_by_id`), surfaced as `garmin_list_workouts` / `garmin_get_workout`.

This is change **E** of the arc — the control-plane / write-and-blob sibling.
Siblings (out of scope here): A `add-garmin-daily-energy`, B
`add-garmin-workout-detail`, C `extend-recovery-fitness`, D
`add-garmin-gear-and-prs`. **No migration** is needed — every operation here acts
on Garmin's side using ids already stored by migration 032.

## What Changes

- **Fix the orphan bug (lead item).** Add a bridge `DELETE /workouts` operation
  that calls garminconnect to delete a Garmin **workout object** by id, and a
  backend control endpoint `DELETE /garmin/workout/{workout_id}` that looks up
  the stored `garmin_workout_id` and removes it. Wire it into the two paths that
  currently leak:
  - **Unschedule** (`DELETE /garmin/schedule/workout/{id}` →
    `garmin_unschedule_workout`) now ALSO deletes the workout object after
    removing the calendar entry, then clears both ids — closing the leak on the
    unschedule path. (This is a behaviour change to the existing unschedule
    requirement, MODIFIED below.)
  - **Re-push** (`pushOne`) now deletes the prior workout object (in addition to
    unscheduling the prior calendar entry) before creating the new one — closing
    the leak on the re-push path. (Behaviour change to the existing schedule
    requirement, MODIFIED below.)
  - A standalone `garmin_delete_workout` MCP tool lets the agent reap a specific
    orphan it finds during reconciliation.
- **Read the Garmin workout library.** Bridge `GET /workouts` (list) and
  `GET /workouts/{id}` (one), surfaced through control endpoints
  `GET /garmin/workouts` and `GET /garmin/workout/{garmin_workout_id}`, and MCP
  read tools `garmin_list_workouts` / `garmin_get_workout` for reconciling the
  library against the plan.
- **Hydration write-back (opt-in).** Bridge `POST /hydration` calling
  garminconnect `add_hydration_data(value_ml, date)`; backend control endpoint
  `POST /garmin/hydration`; MCP write tool `garmin_push_hydration`.
- **FIT/GPX export.** Bridge `GET /activity/{activity_id}/export` calling
  garminconnect `download_activity(activity_id, fmt)`; backend control endpoint
  `GET /garmin/activity/{activity_id}/export`; MCP tool
  `garmin_export_activity` returning the blob (base64-wrapped JSON — transport
  decided in design). Upload is **out of scope**.
- **MCP integration expected-tools list** grows by **five** new tools
  (`garmin_delete_workout`, `garmin_list_workouts`, `garmin_get_workout`,
  `garmin_push_hydration`, `garmin_export_activity`).

## Capabilities

### New Capabilities
<!-- None — all four features extend the existing garmin-control / garmin-bridge / mcp-server capabilities. -->

### Modified Capabilities
- `garmin-control`: new control endpoints (delete/list/get Garmin workout
  object, push hydration, export activity); the unschedule and schedule
  requirements MODIFIED so they also delete the orphaned workout object.
- `garmin-bridge`: new bridge operations (delete workout object, list/get
  library, add hydration, export activity).
- `mcp-server`: five new tools, expected-tools list bumped; the existing
  unschedule scheduling-tools requirement MODIFIED to note that unschedule now
  also reaps the workout object.

## Impact

- **Schema**: **NO migration.** Every operation acts on Garmin via ids already
  stored by migration 032 (`workouts.garmin_workout_id`,
  `workouts.garmin_schedule_id`). Confirmed in design.md.
- **Code**: `internal/garmincontrol/` (new handlers + a `bridgeDeleteWorkout`/
  `bridgeListWorkouts`/`bridgeAddHydration`/`bridgeExportActivity` client, and
  the orphan-reap wired into `pushOne` + `unscheduleWorkout`);
  `internal/mcpserver/tools_garmin.go` (five tools) + `mcp_integration_test.go`
  expected list; `apps/garmin-bridge/garmin_bridge/{app,garmin_client}.py` (four
  bridge ops). `internal/httpserver` wiring unchanged (same package registers
  the routes).
- **Docs/tests**: `task swag` after the new handler annotations;
  `garmincontrol` handler tests (delete-reaps-object on unschedule/re-push,
  disabled-bridge 503, idempotent already-gone 404→success); bridge unit tests
  for the four new client functions; MCP integration expected-tools list +5.
- **Conventions honored**: MCP mirrors control 1:1 (one HTTP call per tool,
  verbatim body via `toToolResult`); `503 garmin_disabled` when the bridge URL
  is unset; idempotent delete/unschedule (an already-gone Garmin object is
  success); write tools auto-derive an idempotency key via
  `effectiveIdempotencyKey`.
