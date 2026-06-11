# mobile-companion — delta for add-companion-chat

## MODIFIED Requirements

### Requirement: The mobile companion is a focused supplement to the agent, not a replacement

The mobile companion app SHALL implement exactly four screens (Today, Camera, Recent, Chat) plus a Settings sheet and a shopping list screen reachable from the Today header. The Chat screen is the constrained nutrition-planning chat backed by the server's `nutrition-chat` capability — deep coaching, goal editing, and training analysis remain with the desktop agent. The app SHALL NOT include an in-app recipe builder or a generalized product search experience.

#### Scenario: Four screens after chat activation

- **WHEN** the user opens the app
- **THEN** the bottom navigation surfaces Today, Camera, Recent, and Chat as the four primary destinations (the formerly reserved fourth slot is now active)
- **AND** Settings and the shopping list are reachable from the Today screen's top-right
- **AND** no recipe builder or all-products search screen exists

#### Scenario: Chat is scoped to nutrition planning

- **WHEN** the server's chat assistant receives an out-of-scope request (per the nutrition-chat system prompt)
- **THEN** the app renders the assistant's redirect-to-desktop-coach reply as ordinary chat content without any app-side filtering

### Requirement: Stale-while-revalidate reads with no offline banner

GET-driven screens (Today, Recent, the product card after a scan, the plan card, the shopping list) SHALL render from a local cache immediately and trigger a background revalidation on screen activation. The app SHALL NOT display an "offline" banner; the data's freshness is implicit in its presence. The sole permitted exception is the chat composer's inline connectivity notice defined by the chat-screen requirement — it MUST NOT appear anywhere outside the chat composer.

#### Scenario: Today renders cached data instantly

- **WHEN** the user opens Today
- **THEN** the screen renders adherence rings and recent meals from `recent_summary` within 16ms (no loading spinner)
- **AND** the app issues `GET /summary/daily?date=...&tz=...` in the background
- **AND** the screen updates if the fresh response differs

#### Scenario: Stale data is shown without warning

- **WHEN** the local cache is older than five minutes and the network is unreachable
- **THEN** Today still renders from the cache
- **AND** no banner, badge, or modal indicates offline state

#### Scenario: Offline shopping list stays bannerless

- **WHEN** the network is unreachable and the user opens the shopping list
- **THEN** cached items render and check-offs enqueue in the outbox
- **AND** no offline indicator appears on the screen

## ADDED Requirements

### Requirement: Chat screen streams the server conversation and is online-only

The Chat screen SHALL send the client-held transcript to `POST /chat` and render the SSE stream live: `text` deltas append to the current assistant bubble; `tool` events render as transient activity chips (name + outcome summary, red on error); `done` finalizes the bubble from the event's complete message; `error` marks the turn failed with a retry affordance that resubmits the identical turn. Transcripts SHALL persist locally (Drift) with one active conversation and a "new chat" reset; transcripts are never synced to the server. When connectivity is absent the composer SHALL disable with an inline notice while the transcript stays readable; when the server returns `503 chat_unavailable` the screen SHALL show a "not configured on this server" state. Recipe references carrying an `external_url` SHALL render as tappable chips opening Cookidoo externally.

#### Scenario: Streamed turn renders deltas then finalizes

- **WHEN** the user sends "what should I eat today?" and the stream emits tool events, text deltas, and a done event
- **THEN** activity chips appear during tool execution, the assistant bubble grows with each delta, and the final bubble content equals the `done` event's message

#### Scenario: Dropped stream offers a safe retry

- **WHEN** the SSE connection drops mid-turn
- **THEN** the turn shows a failed state with a retry action
- **AND** retrying resubmits the identical transcript (server-side idempotency replays any completed writes without duplicates)

#### Scenario: Offline disables only the composer

- **WHEN** the device has no connectivity and the user opens Chat
- **THEN** the transcript renders read-only, the composer is disabled with an inline "Chat needs a connection" notice
- **AND** no other screen shows any offline indicator

#### Scenario: Cookidoo links open externally

- **WHEN** an assistant message references an imported recipe with an `external_url`
- **THEN** a chip with the recipe name is tappable and opens the Cookidoo page in the system browser

### Requirement: Today screen surfaces the day's plan with one-tap eaten

The Today screen SHALL show a plan card listing today's planned meals (from cached `GET /plan` range reads, stale-while-revalidate) with, per entry: product name, slot, quantity, and two actions — "Ate it" (`POST /plan/{id}/eaten` through the outbox with a client-minted idempotency key, optimistic flip to eaten) and skip (`PATCH` to `skipped`). While a prior eaten/skip for the same entry is pending in the outbox, the entry's actions SHALL be disabled. An outbox failure (e.g. `409 plan_entry_already_eaten`) SHALL revert the optimistic state and surface a toast; the next revalidation reconciles. The card SHALL be absent when today has no plan entries.

#### Scenario: One-tap eaten flips optimistically and logs through the outbox

- **WHEN** the user taps "Ate it" on tonight's planned dinner
- **THEN** the entry immediately renders as eaten and the eaten call is enqueued with a client-minted idempotency key
- **AND** when the outbox replays successfully, the next summary revalidation reflects the logged meal in adherence

#### Scenario: Pending transition blocks double-taps

- **WHEN** an eaten call for an entry is still pending in the outbox
- **THEN** that entry's "Ate it" and skip actions are disabled with a syncing hint

#### Scenario: Conflict reverts the optimistic flip

- **WHEN** the outbox replay returns `409 plan_entry_already_eaten` (e.g. marked eaten from the desktop concurrently)
- **THEN** the optimistic state reverts, a toast explains, and the entry shows the server truth after revalidation

### Requirement: Shopping list screen with offline check-off

The shopping list screen SHALL be reachable from a cart icon in the Today header badged with the open-item count. It SHALL render cached items stale-while-revalidate (unchecked first, in creation order), support check/uncheck via outboxed `PATCH /shopping/items/{id}` with optimistic toggle, and offer a "clear bought" action invoking `DELETE /shopping/items?checked=true`. Item creation in-app is a single minimal "add item" affordance (name + optional quantity text) — list composition belongs to the agents.

#### Scenario: Store usage works fully offline

- **WHEN** the user opens the shopping list with no connectivity and checks off six items
- **THEN** all six toggle optimistically and enqueue in the outbox
- **AND** they replay in order once connectivity returns

#### Scenario: Cart badge reflects open items

- **WHEN** the cached list has 9 unchecked items
- **THEN** the Today header cart icon shows a badge of 9
- **AND** the badge updates when items are checked or added (locally or via revalidation)

#### Scenario: Clear bought is explicit

- **WHEN** the user taps "clear bought" with 4 checked items
- **THEN** a confirmation states the count, and on confirm the qualified bulk delete is enqueued
- **AND** unchecked items are untouched
