## Context

The user's nutrition life has three time-scales that interact:

1. **Daily** — what kcal/carbs/protein you targeted today. Today's handled by `nutrition_goals` (the singleton default) and `daily_goal_overrides` (per-date deliberate exception).
2. **Multi-week training periods** — "I'm in the build block, 3 weeks of 4." This is what's missing today. The agent carries it implicitly across conversations; nothing in the data layer reflects it.
3. **Reusable goal-sets** — "a weekday easy ride wants ~2200kcal/700g carbs/180g protein" gets re-used across many dates and possibly across many plans. Today the user re-types those numbers into every override.

Layer 2 (phases) is the gap T1 #1A names. Layer 3 (templates) is what T1 #5 names. Priorities.md flagged them as the same question with two answers, and the user picked "phase + templates, one trunk" — phases are the *trigger* (a date range tagged with type), templates are the *payload* (a reusable goal-set). They ship together because templates without phases have no consumer, and phases without templates can only point at "use the singleton default" — i.e. they would do nothing.

The earlier `add-carb-load-auto-apply` change established an apply-pattern: compute targets, materialize them into `daily_goal_overrides`, atomic transaction. That pattern is **deliberately not used here**. Carb-load is a fixed, deterministic schedule produced from a single race date and body weight — once you run apply, you can throw the params away and the overrides are the truth. Phases are different: they describe *intent over a period*, and templates may evolve mid-phase (the user adjusts the `build` baseline mid-block from 2200kcal to 2400kcal). Materializing would mean changing a template silently leaves stale dates behind; the resolver-time approach makes template edits propagate automatically.

## Goals / Non-Goals

**Goals:**

- One change covers both T1 #5 (templates) and T1 #1A (training-phase). Closing both Tier-1 items in one ship.
- Resolver-time composition. No new write path that the user has to remember to run.
- Backward-compatible default: a user with no phases and no templates sees zero behaviour change. The `goal_source` enum gains a value but existing values + responses are byte-identical.
- The agent can read "what phase is the user in on date D?" in one tool call (`GET /summary/daily` already returns `goal_source`; `phase_name` is the missing breadcrumb).
- Cheap to evolve a template. The user is iterating on what "build week" actually means; a template edit applies to every date currently in a phase pointing at it.

**Non-Goals:**

- **Per-day-type templates inside a phase** (training vs rest). v1 has ONE default template per phase. Per-day-type variation continues to use existing per-date overrides. v2 can revisit if the override-on-workout-days pattern is consistently the same across phases.
- **Workout-day auto-detection.** A logged workout is not a planned training day; conflating them inverts the lifecycle.
- **Phase overlap enforcement.** v1 allows overlaps and resolves to the most-recently-updated phase. Real use will tell us whether this is a misuse pattern worth blocking.
- **Materializing phases into overrides.** Detailed in the next decision.
- **Cascading prompts.** Changing a template doesn't ask "are you sure?" — templates are intentionally cheap to evolve.
- **Template inheritance.** Flat. No "weekday-easy WITH a carb bump" — make a new template.
- **A "phase plan" / multi-phase wrapper.** A 16-week plan is currently expressed as N adjacent phases. v1 doesn't introduce a `training_plan(id, name) -> phases[]` parent. Could be a v2 if naming and lifecycle of plans matters.

## Decisions

### 1. Phases resolve at read-time, not via materialization

```
GET /summary/daily?date=D
  └─> EffectiveFor(D) in goals.Resolver:
        1. overrides.GetOverride(D) → if hit, return (goals, "override")
        2. phases.PhaseContaining(D) → if hit AND phase.default_template_id != null:
             templates.Get(phase.default_template_id) → return (goals, "phase_template", phase.name)
        3. defaults.Get() → if non-null, return (goals, "default")
        4. return (nil, "none")
```

The carb-load auto-apply path **does** write `daily_goal_overrides` because it consumes a fixed input (race date + body weight) and produces a fixed schedule. Phases describe *intent over a period that may evolve*; the template a phase points at may have its bounds adjusted mid-block. Resolver-time keeps those edits propagating; materialization would orphan dates behind every template tweak.

