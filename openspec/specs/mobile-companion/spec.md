# mobile-companion Specification

## Purpose

A focused Flutter Android companion app that supplements — not replaces — the LLM coaching agent. It exists to make three high-frequency interactions instant where typing at the agent would be slow: barcode scan → logged meal, photo capture → logged meal via the backend vision endpoint, and one-tap hydration from an Android home-screen widget. Everything else (chat, recipe building, generalized product search, goal editing) stays in the agent. The app is offline-first: every mutating call is enqueued in a local outbox with a client-minted Idempotency-Key before any network attempt and replayed exactly against the backend's idempotency contract, while reads render from a local cache stale-while-revalidate with no offline banner. The bearer token lives in platform secure storage (Android Keystore via `flutter_secure_storage`, mirrored to EncryptedSharedPreferences for the Kotlin widget) and the app is paired by scanning a QR code printed by the backend. Goals are read-only in v1 — adherence is rendered from the daily summary's `adherence` block, but goal configuration stays with the agent.
## Requirements
### Requirement: The mobile companion is a focused supplement to the agent, not a replacement

The mobile companion app SHALL implement exactly four screens (Today, Camera, Recent, Chat) plus a Settings sheet and a shopping list screen reachable from the Today header. The Chat screen is the in-app coach backed by the server's `nutrition-chat` capability — it spans nutrition planning and endurance-training coaching, including the full write surface, with consequential (training/goal/destructive) writes gated behind an in-app confirmation. The app SHALL NOT include an in-app recipe builder or a generalized product search experience.

#### Scenario: Four screens after chat activation

- **WHEN** the user opens the app
- **THEN** the bottom navigation surfaces Today, Camera, Recent, and Chat as the four primary destinations
- **AND** Settings and the shopping list are reachable from the Today screen's top-right
- **AND** no recipe builder or all-products search screen exists

#### Scenario: Chat is the coach

- **WHEN** the user asks the Chat screen a training, recovery, or fueling question
- **THEN** the app renders the assistant's grounded coaching reply as ordinary chat content
- **AND** the assistant is not limited to nutrition planning (no redirect-to-desktop-coach behavior is expected)

### Requirement: Barcode scan to logged meal in two taps when the product is cached

The camera screen's scan mode SHALL produce a logged meal in at most two user taps for products already present in the local cache. The first tap is the barcode detection moment (which happens automatically when the viewfinder recognizes a code); the second is the "Log" confirmation on the product card. The default quantity on the card SHALL be derived from, in order of preference: `products.last_logged_quantity_g`, then `serving_size_g`, then 100 grams.

#### Scenario: Cached product scan completes in two taps

- **WHEN** the user opens the camera screen with the scan mode active
- **AND** points at a barcode for a product whose row is in `products_cache`
- **THEN** the app displays the product card within 200ms of detection
- **AND** the card pre-fills the quantity with `last_logged_quantity_g` (if non-null), else `serving_size_g`, else `100`
- **AND** the card pre-fills `meal_type` from time of day (breakfast 04:00–10:59, lunch 11:00–14:59, dinner 17:00–22:59, snack otherwise)
- **AND** the "Log" button is the only required action to commit the meal entry

#### Scenario: First-time barcode triggers an OFF fetch

- **WHEN** the user scans a barcode not present in `products_cache`
- **THEN** the app calls `POST /products/lookup/{barcode}` and shows a loading state until the response
- **AND** stores the result in `products_cache` on success
- **AND** falls into the cached-product flow on success (the second tap logs)

#### Scenario: 404 on lookup surfaces the freeform/photo escape hatch

- **WHEN** the lookup returns `404 product_not_found`
- **THEN** the app shows a sheet offering "Describe it" (freeform text → `POST /meals/freeform`) or "Take a photo" (switches to photo mode → `POST /meals/from_photo`)

### Requirement: Photo capture to logged meal via the backend vision endpoint

The camera screen's photo mode SHALL upload a captured image to `POST /meals/from_photo` and present the parsed meal for confirmation. The app SHALL request JPEG output from the platform image picker so HEIC never reaches the backend.

#### Scenario: Photo capture confirmation flow

- **WHEN** the user activates photo mode, captures a photo, and accepts the preview
- **THEN** the app posts a multipart body to `POST /meals/from_photo` with the JPEG bytes, the current `quantity_g` default (100 unless changed), `logged_at = now`, and `meal_type` derived from time of day
- **AND** displays the returned `meal` block with name, nutriments-per-100g, and an editable quantity
- **AND** displays the `inference.confidence` value as a visual hint (color band: green ≥0.75, amber 0.6–0.75, red <0.6)

#### Scenario: Low confidence triggers an editable confirm sheet

