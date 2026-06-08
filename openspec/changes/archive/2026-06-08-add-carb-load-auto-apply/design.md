## Context

The carb-load primitive shipped in `add-race-prep-primitives` is stateless and side-effect-free by deliberate design — the proposal explicitly listed "Carb-load + override auto-apply" as a non-goal, with the reasoning *"don't conflate planning with execution; the agent should make those decisions explicitly."* That deferral was always conditional on usage data: revisit if real-world friction shows the explicit-N-PUTs loop is wasteful in practice.

After a session of building, archiving, and the user's own observation that the workflow today still requires "manual `set_daily_goal_override` × N days after computing the schedule," the data has landed: every race-prep cycle is one tool call followed by N mechanical follow-ups, and the agent has shown it can be trusted with the carb math but not (yet) reliably with the multi-step apply. Closing the friction is now a Tier-1 ship-now item.

The cleanest closing shape is additive: an `apply: true` flag on the existing tool that internally routes to a new POST endpoint. The pure path stays pure; the side-effect path is undeniable from the tool args. The merge-into-existing-overrides semantics protect the user's training-day templates: writing only `carbs_g` and leaving `kcal`/`protein_g`/etc. untouched is the principled answer to "what happens when a carb-load week overlaps a training-day override."

## Goals / Non-Goals

**Goals:**

- Close T1 #3: the agent (or a direct REST caller) gets the carb-load schedule AND the goal-override writes in one round trip.
- Atomicity: if the apply step can't write all N overrides, write zero — leave the goals layer in its prior state.
- Non-destructive merge: never silently clear a non-carb field on an existing override. The apply step is the carbs lane only.
- Discoverability: the `apply: true` flag is visible to the agent in the tool description with the side effect spelled out.
- No new capability surface: this is a one-method-on-the-overrides-repo addition plus a new REST handler. No table changes.

**Non-Goals:**

