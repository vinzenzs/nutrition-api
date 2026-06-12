## Context

The garmin-control package proxies the bridge from the backend's authenticated
REST surface; the bridge (`apps/garmin-bridge`) is the only place garminconnect
is touched. Today the push path (`pushOne` in
`internal/garmincontrol/scheduling.go`) calls the bridge to **create** a
structured workout object (`POST /workouts` → `gc.create_workout`), schedule it
(`POST /schedule`), and stores `garmin_workout_id` + `garmin_schedule_id` on the
workout row (migration 032). The unschedule and re-push paths remove the
**calendar entry** (`DELETE /schedule`) but never the **workout object** — so
every re-push and every unschedule orphans the prior object in the Garmin
library. The id needed to delete it is already persisted; we simply never issue
the delete.

This change adds the missing delete, plus three read/write/blob capabilities the
arc deferred to MCP tools because they don't fit the sync-into-tables model.

Constraints that shape the design:
- **MCP mirrors REST/control 1:1** — each MCP tool issues exactly one HTTP call
  and forwards the body verbatim via `toToolResult`.
- **`503 garmin_disabled`** when `GARMIN_BRIDGE_URL` is unset — every control
  endpoint short-circuits, mirrored by the MCP tool result.
- **Idempotent delete/unschedule** — a Garmin object that is already gone (404)
  is success, matching `unschedule_workout`'s existing 404→no-op behaviour.
- **The bridge holds Garmin logic; the backend forwards bytes.** New garminconnect
  calls live only in `garmin_client.py`; control handlers stay thin.
- **The bridge returns `409 login_required`** (not 503) when no token is stored —
  that is the bridge's contract; the backend forwards the bridge status verbatim.

## Goals / Non-Goals

**Goals:**
- Close the workout-object leak on BOTH the unschedule and re-push paths, and
  expose a standalone delete for reconciliation cleanup.
- Read the Garmin workout library (list + by-id) for plan↔watch reconciliation.
- Push logged hydration back to Garmin, opt-in, as a write tool.
- Export an activity's FIT/GPX blob on demand.

**Non-Goals:**
- **No `upload_activity`.** Pushing a FIT file up to Garmin is out of scope; only
  download/export is in. (Flagged in the prompt; kept out to bound the change.)
- **No new sync mapping.** None of these four features run on the daily
  `POST /sync` path — they are all explicit, on-demand control/MCP calls.
- **No migration / no new columns** (see Decision D1).
- **No change to `workout_builder`** or the structured-workout create/compile
  path — this change only adds delete + read + write-back + export.

## Decisions

