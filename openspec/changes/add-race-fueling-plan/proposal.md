## Why

Today the API has **no durable concept of a race** and **no per-leg fuelling
math**. `race-prep` is stateless: `plan_carb_load` takes a date + body weight
and returns a carb-load schedule, deliberately leaving both a stored race
calendar and in-event fuelling to the agent. That bet is now costing us: with a
real race on **2026-07-24**, the agent reconstructs the same per-leg fuelling
plan ("60–90 g carbs/hr on the bike, 300–800 mg sodium/hr scaled to sweat
rate, nothing solid on the swim") from scratch every conversation — lossy,
non-reproducible, and exactly the kind of evidence-based arithmetic an LLM
should anchor on a deterministic primitive rather than re-derive.

This change introduces a persistent **race** entity with ordered **legs**, and
a deterministic **per-leg fuelling-plan** computation over it. It knowingly
**reverses two non-goals** the archived `add-race-prep-primitives` change
recorded — "stored race calendar" and "race-day in-event fuelling plans" — and
justifies the reversal: the *baseline* hourly carb / sodium / fluid targets are
deterministic given leg duration, discipline, intensity, body weight and sweat
rate; the context-dependent parts the original change worried about (weather,
gut tolerance, course profile) remain **adjustments the agent layers on top of
the computed baseline**, not reasons to withhold the baseline.

## What Changes

- **New `races` entity** (persistent): `{name, race_date, race_type?, location?,
  notes?}` with full CRUD. A race owns an ordered list of legs.
- **New `race_legs`**: ordered `{ordinal, discipline (swim|bike|run|transition|
  other), distance_m?, expected_duration_min?, intensity?}` rows, cascade-deleted
  with the race. Set on race create and editable.
- **Per-leg fuelling-plan computation** — `GET /races/{id}/fueling-plan` takes
  athlete params (`body_weight_kg` required, `sweat_rate_ml_per_hr?`) and returns
  per-leg targets (`carbs_g_per_hr`, `carbs_g_total`, `sodium_mg_per_hr`,
  `sodium_mg_total`, `fluid_ml_per_hr`, `fluid_ml_total`, `rationale`) plus a
  race-level total. **Computed on read, not stored** — same "API records
  primitives, agent does synthesis" stance as carb-load.
- **Discipline-aware math**: swim and transition legs get ~zero intake; bike is
  the primary fuelling window; run reduces solids. Hourly rates band by leg
  duration (evidence-based: ~30–60 g/hr for 1–2.5 h efforts, 60–90 g/hr beyond)
  and scale sodium/fluid by sweat rate when supplied.
- **Unit isolation preserved**: carbs in `_g`, sodium in `_mg`, fluid in `_ml`
  as distinct named fields (mirrors `workout-fuel`); never merged into a shared
  Totals struct.
- **MCP tools** mirroring the REST surface 1:1: `create_race`, `list_races`,
  `get_race`, `update_race`, `delete_race`, `plan_race_fueling`.

## Capabilities

### New Capabilities

- `race-fueling-plan`: A persistent race calendar (race + ordered legs) and a
  deterministic per-leg in-event fuelling-plan computation over it (hourly and
  total carbs/sodium/fluid per leg, discipline- and duration-aware), with
  athlete params supplied at read time. Distinct from the stateless `race-prep`
  carb-load math, which it complements (a future change may let `plan_carb_load`
  take a `race_id`; out of scope here).

### Modified Capabilities

- `mcp-server`: Adds requirements covering the six new race tools and bumps the
  expected-tools list in the integration test. Existing tool requirements are
  unchanged.

## Impact

- **New package** `internal/races/` following the standard per-capability shape
  (`types.go` / `repo.go` / `service.go` / `handlers.go` + per-handler tests).
  The fuelling math lives in a pure `fueling.go` (validation + sentinel errors
  mapping 1:1 to API error codes), so it is unit-testable without the DB.
- **One migration pair** (`NNN_add_races.up/down.sql`) — `races` + `race_legs`
  tables, cascade FK, `UNIQUE(race_id, ordinal)`. Append-only; verify the next
  free number before committing (current head ~016+).
- **Routes** registered in `internal/httpserver/server.go`: `POST/GET/PATCH/
  DELETE /races`, `GET /races/{id}`, `GET /races/{id}/fueling-plan`.
- **MCP**: `internal/mcpserver/` gains `registerRaceTools`; six tools added to
  the integration test's expected-tools list (each issues exactly one HTTP call;
  write tools auto-derive an idempotency key).
- **Idempotency** applies to the write endpoints per the existing middleware
  (POST gets a key; PATCH/DELETE per current conventions). `GET …/fueling-plan`
  is pure-compute, no idempotency.
- **Docs**: `task swag` after handler changes; README MCP table gains the six
  rows; RUN_LOCAL.md gains a short race + fuelling-plan walkthrough.

### Out of scope (explicit non-goals)

- **Stored/applied fuelling plan.** The plan is computed on read; we do not write
  `workout_fuel` targets or goal overrides from it. A future `apply` (analogous
  to carb-load apply) can revisit.
- **`plan_carb_load` taking a `race_id`.** Nice integration, separate change —
  keeps this one focused on the entity + per-leg plan.
- **Race-type presets** (`sprint`/`70.3` → curated leg sets). `race_type` is a
  free annotation; legs are supplied explicitly, same reasoning the carb-load
  change used to reject presets.
- **Cross-race analytics / PRs / results capture.** This is a forward-looking
  fuelling-planning entity, not a results log. Revisit if real demand surfaces.
- **Weather / gut-tolerance / course-profile adjustments.** Those stay in the
  agent's reasoning, layered on the computed baseline.
