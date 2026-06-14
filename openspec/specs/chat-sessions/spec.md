# chat-sessions Specification

## Purpose
TBD - created by archiving change add-chat-sessions. Update Purpose after archive.
## Requirements
### Requirement: Chat conversations are persisted as sessions with full-fidelity turns

The system SHALL persist chat conversations server-side as `chat_sessions` (one header row per conversation) and `chat_messages` (one row per turn, ordered, deleted by cascade when its session is deleted). Each persisted turn SHALL store the complete Anthropic content-block value at full fidelity — a JSON string for plain user text, or the verbatim content-block array including `tool_use` and `tool_result` blocks for assistant and tool turns — so a resumed session reconstructs the exact upstream context. Turn order within a session SHALL be stable and reflect the order turns were produced, including multiple turns created within a single request.

#### Scenario: A completed turn stores its full content-block array

- **WHEN** a `/chat` request produces an assistant turn containing `tool_use` blocks followed by a `tool_result` user turn
- **THEN** each turn is persisted as a `chat_messages` row carrying its verbatim content-block array
- **AND** loading the session reproduces the same ordered blocks that were sent upstream

#### Scenario: Deleting a session removes its turns

- **WHEN** a session with persisted turns is deleted
- **THEN** the session header and all of its `chat_messages` rows are removed

### Requirement: POST /chat/sessions creates a session

The system SHALL expose `POST /chat/sessions` accepting an optional `{title}` and returning `201` with the session header `{id, title, created_at, updated_at, last_message_at}`. An absent or empty `title` SHALL create an untitled session. The endpoint SHALL honor an `Idempotency-Key` header per the standard write-path contract.

#### Scenario: Create with a title

- **WHEN** the client POSTs `{"title":"Race week meals"}`
- **THEN** the response is `201` with the session header and `title` equal to `"Race week meals"`

#### Scenario: Create without a title

- **WHEN** the client POSTs `{}`
- **THEN** the response is `201` with an untitled session header

### Requirement: GET /chat/sessions lists sessions most-recent-first

The system SHALL expose `GET /chat/sessions` returning session headers (no transcript) ordered by most recent activity first (by `last_message_at`, then `created_at`). The list SHALL NOT include the per-turn messages. Each header SHALL carry an `awaiting_confirmation` boolean indicating whether the session's trailing turn is paused awaiting a write confirmation, so the history view can badge it.

#### Scenario: Sessions are ordered by recency

- **WHEN** session A received a turn after session B was created
- **THEN** `GET /chat/sessions` returns A before B
- **AND** neither entry includes a `messages` array

#### Scenario: An awaiting-confirmation session is flagged

- **WHEN** a session's trailing turn is paused awaiting a write confirmation
- **THEN** its list header carries `awaiting_confirmation: true`

### Requirement: GET /chat/sessions/{id} returns the session with its full transcript

The system SHALL expose `GET /chat/sessions/{id}` returning the session header plus its ordered turns at full fidelity — each turn's `role` and verbatim content-block `content`. An unknown id SHALL return `404 session_not_found`. When the session's trailing turn is paused awaiting a write confirmation, the response SHALL include a `pending_confirmation: {turn_id, calls:[{tool_id, name, tier, preview}]}` object — the same pending `write-confirm` calls and server-composed previews a live `proposal` SSE event would carry — and SHALL be `null` otherwise. This lets a client that (re)opens a session reconstruct the confirmation card without re-deriving previews from raw `tool_use` blocks.

#### Scenario: Fetch a session with turns

- **WHEN** the client GETs an existing session that has turns
- **THEN** the response includes the header and an ordered `messages` array whose entries carry `role` and the raw content blocks

#### Scenario: Fetch a session awaiting confirmation

- **WHEN** the client GETs a session whose trailing turn is paused awaiting a write confirmation
- **THEN** the response includes a `pending_confirmation` object listing each pending call's `tool_id`, `name`, `tier`, and human `preview`

#### Scenario: Unknown session

- **WHEN** the client GETs a session id that does not exist
- **THEN** the response is `404 session_not_found`

### Requirement: PATCH /chat/sessions/{id} renames a session

The system SHALL expose `PATCH /chat/sessions/{id}` accepting `{title}` and updating the session title and `updated_at`. A `title` of `""` SHALL clear the title to untitled. An unknown id SHALL return `404 session_not_found`.

#### Scenario: Rename a session

- **WHEN** the client PATCHes `{"title":"Taper week"}` on an existing session
- **THEN** the session's title becomes `"Taper week"` and `updated_at` advances

#### Scenario: Clear the title

- **WHEN** the client PATCHes `{"title":""}`
- **THEN** the session becomes untitled

### Requirement: DELETE /chat/sessions/{id} deletes a session

The system SHALL expose `DELETE /chat/sessions/{id}` removing the session and cascading its turns, returning `204`. An unknown id SHALL return `404 session_not_found`.

#### Scenario: Delete an existing session

- **WHEN** the client DELETEs an existing session
- **THEN** the response is `204` and the session and its turns no longer exist

#### Scenario: Delete a missing session

- **WHEN** the client DELETEs a session id that does not exist
- **THEN** the response is `404 session_not_found`

### Requirement: Session persistence is independent of chat availability

The system SHALL keep the `/chat/sessions` CRUD surface available even when `ANTHROPIC_API_KEY` is unset (sessions are data, not model calls); only `POST /chat` itself returns `503 chat_unavailable` in that state.

#### Scenario: CRUD works without an API key

- **WHEN** the server runs without `ANTHROPIC_API_KEY` and the client creates and lists sessions
- **THEN** the create and list succeed
- **AND** a subsequent `POST /chat` for that session returns `503 chat_unavailable`