- **WHEN** the returned `inference.confidence` is below 0.6
- **THEN** the app shows the parsed name and macros pre-populated in an editable sheet
- **AND** does NOT commit the meal until the user confirms
- **AND** the user can edit name, macros, and quantity before committing

#### Scenario: High confidence uses single-tap commit

- **WHEN** the returned `inference.confidence` is 0.75 or higher
- **THEN** the meal entry is already committed by the time the user sees the confirmation card
- **AND** the card includes an "undo" affordance that calls `DELETE /meals/{id}`

#### Scenario: Vision unavailable degrades to freeform

- **WHEN** the backend returns `503 vision_unavailable`
- **THEN** the app shows a single sheet: "Photo logging isn't configured on this server. Describe the meal instead?"
- **AND** the sheet's primary action falls back to a freeform text input → `POST /meals/freeform`

### Requirement: One-tap hydration via an Android home-screen widget

The system SHALL ship an Android home-screen widget that logs a configured glass of water with a single tap. The widget SHALL function regardless of whether the Flutter app is in the foreground or backgrounded. Tapping the widget SHALL produce one `POST /hydration` request with a freshly minted Idempotency-Key.

#### Scenario: Widget tap with network succeeds without waking the Flutter app

- **WHEN** the user taps the hydration widget on the home screen
- **AND** the device has network connectivity
- **AND** the Flutter app is not running
- **THEN** a Kotlin worker reads the bearer token from EncryptedSharedPreferences
- **AND** issues `POST /hydration` with the configured glass size and a fresh Idempotency-Key
- **AND** on 2xx, updates the widget's local Room snapshot and refreshes the widget UI to reflect the new total
- **AND** the Flutter app is NOT launched

#### Scenario: Widget tap with no network queues to widget_failures

- **WHEN** the user taps the widget and the `POST /hydration` request fails for network reasons (no connectivity, timeout, 5xx)
- **THEN** the Kotlin worker writes a row to the shared `widget_failures` SQLite table with the request body and the Idempotency-Key
- **AND** the widget UI optimistically reflects the increment locally
- **AND** the next foreground of the Flutter app drains `widget_failures` into `pending_writes` for queued replay

#### Scenario: Widget shows today's progress

- **WHEN** the widget is rendered
- **THEN** it shows today's hydration total as a percentage of the configured daily goal
- **AND** updates after every successful tap
- **AND** updates when the Flutter app refreshes the Room snapshot after a foreground sync

### Requirement: Offline-first outbox for every mutating call

Every mutation initiated by the Flutter app SHALL be enqueued in a local `pending_writes` table with a client-generated `Idempotency-Key` before any network attempt. A queue worker SHALL flush pending rows in arrival order, retry on transient failure, and respect the backend's idempotency contract so replays are exact.

#### Scenario: Mutation is enqueued before any network call

- **WHEN** the user commits a meal log
- **THEN** the app inserts a row into `pending_writes` carrying the HTTP method, path, body bytes, and a fresh UUID-v4 `idem_key`
- **AND** the UI optimistically reflects the change (the meal appears on Today/Recent)
- **AND** the queue worker takes the row in the next flush tick

#### Scenario: Queue replay on app foreground

- **WHEN** the Flutter app transitions from `paused`/`inactive` to `resumed`
- **THEN** the queue worker is invoked
- **AND** every `pending_writes` row with `status = 'pending'` is attempted in arrival order

#### Scenario: Queue replay on network state change

- **WHEN** `connectivity_plus` reports a transition from `none` to any other network state
- **THEN** the queue worker is invoked

#### Scenario: WorkManager backstop replay

- **WHEN** the app has been backgrounded with pending rows
- **THEN** a `WorkManager` periodic job (15-minute interval) attempts the queue
- **AND** failures stay queued; successes mark `status = 'done'`

#### Scenario: Replay collapses on backend cache hit

- **WHEN** a pending row is sent and the request was previously persisted (the network ack was lost the first time, not the request itself)
- **THEN** the backend returns the cached 2xx response per the idempotency middleware
- **AND** the local row's `status = 'done'`
- **AND** the local optimistic state is reconciled with the response body

#### Scenario: Permanent failure removes the row from the queue

- **WHEN** a pending row receives a 4xx response that is NOT `409 idempotency_key_conflict`
- **THEN** the row's `status = 'failed_permanent'` with the error body recorded in `last_error`
- **AND** the row is not retried automatically
- **AND** a notification mechanism informs the user on next foreground (toast or notification badge)

#### Scenario: Conflict triggers a clean fail-and-discard

- **WHEN** a pending row receives `409 idempotency_key_conflict`
- **THEN** the row is marked `failed_permanent` (the client should not have been sending different bodies with the same key, but the safe behavior is to surface and stop)
- **AND** the user is notified

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

