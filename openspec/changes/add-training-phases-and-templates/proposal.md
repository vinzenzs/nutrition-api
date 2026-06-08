## Why

Today every day is identical to the system. The user is mid-way through a 16-week build/peak/recovery plan — the agent has to carry "I'm in build block 2, weeks 3–6" implicitly across conversations, and the only way to make adherence reflect "this is a hard training week" is N individual `set_daily_goal_override` calls per block. The single-bound `goal_templates` answer from priorities.md T1 #5 and the broader `training_phases` framing from T1 #1A are the **same question with two answers**; this change cashes them in together with phases as the trunk and templates as the goal-set leaves.

A phase says *what kind of training period this is* (`base`, `build`, `peak`, `recovery`, `race_week`, `off_season`, `other`) over a date range. A template is a reusable goal-set you can attach to a phase. The effective-goals resolver gains one step: per-date override (wins) → phase's default template (NEW) → singleton default → none. Per-day-type variation inside a phase (training-day vs rest-day) keeps using the existing override mechanism — phases give you the *baseline*, overrides give you *deliberate exceptions*. The carb-load auto-apply pattern (concrete materialization into overrides) and the phase pattern (resolver-time lookup) are deliberately different: overrides describe deliberate exceptions, phases describe intent. Materializing phases into overrides would conflate the two.

## What Changes

- **New `goal_templates` table** — named, reusable goal-sets with the same 30-column nutrient projection as `nutrition_goals` (every bound nullable, same `{min?, max?}` Range shape, same validation rules). Keyed by `name` (kebab-case-ish, user-chosen — `weekday-easy-training`, `weekend-long-ride`, `recovery-day`). Two REST verbs: `PUT /goal-templates/{name}` (full-replace upsert, no `Idempotency-Key` per the existing PUT contract), `GET /goal-templates/{name}`, `GET /goal-templates` (list), `DELETE /goal-templates/{name}` (refused with `409 template_in_use` if any phase references it).
- **New `training_phases` table** — `(id UUID, name TEXT, type ENUM, start_date DATE, end_date DATE, default_template_id FK NULL, notes TEXT NULL, created_at, updated_at)`. Types: `base | build | peak | recovery | race_week | off_season | other` (small, named set; `other` is the escape hatch). Date ranges are inclusive on both ends; overlaps are ALLOWED in v1 (most-recently-updated phase wins at resolve time, surfaced in `goal_source.phase_name`). REST: `POST /phases`, `GET /phases?from=&to=` (any phase intersecting the window), `GET /phases/{id}`, `PATCH /phases/{id}` (partial — name / type / dates / default_template_id / notes), `DELETE /phases/{id}`.
- **Resolver extension** in `internal/goals/effective.go` — the existing `EffectiveFor(date)` priority chain (override → default → none) gains a phase step in the middle: **override → phase-default-template → default → none**. The resolver gains a new dependency on `trainingphases.Repo` for the phase lookup. `EffectiveForRange` does one batch fetch of phases intersecting the window plus the existing batch fetch of overrides; no per-day round-trip regression.
- **`GoalSource` enum extended** with `phase_template`. The `goal_source` field on `GET /summary/daily` and per-day in `GET /summary/range` returns `"phase_template"` when a phase covered the date and resolved to a template. The summary payload also gains a sibling `phase_name` field (null when goal_source is anything else) so the agent can name the phase to the user without a follow-up call.
- **MCP tools** (9 new):
  - Phases: `create_phase`, `list_phases`, `get_phase`, `update_phase`, `delete_phase`.
  - Templates: `set_goal_template` (PUT-style — write tools today reject `Idempotency-Key` on PUT per `harden-write-paths`), `list_goal_templates`, `get_goal_template`, `delete_goal_template`.
- **swag** regenerates `docs/` with the new endpoints + the extended summary response shape.

## Capabilities

### New Capabilities
- `training-phases`: Owns the phase concept (CRUD on `training_phases`) and the goal-template concept paired with it (CRUD on `goal_templates`). Phases and templates are introduced together because templates have no consumer until phases reference them — splitting would ship an orphan capability for one release.

### Modified Capabilities
- `nutrition-goals`: The resolver requirement gains a phase-template step in the effective-goals priority chain. The `GoalSource` enum requirement gains a `phase_template` value. The "Daily and range summaries compute adherence against goals" requirement gains a `phase_name` field on each adherence-bearing response. No change to the per-date override CRUD shape or the singleton.
- `mcp-server`: 9 new tools added (phase CRUD × 5 + template CRUD × 4). No existing tool shape changes.

## Impact

- **New schema migration** (likely `017_add_training_phases_and_templates.up.sql`):
  - `goal_templates` table with the same 30 nutrient bound columns as `daily_goal_overrides`, plus `id UUID PK`, `name TEXT UNIQUE NOT NULL`, `notes TEXT NULL`, `created_at`, `updated_at`.
  - `training_phases` table with the columns enumerated above. `default_template_id` is `UUID NULL REFERENCES goal_templates(id) ON DELETE RESTRICT` (so template deletion is refused while phases reference it — matches the `409 template_in_use` REST behaviour). Partial index on `(start_date, end_date)` to keep the "any phase intersecting [from, to]" query cheap. Phase `type` is stored as TEXT with a CHECK constraint listing the seven enum values (cheaper than a true Postgres ENUM for adding a value later).
  - No back-fill needed — both tables start empty.