### D1 — No migration; operate on already-stored Garmin ids
Migration 032 already persists `garmin_workout_id` and `garmin_schedule_id` on
the `workouts` row. Deleting a Garmin workout object needs only
`garmin_workout_id`; listing the library and exporting an activity need no local
state at all (the library list comes from Garmin; the activity id is supplied by
the caller or read from a workout's `external_id`). Hydration write-back reads
the user's logged hydration via the existing REST API and posts a value+date to
Garmin — again no new column. **Therefore this change ships with NO migration.**
*Alternative considered:* a `garmin_deleted_at` audit column on workouts —
rejected; reaping an orphan is a Garmin-side side effect, not a state we need to
query, and clearing `garmin_workout_id` to NULL already records "no live object."

### D2 — Orphan reap wired into unschedule AND re-push
The leak has two sources, so the fix has two wiring points:
- **Unschedule** (`unscheduleWorkout`): after `bridgeUnschedule(scheduleID)`
  succeeds, also call `bridgeDeleteWorkout(*w.GarminWorkoutID)` when a
  `garmin_workout_id` is stored, THEN clear both ids. A workout with a schedule
  id but no workout id (shouldn't happen, but defensively) just skips the object
  delete. An already-absent object (bridge 404→no-op) is success.
- **Re-push** (`pushOne`): before creating the new object, when the workout
  already carries a `garmin_workout_id`, delete the prior object (in addition to
  the existing prior-calendar-entry unschedule). Order: unschedule old entry →
  delete old object → create new object → schedule new entry → store new ids.
Both reuse one `bridgeDeleteWorkout` client method. The standalone
`DELETE /garmin/workout/{workout_id}` endpoint (and `garmin_delete_workout` tool)
exposes the same reap for a workout the agent identifies during reconciliation;
it looks up the row, deletes the stored object, and clears `garmin_workout_id`
(leaving `garmin_schedule_id` untouched — deleting the object does not unschedule
a still-present calendar entry, so the endpoint documents that unschedule is the
fuller operation).
*Alternative considered:* delete the object inside the bridge's `unschedule`
call — rejected; the bridge's `DELETE /schedule` takes a *schedule id*, not a
workout id, and the two Garmin services are distinct. Keeping them separate
backend calls matches the existing one-bridge-call-per-operation shape.

### D3 — Idempotent delete: a 404 from Garmin is success
`bridgeDeleteWorkout` mirrors `unschedule_workout`'s pattern: if garminconnect
raises a 404 / "not found", the bridge treats it as a no-op success
(`{"deleted": true}` / `{"deleted": false, "already_absent": true}`). This keeps
re-push and unschedule safe to retry — a half-completed prior attempt that
already deleted the object does not fail the next one. The backend forwards the
bridge's 2xx verbatim. A genuine Garmin error (5xx, auth) still surfaces as
`502 garmin_error`.

### D4 — Library read is verbatim passthrough (like /garmin/calendar)
`GET /garmin/workouts` and `GET /garmin/workout/{garmin_workout_id}` forward to
the bridge's `GET /workouts` / `GET /workouts/{id}` and return the body verbatim,
exactly like the existing `GET /garmin/calendar` reconciliation read. The bridge
calls garminconnect `get_workouts(start, limit)` and `get_workout_by_id(id)`. The
backend interprets nothing. The MCP `garmin_list_workouts` / `garmin_get_workout`
tools are read-only (no `Idempotency-Key`).

### D5 — Hydration write-back: value+date, opt-in, idempotent-keyed write
`POST /garmin/hydration` accepts `{value_ml, date}` and forwards to the bridge's
`POST /hydration`, which calls garminconnect `add_hydration_data(value_in_ml,
date)`. It is a POST-style write, so `garmin_push_hydration` auto-derives an
idempotency key via `effectiveIdempotencyKey` (the agent may pass an explicit
one). The tool description states this is the ONLY push FROM us TO Garmin and is
opt-in — the agent invokes it deliberately (e.g. "sync today's water to my
watch"); nothing pushes automatically. The value/date come from the caller; the
agent typically reads them from our `GET /summary/hydration/daily` first, but
that is the agent's composition, not a second HTTP call inside the tool.
*Note:* Garmin's `add_hydration_data` sets/replaces the day's value rather than
appending, so re-pushing the same day is naturally idempotent on Garmin's side
too — the derived key just collapses retries before they leave the wrapper.

### D6 — FIT/GPX export transport: base64 in a JSON envelope
`download_activity(activity_id, fmt)` returns raw bytes (a FIT or GPX file).
MCP tool results are JSON text content, so the blob is **base64-encoded inside a
JSON envelope**: the bridge responds
`{"activity_id":…,"format":"fit","filename":…,"content_base64":"<…>"}`, the
control endpoint forwards it verbatim, and `garmin_export_activity` returns it
through `toToolResult` unchanged. The agent decodes/saves the base64 itself. The
`fmt` defaults to `fit`; `gpx`/`tcx`/`csv` are accepted as a passthrough string
(garminconnect maps the format). This keeps the one-HTTP-call-per-tool rule and
avoids streaming binary through the JSON-RPC stdio transport.
*Alternative considered:* return a server-side file path — rejected; the bridge
is stateless and shares no filesystem with the agent, so a path is meaningless to
the caller. Base64 is self-contained.

### D7 — Tool/endpoint count and the expected-tools bump
Five new MCP tools: `garmin_delete_workout` (write/delete),
`garmin_list_workouts` (read), `garmin_get_workout` (read),
`garmin_push_hydration` (write), `garmin_export_activity` (read/blob). The
`mcp_integration_test.go` expected-tools list (currently ends at
`garmin_list_scheduled`) gains exactly these five. The existing
`garmin_unschedule_workout` tool is NOT new — only its underlying backend
behaviour changes (it now reaps the object), so its tool entry stays.

## Risks / Trade-offs

- **Delete reaps the wrong object if `garmin_workout_id` is stale** → the id is
  written atomically with create in `pushOne` and cleared on unschedule, so it is
  always the live object or NULL; a NULL skips the delete. Low risk.
- **Garmin `add_hydration_data` semantics (set vs append)** → documented in D5 as
  set/replace; the tool description tells the agent it overwrites the day, so the
  agent passes the day's *total*, not a delta.
- **Base64 blob size for long activities** → a multi-hour FIT file can be a few
  hundred KB; base64 inflates ~33%. Within the bridge's response handling but the
  control endpoint's `maxBodyBytes` (currently 16 KB) MUST be raised on the
  export path — flagged as a task. The library list/get and hydration responses
  stay small.
- **Reap failure mid-unschedule** → if the calendar entry is removed but the
  object delete fails (Garmin 5xx), the endpoint returns `502 garmin_error` and
  does NOT clear the ids, so a retry re-attempts the object delete (the prior
  unschedule's 404 is a no-op). No partial-clear that would strand the object id.

## Migration Plan

**None.** This change adds no migration and no schema change. It relies entirely
on `workouts.garmin_workout_id` / `workouts.garmin_schedule_id` from migration
032. If a future need for an audit column arises it would take the next free slot
in arc order (B=036, A=037, C=038, D=039 → 040), but this change deliberately
ships without one.

## Open Questions

- **Export `maxBodyBytes` ceiling** — the control proxy currently caps response
  reads at 16 KB, far too small for a FIT blob. Resolve to a per-path higher cap
  (e.g. 8 MB on the export route only) in implementation; flagged, not yet a hard
  number in the spec.
- **`get_workouts` pagination** — garminconnect's `get_workouts(start, limit)` is
  paginated; the spec leaves `start`/`limit` as optional passthrough query params
  with Garmin's defaults, deferring any "fetch-all" looping to a later change if
  the library outgrows one page.
- **Standalone delete vs unschedule overlap** — `garmin_delete_workout` deletes
  only the object (not the calendar entry); the description steers the agent to
  `garmin_unschedule_workout` for the full teardown. Confirmed intentional (D2).
