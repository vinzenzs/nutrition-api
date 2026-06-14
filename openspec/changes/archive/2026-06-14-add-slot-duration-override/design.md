# Design: add-slot-duration-override

## Context

`add-plan-slot-targets` put per-intent **target** overrides on a slot (resolved
into the planned workout's *effective program*, consumed by `GET
/workouts/{id}/program` and the `add-garmin-scheduling` compile path) and
explicitly deferred the **duration** sibling. Session length still lives in two
template-owned places: each step's `duration` (the structured "what to do") and
the template's `estimated_duration_sec` (the scalar materialize uses for the
calendar/time window). So a tempo run that progresses 75 → 80min across the build
needs N templates. This change moves *volume* progression into the slot, exactly
where `add-plan-slot-targets` moved *intensity* progression — same shape, same
column pattern, same resolver — so the plan carries both axes.

## Goals / Non-Goals

**Goals:**
- Let a slot override matching template steps' durations per occurrence, so one
  template covers a progressing series (60 → 75 → 80min).
- Reuse the `workout-templates` Duration shape and validator exactly — zero new
  vocabulary.
- Keep the calendar block and the watch workout consistent: an 80min override
  shows 80min in both, not 75 in one and 80 in the other.
- Fold cleanly into the existing effective-program resolver and its read endpoint.

**Non-Goals:**
- No step *structure* overrides (adding/removing steps, changing repeat counts) —
  durations of existing steps only. A structurally different week points at a
  different template.
- No new MCP tools and no change to how materialize computes dates/sport/name.
- No proportional auto-scaling of a whole template to hit a target total — the
  override names the step intent(s) it changes.

## Decisions

### D1: Overrides are keyed by intent, mirroring `target_overrides`

A slot gains `duration_overrides`: a list of `{intent, duration}` with **at most
one entry per intent**. At resolve time, a step's duration is replaced iff a slot
override exists for that step's intent — identical keying to `target_overrides`,
and the two lists compose independently:

```
template:  [ warmup 10min Z1,  active 55min @tempo,  cooldown 10min Z1 ]   (= 75min)
slot:      duration_overrides: [ { intent: "active", duration: {kind:"time",seconds:3600} } ]
           target_overrides:   [ { intent: "active", target:   {kind:"pace",...} } ]
effective: [ warmup 10min Z1,  active 60min @pace,   cooldown 10min Z1 ]   (= 80min)
```

Keying by intent (not step index) is robust to template edits and reads the way
progression reads ("the work gets longer, the warmup stays 10min"). A single-step
session uses `intent:"active"`, so for that common case a per-intent override
behaves exactly like a whole-session override. Trade-off (same as
`target_overrides`): every step of a matched intent gets the *same* duration —
which is the desired semantics for "all work intervals extend to 4min" and for
single-step sessions; finer control means authoring a distinct template.

### D2: Override durations are restricted to the two bounded kinds

The Duration shape has four kinds: `{kind:"time",seconds}`,
`{kind:"distance",meters}`, `{kind:"lap_button"}`, `{kind:"open"}`. A
**duration override** SHALL accept only the two bounded kinds (`time`,
`distance`) — progression is always a bounded quantity, and an override that
made a step `open`/`lap_button` would erase the very length the override exists to
set (and break the session-length derivation in D4). The bounded kinds are
validated by the **workout-templates Duration validator verbatim** (`seconds > 0`,
`meters > 0`); the two unbounded kinds are rejected with a service-layer sentinel.

### D3: `duration_overrides` is a validated JSONB column on `plan_slots`

```
ALTER TABLE plan_slots ADD COLUMN duration_overrides JSONB NULL;
-- shape: [ { "intent": "active", "duration": { "kind": "time", "seconds": 3600 } }, ... ]
```

Read/written as a unit with the slot, exactly like `target_overrides`. Service-
layer validation: each `intent` is a known intent constant; no duplicate intent;
each `duration` is a bounded kind passing the Duration validator; empty/null means
"no overrides." Nullable column; slot CRUD widens to carry it; PATCH replaces the
list wholesale (`[]` clears, omit leaves unchanged) — the `target_overrides` rule.

### D4: Materialize derives session length from the effective program

Today materialize derives the planned workout's time window from the template's
`estimated_duration_sec` (or a one-hour fallback). With a duration override, that
scalar is stale — it still says 75min while the effective steps sum to 80min,
desyncing the calendar block from the watch. So the derivation becomes, in order:

1. If the effective program's steps are **all bounded by time**, the session
   length is their **sum** (this is the overridden total).
2. Else fall back to the template's `estimated_duration_sec`.
3. Else the one-hour default.

Distance-bounded or mixed programs fall to (2)/(3) — we don't convert distance to
time. This keeps the calendar block, EA/time-window math, and the compiled watch
workout agreeing on one number. The change is local to materialize's window
computation; dates, sport, name, `plan_slot_id` keying, and the `status='planned'`
idempotency guard are all untouched.

### D5: Effective program extends, the resolver and endpoint do not move

`add-plan-slot-targets` defined the effective program as "template steps with
targets replaced by intent-matched overrides, resolved on read, not snapshotted."
This change widens that one sentence to "...targets **and durations** replaced by
intent-matched overrides." The resolver gains a second replacement pass; `GET
/workouts/{id}/program` and the `add-garmin-scheduling` compile path consume the
result unchanged — they already read effective steps, so duration overrides reach
the watch with no compile-side code change. Still resolve-on-read (no snapshot
onto the `workouts` row), preserving the single-source-of-truth stance.

## Risks / Trade-offs

- **Same-intent steps share an override duration.** A template with two distinct
  `active` steps of different intended lengths can't split them via one slot.
  Mitigated by distinct templates for that rare case; the intent model covers the
  common single-work-block and uniform-interval shapes.
- **Distance/mixed programs keep the scalar.** A distance-bounded session with a
  time override on one step won't recompute a total from mixed units — it falls
  back to `estimated_duration_sec`. Acceptable: the schedule progression that
  motivates this change is time-based (the `Plan.md` cells are minutes).
- **Contract coupling with `add-garmin-scheduling`.** That path must read
  effective durations — but it already reads effective *steps*, so the coupling is
  satisfied by the resolver change with no edit there.

## Migration Plan

`ALTER plan_slots ADD duration_overrides JSONB NULL` — additive, no backfill (NULL
= no overrides = today's behavior; session length still from
`estimated_duration_sec`). Down migration drops the column. **Verify the migration
head on disk before scaffolding** (CLAUDE.md warns numbering has been claimed
out-of-band; `add-plan-slot-targets` and `add-garmin-scheduling` both landed
slots — `ls internal/store/migrations` and take the next free number).

## Open Questions

- Should the program endpoint surface the *derived session length* alongside the
  effective steps, so the app gets "tonight: 80min" in one call rather than summing
  client-side? Leaning yes — it's the number D4 already computes; cheap to return.
- Should a duration override whose summed total contradicts a non-null
  `estimated_duration_sec` warn at write time? Leaning no — the effective sum is
  authoritative by D4; a warning adds surface for a single-user system that can
  just read `GET …/program`.
