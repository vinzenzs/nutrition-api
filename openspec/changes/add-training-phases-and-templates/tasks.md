## 1. Schema migration

- [x] 1.1 Confirm the next available migration number with `ls internal/store/migrations/` (per CLAUDE.md, out-of-band work can take a slot). At time of proposal the next slot is `017`; verify before writing.
- [x] 1.2 Create `internal/store/migrations/017_add_training_phases_and_templates.up.sql`:
  - `goal_templates` table — `id UUID PK DEFAULT gen_random_uuid(), name TEXT UNIQUE NOT NULL CHECK (length(trim(name)) > 0 AND length(name) <= 128), notes TEXT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now()`, plus the 30 nullable nutrient bound columns matching `daily_goal_overrides` exactly.
  - `training_phases` table — `id UUID PK DEFAULT gen_random_uuid(), name TEXT NOT NULL CHECK (length(trim(name)) > 0 AND length(name) <= 128), type TEXT NOT NULL CHECK (type IN ('base','build','peak','recovery','race_week','off_season','other')), start_date DATE NOT NULL, end_date DATE NOT NULL CHECK (end_date >= start_date), default_template_id UUID NULL REFERENCES goal_templates(id) ON DELETE RESTRICT, notes TEXT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now()`.
  - Index `CREATE INDEX training_phases_date_range_idx ON training_phases (start_date, end_date)`.
- [x] 1.3 Create `internal/store/migrations/017_add_training_phases_and_templates.down.sql`: drop `training_phases` then `goal_templates` (FK direction matters — phases reference templates).
- [x] 1.4 Run `task migrate:up` against the local dev pool, then `task migrate:down` to confirm both directions clean up.

## 2. Templates — repo + service + handlers

- [x] 2.1 Create `internal/trainingphases/` package with `types.go` defining `Template` and `Phase` structs. `Template` mirrors `goals.Goals` shape (use `*goals.Range` pointers for each of the 15 nutrients) plus `ID uuid.UUID`, `Name string`, `Notes *string`, `CreatedAt time.Time`, `UpdatedAt time.Time`. `Phase` carries `ID uuid.UUID`, `Name string`, `Type PhaseType`, `StartDate time.Time`, `EndDate time.Time`, `DefaultTemplateID *uuid.UUID`, `Notes *string`, `CreatedAt`, `UpdatedAt`, plus a non-persisted sibling `DefaultTemplateName *string` populated by the JOIN-in-handler. `PhaseType` is a string-typed enum with the 7 values + `IsValid()`.
- [x] 2.2 `templates_repo.go` — `TemplatesRepo` over `store.Querier` with `Insert/Upsert(ctx, template)`, `GetByName(ctx, name) (*Template, error)` (sentinel `ErrTemplateNotFound`), `List(ctx) ([]*Template, error)`, `Delete(ctx, name) error` (returns `ErrTemplateInUse` with a `ReferencingPhases []ReferencingPhase` field when FK-RESTRICT trips — detect via pgx error code `23503`). Column projection helper reused from `internal/goals/scan.go` (export it or copy the 30-column projection here).
- [x] 2.3 `templates_service.go` — wraps the repo, runs validation via the same `validateGoals` rules from `internal/goals/handlers.go`. Export the validator from goals or duplicate the check; lean toward exporting so future capabilities don't drift.
- [x] 2.4 `templates_handlers.go` — Gin handlers for `PUT /goal-templates/:name`, `GET /goal-templates/:name`, `GET /goal-templates`, `DELETE /goal-templates/:name`. PUT rejects `Idempotency-Key` per the existing rule (the idempotency middleware already enforces this for PUT). swag annotations on each handler.
- [x] 2.5 Wire templates in `internal/httpserver/server.go`: construct `trainingphases.NewTemplatesRepo(pool)` → `trainingphases.NewTemplatesService(repo)` → `trainingphases.NewTemplatesHandlers(svc).Register(api)`.
- [x] 2.6 Tests: `templates_repo_test.go` (CRUD happy path, full-replace semantics, list ordering, FK-RESTRICT on delete after a phase is created); `templates_handlers_test.go` (PUT happy path, PUT with Idempotency-Key → 400, validation error codes, 404 on missing, 409 on delete-while-referenced).

## 3. Phases — repo + service + handlers

