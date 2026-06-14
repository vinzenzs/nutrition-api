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

- [x] 3.1 Port each domain: move typed arg structs into registry entries with `Build` funcs + tiers; delete bespoke `registerXxxTools` + handlers; migrate unit tests to Build-shape assertions. **DONE — 28/28 domains, all 128 generic tools (+ log_meal_from_photo bespoke):**
  - [x] **Pilot** (commit `54d94f5`): gear, personal-records, athlete-config
  - [x] **Batch 1** (commit `c8d8df2`): garminmisc, dailysummary, fitnessmetrics, recoverymetrics, hydrationbalance, workouttemplates
  - [x] **Batch 2** (commit `fc5ccf5`): summary, raceprep (conditional GET/POST), races, training-phases (+goal-templates), daily-context, energy, goals (PUT). Idempotency keying made method-based (POST/PATCH/DELETE only).
  - [x] **Batch 3** (commit `a08e98c`): training-plan (13 hierarchical tools). Added `Spec.OmitIdempotencyKey` for re-runnable writes (materialize).
  - [x] **Batch 4** (commit `8a57c3d`): garmin (17 tools, manual). login/submit_mfa use `OmitIdempotencyKey`. No multipart at this layer (base64-in-JSON).
  - [x] **Batch 5 — dual-surface** (commit `772eaee`): products, workouts, weight, hydration, goal-overrides, meal-plan, shopping, coach-context, meals. Each shared-name MCP tool got its own MCP-exposed entry (two surfaces filter independently); chat untouched; `goalrange.go`/recorder/`tools_metrics_test` shims removed. `log_meal_from_photo` stays bespoke (multipart, DD5).
  - [x] **Batch 6** (commit `e764729`): workout-fuel — the last domain.
- [x] 3.2 After each group: `go test ./internal/mcpserver/... ./internal/agenttools/...` + golden + `mcp_integration_test.go` green. _(Held green through every batch.)_

## 4. Retire the drift machinery

- [x] 4.1 `AnnouncedToolNames` is now a function derived from `agenttools.MCPRegistry()` (+ bespoke `log_meal_from_photo`); `mcp_integration_test.go` asserts the announced surface EXACTLY equals it (`ElementsMatch`) — stronger than the prior subset check (commit `640ab47`).
- [x] 4.2 Deleted `mcpserver/drift_test.go` + the `chatBespokeTools` allowlist (commit `772eaee`).
- [x] 4.3 Removed `mcpserver`'s `effectiveIdempotencyKey`/`deriveIdempotencyKey` wrappers + test; the one caller (log_meal_from_photo) calls `agenttools.EffectiveIdempotencyKey` directly. (`canonicalJSON`/`stripIdempotencyKey` already lived in `agenttools`.) (commit `640ab47`)

## 5. Cross-cutting

- [x] 5.1 Full `task test` green across the whole suite (incl. `internal/mcpserver`, `internal/agenttools`, `internal/chat`, `internal/chatsessions`).
- [x] 5.2 `task vet` clean; `task swag` produced no `docs/` drift (the port is MCP-client-side only — no REST/handler change).
- [x] 5.3 `internal/chat`'s exposed surface is unchanged: chat consumes `ChatRegistry()` (the 24-tool curated subset), proven by `TestRegistry_ExactSurface` + `TestChatToolDefs_SurfaceAndWebSearch`, green throughout.