- Templates, training-phase context (T1 #1A, #5) — separate concern.
- A generic "plan-and-apply for any macro" abstraction.
- Tracking which overrides were written by which apply (no provenance column).
- Apply with custom date sets (skip-a-day, only-load-days, race-day-only) — the schedule's contiguous range is what gets applied.
- Unapply / rollback. User deletes per-date overrides via the existing endpoint.
- Bulk endpoint for goal overrides as a standalone primitive. The bulk-write loop in the apply handler is intentional scoped — if another writer wants the same pattern later, that's a follow-up.

## Decisions

### 1. New POST endpoint, not `?apply=true` on the GET

```
GET  /race-prep/carb-load              (existing — pure compute)
POST /race-prep/carb-load/apply        (new — compute + persist)
```

GET stays side-effect-free per HTTP semantics; the apply path uses POST. Both share the same query/body validation. The MCP layer hides the dual endpoint behind a single tool with an `apply` flag — REST callers see two endpoints, agents see one tool.

**Alternatives considered:**

- *`?apply=true` query param on GET.* Rejected — GET with side effects breaks caching contracts and HTTP semantics. The "agent sees one tool" ergonomic isn't worth corrupting the REST layer.
- *Single POST endpoint, `compute_only: true` opts out of the apply.* Considered. Rejected — the existing GET is already deployed and used; replacing it with a POST whose default is "also apply" inverts the safer default. Keep GET pure.

### 2. Merge-only writes via a new `UpsertPatch` repo method

The race-prep apply needs read-modify-write semantics on each target date's override row. The existing `OverridesRepo.Upsert` is full-replace (matching `PUT /goals/overrides/{date}`). Adding a parallel partial-update method:

```go
// UpsertPatch overlays only the non-nil fields of patch onto the existing
// override for `date` (or creates a new override containing only those
// fields if no row exists). Used by add-carb-load-auto-apply to write the
// carbs_g bound without touching kcal/protein/other macros that may already
// be set on that date (e.g. by a training-day template).
//
// Caller scope: this is intentionally not exposed via PATCH /goals/overrides/{date}
// today — there's only one writer (the carb-load apply step), and adding the
// REST verb without a concrete second consumer would be premature. If/when a
// second consumer appears, promote this to a verb.
func (r *OverridesRepo) UpsertPatch(ctx context.Context, date time.Time, patch *Goals) error
```

Implementation: fetch via `GetOverride`, branch on `errors.Is(err, ErrOverrideNotFound)`; on hit, walk every field on `patch` and copy non-nil ones onto the existing `*Goals`; on miss, treat `patch` itself as the new row. Then `Upsert` the merged value.

**Alternatives considered:**

- *Add `PATCH /goals/overrides/{date}` as a public verb.* Considered — it would close this and a hypothetical future "patch one nutrient" need in one move. Rejected as YAGNI: only one consumer today, and adding a verb invites the agent to start using it for things that should be full-replace.
- *Do read-modify-write inline in the apply handler without a new repo method.* Rejected — the merge logic is non-trivial (30 nullable fields × null-vs-set semantics) and belongs next to the rest of the override storage code. Inline is bug-prone.
- *Use SQL `INSERT ... ON CONFLICT DO UPDATE SET carbs_g_min = EXCLUDED.carbs_g_min`.* Considered. Rejected — that's the cleanest SQL but encodes "carb-only" into the repo layer, which is the wrong abstraction. The repo accepts a generic `*Goals patch`; the handler decides which fields to populate.

### 3. Min-only carb target shape

The apply step writes `carbs_g: {min: target_carbs_g}` — no `max`. Three reasons:

1. **Carb-load failure mode is asymmetric.** Under-fuelling on a load day is the bad outcome; over-fuelling is fine (within reason). A two-sided range adds an "over" status that doesn't reflect the user's actual concern.
2. **Matches the existing min-only pattern.** `fiber_g`, `iron_mg`, `calcium_mg`, and the other "more is better" nutrients all use min-only Ranges. The unified Range shape supports it natively (`{min}` with `max: null`).
3. **Avoids the precision argument.** A `{min, max}` requires picking a tolerance (±5%? ±10%?); each is a guess. Min-only ducks the question.

If real use surfaces an "over-carb" failure mode (rare — typically GI distress, which the agent observes from context, not from the bound), revisit.

**Alternatives considered:**

- *`{min: target, max: target}` exact-equal.* Rejected — forces "over" status the moment the user logs 1g more than target. Too brittle.
- *`{min: target × 0.95, max: target × 1.05}`.* Mimics the kcal_target backfill from migration 008. Rejected as still arbitrary; min-only is honest.

### 4. Atomic write via `pgx.Tx`

The apply handler opens a transaction on the pool, wraps the override repo with that transaction's `Querier`, runs N `UpsertPatch` calls inside the loop, and commits at the end. If any call returns an error, the deferred rollback fires and zero rows have been written. The response surfaces the partial-failure case as a 500 with no `applied` array.

This works because `OverridesRepo` (like every other repo in the codebase) is constructed over a `store.Querier` — and `pgx.Tx` satisfies that interface. The apply handler does:

```go
tx, err := pool.Begin(ctx)
defer tx.Rollback(ctx)  // no-op after commit
txRepo := goals.NewOverridesRepo(tx)
for _, entry := range schedule {
    txRepo.UpsertPatch(ctx, entry.Date, &goals.Goals{CarbsG: &goals.Range{Min: &entry.TargetCarbsG}})
}
tx.Commit(ctx)
```

Per-row errors abort the whole apply.

**Alternatives considered:**

- *Best-effort: write what you can, return per-row success/failure.* Rejected — partial application of a carb-load plan leaves the goals layer in a half-baked state that the user has to reconcile by hand. Worse than the original N-PUT loop.
- *Server-side bulk endpoint on goals: `PUT /goals/overrides/bulk`.* Considered as a more general primitive. Rejected for v1 — no second consumer; YAGNI. If the apply pattern recurs (e.g. templates), promote then.

### 5. Response shape echoes the schedule + per-date apply outcome

```json
{
  "race_date": "2026-07-24",
  "body_weight_kg": 70,
  "params": { "days_before": 3, "carbs_per_kg_per_day": 10, "race_day_carbs_per_kg": 2 },
  "schedule": [ ...same shape as plan_carb_load... ],
  "applied": [
    { "date": "2026-07-21", "carbs_g_min": 700, "created": true },
    { "date": "2026-07-22", "carbs_g_min": 700, "created": false },
    { "date": "2026-07-23", "carbs_g_min": 700, "created": true },
    { "date": "2026-07-24", "carbs_g_min": 140, "created": true }
  ]
}
```

`created: false` means a pre-existing override was merged into; `created: true` means a new override row was inserted. The agent can use this to surface the right summary message ("I applied N new carb-load days and updated M existing training-day overrides").

`carbs_g_min` echoes the value written, in case the agent wants to confirm the math without re-running it.

**Alternatives considered:**

- *Just return the schedule, no `applied` block.* Rejected — the user (and agent) should see at a glance which dates had existing overrides that got merged into.
- *Return the full merged `goals` object per date.* Rejected — bloats the response with non-carb fields the agent didn't write; encourages the agent to misread an unchanged kcal as something this tool set.

### 6. MCP tool stays one, gains `apply` flag

The existing `plan_carb_load` tool keeps its existing args + adds `apply: boolean` (default `false`). The wrapper branches on `apply`:

```
apply == false  → GET /race-prep/carb-load          (existing behaviour, unchanged)
apply == true   → POST /race-prep/carb-load/apply
```

Tool description gets one paragraph naming the side effect explicitly: "When `apply: true`, also writes the carb_g goal bounds (min-only) for each schedule day. Existing kcal/protein/other macros on those days are preserved. The response includes an `applied` array listing per-date outcomes."

**Alternatives considered:**

- *Separate `plan_and_apply_carb_load` tool.* Was my explore-session lean and the user considered it. They picked the flag — "apply: true" reads cleaner and matches their mental model of the workflow. The side-effect undeniability that motivated the separate-tool argument is preserved via the description text.

## Risks / Trade-offs

- **The merge semantics introduce a new write path (`UpsertPatch`) with one consumer.** If the implementation has a bug (e.g. nil-handling in the field-walk), it ships untested elsewhere. Mitigation: dedicated unit tests on `UpsertPatch` covering every nullable field × {nil-on-existing, set-on-existing, nil-on-patch, set-on-patch} combination. Plus the handler-level integration tests.
- **Transaction wraps N round-trips.** For typical N=4 (3 load days + race day) this is fine. For a hypothetical days_before=7 it's still 8 round-trips inside a tx — under 50ms total. Mitigation: none needed today; if N ever grows (e.g. multi-race apply), a single bulk SQL would replace the loop.
- **Partial-failure UX is binary.** All-N succeed or zero do. The user can't recover by retrying for the days that worked. Mitigation: this is the right semantics — partial-applied carb loads are worse than not-applied. The error response names which date triggered the rollback so the user can fix that one and retry.
- **No provenance.** Once written, an override looks the same whether it was set by `set_daily_goal_override`, by `plan_carb_load(apply=true)`, or by a future template. The agent can't query "which dates did the last carb-load apply touch." Mitigation: out of scope; if needed later, add an `origin` enum column to `daily_goal_overrides` (`manual | carb_load_apply | template_apply | …`).
- **Min-only target means no "over" adherence signal.** If the user massively overshoots the carb-load target (e.g. eats 1200g on a 700-target day), adherence reports `on` rather than `over`. Documented; aligned with the asymmetric failure-mode argument above. Revisit if real use surfaces over-carb GI distress as a tracked concern.
- **The flag-on-existing-tool decision means the agent's existing call sites work unchanged.** Win for backward compat. Mild risk: an agent currently in the habit of "compute then loop" might keep doing that out of muscle memory. Mitigation: the tool description explicitly suggests the flag for the standard workflow.

## Migration Plan

No schema change. Forward = deploy the new handler + repo method + MCP arg. Rollback = remove them (the `UpsertPatch` method is purely additive; existing callers are unaffected).

## Open Questions

- Whether the apply step should ever overwrite an existing carb-load row blindly (no merge logic for the `carbs_g` field itself — it's always replaced) or refuse if the existing carb bound differs by more than X. Tentative: always replace. Re-running `plan_carb_load(apply=true)` with the same race_date and different `body_weight_kg` should refresh the targets, not error out.
- Whether the response should include the dates that were touched even if they had a pre-existing carb target identical to what we just wrote (`created: false`, value unchanged). Tentative: yes, include them — the apply happened semantically even if the row is binary-equal.
- Whether to emit a structured log line per apply for future-debug observability. Tentative: yes, single-log per successful apply with `(race_date, days_before, applied_count, new_count)` — cheap, useful in the rare partial-failure case.
