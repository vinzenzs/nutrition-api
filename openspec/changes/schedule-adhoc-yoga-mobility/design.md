## Context

Today scheduling a workout on the Garmin calendar requires a *planned workout row* that originated from a training-plan slot. The flow is: create plan → materialize → `POST /garmin/schedule/workout {workout_id}` → `pushOne` compiles the workout's effective program via the bridge, schedules it on the workout's date, and stores `garmin_workout_id` + `garmin_schedule_id` on the row (`internal/garmincontrol/scheduling.go:248`).

Ad-hoc yoga/mobility is not plan-bound, so the vault keeps shelling out to `garmin.py schedule-yoga`. Two things block a server-side path:

1. The shared `sport` vocabulary (`run|bike|swim|strength|other`) has no `yoga`/`mobility`, enforced by CHECK constraints on `workouts` (`migrations/012`) and `workout_templates` (`migrations/030`) and by `ValidSport`/`validSport` in the two `types.go` files. A yoga session therefore lands as `other`/`strength` and loses its real sport on the watch.
2. `garmin_schedule_workout` only accepts an existing planned-workout id; there is no way to push a bare template to a date.

Two pieces of existing machinery make the second gap much smaller than it appears:

- `EffectiveProgram` already returns the **raw template steps** when `PlanSlotID == nil` (`internal/trainingplan/service.go:463-464`), so a planned workout with a template but no slot compiles correctly with zero changes.
- `FindOpenPlanned` matches open planned workouts by **sport + date window** (`internal/workouts/repo.go:233`), so a completed Garmin yoga activity later reconciles into the ad-hoc planned row exactly like a plan-materialized one.

## Goals / Non-Goals

**Goals:**
- A coach over MCP can create a yoga/mobility template and schedule it to a date in one call, replacing `garmin.py schedule-yoga`.
- Ad-hoc sessions keep their real sport end-to-end and behave identically to planned workouts for re-push, unschedule, listing, training-context, and completion reconciliation.
- Reuse the existing `pushOne` / unschedule / reconcile paths rather than forking a parallel scheduler.

**Non-Goals:**
- Teaching the Garmin **bridge** (`garmin.py`) to accept `yoga`/`mobility` — that is a separate vault-side change tracked as a dependency. This change only ensures the backend stores and forwards the sport faithfully.
- A general "create ad-hoc planned workout (no Garmin)" primitive. The new tool is one-shot create+schedule; a pure local-create path is out of scope until a use case needs it.
- Recurring/repeating yoga scheduling. One date per call.

## Decisions

### D1: Widen the shared sport enum to add `yoga` and `mobility`
Both `workouts` and `workout_templates` reference one vocabulary. Add both values in one migration that drops and re-adds each table's `sport` CHECK constraint, plus the two `types.go` validators. Garmin treats yoga as a first-class sport; mobility has no native Garmin sport and is included because the user wants to log/schedule mobility distinctly on our side — the bridge will decide its Garmin mapping.

*Alternative considered:* a `mobility`-as-`other` alias. Rejected — it discards the real sport in our own data (summaries, training context, reconciliation by sport) just to dodge a bridge mapping; the bridge can map mobility however it likes without us lying about the sport.

### D2: One-shot `garmin_schedule_template` that creates an ad-hoc planned workout, then delegates to `pushOne`
The new handler: parse `template_id` + `date` → load template → create a planned workout row (`source=manual`, `status=planned`, `template_id` set, `plan_slot_id=NULL`, `started_at=date@00:00 local`, `ended_at` per D3) → call the **existing** `pushOne(workoutID)` → return the workout with its Garmin ids. Because `EffectiveProgram` handles the nil slot, `pushOne` needs no change.

*Alternative considered:* two-step (separate create-from-template tool + existing schedule tool). Rejected for the coach UX (two round-trips, intermediate un-scheduled state) — confirmed with the user. The one-shot still composes from the same primitives internally.

*Alternative considered:* schedule the template to Garmin **without** a local workout row. Rejected — there would be nowhere to persist `garmin_workout_id`/`garmin_schedule_id` (so no unschedule/re-push), the session would be invisible to `/workouts` and training-context, and it could not reconcile with the completed activity. The row is the feature, not overhead.

### D3: `ended_at` = `started_at` + Σ(template timed step durations), fallback +60min
Walk the template's step program (recursing repeat groups) summing time-based durations; if the total is zero (rep-based or untimed program) default to 60 minutes. This keeps `ended_at > started_at` (table CHECK) honest for energy/summary math instead of a flat guess. Confirmed with the user. The duration computation lives next to the new repo/handler code; if a reusable summed-duration helper already exists for templates it is reused, otherwise a small local walker is added.

### D4: Reuse existing unschedule + reconciliation, add nothing new there
Unschedule is `DELETE /garmin/schedule/workout/{id}` on the created workout id — already idempotent and already reaps the Garmin object. Reconciliation with the completed activity is automatic via `FindOpenPlanned` (sport+date). No new endpoints for the lifecycle tail.

### D5: New repo method, not a reuse of `UpsertPlannedFromSlot`
`UpsertPlannedFromSlot` is keyed on `plan_slot_id` via the partial-unique index and upserts. Ad-hoc rows have no slot, so a distinct `CreateAdhocPlannedFromTemplate` (plain INSERT, `plan_slot_id NULL`) keeps the two paths disjoint and avoids overloading the slot-conflict semantics.

### D6: MCP tool tiering and idempotency
`garmin_schedule_template` registers in `internal/agenttools/registry_garmin.go` as `TierWriteAuto` with idempotency-key support, matching the other Garmin write tools. The mcpserver expected-tools list and `announced_schemas.json` are updated; the integration test count is bumped.

## Risks / Trade-offs

- **Bridge rejects `yoga`/`mobility`** → The backend now stores/forwards the real sport, but until the vault bridge maps these, a push may 502. Mitigation: ship the enum + endpoint; document the bridge dependency in the proposal; the failure is a clean bridge error, not data corruption. The local row and sport are still correct and reconcile later.
- **`mobility` has no native Garmin sport** → bridge-side mapping decision (likely → yoga or generic). Out of scope here; called out as the dependency.
- **Duplicate ad-hoc rows if the coach calls twice for the same day** → no slot uniqueness to dedup on. Mitigation: idempotency-key on the tool guards accidental retries; genuine "two yoga sessions same day" is legitimately two rows (consistent with manual workouts having no implicit dedup).
- **Existing spec scenario uses `sport:"yoga"` as the invalid example** (`workouts` bulk upsert) → must be re-pointed to a still-invalid value (e.g. `"pilates"`) in the delta, or the suite asserts a now-valid sport is rejected.

## Migration Plan

1. Add the sequential `*_add_yoga_mobility_sports` migration (up: drop + re-add both CHECKs with the wider set; down: re-add the narrow set — down is safe only if no rows use the new values, acceptable for this single-user app).
2. Land code + tool + tests; run `task swag`; bump the mcpserver expected-tools count and regenerate `announced_schemas.json`.
3. Vault follow-up (separate): teach `garmin.py` to accept `yoga`/`mobility` and switch `/yoga schedule` to call `garmin_schedule_template`. Until then the endpoint works end-to-end for any sport the bridge already accepts.

Rollback: revert the code; the down migration restores the narrow CHECK (drop any yoga/mobility rows first if present).

## Open Questions

- None blocking. Bridge mapping of `mobility` is deferred to the vault change by design.
