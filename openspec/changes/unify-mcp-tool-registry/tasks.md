# Tasks: unify-mcp-tool-registry

> Depends on `expand-chat-to-coach` phase 1 (agenttools exists) + task 1.5 (shared idempotency derivation). Ported group-by-group (DD7); keep `mcp_integration_test.go` green after every group.

## 1. agenttools: carry the full surface

- [x] 1.1 Add a visibility marker to `agenttools.Spec` (DD1) — added `ChatExposed`/`MCPExposed` bools; `Registry()` is now the union, `ChatRegistry()`/`MCPRegistry()` filter; `chatSpecs()` marks the existing 24 chat tools chat-exposed; `internal/chat` consumes `ChatRegistry()`, `internal/mcpserver` iterates `MCPRegistry()`.
- [x] 1.2 Add an optional typed schema source to `Spec` (DD2): `SchemaType any` added; the MCP server reflects it via `jsonschema.ForType(reflect.TypeOf(SchemaType), &ForOptions{})` — the exact call the SDK uses internally (server.go setSchema:436), which announces the unresolved schema, so parity is by construction. Hand-written `Schema` strings still drive the chat surface.
- [x] 1.3 Golden test (`schema_golden_test.go`): for each `MCPRegistry()` entry the reflected schema MUST equal the frozen `testdata/announced_schemas.json` baseline (captured pre-port from the live surface via `golden_capture_test.go`, `-tags=goldengen`). Safety gate before deleting any bespoke registration. _(Note: the 3 pilot reads were moved verbatim and their baseline was captured post-move; the gate is fully independent for every domain the workflow ports next.)_
- [x] 1.4 Confirmed `agenttools.EffectiveIdempotencyKey`/`DeriveIdempotencyKey` exist (landed in `expand-chat-to-coach` 1.5); added `agenttools.ExplicitIdempotencyKey(raw)` to read an agent-supplied key out of raw input (DD4).
- [ ] 1.5 Split the registry into per-domain files (`registry_meals.go`, `registry_workouts.go`, …) concatenated by `Registry()` (DD-risk: big file). _(Pattern established: `registry_garmin_inventory.go` is the first; `mcpOnlySpecs()` concatenates per-domain slices. Remaining domains land in 3.1.)_

## 2. mcpserver: generic dispatch path

- [x] 2.1 `apiClient` adapter: `dispatchMCP` executes an `agenttools.HTTPCall` via the existing private `c.do(ctx, Method, Path, Query, Body, key)` — already query-aware for every verb (apiclient.go:115), so one call faithfully replaces the per-verb `Get/Post/Patch/Put/Delete` wrappers.
- [x] 2.2 `dispatchMCP(ctx, c, spec, raw)` (DD3): `spec.Build(raw)` → `HTTPCall` → `c.do` → existing `toToolResult`; attaches `EffectiveIdempotencyKey(ExplicitIdempotencyKey(raw), name, raw)` for write tiers (DD4). (204-empty-body delete handled identically by `toToolResult`; the old bespoke special-case was redundant.)
- [x] 2.3 Generic registration loop (`registerSharedTools`) over `agenttools.MCPRegistry()` using the SDK's low-level `server.AddTool(&mcp.Tool{Name,Description,InputSchema}, untypedHandler)` — confirmed exposed (server.go:238); no per-tool shim needed. Coexists with not-yet-ported bespoke registrations (never shares a name).
- [x] 2.4 Multipart escape hatch (DD5): the generic loop skips `log_meal_from_photo` (const `multipartPhotoTool`); it stays a registry entry for discovery and keeps its bespoke registration. _(Lands as a registry entry when the meals domain is ported in 3.1.)_

## 3. Port tool groups (DD7 — repeat per group, integration test green each time)

- [ ] 3.1 Port each domain: move typed arg structs into registry entries with `Build` funcs + tiers; delete bespoke `registerXxxTools` + handlers; migrate unit tests to Build-shape assertions. **Progress — 18/~28 domains, 78 tools:**
  - [x] **Pilot** (commit `54d94f5`): gear, personal-records, athlete-config
  - [x] **Batch 1** (commit `c8d8df2`): garminmisc, dailysummary, fitnessmetrics, recoverymetrics, hydrationbalance, workouttemplates
  - [x] **Batch 2** (commit `fc5ccf5`): summary, raceprep (conditional GET/POST), races, training-phases (+goal-templates), daily-context, energy, goals (PUT). Idempotency keying made method-based (POST/PATCH/DELETE only).
  - [x] **Batch 3** (commit `a08e98c`): training-plan (13 hierarchical tools). Added `Spec.OmitIdempotencyKey` for re-runnable writes (materialize).
  - [x] **Batch 4** (commit `8a57c3d`): garmin (17 tools, manual). login/submit_mfa use `OmitIdempotencyKey`. No multipart at this layer (base64-in-JSON).
  - [ ] **Dual-surface** (manual reconciliation — tool already a chat entry; one Spec must serve both surfaces with the MCP-reflected schema matching the golden): products, meal-plan, shopping, workouts, weight, hydration, goal-overrides, meals (+`log_meal_from_photo` multipart, DD5), coach-context. _(NB: `goalrange.go` + `recorders_test.go` shims in mcpserver are removable once goal-overrides / mealplan+shopping are ported.)_
- [x] 3.2 After each group: `go test ./internal/mcpserver/... ./internal/agenttools/...` + golden + `mcp_integration_test.go` green. _(Held green through pilot + batch 1.)_

## 4. Retire the drift machinery

- [ ] 4.1 Replace `AnnouncedToolNames` with a registry-derived function (or delete it) and update `mcp_integration_test.go` to assert announced surface == registry names (DD6).
- [ ] 4.2 Delete `mcpserver/drift_test.go` + the `chatBespokeTools` allowlist (now redundant — both surfaces are one registry).
- [ ] 4.3 Remove `mcpserver`'s `effectiveIdempotencyKey`/`deriveIdempotencyKey`/`canonicalJSON`/`stripIdempotencyKey` (superseded by `agenttools`).

## 5. Cross-cutting

- [ ] 5.1 Full `task test` green (`internal/mcpserver`, `internal/agenttools`, `internal/chat`).
- [ ] 5.2 `task vet`; `task swag` (no REST/handler change expected — verify the spec didn't drift).
- [ ] 5.3 Confirm `internal/chat`'s exposed surface is unchanged (curated subset still filtered correctly after DD1).
