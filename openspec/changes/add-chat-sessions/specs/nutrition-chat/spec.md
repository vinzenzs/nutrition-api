## MODIFIED Requirements

### Requirement: POST /chat runs a server-side agent loop and streams SSE

The system SHALL expose `POST /chat` accepting `{session_id, message}` — the id of an existing chat session and the single new user message — and responding `text/event-stream` with exactly four event types: `text` (assistant output delta), `tool` (`{name, status: started|ok|error, summary}` — tool name and outcome only, never raw request/response bodies), `done` (`{message, stop_reason, usage}` carrying the complete final assistant text), and `error` (terminal, typed code). The server SHALL load the session's stored turns (truncated to the most recent `CHAT_MAX_HISTORY_MESSAGES`) as the conversation history, persist the new user message and every assistant/`tool_result` turn the loop produces, and bump the session's last-activity timestamp. The loop SHALL call Anthropic's Messages API with streaming, using the model from `CHAT_MODEL` (default `claude-sonnet-4-6`), authenticated via `ANTHROPIC_API_KEY`. A missing or unknown `session_id` SHALL return `404 session_not_found`, and an empty `message` SHALL return `400 empty_message`, both before any stream is started.

#### Scenario: A grounded recommendation streams text and tool events

- **WHEN** the client POSTs `{session_id, message: "what should I eat today?"}` for an existing session
- **THEN** the response streams `tool` events for the grounding reads (e.g. `get_daily_context`) followed by `text` deltas with the recommendation
- **AND** terminates with one `done` event whose `message` equals the concatenated deltas and whose `usage` reports upstream token counts

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
