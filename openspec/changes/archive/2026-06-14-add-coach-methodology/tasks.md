# Tasks: add-coach-methodology

## 1. Migration

- [x] 1.1 Confirm the migration head on disk (`041_add_garmin_misc_mirror` is current; if `add-slot-duration-override` landed first it took `042`) and take the next free number via `task migrate:new NAME=add_methodology_columns`
- [x] 1.2 Up: `ALTER training_phases ADD COLUMN methodology TEXT NULL` and `ALTER training_plans ADD COLUMN methodology TEXT NULL`. Down: drop both columns

## 2. training-phases: column + write/read

- [x] 2.1 `internal/trainingphases/types.go`: add `Methodology *string` to `Phase` (json `methodology,omitempty`)
- [x] 2.2 `repo.go`: select/insert/update `methodology`; `service.go` + `handlers.go`: phase create/update accept it, reads return it; replacing `methodology` leaves `notes` untouched (and vice versa)
- [x] 2.3 Tests: create/update with methodology round-trips on read; methodology and notes are independent; absent methodology serializes null

## 3. training-plan: column + write/read

- [x] 3.1 `internal/trainingplan/types.go`: add `Methodology *string` to the plan type (json `methodology,omitempty`)
- [x] 3.2 `repo.go` + `handlers.go`: `PATCH /training-plans/{id}` accepts `methodology`; `GET /training-plans/{id}` and the nested tree return it; replacing it leaves `notes` untouched
- [x] 3.3 Tests: PATCH sets methodology, GET returns it; methodology/notes independence

## 4. coach-context: surface phase methodology in the bundle

- [x] 4.1 `internal/coachcontext/types.go`: add `Methodology *string` to `PhaseLite` (json `methodology`)
- [x] 4.2 Context builder: populate `PhaseLite.Methodology` from the resolved covering phase; null when unset (no extra query — the phase row is already loaded)
- [x] 4.3 Tests: `/context/training` includes the covering phase's methodology when set; null when the phase has none; quiet-history and clamping behavior unchanged

## 5. MCP payloads

- [x] 5.1 `internal/mcpserver`: widen `patch_training_plan` and the phase-write tool payloads with `methodology` (no new tools)
- [x] 5.2 Confirm `mcp_integration_test.go` expected-tools list is unchanged; add a payload assertion that `methodology` is forwarded

## 6. Docs + verification

- [x] 6.1 `task swag`; README REST/MCP tables note the `methodology` field on phase, plan, and the `/context/training` phase slice
- [x] 6.2 `task vet` + `task test` green; `openspec validate add-coach-methodology --strict` passes