### Requirement: Bearer token storage in platform secure storage

The bearer token SHALL be stored in the Android Keystore via `flutter_secure_storage`. The Kotlin widget SHALL read the same token through EncryptedSharedPreferences (the same Keystore-backed primitive). The token SHALL NEVER be written to plain `SharedPreferences`, the Drift database, or the SQLite outbox row bodies.

#### Scenario: Token storage on pairing

- **WHEN** the user completes pairing by scanning the QR code displayed by `task dev:pair`
- **THEN** the token from the QR payload is written to `flutter_secure_storage` under a known key
- **AND** the same value is mirrored to EncryptedSharedPreferences for the Kotlin widget

#### Scenario: Outbox row bodies never contain the token

- **WHEN** any `pending_writes` row is inspected
- **THEN** the `body` column contains the request body bytes only
- **AND** does not contain the bearer token in any header or form field

### Requirement: Goals are read-only in v1

The app SHALL render goal-derived progress (rings on Today, status bands on macros and tracked micros) using the `adherence` block from the daily summary response. The app SHALL NOT expose a UI for editing goals — goals are configured via the agent (`set_goals` tool) or a direct REST call.

#### Scenario: Today renders adherence from the API

- **WHEN** the daily summary response includes an `adherence` block
- **THEN** the rings reflect the per-nutrient `status` field (under / on / over)
- **AND** no "edit goals" button is present anywhere in the app

#### Scenario: No goals set yet

- **WHEN** the user has not yet configured goals (`GET /goals` returns `{"goals": null}`)
- **THEN** Today shows raw totals (no rings)
- **AND** a small hint reads "Set goals via the assistant"

### Requirement: Pairing flow uses a QR code printed by the backend

The app's first-run experience SHALL be a pairing screen that scans a QR code. The QR payload SHALL be JSON of the shape `{"base_url": "<url>", "token": "<bearer>"}`. The backend SHALL provide a `task dev:pair` helper that prints this QR (e.g. via `qrencode` to the terminal) using the value of `MOBILE_API_TOKEN` and the configured `HTTP_ADDR`.

#### Scenario: First-run scan succeeds

- **WHEN** the user opens the app for the first time and scans the QR from `task dev:pair`
- **THEN** the JSON payload is parsed, `base_url` is validated as a syntactically valid URL, and `token` is non-empty
- **AND** both values are persisted (token to secure storage, base_url to preferences)
- **AND** the app navigates to Today

#### Scenario: Malformed pairing payload is rejected

- **WHEN** the scanned QR contains malformed JSON or missing fields
- **THEN** the pairing screen shows an inline error
- **AND** does not persist anything
- **AND** prompts the user to try again

### Requirement: Implementation MAY vary by platform; spec invariants MUST NOT

This capability spec describes platform-agnostic invariants. The v1 implementation is Flutter on Android. Future iOS or Web ports MUST satisfy the same scenarios above, with platform-specific substitutions (Keychain for Keystore, Service Worker for WorkManager, etc.). The capability SHALL NOT be modified to add platform-specific carve-outs except where a scenario is genuinely impossible on the target platform.

#### Scenario: iOS port substitutes Keychain for Keystore

- **WHEN** an iOS implementation of this capability is shipped
- **THEN** the "bearer token storage in platform secure storage" requirement is satisfied via iOS Keychain
- **AND** the rest of the spec applies unchanged

#### Scenario: Web port omits the widget requirement explicitly

- **WHEN** a Web (PWA) implementation is shipped
- **THEN** the "one-tap hydration via an Android home-screen widget" requirement is recorded as not-applicable in that port's release notes
- **AND** the PWA implementation surfaces a different killer hydration interaction (e.g. a quick action in a notification) documented in the change that ships it
</content>
</invoke>

### Requirement: Camera food-search mode shows previously-used foods, most-recently-used first

The Camera screen's food-search mode SHALL, on open with no query, present the
user's previously-used foods ordered most-recently-used first, so re-logging a
familiar food needs no typing. Ordering SHALL follow the backend's
`last_logged_at DESC NULLS LAST` recency. The list SHALL render from the local
`products_cache` immediately (stale-while-revalidate) and revalidate against
`GET /products` in the background, with no offline banner.

#### Scenario: Food-search mode opens to recent foods

- **WHEN** the user switches the Camera screen to food-search mode without typing a query
- **THEN** the app renders previously-used foods from `products_cache`, most-recently-used first
- **AND** issues `GET /products` in the background and reconciles the list if the fresh response differs

#### Scenario: Recent list works offline

- **WHEN** the network is unreachable and the user opens food-search mode
- **THEN** the recent foods still render from the cache
- **AND** no banner, badge, or modal indicates offline state

### Requirement: Camera food-search mode searches foods by name as the user types

