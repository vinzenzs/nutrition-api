## MODIFIED Requirements

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
