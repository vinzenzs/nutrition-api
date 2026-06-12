# Proposal: add-workout-reconciliation

## Why

Once the plan materializes **planned** workouts (`add-training-plan`) and the
Garmin bridge imports **completed** activities (`add-garmin-bridge`), every
training day ends up with two rows that mean the same session — a planned run
and the actual run — sitting side by side forever. The planned row never closes,
the app shows a duplicate, and "did I do my prescribed session?" can't be
answered. This change closes the loop: when a Garmin import confidently matches a
planned workout, it **fulfills** that row in place — flipping it
`planned → completed`, filling in the actuals, and keeping the prescription link
— instead of spawning a sibling. The `workouts.status` lifecycle was always meant
for exactly this.

## What Changes

- **Modified `workouts` capability**: the completed-activity ingestion path
  (`POST /workouts`, `POST /workouts/bulk`) gains a **reconciliation step**.
  When an incoming `source='garmin'` activity is first seen (its `external_id`
  is not already stored), the service tries to match an **open planned workout**
  — `status='planned'`, `external_id IS NULL`, same local calendar day, same
  sport — and:
  - **exactly one match** → **merge**: set the planned row's `external_id`,
    `source`, and actual metrics (kcal/avg_hr/tss/distance/power/etc.), flip
    `status` to `completed`, and keep `template_id` + `plan_slot_id` (the
    prescription). No new row.
  - **no match** → create a standalone completed row (today's behavior).
  - **ambiguous (more than one candidate)** → create a standalone completed row
    **and flag it** as needing manual/agent linking, rather than guess.
- **Idempotency preserved**: once merged, the row carries the `external_id`, so
  the daily re-sync follows the existing `external_id` UPSERT path and updates it
  in place — reconciliation runs only on first sight.
- **An explicit link / unlink escape hatch** for the ambiguous and mistaken
  cases: `POST /workouts/{plannedId}/fulfill {completed_id}` to merge two
  existing rows, and `POST /workouts/{id}/unfulfill` to reverse a merge (clear
  `external_id` + actuals, restore `status='planned'`).
- **MCP tools** for the manual paths: `fulfill_workout`, `unfulfill_workout`
  (the automatic merge needs no tool — it happens during sync).
- **No migration**: reuses existing columns (`status`, `external_id`,
  `template_id`, `plan_slot_id`, the metric columns). A small nullable marker for
  the "ambiguous, needs linking" flag may be added — see design.
- **Depends on** the `materialize` guard (`WHERE status='planned'`) already folded
  into `add-training-plan`, which keeps re-materialization from reverting a
  fulfilled row.

## Capabilities

### New Capabilities

<!-- None. This modifies the existing workouts ingestion behavior. -->

### Modified Capabilities

- `workouts`: the Garmin ingestion path reconciles a completed activity against
  an open planned workout (merge-on-match), plus explicit fulfill/unfulfill
  endpoints and the ambiguity flag.

## Impact

- **Depends on** `add-training-plan` (planned workouts with `plan_slot_id`/
  `template_id` to fulfill) and `add-garmin-bridge` (the importer). Lands after
  both.
- **New code**: reconciliation logic in `internal/workouts/service.go`, the
  fulfill/unfulfill handlers, MCP tools; possibly one nullable column for the
  ambiguity flag.
- **Out of scope (future)**: the *reverse* direction — materializing a plan
  matching against activities imported *before* the plan existed; matching with
  a day-of tolerance window; full plan-adherence analytics (a separate
  capability that would sit on top of this primitive).
- **No breaking changes**: additive; the no-match path is exactly today's
  behavior.