- [x] 3.1 `phases_repo.go` — `PhasesRepo` with `Insert(ctx, phase) error`, `GetByID(ctx, id) (*Phase, error)` (joins `goal_templates` to populate `DefaultTemplateName`), `ListIntersecting(ctx, from, to time.Time) ([]*Phase, error)` (also joins template name), `Patch(ctx, id, patchParams) error`, `Delete(ctx, id) error` (sentinel `ErrPhaseNotFound`). `PatchParams` carries `*string Name`, `*PhaseType Type`, `*time.Time StartDate`, `*time.Time EndDate`, `*uuid.UUID DefaultTemplateID` (use empty-string sentinel for clearing — same convention as `add-meal-workout-link`'s `ClearWorkoutID` flag, except here it's an explicit JSON `null` since this is a UUID FK, not a string). Document the convention in the handler layer.
- [x] 3.2 Method on `PhasesRepo` used by the goals resolver: `PhaseFor(ctx, date) (*Phase, error)` returning the most-recently-updated phase covering `date`, or `ErrPhaseNotFound` if none. Limit 1. Used in the single-day adherence path.
- [x] 3.3 Method `PhasesIntersectingRange(ctx, from, to) ([]*Phase, error)` returning every phase intersecting `[from, to]`. Used in the range adherence path so the resolver can do the per-day pick in memory without N round-trips.
- [x] 3.4 `phases_service.go` — wraps the repo, validates: `Name` length+nonempty, `Type.IsValid()`, `StartDate <= EndDate`, `DefaultTemplateID` (when supplied non-nil) exists in `goal_templates` (look it up; return `ErrTemplateNotFound` if missing). Patch validates the resulting `start <= end` after partial update.
- [x] 3.5 `phases_handlers.go` — Gin handlers for `POST /phases`, `GET /phases/:id`, `GET /phases?from=&to=`, `PATCH /phases/:id`, `DELETE /phases/:id`. POST and PATCH bodies decoded with `DisallowUnknownFields` (rejects typos). swag annotations.
- [x] 3.6 Wire in `internal/httpserver/server.go` mirror to step 2.5. The handler registration goes alongside the templates one.
- [x] 3.7 Tests: `phases_repo_test.go` (CRUD, range intersection edge cases — same date, fully inside, fully outside, partial overlap on each end; overlap by 1 day vs no overlap; updated_at-DESC tie-break in PhaseFor; deletion).
- [x] 3.8 Tests: `phases_handlers_test.go` (POST happy path, POST validation errors for type/dates/name, POST with non-existent default_template_id returns 400 template_not_found, GET by id with and without template, GET range with no `from`/`to` → 400, PATCH partial, PATCH validation, DELETE happy path, DELETE non-existent → 404).

## 4. Goals resolver extension

- [x] 4.1 Update `internal/goals/effective.go` `Resolver` struct to carry `phases *trainingphases.PhasesRepo` and `templates *trainingphases.TemplatesRepo`. Update `NewResolver(defaults, overrides, phases, templates)` signature.
- [x] 4.2 Update `EffectiveFor(ctx, date)` to insert the phase step between override and default: after `overrides.GetOverride(date)` misses, call `phases.PhaseFor(ctx, date)`; if found AND `DefaultTemplateID != nil`, fetch the template via `templates.GetByID(ctx, *DefaultTemplateID)` (add this method if not already present), convert template to `goals.Goals`, return `(goals, GoalSourceTemplate, phase.Name)`. If phase has no template, continue to defaults.
- [x] 4.3 Add `GoalSourceTemplate GoalSource = "phase_template"` to the existing enum in `internal/goals/effective.go`.
- [x] 4.4 Extend `EffectiveFor` return signature to optionally return a `phaseName string` (empty when goal_source != phase_template). Existing call sites (summary handlers) update to consume the third value.
- [x] 4.5 Update `EffectiveForRange(ctx, from, to)` to batch-fetch phases via `phases.PhasesIntersectingRange(ctx, from, to)` once, then do the per-day pick in memory. Also batch-fetch the templates needed (one query: `WHERE id IN (<set of distinct default_template_ids>)`). Build a `map[date] (Goals, GoalSource, phaseName)` and return it (extending the existing pair-of-maps return shape — add a third map for phase names).
- [x] 4.6 Update `internal/httpserver/server.go` to construct phases + templates repos before building the resolver, and pass them in.
- [x] 4.7 Tests `effective_test.go`: existing scenarios still pass (no phase configured → original chain behaviour); phase-with-template wins over default; override wins over phase; phase-without-template falls through to default; overlap with updated_at tie-break; phase covering some days of a range but not others.

## 5. Summary handler — phase_name field + adherence with phase template

- [x] 5.1 Update `internal/summary/handlers.go` daily handler to consume the new third return value from `Resolver.EffectiveFor` and include `phase_name` in the response (use `omitempty` to match the codebase convention — field is absent when goal_source != phase_template).
- [x] 5.2 Update `internal/summary/handlers.go` range handler similarly: per-day entries get `phase_name` from the resolver's range map.
- [x] 5.3 Update the `goal_source` field's swag enum on both handlers to include `phase_template`.
- [x] 5.4 Tests in `internal/summary/handlers_test.go`: existing tests still pass; new tests for phase_template goal_source on daily and range; new test for phase_name presence/absence on each goal_source.

## 6. MCP tools — phases (5 tools)

