## Context

The API is a single-user endurance-fuelling backend. `race-prep` today is a
**stateless** capability: `plan_carb_load` and `recommend-workout-fuel` compute
numbers from parameters and store nothing. The archived `add-race-prep-primitives`
change explicitly deferred a stored race calendar and in-event fuelling, betting
the agent would carry both. With a real race on 2026-07-24 that bet is failing in
practice: per-leg fuelling is recomputed from scratch each conversation, with no
durable, reproducible artifact to anchor adherence or rehearsal data against.

This change adds the durable substrate (`races` + ordered `race_legs`) and a
deterministic per-leg fuelling-plan computation over it. It reuses two patterns
already in the codebase: the per-capability package shape, and the **unit
isolation** rule from hydration/workout-fuel (carbs in `_g`, sodium in `_mg`,
fluid in `_ml`, never merged).

## Goals / Non-Goals

**Goals:**

- A persistent `race` + `race_legs` model the agent can create once and reuse.
- A deterministic per-leg fuelling baseline (hourly + total carbs/sodium/fluid),
  discipline- and duration-aware, computed from athlete params at read time.
- Math that is honest about its inputs: loud when a default (e.g. sweat rate) is
  substituted, so the agent knows what to adjust.
- Fit the existing repo/service/handler/MCP conventions exactly.

**Non-Goals:**

- Storing or applying the computed plan (no write-through to `workout_fuel` or
  goal overrides). Compute-on-read only.
- Race results / PRs / post-race analytics. This is a planning entity.
- Presets that expand a `race_type` into canned legs. Legs are explicit.
- Modelling weather, gut tolerance, or course profile — agent-side adjustments.

## Decisions

### 1. A new `race-fueling-plan` capability, not an extension of `race-prep`

`race-prep` is defined as *stateless computation primitives*. Adding a stored
entity and a stateful CRUD surface under it would muddy that contract. A new
capability keeps "math-only carb-load" and "persistent race + per-leg plan"
cleanly separable. They compose later (a `plan_carb_load(race_id)` integration)
without coupling now.

**Alternative considered:** fold into `race-prep`. Rejected — it would force the
race-prep spec to stop being "storage-free," contradicting its own stated scope.

### 2. Persistent `races` + child `race_legs` (reversing a prior non-goal)

```
races
  id            UUID PK
  name          TEXT NOT NULL
  race_date     DATE NOT NULL
  race_type     TEXT NULL          -- free annotation: 'sprint'|'olympic'|'70.3'|… (not validated into presets)
  location      TEXT NULL
  notes         TEXT NULL
  created_at, updated_at TIMESTAMPTZ

race_legs
  id                     UUID PK
  race_id                UUID NOT NULL REFERENCES races(id) ON DELETE CASCADE
  ordinal                INT  NOT NULL         -- 1-based order within the race
  discipline             TEXT NOT NULL CHECK (discipline IN ('swim','bike','run','transition','other'))
  distance_m             NUMERIC NULL
  expected_duration_min  INT NULL CHECK (expected_duration_min IS NULL OR expected_duration_min > 0)
  intensity              TEXT NULL             -- free annotation ('easy'|'moderate'|'hard'|'race' or a zone)
  UNIQUE(race_id, ordinal)
```

Legs are supplied on race create (nested array) and replaced wholesale on a
dedicated leg-edit path. Cascade delete keeps orphans impossible. The reversal of
the "no stored race calendar" non-goal is justified in the proposal: a single
durable race the agent reuses beats re-stating legs every turn.

**Alternative considered:** legs as a JSON column on `races`. Rejected — ordered,
queryable, individually-validated rows fit the repo/Querier pattern and let the
fuelling computation iterate cleanly; JSON would push validation into app code
and lose the `UNIQUE(race_id, ordinal)` guard.

### 3. Fuelling plan is computed on read, never stored

`GET /races/{id}/fueling-plan?body_weight_kg=&sweat_rate_ml_per_hr=` returns the
plan. Athlete params (weight, sweat rate) vary over a training block and are not
properties of the race, so storing a plan would immediately go stale. This mirrors
`plan_carb_load` (compute, optionally apply later) and the "API records primitives,
agent does synthesis" principle.

### 4. The deterministic math (the core)

Let `D = sum(expected_duration_min)` over all legs with a duration (the total
race effort). Per-leg outputs derive from `D`, the leg's discipline, and the
athlete params.

**Carbs — banded by total race duration, gated by discipline:**

```
base_carbs_g_per_hr(D):           75–150 min →  60     ≥150 min → 90     <75 min → 0
discipline_intake_factor:         swim 0.0   transition 0.0   bike 1.0   run 0.7   other 0.8
leg.carbs_g_per_hr  = round(base_carbs_g_per_hr(D) * factor)
leg.carbs_g_total   = round(leg.carbs_g_per_hr * leg.duration_hr)        (0 if no duration)
```

