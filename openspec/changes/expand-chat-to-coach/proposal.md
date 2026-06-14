# Proposal: expand-chat-to-coach

## Why

Today the app has **two** AI surfaces that were deliberately built apart: the desktop coach (the full `internal/mcpserver` tool surface — training, Garmin, recovery, race prep — driven by an external MCP client) and the in-app `/chat` loop (`internal/chat`), a narrow meal-planner whose own system prompt redirects every non-food question "to the desktop coaching agent." The phone can talk to the planner, but not the coach.

There is no "coach" object on the server to expose — the coach is a *persona* (system prompt + full tool surface + LLM) that only assembles itself inside whatever desktop MCP client the user runs. To put it on the phone, the server-side `/chat` agent loop has to **become** the coach. It is already the right host: it streams SSE, dispatches tools as loopback HTTP under the caller's bearer, and persists durable sessions. It was simply scoped down to food on purpose.

The user wants the full coaching capability — including training writes — from the Flutter companion, with a human in the loop on the consequential writes.

## What Changes

- **The planner becomes the coach (unified, one chat).** `/chat`'s curated allowlist grows from ~14 nutrition tools to the full coach surface, and its system prompt is rewritten from "nutrition planning ONLY, redirect everything else" to a grounded endurance-coaching persona. The meal-planning behavior is retained as a subset, not a separate mode.
- **A shared tool registry (`internal/agenttools`).** Both `internal/mcpserver` (127 tool registrations) and `internal/chat`'s allowlist already do "exactly one loopback HTTP call per tool under the caller's bearer." This change introduces the shared registry and has **`internal/chat`** consume it; porting mcpserver's 127 tools (and rewriting its 201 tests) is deferred to a later change as the highest-risk, zero-user-value slice that would churn the working desktop coach. A drift-guard test keeps chat's surface in parity with mcpserver's announced tools meanwhile (see design D4).
- **Tiered, human-confirmed writes via pause-and-resume.** Each tool carries a tier: `read` (never gated), `write-auto` (the existing low-stakes planner writes — plan a meal, add shopping items, import a recipe — fire as today), and `write-confirm` (training/goal/destructive writes). When an assistant turn contains any `write-confirm` call, the loop does **not** dispatch it: it emits a `proposal` event describing the pending writes, persists the assistant turn, and ends the stream with `stop_reason: "awaiting_confirmation"`. The phone renders a confirm card; on the user's decision a new `POST /chat/sessions/{id}/confirm` request executes the approved calls (declined ones get a synthetic "user declined" `tool_result`) and resumes the loop, streaming the continuation. This survives the app backgrounding for minutes — no connection is held open.
- **Companion UI: chat grows up into the coach.** The Chat screen drops its "scoped to nutrition planning" framing, renders the new `proposal` event as an approve/reject card (an evolution of the existing tool-status chips), and resumes via the confirm endpoint.

## Capabilities

### New Capabilities

_None — all changes extend existing capabilities._

### Modified Capabilities

- `nutrition-chat`: the SSE contract gains a fifth event (`proposal`) and the `awaiting_confirmation` stop reason; the tool allowlist requirement is replaced by the full coach surface with a tier classification; the system-prompt requirement is replaced (planner → coach); a new requirement specifies the pause-and-resume confirmation protocol and the `/chat/sessions/{id}/confirm` endpoint (per-item decisions, implicit-reject on a new message, state-driven recovery).
- `chat-sessions`: `GET /chat/sessions/{id}` gains a `pending_confirmation` object (server-composed previews) and the list headers gain an `awaiting_confirmation` flag, so a killed-and-reopened app can reconstruct the confirmation card.
- `mobile-companion`: the "Chat is scoped to nutrition planning" requirement is replaced by "Chat is the coach"; a new requirement covers the write-confirm card and its resume flow. **BREAKING** at the spec level (the v1 scoping promise is retired).

## Impact

- **Backend** (`internal/chat/`, `internal/mcpserver/`): a shared tool-spec registry (likely a new `internal/agenttools/` or similar — decide in design) both packages consume; the agent loop's inner dispatch gains a pause branch keyed on tool tier; `sanitizeHistory`/`opensCleanly` learn to preserve an intentionally-paused trailing `tool_use` turn; new confirm handler + service path; new `proposal` SSE event; system-prompt rewrite. Wiring in `internal/httpserver/server.go`.
- **No migration.** A paused turn is just a stored assistant turn with an unanswered `tool_use` — the existing `chat_sessions`/`chat_messages` full-fidelity store already represents it; confirmation state is derived from the trailing turn.
- **MCP server behavior unchanged** — the refactor is internal; the desktop coach's tool surface is identical after it consumes the shared registry. Bump the `mcp_integration_test.go` expected-tools list only if names move.
- **App code** (`apps/companion/lib/ui/chat/`): proposal-card rendering, confirm/reject actions calling the resume endpoint, prompt/scope copy changes.
- **Sequencing.** This is the spine of a small arc and may be split at apply time: (1) shared registry refactor (no behavior change), (2) confirm protocol + tiering on the existing surface, (3) broaden allowlist + coach prompt, (4) companion UI. Each is independently shippable in that order.

## Non-goals

- No second chat surface or per-session "mode" switch — the planner and coach are one assistant (per the user's choice of the unified approach).
- No held-open-connection / WebSocket approval channel — pause-and-resume across stateless turns is the chosen mechanism.
- No change to the EA/summary unit-isolation rules or the Loucks formula; the coach reads context, it does not merge new derived totals.
- No per-user scoping — single-user project; one bearer identity owns every session.
