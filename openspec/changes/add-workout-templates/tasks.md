# Tasks: add-workout-templates

## 1. Migration

- [x] 1.1 Verify migration head is `029`, then `task migrate:new NAME=add_workout_templates` (expect `030`)
- [x] 1.2 Write the up migration: `workout_templates` table per design D4 (sport CHECK reusing the workouts vocabulary, `steps JSONB NOT NULL` with the array/non-empty CHECK, `estimated_duration_sec` CHECK, index on `(sport)`); write the down migration to drop it

## 2. Capability package — types + step model

- [x] 2.1 `internal/workouttemplates/types.go`: `Template` struct mirroring the row (JSON tags, `omitempty` on nullables); a `Step` model with the two node kinds (single step + repeat group), `Intent`, `Duration{Kind,...}`, `Target{Kind,...}` types and their enum constants
- [x] 2.2 Reuse the `workouts` sport vocabulary (shared constant or mirrored values) so template `sport` and workout `sport` agree

## 3. Repo + service

- [x] 3.1 `repo.go`: CRUD against `store.Querier` (create, get, list with optional `sport` filter, patch, delete); steps marshalled to/from JSONB
- [x] 3.2 `service.go`: sentinel errors mapped 1:1 to API codes; **step validator** per design D2 (non-empty; valid intent/duration/target kinds; zone bounds `1..5`; `low <= high`; `repeat.count >= 2`, non-empty, single-level); patch semantics (supplied `steps` replaces wholesale, omitted fields unchanged)
- [x] 3.3 Unit tests for the validator: accept a valid structured template; reject empty steps, nested repeat, out-of-range zone, inverted range, unknown duration/target kind

## 4. Handlers + wiring

- [x] 4.1 `handlers.go`: `POST/GET/GET{id}/PATCH/DELETE /workout-templates` with swag annotations and `Register(rg *gin.RouterGroup)`
- [x] 4.2 Wire the package in `internal/httpserver/server.go` (instantiate repo+service, register routes behind auth + idempotency)

## 5. MCP tools

- [x] 5.1 `internal/mcpserver/server.go`: `registerWorkoutTemplateTools` with `create_/list_/get_/patch_/delete_workout_template`, one HTTP call each, body forwarded verbatim; write tools auto-derive an idempotency key
- [x] 5.2 Bump the expected-tools list in `mcp_integration_test.go`

## 6. Integration tests

- [x] 6.1 Handler integration tests (testcontainers): create→get round-trip incl. steps; list `?sport=` filter; patch replaces steps / leaves omitted unchanged; delete→404; get-missing→404; malformed steps rejected at the HTTP boundary

## 7. Docs + verification

- [x] 7.1 `task swag`; README REST table gains the five endpoints; README MCP table gains the five tools
- [x] 7.2 `task vet` + `task test` green; `openspec validate add-workout-templates --strict` passes
