## MODIFIED Requirements

### Requirement: Chat screen streams the server conversation and is online-only

The Chat screen SHALL hold the current `session_id` and send `{session_id, message}` to `POST /chat` (creating a session via `POST /chat/sessions` when starting a new conversation), and render the SSE stream live: `text` deltas append to the current assistant bubble; `tool` events render as transient activity chips (name + outcome summary, red on error); `done` finalizes the bubble from the event's complete message; `error` marks the turn failed with a retry affordance that resubmits the identical turn. The server session SHALL be the source of truth for transcript content; the app MAY cache the rendered transcript locally for instant display but SHALL NOT treat a local copy as authoritative, and a "new chat" action SHALL create a fresh server session. The active `session_id` MAY instead be one reopened from history (see the session-history requirement), in which case new turns append to that existing server session. The Chat app bar SHALL expose a history affordance opening the session browser. When connectivity is absent the composer SHALL disable with an inline notice while the cached transcript stays readable; when the server returns `503 chat_unavailable` the screen SHALL show a "not configured on this server" state. Recipe references carrying an `external_url` SHALL render as tappable chips opening Cookidoo externally.

#### Scenario: Streamed turn renders deltas then finalizes

- **WHEN** the user sends "what should I eat today?" and the stream emits tool events, text deltas, and a done event
- **THEN** activity chips appear during tool execution, the assistant bubble grows with each delta, and the final bubble content equals the `done` event's message

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

## ADDED Requirements

### Requirement: Chat session history is browsable, reopenable, and manageable

The companion SHALL provide a session-history screen, reached from the Chat app bar, that lists past conversations newest-first by fetching `GET /chat/sessions`, showing each session's title (or an "Untitled" placeholder when the backend has none) and a relative last-activity time. The screen SHALL be online-only and SHALL NOT maintain an offline cache of the list; it SHALL refetch on open and surface loading, empty, and error (retryable) states. Tapping a session SHALL reopen it: the app fetches `GET /chat/sessions/{id}`, reconstructs the visible transcript (user text turns and assistant text; `tool_use` / `tool_result` blocks are omitted from the rendered view), adopts that `session_id` as the active conversation, and returns to the Chat screen so the user can continue — new turns appending to the same server session. The screen SHALL allow deleting a session via `DELETE /chat/sessions/{id}` and renaming a session via `PATCH /chat/sessions/{id}` (an empty title clears to untitled). Deleting the currently-active session SHALL reset the Chat screen to a fresh conversation.

#### Scenario: Browse lists conversations newest-first

- **WHEN** the user opens the session-history screen with several stored sessions
- **THEN** the screen fetches `GET /chat/sessions` and renders them most-recent-first with title and relative time
- **AND** no transcript bodies are loaded for the list

#### Scenario: Reopen and continue

- **WHEN** the user taps a past session
- **THEN** the app loads its transcript, renders the visible text turns, and adopts its `session_id`
- **AND** a subsequent message is sent as `{session_id, message}` against that same session and appends to it

#### Scenario: Reopened transcript renders text only

- **WHEN** a reopened session contains assistant turns with `tool_use` blocks and `tool_result` replies
- **THEN** the rendered transcript shows the user and assistant text bubbles without tool-activity chips

#### Scenario: Delete a session

- **WHEN** the user deletes a session from the list
- **THEN** the app calls `DELETE /chat/sessions/{id}` and removes it from the list
- **AND** if it was the active conversation, the Chat screen resets to a fresh chat

#### Scenario: Rename a session

- **WHEN** the user renames a session to "Race week meals"
- **THEN** the app calls `PATCH /chat/sessions/{id}` and the list shows the new title

#### Scenario: Offline browse fails into a retryable error

- **WHEN** the device has no connectivity and the user opens the session-history screen
- **THEN** the screen shows a retryable error state rather than a stale cached list