The cost is one extra repo round-trip per `EffectiveFor` call (phase lookup). For `EffectiveForRange(from, to)` we issue ONE batch query for phases intersecting the window, matching the existing pattern for overrides batch-fetching. No per-day round-trip regression.

**Alternatives considered:**

- *Materialize on phase-create.* Walk every date in `[start, end]`, write an override row pointing at the template's bounds. Rejected: template edits would silently orphan dates; phase shortening would leave overrides behind; the user would have to learn the difference between "phase override" and "user override" because the table doesn't.
- *Hybrid: resolve at read time but cache materialized snapshots.* Premature. The read path is one extra query; cache invalidation on template/phase edits is not free. Revisit if `EffectiveForRange` ever shows up on a profile.

### 2. Phases own a single `default_template_id` (nullable); per-day variation via overrides

A phase has at most one default template. Within a phase, per-date overrides win over the phase's template. This means a `build` phase with `weekday-easy-training` as default and a per-date override for Saturday's long ride still works the way the user expects.

The proposal explicitly does NOT introduce `phase_default_templates(phase_id, day_type, template_id)`. Adding it later is non-breaking — phases keep their `default_template_id`, and a new per-day-type table layers on top with priority "per-date override → per-day-type template → phase default template → singleton default."

**Alternatives considered:**

- *Per-day-type from v1.* Pros: avoids the eventual schema split. Cons: requires defining "day_type" up front. Is it `training | rest`? Or `easy_workout | hard_workout | long_workout | rest | race`? Coupling the schema to a day_type taxonomy before real use is premature. v1 ships one template per phase; usage clarifies the taxonomy.
- *Phase-level goal overrides instead of templates.* I.e. the phase row itself carries 30 nutrient columns, no template table. Rejected — templates are *reusable*; the user wants to set `weekday-easy-training` once and reuse it across `build_1`, `build_2`, and `build_3` (which share a baseline but have different date ranges and notes). Without templates, that's 3× the data entry.

### 3. Template identity by `name`, not by `id`

`PUT /goal-templates/weekday-easy-training` (NOT `PUT /goal-templates/{uuid}`). The endpoint takes the user-readable name as the key. Internally the table has both a UUID `id` (for the FK from `training_phases`) and a `UNIQUE NOT NULL name`. The REST surface uses `name`; the FK uses `id`.

Reason: the user names templates and references them by name in conversation with the agent. UUID-in-URL is hostile. The same trade-off as products having both `id` and `barcode` — internal FK is the UUID, but the agent-facing tool takes the human-readable identifier.

`set_goal_template` MCP tool accepts the template name as input; the wrapper PUTs to `/goal-templates/{name}`. No UUID exposure on the agent surface.

**Alternatives considered:**

- *UUID everywhere.* Rejected — the user types template names to the agent constantly; UUID-in-conversation is friction.
- *Name everywhere, drop the UUID.* Rejected — renaming a template would orphan FKs from `training_phases.default_template_id`. UUID FK + name UNIQUE constraint lets the name evolve without breaking the FK.

### 4. Phase type as TEXT + CHECK constraint, not Postgres ENUM

```sql
type TEXT NOT NULL CHECK (type IN
  ('base', 'build', 'peak', 'recovery', 'race_week', 'off_season', 'other'))
```

Reason: adding a new enum value in Postgres requires `ALTER TYPE ... ADD VALUE`, which doesn't run inside a transaction the way `golang-migrate` expects. A TEXT + CHECK is one CHECK-constraint replacement migration to extend the enum later. Tiny perf hit on the index side is irrelevant at single-user scale.

This matches the pattern `add-workouts-capability` used for `workouts.source` and `workouts.sport`.

### 5. `goal_source` enum gains `phase_template`; `phase_name` is a new sibling field

The summary handler today returns `goal_source: "override" | "default" | "none"`. After this change it returns `"override" | "phase_template" | "default" | "none"`. The new value lands between override and default — agents that switch on the field need an explicit case for `phase_template`.