- [x] 6.1 Create `internal/mcpserver/tools_trainingphases.go` with `CreatePhaseArgs`, `ListPhasesArgs`, `GetPhaseArgs`, `UpdatePhaseArgs`, `DeletePhaseArgs` structs + handlers. Match the pattern from `tools_goal_overrides.go`.
- [x] 6.2 `handleCreatePhase` POSTs to `/phases` with the args minus `idempotency_key`; sets derived-or-explicit idempotency key.
- [x] 6.3 `handleListPhases` GETs `/phases?from=&to=` from `From` and `To` args; no Idempotency-Key.
- [x] 6.4 `handleGetPhase` GETs `/phases/{phase_id}`.
- [x] 6.5 `handleUpdatePhase` PATCHes `/phases/{phase_id}` with the args minus `phase_id` and minus `idempotency_key`; sets idempotency key.
- [x] 6.6 `handleDeletePhase` DELETEs `/phases/{phase_id}`; sets idempotency key; returns empty content on 204.
- [x] 6.7 Register all 5 tools in `registerTrainingPhasesTools(server, c *apiClient)` with descriptions matching the spec (especially: `create_phase` description names the default_template_id semantic + the override-still-wins rule).
- [x] 6.8 Call `registerTrainingPhasesTools` from `internal/mcpserver/server.go` alongside the other `register*Tools` calls.
- [x] 6.9 Tests in `tools_trainingphases_test.go`: each handler hits the right method/path; write tools set Idempotency-Key (derived); read tools don't; explicit `idempotency_key` arg forwarded verbatim; backend 4xx forwarded with isError=true.

## 7. MCP tools — templates (4 tools)

- [x] 7.1 In the same `tools_trainingphases.go` file (or a sibling `tools_goal_templates.go` if it grows long): add `SetGoalTemplateArgs`, `ListGoalTemplatesArgs`, `GetGoalTemplateArgs`, `DeleteGoalTemplateArgs` + handlers.
- [x] 7.2 `handleSetGoalTemplate` PUTs `/goal-templates/{name}` with the args minus `name`; NO Idempotency-Key (PUT). Schema does NOT expose `idempotency_key`.
- [x] 7.3 `handleListGoalTemplates` GETs `/goal-templates`.
- [x] 7.4 `handleGetGoalTemplate` GETs `/goal-templates/{name}`.
- [x] 7.5 `handleDeleteGoalTemplate` DELETEs `/goal-templates/{name}`; sets idempotency key; returns empty content on 204.
- [x] 7.6 Register 4 tools in `registerGoalTemplatesTools` (or fold into the phases registration function — either is fine). Call from `server.go`.
- [x] 7.7 Tests in `tools_goal_templates_test.go`: PUT goes through without Idempotency-Key; PUT body excludes `name` (the wrapper consumed it for the URL); DELETE sets Idempotency-Key; 409 template_in_use forwarded with `referencing_phases` array intact.

## 8. MCP integration test — expected-tools assertion

- [x] 8.1 Update the expected-tools list in `internal/mcpserver/mcp_integration_test.go` to add the nine new names: `create_phase`, `list_phases`, `get_phase`, `update_phase`, `delete_phase`, `set_goal_template`, `list_goal_templates`, `get_goal_template`, `delete_goal_template`.

## 9. Documentation regen + spot updates

- [x] 9.1 Run `task swag` to regenerate `docs/swagger.{json,yaml}` and `docs/docs.go` covering the new endpoints + the extended summary response shape.
- [x] 9.2 Add a "Training phases" subsection to `README.md` after the "Daily goal overrides" subsection. Mirror the structure (intent paragraph + curl examples + response shape). Cover: PUT template, POST phase pointing at the template, GET /summary/daily showing `goal_source: phase_template` and `phase_name`.
- [x] 9.3 Add an end-to-end example to `RUN_LOCAL.md`: create `weekday-easy-training` template, create a `build` phase with that template for the next 14 days, override one workout day with a higher carb bound, GET /summary/range and verify the per-day mix of `phase_template` / `override` / `default`.

## 10. Verify and hand off

- [x] 10.1 Run `task test` — all unit + handler + integration tests pass. (Re-run any package showing the documented testcontainers parallel-boot flake alone.)
- [x] 10.2 Run `task build` and exercise the round-trip via manual curl: PUT template → POST phase → GET /phases → GET /summary/daily → confirm `goal_source` and `phase_name` populated correctly. Then PATCH the phase's `default_template_id` and re-GET summary; confirm the new template's bounds drive adherence.
- [x] 10.3 Verify `openspec status --change "add-training-phases-and-templates"` reports all artifacts done and all tasks done.
- [ ] 10.4 Commit per CLAUDE.md's "commit after every /opsx:apply" convention: `feat(training-phases): add training phases + goal templates with resolver-time effective-goals chain` — include the change dir, the new `internal/trainingphases/` package, the migration pair, the modified goals + summary + mcpserver files, and the doc updates.
- [ ] 10.5 Ready for `/opsx:archive add-training-phases-and-templates` — at archive time the three delta specs sync into main specs: a new `openspec/specs/training-phases/spec.md` is created, `openspec/specs/nutrition-goals/spec.md` is updated, and `openspec/specs/mcp-server/spec.md` gains the new requirements.