- **New `internal/trainingphases/` package**: `types.go` (Phase, Template structs), `phases_repo.go`, `templates_repo.go`, `phases_service.go`, `templates_service.go`, `handlers.go` registering `/phases/*` and `/goal-templates/*`. Mirrors the `internal/<capability>/` shape used by every other capability.
- **Extended resolver** in `internal/goals/effective.go`: `NewResolver(defaults, overrides, phases, templates)` signature change. Existing callers in `internal/httpserver/server.go` and tests update accordingly. `EffectiveFor(date)` and `EffectiveForRange(from, to)` get the new step; behaviour is unchanged when no phases exist (the singleton `nil` case is the existing path).
- **Summary handler shape**: `GET /summary/daily` and `GET /summary/range` per-day response gain `"phase_name": "<string>|null"`. When `goal_source == "phase_template"`, `phase_name` is the phase's `name`. Otherwise it's `null` (or absent — choose at design time; lean toward "null" for shape stability). The existing `adherence` shape is unchanged.
- **`internal/mcpserver/tools_trainingphases.go`** (new file) registering the 9 tools. Each is a thin HTTP wrapper around the new REST endpoints. Write tools (`set_goal_template` PUT-style, `create_phase` / `update_phase` / `delete_phase` / `delete_goal_template`) handle `Idempotency-Key` per the existing rules (PUT → no key; POST/PATCH/DELETE → derived-or-explicit). Integration test in `mcp_integration_test.go` adds the 9 new tool names to the expected-tools assertion.
- **Tests**:
  - Repo tests on `training_phases` (CRUD + range query + overlap behaviour) and `goal_templates` (CRUD + `template_in_use` rejection on delete).
  - Resolver tests: existing chain behaviour preserved when no phase exists; phase wins over default when no override; override wins over phase; phase-without-template falls through to default; deleted phase has no residual effect; overlapping phases — most-recently-updated wins.
  - Summary handler tests: `goal_source: "phase_template"` and `phase_name: "<name>"` appear correctly across daily and range; `phase_name` is null on default/override days; range with phase + override on different days reports the right per-day mix.
  - MCP wrapper tests for each new tool (read → no idempotency key; write → derived-or-explicit key; PUT → no key).
- **Documentation**: `README.md` gains a "Training phases" subsection mirroring the existing "Daily goal overrides" subsection. `RUN_LOCAL.md` gains an example showing the typical workflow: create a `weekday-easy-training` template, create a `build` phase with that template as default, override a single workout day with a higher carb bound, verify the summary shows phase_template on most days and override on the workout day.
- **`task swag`** to regenerate `docs/swagger.{json,yaml}` + `docs/docs.go` covering the new endpoints and the extended summary shape.

### Out of scope (explicit non-goals)

- **Per-day-type templates within a phase**. A phase has ONE default template in v1. If a `build` phase wants different baselines for training days vs rest days, the user creates per-date overrides on workout days (or chooses to model it as two adjacent phases — micro-phases, fine in v1). Reason: identifying "training days" requires a workout existing on that date, which conflates planned vs logged training. v2 can layer `(phase, day_type) → template_id` mappings if real use demands it.
- **Workout-day auto-detection.** The resolver does not infer "this is a training day because there's a workout planned." A workout is a logged event; phases are planned periods. Different lifecycles.
- **Phase overlap enforcement.** v1 allows overlaps; resolver picks the most-recently-updated overlapping phase. A CHECK constraint or a service-level validator can land in v2 if usage shows overlaps are consistently accidental.
- **Materializing phases into per-date overrides.** Apply-time materialization (the carb-load pattern) explicitly does NOT apply here. Phases describe intent; overrides describe deliberate exceptions. Materializing would mean "delete the phase" leaves stale overrides behind that the user has to clean up — exactly the friction this change is trying to fix.
- **History / audit log of phase changes.** The phase's `(start_date, end_date)` plus its row's `updated_at` is the history; we don't track "this phase used to point at template X."
- **Template inheritance or composition.** Templates are flat goal-sets, not classes with override-of-base semantics. If you want "weekday-easy with a carb bump," create a new template; don't introduce a parent/child relationship.
- **Cascading goal updates**: changing a template's bounds applies on next resolve for every phase pointing at it. No event, no notification, no "are you sure?" prompt. Templates are intentionally cheap to evolve.
- **Cross-bound validation**: e.g. a phase whose template's `kcal.min` is greater than the singleton default's `kcal.max` is not flagged. The agent reasons about consistency; the API records primitives.
- **Bulk-import / templates marketplace.** No "import a Loucks-aligned recovery template" pre-bundled. The user creates their own templates from coaching context.