The food-search mode SHALL filter to a name search as the user types, calling
`GET /products/search?q=<query>` (debounced) and rendering results recency-first
as the backend returns them. When the network is unreachable, search SHALL
degrade to a case-insensitive substring filter over the cached foods.

#### Scenario: Typing filters to search results

- **WHEN** the user types `yog` in the food-search field
- **THEN** the app issues a debounced `GET /products/search?q=yog`
- **AND** renders matching foods recency-first
- **AND** writes the responses through to `products_cache`

#### Scenario: Offline search falls back to the cache

- **WHEN** the network is unreachable and the user types a query
- **THEN** the app filters the cached foods by case-insensitive substring match
- **AND** shows those results without an offline banner

### Requirement: Selecting a food in food-search mode logs it via the shared product card and outbox

Tapping a food in food-search mode SHALL open the same product card the barcode
scan flow uses, with the quantity pre-filled from `last_logged_quantity_g` (else
`serving_size_g`, else 100) and `meal_type` derived from time of day, and
committing SHALL enqueue `POST /meals` through the offline outbox exactly as the
scan flow does.

#### Scenario: Logging a recent food reuses the scan-flow card

- **WHEN** the user taps a food in food-search mode
- **THEN** the app presents the product card pre-filled with the last-logged quantity (or `serving_size_g`, or 100) and the time-of-day meal type
- **AND** the "Log" button enqueues `POST /meals` with the food's `product_id` into `pending_writes` with a fresh Idempotency-Key
- **AND** the meal optimistically appears on Today/Recent

### Requirement: Quick-create a food logs it now and saves it for reuse

The food-search mode SHALL offer a quick-create action that captures a food's
name and macros (kcal, protein, carbs, fat) and, in a single outbox-queued call,
both logs the food now and saves it as a reusable product. This SHALL be
implemented as `POST /meals/freeform` with `save_as_product: true`, so the food
is created server-side (`source = "manual"`) and appears in recent/search
thereafter. The quick-create action SHALL be always available and SHALL be
emphasized when a search query returns no matches.

#### Scenario: Quick-create logs and saves in one queued call

- **WHEN** the user picks "Create new food", enters a name and macros, sets a quantity, and confirms
- **THEN** the app enqueues one `POST /meals/freeform` row carrying the name, `nutriments_per_100g`, `quantity_g`, `meal_type`, `logged_at`, and `save_as_product: true`
- **AND** the meal optimistically appears on Today/Recent
- **AND** after the next sync the created food is returned by `GET /products` and appears in food-search mode's recent list

#### Scenario: Quick-create is offered when search finds nothing

- **WHEN** a search query returns no matching foods
- **THEN** the food-search mode emphasizes the "Create new food" action, pre-filling the name field with the query text

#### Scenario: Quick-create does not duplicate an existing food

- **WHEN** the quick-create name matches an existing product's name on the backend
- **THEN** the backend's `save_as_product` path reuses the existing product rather than creating a duplicate
- **AND** the meal is logged against that product

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

### Requirement: The Chat screen confirms consequential writes before they fire

The Chat screen SHALL render a pending write confirmation as a card listing each pending write's human `preview` with a **per-item** toggle, an "Apply selected" action, and a reject affordance, and SHALL NOT let any `write-confirm` action take effect until the user applies it. The card SHALL be rendered identically whether it arrives as a live `proposal` SSE event or is reconstructed from a session's `pending_confirmation` on (re)open, so it survives the app being backgrounded or killed. On apply, the app SHALL call `POST /chat/sessions/{id}/confirm` with the per-call decisions and resume rendering the streamed continuation on the same screen. The composer SHALL remain usable while a card is pending: sending a new message implicitly rejects the pending writes (the server resolves them) and proceeds.

#### Scenario: A proposed training write shows a per-item confirm card

- **WHEN** the coach proposes one or more `write-confirm` actions (e.g. scheduling workouts or changing a goal)
- **THEN** the Chat screen shows a card with one toggle per pending write and its preview
- **AND** nothing is written until the user taps "Apply selected"

#### Scenario: Applying a subset writes only the selected actions

- **WHEN** the user deselects one item and taps "Apply selected"
- **THEN** the app POSTs per-call decisions approving only the selected items and rejecting the rest
- **AND** renders the resumed stream (tool-status chips, then text) inline

#### Scenario: Typing instead of confirming implicitly rejects

- **WHEN** the user ignores the card and sends a new message
- **THEN** the pending writes are implicitly rejected server-side and the new turn proceeds, with nothing written

#### Scenario: Killing the app mid-confirmation loses nothing

- **WHEN** the app is closed while a card is pending and later reopened to that session
- **THEN** the card is reconstructed from the session's `pending_confirmation` and the user can still apply or reject