Rationale: 30–60 g/hr for 1–2.5 h and up to ~90 g/hr beyond is standard
(Jeukendrup / ACSM); we anchor the upper-evidence value as the *baseline target*
the agent dials down for gut tolerance. Discipline factor reflects intake
capacity: you cannot eat while swimming or in transition; solids are hard on the
run.

**Fluid — from sweat rate when supplied, else a flagged default:**

```
if sweat_rate_ml_per_hr supplied:  base_fluid = min(sweat_rate_ml_per_hr, 1000)   (cap practical absorption)
else:                              base_fluid = 600   + rationale notes "default sweat rate assumed"
leg.fluid_ml_per_hr = base_fluid * (discipline == swim|transition ? 0 : 1)
leg.fluid_ml_total  = round(leg.fluid_ml_per_hr * leg.duration_hr)
```

**Sodium — derived from fluid loss × sweat-sodium concentration:**

```
sweat_sodium_mg_per_l = 800           (mid of 0.5–1.2 g/L literature; fixed baseline)
if sweat_rate supplied:  sodium_per_hr = round(sweat_rate_ml_per_hr/1000 * 800)
else:                    sodium_per_hr = 600     + rationale notes the default
leg.sodium_mg_per_hr = sodium_per_hr * (discipline == swim|transition ? 0 : 1)
leg.sodium_mg_total  = round(leg.sodium_mg_per_hr * leg.duration_hr)
```

**Race totals:** element-wise sums of the per-leg `_total` fields, plus
`total_duration_min = D`. `body_weight_kg` is required (validated 30–200) and
carried for the agent's downstream g/kg reasoning even though the banded model
keys off duration, not weight — keeping the door open to a weight-scaled variant
without a contract change.

Legs with no `expected_duration_min` contribute 0 to every total and carry a
rationale noting "duration unknown — no fuelling computed."

### 5. Unit isolation and rounding

Output fields are `carbs_g_*`, `sodium_mg_*`, `fluid_ml_*` — three distinct unit
families, never a shared Totals struct (the hydration/workout-fuel rule). Numbers
round at the response boundary (`numfmt.Round1` for any fractional values, integer
math where possible), consistent with the rest of the API.

### 6. Validation → sentinel errors → 1:1 API error codes

`service.go` validates and returns sentinels the handler maps to codes, e.g.
`race_name_required`, `race_date_invalid`, `leg_ordinal_duplicate`,
`leg_discipline_invalid`, `body_weight_kg_out_of_range`,
`sweat_rate_out_of_range`, `race_not_found`. Same pattern as every other package.

## Risks / Trade-offs

- **Sports-science precision vs determinism.** The banded model is a defensible
  baseline, not a personalized prescription. → Mitigation: the response is loud
  that it's a baseline (per-leg `rationale`), and the agent owns adjustments. We
  optimize for reproducible anchoring, not clinical accuracy.
- **Reversing a deliberate non-goal could invite scope creep** (results, presets,
  apply). → Mitigation: those stay explicit non-goals here; the entity is
  intentionally minimal (plan only, no results).
- **Sweat-rate default silently wrong.** A default of 600 ml/hr can be far off in
  heat. → Mitigation: the plan flags every defaulted input in `rationale`, and
  `sweat_rate_ml_per_hr` is a first-class optional param so the agent supplies a
  measured value (and 6C sweat-rate-test, if built, feeds it).
- **Leg edit semantics.** Wholesale-replace of legs is simpler than per-leg PATCH
  but loses partial edits. → Mitigation: acceptable for a handful of legs; a
  per-leg surface can come later if needed.

## Migration Plan

- One append-only migration pair adds `races` + `race_legs`. Verify the next free
  migration number before committing (head ~016+; out-of-band slots have happened).
- Purely additive — no existing table or endpoint changes. Rollback = `down`
  migration drops both tables; nothing depends on them.
- Wire the new package + routes in `httpserver/server.go`; register MCP tools and
  bump the expected-tools integration list. `task swag` after handlers.

## Open Questions

- **Weight-scaled carbs?** The model bands on duration; a `g/kg/hr` variant is
  plausible. Deferred — `body_weight_kg` is already carried so adding it later is
  non-breaking.
- **Should `intensity` feed the math?** Currently a free annotation only. Could
  nudge sodium/fluid. Left out of v1 to keep the band model legible; revisit with
  real rehearsal data.
- **Transitions as legs vs gaps.** Modelled as zero-intake legs so total duration
  is honest. If the agent finds them noise, they can simply be omitted.
