# Tasks: add-plan-slot-targets

## 1. Migration

- [ ] 1.1 Verify the migration head (training-plan's `031` is current on disk; confirm `add-garmin-scheduling` hasn't claimed the next slot), then `task migrate:new NAME=add_plan_slot_target_overrides` (expect `032`)
- [ ] 1.2 Up: `ALTER plan_slots ADD COLUMN target_overrides JSONB NULL`. Down: drop the column

## 2. Types + validation (reuse the templates Target validator)

- [ ] 2.1 `internal/trainingplan/types.go`: add `TargetOverrides []SlotTargetOverride` to `PlanSlot` (json `target_overrides,omitempty`); `SlotTargetOverride{ Intent string; Target workouttemplates.Target }`
- [ ] 2.2 `service.go`: validate overrides — each `intent` a known template intent constant; no duplicate intent; delegate each `target` to the existing `workouttemplates` Target validator; null/empty allowed. Sentinel errors mapped 1:1
- [ ] 2.3 Unit tests: valid pace override accepted; duplicate intent rejected; inverted pace range / out-of-range zone / unknown kind rejected (validator delegation holds)

## 3. Slot CRUD widening

- [ ] 3.1 `repo.go` + `handlers.go`: slot create/patch accept `target_overrides` (marshalled to/from JSONB); nested plan `GET` returns it; PATCH replaces the list wholesale (`[]` clears, omitted leaves unchanged)
- [ ] 3.2 Tests: create-with-overrides round-trips in the nested GET; patch replace / clear / omit semantics

## 4. Effective-program resolver + endpoint

- [ ] 4.1 `service.go`: `EffectiveProgram(ctx, workoutID)` — load the workout, its template (via `template_id`) and slot (via `plan_slot_id`), apply per-intent target replacement to the template steps; template-less workout → metadata, empty steps
- [ ] 4.2 `handlers.go`: `GET /workouts/{id}/program` returning effective steps + sport/name; swag annotations; wire in `internal/httpserver` (auth)
- [ ] 4.3 Tests: override replaces only the matching intent's target, leaves others + durations + repeat structure intact; no-overrides yields template verbatim; template-less returns metadata only

## 5. MCP tools

- [ ] 5.1 `internal/mcpserver`: widen `add_plan_slot` / `patch_plan_slot` payloads with `target_overrides`; add `get_workout_program` (`GET /workouts/{id}/program`, verbatim)
- [ ] 5.2 Bump expected-tools list in `mcp_integration_test.go` (+`get_workout_program`)

## 6. Cross-proposal contract note

- [ ] 6.1 Update `add-garmin-scheduling` design D1 + tasks: the compile path builds from a planned workout's **effective** steps (template + slot overrides), via `EffectiveProgram`, not raw template steps

## 7. Docs + verification

- [ ] 7.1 `task swag`; README REST table gains `GET /workouts/{id}/program` and the `target_overrides` slot field; README MCP table gains `get_workout_program`
- [ ] 7.2 `task vet` + `task test` green; `openspec validate add-plan-slot-targets --strict` passes
