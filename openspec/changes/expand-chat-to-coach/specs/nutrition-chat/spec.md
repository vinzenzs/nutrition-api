## MODIFIED Requirements

### Requirement: POST /chat runs a server-side agent loop and streams SSE

The system SHALL expose `POST /chat` accepting `{session_id, message}` — the id of an existing chat session and the single new user message — and responding `text/event-stream` with exactly five event types: `text` (assistant output delta), `tool` (`{id, name, status: started|ok|error, summary}` — a tool-call identity, the tool name, and the outcome only, never raw request/response bodies), `proposal` (`{turn_id, calls:[{tool_id, name, tier, preview}]}` — pending `write-confirm` calls awaiting the user's decision, where `preview` is a short server-composed human description of each write, never a raw body), `done` (`{message, stop_reason, usage}` carrying the complete final assistant text), and `error` (terminal, typed code). For each dispatched tool call the loop SHALL emit a `started` event before dispatch and a terminal `ok`/`error` event after, both carrying the same `id`. The server SHALL load the session's stored turns (truncated to the most recent `CHAT_MAX_HISTORY_MESSAGES`) as the conversation history, persist the new user message and every assistant/`tool_result` turn the loop produces, and bump the session's last-activity timestamp. The loop SHALL call Anthropic's Messages API with streaming, using the model from `CHAT_MODEL` (default `claude-sonnet-4-6`), authenticated via `ANTHROPIC_API_KEY`. A missing or unknown `session_id` SHALL return `404 session_not_found` and an empty `message` SHALL return `400 empty_message`, both before any stream is started. A `/chat` message sent while the session's trailing turn awaits a write confirmation SHALL **implicitly reject** the pending `write-confirm` calls — appending a declined `tool_result` for each so the conversation history is well-formed — and then proceed with the new message in the same turn.

#### Scenario: A grounded recommendation streams text and tool events

- **WHEN** the client POSTs `{session_id, message: "what should I eat today?"}` for an existing session
- **THEN** the response streams `tool` events for the grounding reads (e.g. `get_daily_context`) followed by `text` deltas with the recommendation
- **AND** terminates with one `done` event whose `message` equals the concatenated deltas and whose `usage` reports upstream token counts

#### Scenario: A write-confirm call pauses the stream with a proposal

- **WHEN** the assistant turn requests a `write-confirm` tool (e.g. scheduling a workout)
- **THEN** the loop does NOT dispatch it, emits a `proposal` event describing the pending write(s), and terminates with a `done` event carrying `stop_reason: "awaiting_confirmation"`
- **AND** the assistant turn (its `tool_use` blocks, no `tool_result` yet) is persisted so the session can be resumed

#### Scenario: The card preview reflects the actual request, not the model's narration

- **WHEN** a `proposal` event is emitted for a pending `write-confirm` call
- **THEN** each call's `preview` is composed by the server from the call's input (deterministically), so it cannot disagree with the request that will be sent on approval
- **AND** the assistant's own framing of the change appears only in the `text` stream, not as the card's authoritative preview

#### Scenario: A turn of only read/write-auto calls does not pause

- **WHEN** the assistant turn requests only `read` and `write-auto` tools (e.g. `get_daily_context` then `add_shopping_items`)
- **THEN** every call is dispatched inline as today, emitting paired `started`/`ok` `tool` events, with no `proposal` event

#### Scenario: A new message implicitly rejects pending writes

- **WHEN** the client POSTs a new `/chat` message for a session whose trailing turn awaits a write confirmation
- **THEN** the server appends a declined `tool_result` for each pending `write-confirm` call (so the history never opens on a dangling `tool_use`)
- **AND** proceeds with the new message, streaming normally — the pending writes never fire

#### Scenario: Turns are persisted and resumable

- **WHEN** a `/chat` turn completes for a session
- **THEN** the new user message and the assistant/tool turns are persisted to that session in order
- **AND** a subsequent `/chat` turn for the same session loads them as prior context without the client resending any transcript

#### Scenario: Unknown session refuses before streaming

- **WHEN** the client POSTs a `session_id` that does not exist
- **THEN** the response is `404 session_not_found` as plain JSON and no SSE stream is started

#### Scenario: Missing API key refuses before streaming

- **WHEN** `ANTHROPIC_API_KEY` is unset and the client POSTs to `/chat`
- **THEN** the response is `503 chat_unavailable` as a plain JSON error (no SSE stream is started)

#### Scenario: Upstream failure mid-stream emits a typed error event

- **WHEN** the Anthropic API returns a 429 or 5xx after streaming has begun
- **THEN** the stream emits an `error` event with code `upstream_unavailable` and terminates
- **AND** the user turn and any already-completed tool rounds remain persisted to the session, so a retry resumes with full context rather than replaying

### Requirement: The tool surface is the full coach surface, tiered for confirmation

The chat loop SHALL expose a **curated** coaching tool surface (~15–25 tools), not the desktop MCP server's full surface — favoring a few **aggregate context reads** (e.g. `get_daily_context`, `get_training_context`, `get_recovery_context`) over many granular reads, plus the `write-auto` meal-planning writes and the `write-confirm` actions worth proposing conversationally — sourced from a shared tool registry (`internal/agenttools`). A drift-guard test SHALL assert every tool the chat loop exposes exists in the desktop MCP server's announced surface (modulo a documented allowlist of chat-bespoke convenience tools), so the two cannot silently diverge even though the MCP server is not yet ported onto the shared registry. Each tool SHALL carry a `tier`: `read` (never gated), `write-auto` (low-stakes nutrition-planning writes that dispatch inline), or `write-confirm` (training, goal, and destructive writes that pause for human confirmation). The existing planner writes — `import_cookidoo_recipe`, `update_product`, `create_planned_meal`, `update_planned_meal`, `mark_planned_meal_eaten`, `add_shopping_items`, `update_shopping_item`, `clear_checked_shopping_items` — SHALL be `write-auto`. Training/Garmin/goal/override edits and all delete endpoints SHALL be `write-confirm`. The loop SHALL also include Anthropic's `web_search` server tool restricted via `allowed_domains` to Cookidoo hosts.

#### Scenario: Planner writes stay fast

- **WHEN** the user accepts three days of dinners
- **THEN** the `create_planned_meal` and `add_shopping_items` calls dispatch inline without a confirmation pause (they are `write-auto`)

#### Scenario: Coaching reads are aggregate, not granular

- **WHEN** the coach grounds before training advice
- **THEN** it calls a small number of aggregate context reads (e.g. `get_training_context`, `get_recovery_context`) rather than many granular per-metric Garmin tools
- **AND** the exposed surface stays around 15–25 tools, not the desktop server's full 127

#### Scenario: Training and destructive writes are gated

- **WHEN** the agent requests a goal change, a workout schedule, or any delete
- **THEN** that call is `write-confirm` and the turn pauses with a `proposal` rather than dispatching

#### Scenario: Chat sources from the shared registry, guarded against drift

- **WHEN** the chat tool defs are constructed
- **THEN** they derive from the shared `internal/agenttools` registry (name, schema, loopback HTTP mapping, tier)
- **AND** a test asserts every exposed tool name exists in the MCP server's announced surface (modulo the documented chat-bespoke allowlist), failing if a coach tool is added to one surface but not reconciled with the other

### Requirement: The system prompt scopes the assistant to grounded endurance coaching

The system prompt SHALL be assembled server-side from a baked-in template plus config (`CHAT_DIETARY_PREFERENCES`, default `vegetarian`; the user timezone) and MUST NOT be overridable by the client request. It SHALL: cast the assistant as the user's endurance-fueling and training coach (nutrition planning is a retained subset, not the whole scope); mandate grounding reads before any recommendation — daily nutrition context before food advice, and training/recovery/Garmin context before training advice; honor the dietary preference; retain the meal-planning selection contract (on choice, persist planned meals for the agreed dates plus one consolidated merged shopping list); forbid inventing nutriment or metric values; and defer medical questions to a professional. The prompt SHALL direct the assistant to **proactively propose** `write-confirm` actions when its advice implies a change (the card is the user's commit point, so proposing is safe), while **batching** related changes into a single proposal, **not re-proposing** an action the user declined earlier in the conversation, and stating plainly in the text what a proposed write will do.

