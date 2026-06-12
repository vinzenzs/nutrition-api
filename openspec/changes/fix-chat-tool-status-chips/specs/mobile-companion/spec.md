# mobile-companion — delta for fix-chat-tool-status-chips

## MODIFIED Requirements

### Requirement: Chat screen streams the server conversation and is online-only

The Chat screen SHALL hold the current `session_id` and send `{session_id, message}` to `POST /chat` (creating a session via `POST /chat/sessions` when starting a new conversation), and render the SSE stream live: `text` deltas append to the current assistant bubble; `tool` events render as activity chips, **coalesced by the event `id`** so a call's `started` and terminal `ok`/`error` events update **one** chip in place rather than appending a second — the chip is **labeled by the tool name**, its status shown by the avatar icon (running, done, or error) with an error styled red, and its outcome `summary` surfaced only on error; `done` finalizes the bubble from the event's complete message; `error` marks the turn failed with a retry affordance that resubmits the identical turn. There SHALL NOT be a residual "running" chip once a call has completed. The server session SHALL be the source of truth for transcript content; the app MAY cache the rendered transcript locally for instant display but SHALL NOT treat a local copy as authoritative, and a "new chat" action SHALL create a fresh server session. The active `session_id` MAY instead be one reopened from history (see the session-history requirement), in which case new turns append to that existing server session. The Chat app bar SHALL expose a history affordance opening the session browser. When connectivity is absent the composer SHALL disable with an inline notice while the cached transcript stays readable; when the server returns `503 chat_unavailable` the screen SHALL show a "not configured on this server" state. Recipe references carrying an `external_url` SHALL render as tappable chips opening Cookidoo externally.

#### Scenario: Streamed turn renders deltas then finalizes

- **WHEN** the user sends "what should I eat today?" and the stream emits tool events, text deltas, and a done event
- **THEN** activity chips appear during tool execution, the assistant bubble grows with each delta, and the final bubble content equals the `done` event's message

#### Scenario: A tool call renders one chip that transitions running to done

- **WHEN** the stream emits a `started` tool event and then an `ok` tool event with the same `id`
- **THEN** exactly one chip exists for that `id`, labeled by the tool name
- **AND** its icon transitions from running to done when the second event arrives
- **AND** no separate "running" chip remains

#### Scenario: A failed tool call shows the error on its single chip

- **WHEN** a call's terminal event has `status: "error"`
- **THEN** the same coalesced chip turns red and shows its error `summary`

#### Scenario: Two calls to the same tool render two chips

- **WHEN** the stream emits two started/terminal pairs with distinct `id`s for the same tool name
- **THEN** two chips are rendered, one per `id`

#### Scenario: New chat creates a server session

- **WHEN** the user starts a new conversation
- **THEN** the app creates a session via `POST /chat/sessions` and sends subsequent turns with that `session_id`

#### Scenario: Chat app bar opens the history browser

- **WHEN** the user taps the history affordance in the Chat app bar
- **THEN** the session-history screen opens listing past conversations

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
