# Proposal: add-plan-slot-targets

## Why

A workout template carries effort targets (pace, HR/power zone, RPE) on its
steps, so "🏃 Run — Tempo 30min @ 7:15" is fully expressible today. But the
target lives on the **template**, not the plan **slot** — so an 18-week plan
where the *same* tempo run progresses week to week (7:30 → 7:15 → 7:00) forces a
separate template per pace, multiplying the library. This change lets a plan slot
carry **per-intent target overrides** that supersede the template's targets when
the session is materialized and compiled, so one template can be reused across
the build at progressing paces (or power, or HR) — turning the plan into the
place progression actually lives.

## What Changes

- **Modified `training-plan` capability**: `plan_slots` gain an optional
  `target_overrides` — a small list of `{intent, target}` entries. At most one
  entry per intent. Each `target` reuses the **exact** `workout-templates` Target
  shape (`pace` / `hr_zone` / `power_zone` / `hr_bpm` / `power_w` / `rpe`) and the
  same validator, so no new target vocabulary is introduced.
- **Effective program** is defined as the contract every consumer reads: a
  planned workout's effective steps = its template's steps with each step's
  target **replaced** when the step's `intent` matches a slot override (e.g.
  override `interval` → only the work intervals change; warmup/cooldown stay).
  Steps without a matching override are unchanged. The override changes targets
  only — never durations or step structure.
- **A read endpoint to observe it**: `GET /workouts/{id}/program` returns the
  effective steps for a planned workout (template steps + its slot's overrides),
  so the app/agent can show "tonight: 5×3min @ 7:15" and the feature is testable
  entirely offline, before any Garmin push.
- **Slot CRUD widens**: `POST/PATCH …/slots` accept `target_overrides` and the
  nested plan `GET` returns them. PATCH replaces the override list wholesale (it
  is a small unit, like template steps).
- **MCP**: `add_plan_slot` / `patch_plan_slot` payloads widen to carry
  `target_overrides`; a new `get_workout_program` tool returns the effective
  program. Expected-tools list bumped by one.
- **Migration**: `ALTER plan_slots ADD target_overrides JSONB` (nullable;
  validated at the service layer).

## Capabilities

### New Capabilities

<!-- None. This extends the training-plan capability. -->

### Modified Capabilities

- `training-plan`: plan slots carry optional per-intent target overrides; a
  planned workout's effective program resolves template steps against them, and a
  read endpoint + MCP tool expose that resolved program.

## Impact

- **Depends on** `add-training-plan` (slots) and `add-workout-templates` (the
  `Target` type + validator). Both implemented.
- **Touches the `add-garmin-scheduling` contract**: when that change builds the
  watch-compile path, it MUST compile a planned workout's **effective** steps
  (template + slot overrides), not the raw template steps. This proposal defines
  "effective program" as that single source of truth; `add-garmin-scheduling`'s
  design note is updated to consume it. No code overlap — only the contract.
- **New code**: `plan_slots.target_overrides` (migration + types + validation +
  CRUD), the program resolver + endpoint, MCP tool; reuses the workout-templates
  `Target` validator verbatim.
- **No breaking changes**: additive; a slot with no overrides behaves exactly as
  today.
- **Deliberately out of scope**: a `pace_zone` target kind + a time-varying
  athlete threshold profile (recomputing absolute paces as fitness changes). That
  is the richer, separate model; per-slot overrides solve explicit progression
  without it. Also out of scope: overriding step *durations* (longer intervals as
  the plan progresses) — targets only.
