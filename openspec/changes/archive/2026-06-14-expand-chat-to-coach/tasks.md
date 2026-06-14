# Tasks: expand-chat-to-coach

> Ordered as a sub-arc — phases 1–4 are independently shippable in sequence. May be split into separate OpenSpec changes at apply time (see design.md Migration Plan).

## 1. Shared tool registry (pure refactor, no behavior change)

- [x] 1.1 Extract a shared tool-spec type (`name`, `description`, JSON-Schema `input`, `tier`, `build(input) -> httpCall`) into a new package (e.g. `internal/agenttools/`).
- [x] 1.2 Move `internal/chat`'s `registry()` entries onto the shared type; `internal/chat` builds Anthropic tool defs + dispatches from it.
- [x] 1.3 ~~Port `internal/mcpserver`'s tool registrations to build from the same shared entries.~~ **DEFERRED at apply time** (see design.md D4-amended): mcpserver is 127 tools / 201 unit tests, ~2.5× the design's ~50 estimate; a full port is the highest-risk, zero-user-value slice and touches the working desktop coach. Instead: `internal/chat` consumes the shared `internal/agenttools` registry now, and a drift-guard test asserts every shared-registry tool name exists in mcpserver's announced surface (modulo a documented allowlist of chat-bespoke convenience tools). Full mcpserver consumption is a separate future change.
- [x] 1.4 Assert zero behavior diff for chat: `go test ./internal/chat/... ./internal/agenttools/...`; mcpserver behavior unchanged (not ported); drift-guard test green; `mcp_integration_test.go` expected-tools list unchanged.
- [x] 1.5 Consolidate idempotency-key derivation (D12): lift mcpserver's richer `deriveIdempotencyKey`/`effectiveIdempotencyKey` (strips `idempotency_key`, UseNumber) into `agenttools.DeriveIdempotencyKey`/`EffectiveIdempotencyKey`; `chat`'s dispatcher consumes it (drop its local copy). Keep transport/header in the consumer. Test: `{"quantity_g":45.0}` now yields one stable key; chat tool replay still works.

## 2. Tiering + pause-and-resume confirmation protocol

- [x] 2.1 Add `tier` (`read` | `write-auto` | `write-confirm`) to the shared spec; classify existing chat writes as `write-auto`. _(Landed with the Phase 1 registry: `agenttools.Tier` + per-tool tiers, covered by `TestRegistry_SchemasValidAndTiers`. The turn-level gate that consumes it is 2.2.)_
- [x] 2.2 Loop turn-level gate: auto-execute a turn only if all calls are `read`/`write-auto`; if any is `write-confirm`, pause — emit `proposal`, persist the assistant turn, `done` with `stop_reason: "awaiting_confirmation"`.
- [x] 2.3 Add the `proposal` SSE event (`{turn_id, calls:[{tool_id,name,tier,preview}]}`) to `sse.go`; compose a human `preview` per pending write.
- [x] 2.4 `POST /chat/sessions/{id}/confirm` handler + service path: validate trailing-turn shape (`404 session_not_found`, `409 nothing_to_confirm`, `400 invalid_confirmation`), dispatch approved + auto calls in order, synthesize declined `tool_result`s, append one `tool_result` user turn, resume the loop and stream.
- [x] 2.5 `sanitizeHistory`/`opensCleanly`: preserve the trailing awaiting-confirmation `tool_use` turn; still drop truncation-dangling ones.
- [x] 2.6 `/chat` on an awaiting-confirmation session implicitly rejects pending `write-confirm` calls (appends declined `tool_result`s) then proceeds with the new message — no `409`.
- [x] 2.7 State-driven resume: confirm endpoint also continues a session whose trailing turn is a `tool_result` with no following assistant (recovery after a continuation stream died); re-sent confirm does not re-execute writes (content-derived idempotency key). Test it.
- [x] 2.8 Cold-open (D9): `GET /chat/sessions/{id}` returns `pending_confirmation` (server-composed previews) when paused, null otherwise; `GET /chat/sessions` headers carry `awaiting_confirmation`.
- [x] 2.9 Tests: pause on a `write-confirm` turn, per-item approve subset→executes selected + declines rest + resumes, implicit-reject via new message, continuation-died→re-confirm recovers, sanitize keeps paused / drops truncated.

