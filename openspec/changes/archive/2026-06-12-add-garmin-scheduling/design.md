# Design: add-garmin-scheduling

## Context

`garmin.py` writes to Garmin in two steps: upload a structured workout to the
Garmin library (`cmd_create_workouts`, `_make_workout_payload` /
`_make_yoga_payload`), then place it on the calendar for a date
(`_schedule_workout`), with `_delete_scheduled_workout` to remove it and
`_get_calendar` to read what is scheduled. The `schedule-week` command loops a
plan week. This change moves that capability behind the API so the agent and app
can push the plan to the watch.

The pieces already exist after the prior Garmin changes: the **bridge** owns all
Garmin I/O (`add-garmin-bridge`), the **`garmin-control`** package proxies to it
and `GARMIN_BRIDGE_URL` is configured (`add-garmin-mcp-login`), templates carry
**structured steps** (`add-workout-templates`), and the plan materializes into
**planned workouts** (`add-training-plan`). This change connects them: planned
workout → compiled Garmin workout → scheduled on the calendar, with the Garmin
ids tracked on the workout row.

## Goals / Non-Goals

**Goals:**
- Push a planned workout (and a whole plan week / date range) to the Garmin
  calendar as a structured, watch-guided workout.
- Unschedule and re-push cleanly, never double-creating in the Garmin library.
- Keep the garminconnect payload shape entirely inside the bridge.

**Non-Goals:**
- No new auth identity or token handling (reuses `add-garmin-auth-token` /
  bridge credentials).
- No Garmin code in Go (the payload builder is Python, in the bridge).
- No reconciliation of completed vs planned (that is the read-sync side +
  a future change).

## Decisions

### D1: The bridge owns the step → Garmin payload translation

The backend sends the bridge a sport, a name, and **our** step model (the JSONB
from `add-workout-templates`); the bridge translates it to garminconnect's
`executableStepDTO` (endCondition time/distance/lap.button/no.end; target
heartrate/power/pace/no.target) and `repeatGroupDTO` (numberOfIterations + nested
steps), uploads it, and returns the Garmin workout id. This keeps Garmin's
churning payload schema out of Go — when Garmin changes it, the fix is a `pip`
bump and a bridge edit, consistent with the read-sync design.

> **Retrofit (add-plan-slot-targets):** the backend now sends the planned
> workout's **effective program** — its template steps with the plan slot's
> per-intent target overrides applied, via `trainingplan.Service.EffectiveProgram`
> — rather than the raw template steps. The override resolution happens before
> this step; the bridge translation below is unchanged.

```
backend (planned workout + template steps)
   └─POST /workouts──▶ bridge: build garmin payload, create in library
                              └─▶ { garmin_workout_id }
   └─POST /schedule { garmin_workout_id, date }──▶ bridge: schedule on calendar
                              └─▶ { garmin_schedule_id }
```

### D2: `workouts` tracks the Garmin ids (the modification)

```
ALTER TABLE workouts
  ADD COLUMN garmin_workout_id  TEXT NULL,
  ADD COLUMN garmin_schedule_id TEXT NULL;
```

Storing both on the workout row makes unschedule possible (delete by
`garmin_schedule_id`) and makes re-push safe: if a workout already has a
`garmin_schedule_id`, the orchestration unschedules the old entry before
scheduling the new one, so edits to the plan propagate without leaving orphans on
the calendar. These are opaque ids from Garmin — never parsed, only echoed back
to the bridge.

### D3: Orchestration endpoints in `garmin-control`

- `POST /garmin/schedule/workout {workout_id}` — load the workout (must be
  `status='planned'` with a `template_id`), load the template steps, call the
  bridge to create + schedule, store `garmin_workout_id` + `garmin_schedule_id`,
  return the updated workout. If already scheduled, unschedule first (idempotent
  re-push).
- `DELETE /garmin/schedule/workout/{workout_id}` — require a stored
  `garmin_schedule_id`, call the bridge to delete, clear both ids.
- `POST /garmin/schedule/plan {plan_id, scope}` — resolve planned workouts in
  the scope (a plan-week or date range, mirroring `materialize`) and push each
  via the single-workout path; return the per-workout results (partial success
  reported, not all-or-nothing).
- `GET /garmin/calendar?from&to` — read-through to the bridge's `GET /calendar`.

All return `503 garmin_disabled` when `GARMIN_BRIDGE_URL` is unset, matching the
login proxy. Endpoints require authentication (any first-party identity — this is
the athlete acting on their own calendar), unlike the garmin-identity-only token
endpoints.

### D4: Single-workout path is the unit; week/plan is a loop over it

`POST /garmin/schedule/plan` does not get its own bridge call — it loops the
single-workout orchestration so there is one code path for compile-schedule-track
and one place to get idempotency and error handling right. Per-item failures are
collected and returned rather than aborting the batch (one bad template should
not block the rest of the week).

### D5: `create-workouts` (library upload without scheduling) is folded in

`garmin.py`'s standalone `create-workouts` uploads templates to the library
without dating them. In practice every scheduled workout already creates a
library entry (D1), so a separate "upload only" path is not needed for the plan
flow. If pure library population is ever wanted, it is the `POST /workouts`
bridge endpoint called without the following `/schedule` — exposed later if a use
case appears. Not in scope now to keep the surface tight.

## Risks / Trade-offs

- **Garmin payload fragility**: the structured-workout API is the most complex
  part of garminconnect. Isolating it in the bridge (D1) contains the blast
  radius to Python, but a Garmin change can still break scheduling until the
  bridge is updated. Acceptable and consistent with the read-sync stance.
- **Orphaned calendar entries**: if the backend stores a `garmin_schedule_id`
  but a later unschedule fails at the bridge, the calendar entry lingers.
  Mitigation: `GET /garmin/calendar` lets the agent reconcile; unschedule is
  idempotent (deleting an already-gone id is a no-op success).
- **Re-push churn**: re-pushing an edited week unschedules and reschedules each
  item, creating new Garmin ids each time. Fine for a single user; documented.

## Migration Plan

`032_add_workout_garmin_schedule_ids`: `ALTER workouts` to add the two nullable
text columns. Additive, no backfill. Down migration drops them.

## Open Questions

- Should scheduling be restricted to a specific identity (e.g. agent-only)?
  Current call: any authenticated first-party identity, since both the app and
  the coach agent legitimately schedule. Revisit if that proves too broad.