#### Scenario: Client cannot override the system prompt

- **WHEN** the client transcript contains a `system` role message
- **THEN** the request is rejected with a `400` validation error

#### Scenario: Coaching answers are grounded

- **WHEN** the user asks "should I do my long run tomorrow given my recovery?"
- **THEN** the upstream conversation shows recovery/training grounding tool calls preceding the advice

#### Scenario: The coach proactively proposes a change it recommends

- **WHEN** grounding reveals a gap the coach recommends fixing (e.g. a low protein goal for the training block)
- **THEN** the coach may propose the corresponding `write-confirm` action in the same turn (surfacing a card), rather than only describing it
- **AND** nothing is written until the user approves

#### Scenario: Medical questions are deferred

- **WHEN** the user asks for a medical diagnosis or treatment
- **THEN** the coach declines and suggests a professional, without proposing any write

#### Scenario: Meal planning still works as a subset

- **WHEN** the user asks "what should I eat tomorrow?"
- **THEN** the assistant grounds via `get_daily_context` and offers 2–3 options consistent with the configured dietary preference, exactly as the prior planner did

## ADDED Requirements

### Requirement: Write-confirm calls resume via an explicit confirmation endpoint

The system SHALL expose `POST /chat/sessions/{id}/confirm` accepting `{decisions: [{tool_id, approve}]}` that resolves a session paused on a `write-confirm` turn and resumes the agent loop, streaming SSE with the same event contract as `/chat`. The decisions MUST cover exactly the pending `write-confirm` calls of the session's trailing assistant turn. On resume the server SHALL dispatch the approved calls (and any `write-auto`/`read` calls in the same turn) in their original order, synthesize a declined `tool_result` ("the user declined this action") for each rejected call, append one `tool_result` user turn containing all results, and continue the loop. Dispatched writes SHALL derive their idempotency key from the tool name and canonical input exactly as in `/chat`, so a re-sent confirmation replays rather than double-writes. Resume SHALL be state-driven: when the trailing turn is a `tool_result` with no following assistant turn (a continuation owed after a prior resume's stream died mid-answer), the endpoint SHALL ignore `decisions` and simply continue the loop, so re-sending the same confirmation recovers the answer without re-executing the writes. The endpoint SHALL return `404 session_not_found` for an unknown session, `409 nothing_to_confirm` when the trailing turn is a completed assistant text turn, and `400 invalid_confirmation` when the decisions do not match the pending calls — all before any stream is started.