## 3. Broaden the surface + coach persona

- [x] 3.0 Aggregate context reads (D10/D11). **Split into its own change `add-coach-context-endpoints`** (committed `af841be`): `GET /context/training` + `GET /context/recovery` (new `internal/coachcontext` package) shipped dual-surface — REST + MCP tools `get_training_context`/`get_recovery_context` in `AnnouncedToolNames`. The matching `agenttools` entries land in 3.1.
- [x] 3.1 Add the **curated** coach tools to the chat registry (`agenttools.coach.go`): 2 aggregate reads (`get_training_context`, `get_recovery_context`) + 8 `write-confirm` actions (`log_workout`, `patch_workout`, `delete_workout`, `log_weight`, `log_hydration`, `log_meal_freeform`, `set_daily_goal_override`, `delete_daily_goal_override`) — total 24, all names ⊆ `AnnouncedToolNames` (drift guard green). Each write-confirm tool has a `Format` preview formatter (D6). Dispatcher skips `Idempotency-Key` on PUT (override) writes.
- [x] 3.2 Rewrote `buildSystemPrompt`: endurance-coaching persona; grounding-before-advising (daily/training/recovery context), dietary preference + timezone, retained meal-planning selection contract, propose-but-user-confirms disclosure with anti-spam rules; server-assembled, non-overridable.
- [x] 3.3 Updated `internal/chat` + `agenttools` surface tests for the broadened registry; added `TestConfirm_RealCoachToolPauses` proving a production write-confirm tool pauses with its code-composed preview; confirmed out-of-surface writes are now in-surface but `write-confirm`.
- [x] 3.4 No handler request/response structs changed in phase 3 (the confirm endpoint was added + swag-regenerated in phase 2); docs already current.

## 4. Companion UI

- [x] 4.1 Added `ChatProposalEvent` + `ChatPending`/`ChatPendingCall` to the union (parsed in `chat_client`); `ChatState.pending` + `_finalizePaused` keeps the card pending on `done{awaiting_confirmation}` with the composer re-enabled.
- [x] 4.2 `_ProposalCard` renders in-transcript with per-item `SwitchListTile` toggles + "Apply selected" + "Reject all"; one widget fed by either a live `proposal` event or `pending_confirmation` from session detail.
- [x] 4.3 "Apply selected" → `ChatNotifier.confirm(approvals)` → `ChatClient.confirm` (`POST /chat/sessions/{id}/confirm`) and resumes the shared `_runStream`; the card clears as the resumed tool chips/text stream in.
- [x] 4.4 Typing while pending sends normally and drops the card optimistically (server implicitly rejects); `openSession` rebuilds the card from `pending_confirmation`; history tiles badge `awaiting_confirmation`. (Covered by `chat_pending_test` + the cold-open provider test.)
- [x] 4.5 Dropped the "scoped to nutrition planning" copy — empty-state + composer hint now frame chat as the coach.
- [ ] 4.6 Manual on-device e2e (**owner: user** — cannot run a device here): coaching question → per-item card → apply subset → only selected writes land; type-to-reject path; kill-app-then-reopen reconstructs the card; continuation-died→reopen recovers.

## 5. Cross-cutting

- [x] 5.1 `task test` green across `internal/chat`, `internal/mcpserver`, `internal/agenttools`, `internal/coachcontext`, `internal/chatsessions` (+ full suite; the lone `goals` failure was testcontainers parallel-boot contention, green when re-run alone per CLAUDE.md). Companion `flutter test` + `flutter analyze` green.
- [x] 5.2 `task vet` clean + `task swag` regenerated.
- [x] 5.3 Spec deltas synced into `openspec/specs/` on archive (`nutrition-chat`, `mobile-companion`). MCP expected-tools list (`AnnouncedToolNames`) bumped only for the two dual-surface aggregates (in the `add-coach-context-endpoints` change), not for chat-surface names (all already present).
