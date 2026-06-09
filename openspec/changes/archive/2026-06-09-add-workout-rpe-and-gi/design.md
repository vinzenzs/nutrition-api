## Context

The training cycle the API supports has matured: workouts, workout-fuel entries, hydration, training phases, and the daily-context aggregator are all in place. What's missing is the *outcome* data for fueling rehearsal: how did the workout feel, and did the fueling sit right.

These are subjective measures captured *after* the session — they describe how the strategy worked, which is the input to iterating that strategy for the next rehearsal and ultimately race day. Without them, the data layer can describe "what you took" but not "whether it worked."

The 70.3 build phase the user is heading into mandates fueling rehearsal on every long ride starting Week 9. Each rehearsal session generates: N fuel entries (already captured), one RPE for perceived effort, one GI distress score. The latter two are missing — this change adds them at the per-session grain.

## Goals / Non-Goals

**Goals:**

- Two nullable integer fields on `workouts`: `rpe` (1–10) and `gi_distress_score` (1–5). Both validated at the DB layer (CHECK) and the handler layer (explicit error codes).
- Per-session grain: one number per workout, matching how athletes natively log this data.
- Backward-compatible: existing workouts and existing API callers see no shape change unless they query for the new fields. The Garmin importer is unaffected.
- The agent reads RPE + GI alongside the existing fueling aggregation (`/workouts/{id}/fueling`) so the rehearsal data lands in the same call the agent already uses for "how did the fueling work."

**Non-Goals:**

- Per-fuel-entry GI score. Discussed in decisions below.
- A separate `effort_load` or `subjective_load` parallel field. RPE is the industry standard; one field, Borg CR-10.
- Back-filling historical workouts. NULL means "not rehearsed"; a meaningful signal.
- A v2 "rehearsal session" type/tag on workouts to distinguish rehearsal from non-rehearsal rides. The presence of `rpe` + `gi_distress_score` alone signals "this was rehearsed"; an explicit type adds a second source of truth.
- Cross-session aggregations ("average RPE across last 4 long rides"). Belongs to a future analytics capability.
- A GI ontology (cramping vs nausea vs bloating). The 1–5 score captures severity, not type. If type matters later, add a separate nullable enum; v1 keeps the shape minimal.

## Decisions

### 1. Both fields on `workouts`, not on `workout_fuel_entries`

Athletes log this data once per session, not per gel. The cognitive load of asking "what RPE for this third gel?" is wrong — RPE is a *whole-session* perceived effort number by definition (Borg CR-10). For GI distress, per-entry would let you attribute episodes to specific products, but in practice you log:

```
After the ride: "I had 3 gels, 1 bar, 2 bottles of mix. Cramped at 90min. RPE 7/10."
```

That's *one* GI signal for the session ("did I have GI distress today: yes, mild=2"), not five separate GI assessments per fuel entry. Per-product diagnostic ("the SIS gel was when it started") goes in the existing `workout_fuel_entries.note` free-text field. The agent can read those notes alongside the workout's session-level GI score.

**Alternatives considered:**

- *Per-entry GI on `workout_fuel_entries`.* Lets you attribute "SIS = GI 4, Maurten = GI 1" structurally. Rejected — premature; the user's framing is workout-level rehearsal evaluation, and per-product structure can be layered on later if the comparison pattern actually emerges from notes-field usage. The `note` field is intentionally a low-friction landing pad for that data today.
- *RPE per fuel entry.* Conceptually wrong — RPE is whole-session perceived effort.
- *Split: RPE on workouts, GI on workout_fuel_entries.* Considered. Rejected — the per-entry GI grain shifts complexity onto the user's logging flow ("which gel gets the GI score?") without a clear payoff in v1.

### 2. Integer range, not enum

`rpe INTEGER NULL CHECK (rpe IS NULL OR (rpe BETWEEN 1 AND 10))` and `gi_distress_score INTEGER NULL CHECK (...BETWEEN 1 AND 5)`.

Reasons:
- The Borg CR-10 RPE scale IS the scale — 1 through 10, integers, well-known.
- The GI severity scale is conventionally 1–5 in sports nutrition rehearsal literature.
- Integer makes range queries trivial (`WHERE rpe > 7`, `WHERE gi_distress_score >= 3`).
- A Postgres ENUM would require an `ALTER TYPE ... ADD VALUE` migration to extend, which has trans­actional problems. CHECK constraint is mutable in one migration.

Both fields use the `INTEGER NULL CHECK (col IS NULL OR (col BETWEEN x AND y))` pattern — NULL passes the CHECK, valid ranges pass, anything else fails at the DB layer. Defence-in-depth alongside the handler-level validation.

### 3. Nullable, no default

Both fields default to `NULL`. NULL means "not rehearsed / not measured" and is a meaningful signal — not every workout is a fueling rehearsal. A Z1 spin doesn't need RPE+GI; logging-friction is the enemy.

The Garmin importer (`source: garmin`) will continue to insert workouts with NULL for these fields. The user adds them manually via PATCH after the ride: `patch_workout(workout_id, rpe: 7, gi_distress_score: 2)`.