`phase_name` is a new sibling field on the same daily/per-day-in-range payload. It is:

- The phase's `name` (e.g. `"build-block-2"`) when `goal_source == "phase_template"`.
- `null` for every other goal_source.

I lean toward emitting `null` rather than omitting the field, because the schema becomes more predictable for downstream consumers (no "is the key absent or null?" branching). The existing `omitempty` JSON pattern would suggest omit, though — and the codebase consistently uses `omitempty`. Pick one at implementation time; either is defensible. **Tentative: use `omitempty`** to match the codebase, and document the agent expectation as "field is absent when goal_source != phase_template."

### 6. `default_template_id` on phases is nullable

A phase can exist without a default template — useful for "I want to mark this 4-week block as `recovery` to remember context, but I haven't decided on a template yet." Effective-goals resolution falls through to the singleton default when `phase.default_template_id IS NULL`. `goal_source` in that case is `"default"` (not `"phase_template"`) — because no template resolved.

If the user wants the phase to *also* be visible in the summary even without a template, that's a future enhancement (e.g. `phase_name` could resolve from the phase even when goal_source is default — a separate field, distinct from the goal_source). v1 keeps `phase_name` strictly tied to `phase_template` goal_source.

### 7. Delete semantics: phases freely deletable; templates refused-if-referenced

```
DELETE /phases/{id}          → 204 (no FK constraints to worry about; resolver
                                stops finding the phase the moment it's gone)
DELETE /goal-templates/{name} → 204 if no phase references the template's id
                              → 409 template_in_use with `referencing_phases: [...]`
                                listing the phase ids+names blocking the delete
```

The DB enforces this via `default_template_id UUID NULL REFERENCES goal_templates(id) ON DELETE RESTRICT`. The handler turns the FK-violation error into `409`.

**Alternatives considered:**

- *`ON DELETE SET NULL`*: deleting a template silently un-templates phases pointing at it. Rejected — silent data loss; the phase now resolves to the singleton, and the user is none the wiser until they read a summary and see `goal_source: "default"` where they expected `phase_template`.
- *Soft-delete templates.* Rejected — single-user project, no audit requirement, the YAGNI cost is real.

### 8. Range queries on phases use a partial index on `(start_date, end_date)`

```sql
CREATE INDEX training_phases_date_range_idx
  ON training_phases (start_date, end_date);
```

The hot query is `WHERE start_date <= $to AND end_date >= $from` (any phase intersecting `[from, to]`). A composite B-tree index on both columns satisfies it. At single-user scale this index isn't load-bearing — phases are dozens, not millions — but it's free at this scale and matches the pattern other range-query tables use.

### 9. Overlap resolution: most-recently-updated phase wins

If two phases cover the same date (e.g. an overlapping `recovery` and `race_week`), the resolver picks the one with the most recent `updated_at`. This means moving a date "from recovery to race_week" is just `PATCH /phases/{race-week-id}` — the touch of `updated_at` flips the priority. Same idiom the rest of the codebase uses for "last write wins."

Documented in the resolver requirement. A hint in the agent tool description: "If you have overlapping phases by design, the most-recently-updated wins for adherence — touch a phase to make it active over its overlap."

**Alternatives considered:**

- *Reject overlaps at insert time.* Cleaner but breaks the "I want a race_week INSIDE my build block" use case which is genuinely real (race week is the last 7 days of a 4-week peak block).
- *Explicit priority field on phases.* Rejected — adds a config knob the user has to think about. `updated_at` is implicit and matches what humans expect ("the thing I just edited applies").

### 10. MCP tools mirror REST 1:1

Nine new tools (5 phases + 4 templates). Each is a thin HTTP wrapper. Write tools follow the existing rules:

