# Proposal: add-slot-duration-override

## Why

`add-plan-slot-targets` let a plan slot carry per-intent **target** overrides, so
one template can progress in pace/power/HR across an 18-week build without forking
a template per intensity. It explicitly deferred the sibling case: **duration**.
But a hand-edited training schedule progresses session *length* exactly the same
way — the same "tempo+brick" runs 75min one week and 80min the next. Today that
forces a separate template per duration (the template owns
`estimated_duration_sec` and the step durations; the slot can't touch either).
This change lets a plan slot carry **per-intent duration overrides** that
supersede the matching template steps when the session is materialized and
compiled — so one template is reused across the build at progressing durations,
and the plan becomes the place *both* axes of progression (intensity and volume)
actually live.

This is also the last server-side gap before the hand-curated `Plan.md` schedule
can be retired as the schedule's source of truth and compiled one-way into the
server `training-plan`: a table cell like `80min tempo+brick` has no lossless home
on a slot until a duration override exists. (The one-time parse + data load + the
`Plan.md` → `Methodology.md` split run vault-side, out of this repo.)

## What Changes

- **Modified `training-plan` capability**: `plan_slots` gain an optional
  `duration_overrides` — a small list of `{intent, duration}` entries, at most one
  per intent. Each `duration` reuses the **exact** `workout-templates` Duration
  shape, restricted to the two bounded kinds (`{kind:"time",seconds}` with
  `seconds > 0`, `{kind:"distance",meters}` with `meters > 0`), and is validated by
  the same rules — no new duration vocabulary, and the unbounded kinds
  (`lap_button`, `open`) are rejected as overrides because progression is always a
  bounded quantity.
- **Effective program extends** the contract `add-plan-slot-targets` defined: a
  planned workout's effective steps = its template's steps with each step's
  **target** replaced when a target override matches its intent (unchanged from
  today) **and** each step's **duration** replaced when a duration override matches
  its intent. The two override lists are independent and compose; step structure
  and intents are otherwise untouched.
- **Materialize honours the override**: the planned workout's session length (and
  thus the calendar block and any downstream EA/time-window math) is derived from
  the **effective** program's summed step durations when they are fully bounded,
  falling back to the template's `estimated_duration_sec`, then the one-hour
  default — so the calendar block and the watch workout agree on 80min instead of
  drifting (one says 75, the other 80).
- **Slot CRUD widens**: `POST/PATCH …/slots` accept `duration_overrides` and the
  nested plan `GET` returns them. PATCH replaces the override list wholesale
  (supplying `[]` clears it, omitting it leaves it unchanged) — identical to the
  `target_overrides` rule.
- **`GET /workouts/{id}/program`** (the effective-program read from
  `add-plan-slot-targets`) now reflects duration overrides too, so the resolved
  durations are observable and testable entirely offline, before any Garmin push.
- **MCP**: `add_plan_slot` / `patch_plan_slot` payloads widen to carry
  `duration_overrides`. No new tools — expected-tools list unchanged.
- **Migration**: `ALTER plan_slots ADD duration_overrides JSONB` (nullable;
  validated at the service layer, mirroring `target_overrides`).

## Capabilities

### New Capabilities

<!-- None. This extends the training-plan capability. -->

### Modified Capabilities

- `training-plan`: plan slots carry optional per-intent **duration** overrides; a
  planned workout's effective program resolves template step durations against
  them (alongside the existing target overrides), and materialize derives the
  session length from that effective program.

## Impact

- **Depends on** `add-plan-slot-targets` (the effective-program contract + the
  per-slot JSONB-overrides + wholesale-replace PATCH pattern this mirrors exactly)
  and `add-workout-templates` (the `Duration` type + validator). Both implemented.
- **Consumes the same `add-garmin-scheduling` contract**: the watch-compile path
  already compiles a planned workout's **effective** steps, not the raw template —
  so once duration overrides feed the effective program, the watch workout picks
  them up with no change to the compile code (only the contract widens from
  "targets replaced" to "targets and durations replaced").
- **New code**: `plan_slots.duration_overrides` (migration + types + validation +
  CRUD), extension of the effective-program resolver to apply duration overrides,
  the materialize session-length derivation; reuses the workout-templates
  `Duration` validator verbatim. `task swag` after handler/struct changes.
- **No breaking changes**: additive; a slot with no duration overrides behaves
  exactly as today (session length from `estimated_duration_sec`).
- **Deliberately out of scope**: per-slot overriding of *step structure* (adding or
  removing steps / repeat counts) — overrides touch existing steps' durations only;
  a slot whose week needs a structurally different session points at a different
  template. Also out of scope: the entire vault-side migration (parse `Plan.md`,
  one-time data load, `Plan.md` → `Methodology.md` split) and the coach-methodology
  surface (its own follow-up change).
