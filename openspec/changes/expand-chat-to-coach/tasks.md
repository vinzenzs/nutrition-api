# Tasks: expand-chat-to-coach

> Ordered as a sub-arc — phases 1–4 are independently shippable in sequence. May be split into separate OpenSpec changes at apply time (see design.md Migration Plan).

## 1. Shared tool registry (pure refactor, no behavior change)

- [x] 1.1 Extract a shared tool-spec type (`name`, `description`, JSON-Schema `input`, `tier`, `build(input) -> httpCall`) into a new package (e.g. `internal/agenttools/`).
- [x] 1.2 Move `internal/chat`'s `registry()` entries onto the shared type; `internal/chat` builds Anthropic tool defs + dispatches from it.
- [x] 1.3 ~~Port `internal/mcpserver`'s tool registrations to build from the same shared entries.~~ **DEFERRED at apply time** (see design.md D4-amended): mcpserver is 127 tools / 201 unit tests, ~2.5× the design's ~50 estimate; a full port is the highest-risk, zero-user-value slice and touches the working desktop coach. Instead: `internal/chat` consumes the shared `internal/agenttools` registry now, and a drift-guard test asserts every shared-registry tool name exists in mcpserver's announced surface (modulo a documented allowlist of chat-bespoke convenience tools). Full mcpserver consumption is a separate future change.
- [x] 1.4 Assert zero behavior diff for chat: `go test ./internal/chat/... ./internal/agenttools/...`; mcpserver behavior unchanged (not ported); drift-guard test green; `mcp_integration_test.go` expected-tools list unchanged.

## 2. Tiering + pause-and-resume confirmation protocol

- [x] 2.1 Add `tier` (`read` | `write-auto` | `write-confirm`) to the shared spec; classify existing chat writes as `write-auto`. _(Landed with the Phase 1 registry: `agenttools.Tier` + per-tool tiers, covered by `TestRegistry_SchemasValidAndTiers`. The turn-level gate that consumes it is 2.2.)_
- [ ] 2.2 Loop turn-level gate: auto-execute a turn only if all calls are `read`/`write-auto`; if any is `write-confirm`, pause — emit `proposal`, persist the assistant turn, `done` with `stop_reason: "awaiting_confirmation"`.
- [ ] 2.3 Add the `proposal` SSE event (`{turn_id, calls:[{tool_id,name,tier,preview}]}`) to `sse.go`; compose a human `preview` per pending write.
- [ ] 2.4 `POST /chat/sessions/{id}/confirm` handler + service path: validate trailing-turn shape (`404 session_not_found`, `409 nothing_to_confirm`, `400 invalid_confirmation`), dispatch approved + auto calls in order, synthesize declined `tool_result`s, append one `tool_result` user turn, resume the loop and stream.
- [ ] 2.5 `sanitizeHistory`/`opensCleanly`: preserve the trailing awaiting-confirmation `tool_use` turn; still drop truncation-dangling ones.
- [ ] 2.6 `/chat` on an awaiting-confirmation session implicitly rejects pending `write-confirm` calls (appends declined `tool_result`s) then proceeds with the new message — no `409`.
- [ ] 2.7 State-driven resume: confirm endpoint also continues a session whose trailing turn is a `tool_result` with no following assistant (recovery after a continuation stream died); re-sent confirm does not re-execute writes (content-derived idempotency key). Test it.
- [ ] 2.8 Cold-open (D9): `GET /chat/sessions/{id}` returns `pending_confirmation` (server-composed previews) when paused, null otherwise; `GET /chat/sessions` headers carry `awaiting_confirmation`.
- [ ] 2.9 Tests: pause on a `write-confirm` turn, per-item approve subset→executes selected + declines rest + resumes, implicit-reject via new message, continuation-died→re-confirm recovers, sanitize keeps paused / drops truncated.

## 3. Broaden the surface + coach persona

- [ ] 3.0 Aggregate context reads (D10): add `get_training_context` / `get_recovery_context` style `/context/*` endpoints where no aggregate exists, so the coach grounds via a few reads not ~30 granular Garmin tools. (Candidate for its own small change ahead of this phase — the desktop coach benefits too.)
- [ ] 3.1 Add the **curated** coach tools (~15–25) to the chat registry — aggregate context reads + the `write-confirm` actions worth proposing (training/goal/log/delete) — with correct tiers. Not the full 127.
- [ ] 3.2 Rewrite `buildSystemPrompt`: endurance-coaching persona; keep grounding-before-recommending, dietary preference, timezone, meal-planning selection contract; add training-context grounding and the confirm-write disclosure; stays server-assembled and non-overridable.
- [ ] 3.3 Update `internal/chat` tool/prompt tests for the broadened surface; confirm out-of-surface deletes are now in-surface but `write-confirm`.
- [ ] 3.4 `task swag` if any handler request/response structs changed (confirm endpoint).

## 4. Companion UI

- [ ] 4.1 Add `ChatProposalEvent` to the `ChatEvent` sealed union and a `pending` field to `ChatState`; on `done` with `stop_reason: "awaiting_confirmation"`, finalize the text bubble but keep the card pending (composer stays usable).
- [ ] 4.2 Render the card in-transcript with **per-item toggles** + "Apply selected" + reject; one rendering path fed by either a live `proposal` event or `pending_confirmation` from session detail.
- [ ] 4.3 Wire "Apply selected" to `POST /chat/sessions/{id}/confirm` with per-call decisions and resume `_runStream` on the returned SSE; card collapses to a compact resolved record as resumed tool chips/`done` arrive.
- [ ] 4.4 Typing a new message while a card is pending sends normally (server implicitly rejects); reconstruct the card on session (re)open from `pending_confirmation`; badge awaiting sessions in the history list.
- [ ] 4.5 Drop the "scoped to nutrition planning" copy/affordances; chat is the coach.
- [ ] 4.6 Manual on-device e2e: coaching question → per-item card → apply subset → only selected writes land; type-to-reject path; kill-app-then-reopen reconstructs the card; continuation-died→reopen recovers.

## 5. Cross-cutting

- [ ] 5.1 `task test` green across `internal/chat`, `internal/mcpserver`, and any new package.
- [ ] 5.2 `task vet` + `task swag`.
- [ ] 5.3 Update `openspec/specs/` deltas on archive; bump the MCP expected-tools list only if names moved.