**Alternatives considered:**

- *NOT NULL with default 0 or -1.* Rejected — 0 isn't valid on either scale (would fail CHECK), and a sentinel "no value" via -1 trades NULL semantics for a magic number. NULL is the right Postgres idiom.
- *Default to 5 ("middle RPE") for missing values.* Rejected — middle-of-scale defaulting silently corrupts the analytics view.

### 4. Validation at both DB and handler layers

The DB has CHECK constraints (defence-in-depth, blocks any direct SQL writes that bypass the handler). The handler layer rejects out-of-range with explicit error codes:

- `400 rpe_invalid` with `range: {min: 1, max: 10}`
- `400 gi_distress_score_invalid` with `range: {min: 1, max: 5}`

Pattern matches `body_weight_kg_invalid` from race-prep — clients can show `range.min/max` in UI without re-encoding the rule.

### 5. JSON shape: `omitempty` on the pointer fields

```go
type Workout struct {
    ...
    RPE              *int `json:"rpe,omitempty"`
    GIDistressScore  *int `json:"gi_distress_score,omitempty"`
    ...
}
```

When NULL in DB → field absent from JSON. Matches the existing pattern on `KcalBurned`, `AvgHR`, `TSS`, `Notes`. No magic null-handling at the boundary; just consistent omitempty.

### 6. PATCH semantics: tri-state for clearing

`PATCH /workouts/:id` accepts the fields with three behaviours:

- Field **missing** from body → leave unchanged.
- Field **present with integer** → set to that value (validated).
- Field **present with `null`** → clear to NULL.

The third case (clear) lets the user retract a logged value ("never mind, that GI score was off"). Implementation: the patch struct uses `*int` plus a sentinel; the handler decodes JSON twice (first into the struct, then peeks the raw object's keys to detect explicit-null) or uses a `json.RawMessage` approach. Mirrors the empty-string-clears convention from `add-meal-workout-link` but for nullable integers.

**Alternatives considered:**

- *Two-state: set or missing; clearing requires re-PATCHing all other fields.* Rejected — non-discoverable and friction-heavy.
- *Empty-string-clears for numeric fields.* Rejected — JSON null is the right idiom for clearing a number; empty string would require client-side gymnastics.

### 7. `/workouts/{id}/fueling` surfaces the rehearsal data alongside fueling totals

The existing fueling aggregator already echoes a `workout` block. Two fields added there (`rpe`, `gi_distress_score`) so the agent reads "session perceived effort + GI + carbs/sodium/caffeine totals" in one call — the natural shape for evaluating a rehearsal.

No new endpoint required.

### 8. MCP tool descriptions explicitly name the scales

`log_workout` and `patch_workout` tool descriptions get one sentence each on:

- "RPE: Borg CR-10 perceived effort, 1–10 integer. Per session, logged after the workout."
- "GI distress: 1 = no distress, 5 = severe (couldn't continue / had to stop). Per session."

The agent's prompt to the user becomes the obvious: "RPE? Any GI distress?" Two questions, two integers, one PATCH call.

## Risks / Trade-offs

- **Per-entry diagnostic lost (or moved to `note`).** If the user really wants "Maurten = GI 1 vs SIS = GI 4" structured comparison, this design forces them to live in `workout_fuel_entries.note` free-text. Cost: agent has to parse English. Mitigation: the agent is already parsing English notes everywhere; structured per-entry GI is a v2 if the workout-level data shows the comparison is consistently buried.
- **Subjective data is, well, subjective.** RPE drift across weeks (the user calibrates differently after fatigue, sleep, etc.) is a known limitation. The data is still directional — and the alternative is having nothing.
- **PATCH-null-clears is the third such convention.** The codebase already has empty-string-clears for `workout_id` on meals/hydration. Now numeric-null-clears for RPE/GI. Two different idioms for two different types. Both are defensible (empty string only makes sense for stringy IDs; null is canonical for numerics) but adds a sentence to the design vocabulary. Mitigation: documented in this design + the spec scenarios.
- **CHECK constraint mutability.** If the user ever wants to extend RPE to 1–11 (Borg has a CR100 variant), it's a CHECK-constraint replacement migration. Cheap, but not free.

## Migration Plan

Forward = apply migration 018, deploy code, regenerate docs. Rollback = `018_*.down.sql` drops both columns; the handler/MCP code that read the fields gracefully degrades (the pointer fields just stop being populated).

No back-fill, no transformation, no breaking change.

## Open Questions

- Should the `/workouts` LIST endpoint default to including these fields? Tentative: yes — they're cheap (two integers) and consumers can ignore.
- Should there be a convenience MCP tool `set_rehearsal_data(workout_id, rpe, gi)` distinct from `patch_workout`? Tentative: no — `patch_workout` is the right tool; tool-bloat is real.
- Should `daily_context` include the latest workout's RPE/GI in its workout block? Tentative: yes, transparent — the WorkoutLite projection used by the aggregator should add the two fields. (Daily-context already shipped; this is a minor follow-up.)