| Tool                       | REST                            | Idempotency-Key                                       |
|----------------------------|---------------------------------|-------------------------------------------------------|
| `create_phase`             | `POST /phases`                  | derived or explicit                                   |
| `list_phases`              | `GET /phases?from=&to=`         | none (read)                                           |
| `get_phase`                | `GET /phases/{id}`              | none                                                  |
| `update_phase`             | `PATCH /phases/{id}`            | derived or explicit                                   |
| `delete_phase`             | `DELETE /phases/{id}`           | derived or explicit                                   |
| `set_goal_template`        | `PUT /goal-templates/{name}`    | none (PUT — `idempotency_unsupported_for_put`)       |
| `list_goal_templates`      | `GET /goal-templates`           | none                                                  |
| `get_goal_template`        | `GET /goal-templates/{name}`    | none                                                  |
| `delete_goal_template`     | `DELETE /goal-templates/{name}` | derived or explicit                                   |

No bulk-apply tool. Phases ARE the bulk-apply mechanism — set the phase's default template once, every date in the range inherits.

## Risks / Trade-offs

- **Schema bigger than the minimal templates-only sketch.** A templates-only proposal would have added one table (`goal_templates`) and a per-date-override write that picks a template's bounds. This change adds two tables and a resolver extension. Trade-off: ONE change covers both T1 items, vs shipping templates now and phases later (with a refactor when phases land). User explicitly picked one-shot in the framing question.
- **Resolver gains a new step.** The hot path on `EffectiveFor(date)` was 1 query (override) + 1 query (default). It becomes up to 3 queries (override + phase + template). Mitigation: range fetches still issue one batch per layer; single-day reads pay 1 extra round-trip, which at single-user scale is invisible. If a profile ever shows it, batch the phase + template lookup into one JOIN.
- **`goal_source` enum extension is a wire-compatible additive change.** Existing agent/client switch-statements that don't recognize `phase_template` will fall through their default arm. Documented in the spec, but worth knowing.
- **`phase_name` field with `omitempty`** means downstream consumers checking for "phase_template adherence" need to read `goal_source` first, then look for `phase_name`. Acceptable; the field's presence/absence carries the same info.
- **Template edits propagate retroactively.** Editing a template's bounds changes how YESTERDAY's adherence reads against today's summary call. This is the deliberate semantics — templates are intent — but a user who expects "what I logged adheres to what I committed to that day" might be surprised. Mitigation: document this in the template description; the user can always create a new template (`weekday-easy-training-v2`) and point future phases at it if they want the historical baseline preserved.
- **No overlap enforcement** means a user mid-restructure can briefly have a stale `recovery` phase overlapping a fresh `race_week` phase; `updated_at` resolution makes this self-correcting (the user just edited `race_week`), but it does mean adherence reads are a function of write order during the restructure. Acceptable for single-user.

## Migration Plan

Forward = apply migration 017, deploy code, regenerate docs. Rollback = `017_*.down.sql` drops both tables; the resolver branch on `phases` short-circuits when the table is empty (every existing test path continues to work because nobody created a phase yet); the `phase_template` enum value disappears from new responses (existing responses on disk are wire-incompatible only with code that switched on the value — none in production yet).

Empty-tables start state means no back-fill. First write to either table happens via the new REST endpoints.

## Open Questions

- **Does `goal_source: "phase_template"` need any extra metadata?** Currently `phase_name` is sibling. Should the `adherence` shape also include `template_name`? Tentative: no — the user references templates by their phase's name in conversation, not by template name directly. Add later if needed.
- **Should the phase response from `GET /phases/{id}` inline the template's goals, or just the `default_template_id`?** Inlining saves one round-trip but couples the response to template shape. Tentative: just the FK + the template's name as a sibling field (`default_template_name`). The agent can `get_goal_template(name)` for the actual goals when needed.
- **Phase `notes` field — is it free-text agent-readable, or structured (e.g. coach reference, plan week number)?** Free-text v1. Real use might reveal a structured sub-shape; defer.
- **Should there be a `current_phase` shortcut endpoint (`GET /phases/current`)?** Convenient for the agent ("what phase am I in right now?"). Could be a one-liner derived from `GET /phases?from=today&to=today`. Tentative: skip for v1; the range endpoint serves it.
