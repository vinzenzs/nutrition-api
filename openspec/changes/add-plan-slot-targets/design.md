# Design: add-plan-slot-targets

## Context

`add-workout-templates` put effort targets on template steps (the `Target` type:
`pace` as `low/high_sec_per_km`, `hr_zone`/`power_zone` as 1–5, `hr_bpm`,
`power_w`, `rpe`). `add-training-plan` made a plan reference templates via slots
and materialize them into planned workouts. So "run @ 7:15" exists, but only as a
template property — the same tempo run at a progressing pace across the build
needs N templates. This change moves *progression* into the slot, where it
belongs, without inventing new target vocabulary.

## Goals / Non-Goals

**Goals:**
- Let a slot override a template's targets per occurrence, so one template covers
  a progressing series (7:30 → 7:15 → 7:00).
- Reuse the existing `Target` shape and validator exactly — zero new vocabulary.
- Make the resolved program observable and testable offline (no Garmin needed).
- Define the effective-program contract that the Garmin compile path consumes.

**Non-Goals:**
- No `pace_zone` target kind and no athlete threshold profile (the recompute-as-
  fitness-changes model). That is a larger separate change.
- No duration/structure overrides — targets only.
- No change to how materialize computes dates/sport/name.

## Decisions

### D1: Overrides are keyed by intent, not step index

A template has many steps, each with an `intent` (`warmup`, `active`,
`interval`, `recovery`, `rest`, `cooldown`). A slot override is a list of
`{intent, target}` with **at most one entry per intent**. At resolve time, a
step's target is replaced iff a slot override exists for that step's intent:

```
template:  [ warmup Z1,  5×(interval Z4, recovery Z1),  cooldown Z1 ]
slot override: { intent: "interval", target: pace 7:15 }
effective: [ warmup Z1,  5×(interval @7:15, recovery Z1),  cooldown Z1 ]
```

Keying by intent (not step index) is robust to template edits and matches how
progression actually reads — "the work intervals get faster, the warmup stays
easy." A simple single-step run uses `intent: "active"`. The trade-off: all steps
of the same intent get the same override target; for finer control, author
distinct templates. This is the right 90%-case model.

### D2: `target_overrides` is a validated JSONB column on `plan_slots`

```
ALTER TABLE plan_slots ADD COLUMN target_overrides JSONB NULL;
-- shape: [ { "intent": "interval", "target": { "kind": "pace", "low_sec_per_km": 435, "high_sec_per_km": 435 } }, ... ]
```

Read and written as a unit with the slot. **Validation lives in the service
layer and reuses the `workout-templates` Target validator verbatim** (positive
pace bounds, zones 1–5, `low <= high`). Additional slot-level rules: each
`intent` is a known intent constant; no duplicate intent in the list; an empty/
null list means "no overrides." Nullable column; the existing slot CRUD widens to
carry it.

### D3: "Effective program" is the contract, resolved not stored

A planned workout's **effective steps** = its template's `steps` with each step's
`target` replaced when the step's `intent` matches an entry in the workout's
slot's `target_overrides`. This is resolved on read (template + slot lookup via
the workout's `template_id` / `plan_slot_id`), **not snapshotted** onto the
workout row — consistent with `add-training-plan`'s "template is the prescription"
stance (no step storage on `workouts`). Every consumer uses effective steps:

- **Display / app / agent**: `GET /workouts/{id}/program`.
- **Garmin compile** (`add-garmin-scheduling`): builds the watch workout from the
  effective steps, not the raw template — its design note is updated to say so.

**Alternative considered — snapshot at materialize**: freeze resolved steps onto
the planned workout. Rejected: it duplicates template data, drifts when the
template or override is edited, and expands the `workouts` row with a steps blob
the capability deliberately avoids. Resolve-on-read keeps one source of truth.

### D4: The program endpoint

`GET /workouts/{id}/program` returns the workout's effective steps (and enough
metadata to render: sport, name). It is defined under the `training-plan`
capability (the resolution spans template + slot, which is plan logic) but uses
the natural `/workouts/{id}` path. It requires the workout to have a
`template_id`; a planned workout with no template (e.g. a manually-created
planned row) returns its bare metadata with no steps, not an error. This endpoint
is what makes the override observable and the whole change testable without
Garmin.

### D5: PATCH replaces the override list wholesale

Like template `steps`, `target_overrides` is a small unit; a `PATCH …/slots/{id}`
that supplies `target_overrides` replaces the whole list, and omitting it leaves
it unchanged. Supplying `[]` clears all overrides. No per-entry patching.

## Risks / Trade-offs

- **Same-intent steps share an override.** A template with two semantically
  different `active` steps can't give them different paces via one slot. Mitigated
  by authoring distinct templates for that rare case; the intent model covers the
  common progression shape.
- **Resolve-on-read cost.** `GET …/program` and the Garmin compile do a template
  + slot lookup per workout. Negligible at single-user scale; both are already
  fetching the template.
- **Contract coupling with `add-garmin-scheduling`.** That change must read
  effective steps. Since it is still a draft proposal, the coupling is a one-line
  design update there, not rework.

## Migration Plan

`ALTER plan_slots ADD target_overrides JSONB NULL` — additive, no backfill (NULL
= no overrides = today's behavior). Down migration drops the column. Verify the
migration head before scaffolding (training-plan's `031` is the current head on
disk; expect `032`, but `add-garmin-scheduling` may also claim a slot — confirm).

## Open Questions

- Should `GET /workouts/{id}/program` also return the *materialized* date/time
  window (it's on the workout row already) for a one-call "tonight's session"
  view? Leaning yes — cheap, and the app wants it.
- A future `pace_zone` + threshold-profile model would make most overrides
  unnecessary (paces recompute from a moving threshold). Deferred deliberately;
  this change is the explicit-progression tool that is useful regardless.
