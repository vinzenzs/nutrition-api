# mobile-companion Specification

## Purpose

A focused Flutter Android companion app that supplements — not replaces — the LLM coaching agent. It exists to make three high-frequency interactions instant where typing at the agent would be slow: barcode scan → logged meal, photo capture → logged meal via the backend vision endpoint, and one-tap hydration from an Android home-screen widget. Everything else (chat, recipe building, generalized product search, goal editing) stays in the agent. The app is offline-first: every mutating call is enqueued in a local outbox with a client-minted Idempotency-Key before any network attempt and replayed exactly against the backend's idempotency contract, while reads render from a local cache stale-while-revalidate with no offline banner. The bearer token lives in platform secure storage (Android Keystore via `flutter_secure_storage`, mirrored to EncryptedSharedPreferences for the Kotlin widget) and the app is paired by scanning a QR code printed by the backend. Goals are read-only in v1 — adherence is rendered from the daily summary's `adherence` block, but goal configuration stays with the agent.

## Requirements

### Requirement: The mobile companion is a focused supplement to the agent, not a replacement

The mobile companion app SHALL implement exactly three screens (Today, Camera, Recent) plus a Settings sheet. It SHALL NOT include a chat surface, an in-app recipe builder, or a generalized product search experience — those interactions remain in the agent. The bottom navigation SHALL reserve a slot for a future chat affordance so v2 can introduce it without restructuring.

#### Scenario: Three screens at first launch

- **WHEN** the user opens the app for the first time after pairing
- **THEN** the bottom navigation surfaces Today, Camera, and Recent as the three primary destinations
- **AND** Settings is reachable from the Today screen's top-right
- **AND** no chat, recipe builder, or all-products search screen exists

#### Scenario: V2 chat slot is reserved

- **WHEN** the app is built for v1
- **THEN** the bottom navigation has a fourth slot wired but disabled
- **AND** the slot displays a placeholder (e.g. a `+` icon with a tooltip "coming soon") so the nav layout matches what v2 will ship

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

GET-driven screens (Today, Recent, the product card after a scan) SHALL render from a local cache immediately and trigger a background revalidation on screen activation. The app SHALL NOT display an "offline" banner; the data's freshness is implicit in its presence.

#### Scenario: Today renders cached data instantly

- **WHEN** the user opens Today
- **THEN** the screen renders adherence rings and recent meals from `recent_summary` within 16ms (no loading spinner)
- **AND** the app issues `GET /summary/daily?date=...&tz=...` in the background
- **AND** the screen updates if the fresh response differs

#### Scenario: Stale data is shown without warning

- **WHEN** the local cache is older than five minutes and the network is unreachable
- **THEN** Today still renders from the cache
- **AND** no banner, badge, or modal indicates offline state

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