#### Scenario: Approving a proposal executes the write and resumes

- **WHEN** the client POSTs `confirm` approving the pending `schedule_workout` call
- **THEN** the server dispatches it (emitting `started`/`ok` `tool` events), appends its `tool_result`, and resumes streaming the assistant's continuation to a `done` event

#### Scenario: Rejecting a proposal continues without the write

- **WHEN** the client POSTs `confirm` rejecting the pending call
- **THEN** no write is dispatched, a synthetic declined `tool_result` is appended, and the loop resumes so the agent can adapt its reply

#### Scenario: Re-sent confirmation does not double-write

- **WHEN** an identical `confirm` is sent twice (e.g. after a dropped connection) for the same approved call
- **THEN** the second dispatch carries the same conversation-position idempotency key and returns the original response without inserting a duplicate

#### Scenario: Confirming a session that is not paused is refused

- **WHEN** the client POSTs `confirm` for a session whose trailing turn is not awaiting confirmation
- **THEN** the response is `409 nothing_to_confirm` and no stream is started

### Requirement: A session awaiting confirmation is preserved across history loads

The conversation-history loader SHALL preserve a session's trailing assistant turn whose `write-confirm` `tool_use` blocks are unanswered (the resume anchor for a pending confirmation), while still dropping a `tool_use` turn left dangling by truncation. Only the confirmation endpoint SHALL answer the preserved turn.

#### Scenario: Paused turn survives a reload

- **WHEN** a session paused on a `write-confirm` turn is reloaded (e.g. the app is backgrounded and reopened)
- **THEN** the trailing unanswered assistant `tool_use` turn is retained, and `POST /chat/sessions/{id}/confirm` can still resolve it

#### Scenario: Truncation litter is still dropped

- **WHEN** history truncation leaves a non-trailing `tool_use` turn without its `tool_result`
- **THEN** the loader drops it so the upstream `messages` array never opens on a dangling `tool_use`
