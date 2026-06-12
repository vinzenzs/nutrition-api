## MODIFIED Requirements

### Requirement: Chat screen streams the server conversation and is online-only

The Chat screen SHALL hold the current `session_id` and send `{session_id, message}` to `POST /chat` (creating a session via `POST /chat/sessions` when starting a new conversation), and render the SSE stream live: `text` deltas append to the current assistant bubble; `tool` events render as transient activity chips (name + outcome summary, red on error); `done` finalizes the bubble from the event's complete message; `error` marks the turn failed with a retry affordance that resubmits the identical turn. The server session SHALL be the source of truth for transcript content; the app MAY cache the rendered transcript locally for instant display but SHALL NOT treat a local copy as authoritative, and a "new chat" action SHALL create a fresh server session. When connectivity is absent the composer SHALL disable with an inline notice while the cached transcript stays readable; when the server returns `503 chat_unavailable` the screen SHALL show a "not configured on this server" state. Recipe references carrying an `external_url` SHALL render as tappable chips opening Cookidoo externally.

#### Scenario: Streamed turn renders deltas then finalizes

- **WHEN** the user sends "what should I eat today?" and the stream emits tool events, text deltas, and a done event
- **THEN** activity chips appear during tool execution, the assistant bubble grows with each delta, and the final bubble content equals the `done` event's message

#### Scenario: New chat creates a server session

- **WHEN** the user starts a new conversation
- **THEN** the app creates a session via `POST /chat/sessions` and sends subsequent turns with that `session_id`

#### Scenario: Dropped stream offers a safe retry

- **WHEN** the SSE connection drops mid-turn
- **THEN** the turn shows a failed state with a retry action
- **AND** retrying resubmits the identical `{session_id, message}` (the server resumes from the persisted session and replays any completed writes without duplicates)

#### Scenario: Offline disables only the composer

- **WHEN** the device has no connectivity and the user opens Chat
- **THEN** the cached transcript renders read-only, the composer is disabled with an inline "Chat needs a connection" notice
- **AND** no other screen shows any offline indicator

#### Scenario: Cookidoo links open externally

- **WHEN** an assistant message references an imported recipe with an `external_url`
- **THEN** a chip with the recipe name is tappable and opens the Cookidoo page in the system browser
