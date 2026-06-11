## MODIFIED Requirements

### Requirement: The mobile companion is a focused supplement to the agent, not a replacement

The mobile companion app SHALL implement exactly three screens (Today, Camera,
Recent) plus a Settings sheet. The Camera screen is the app's single "add food"
surface and SHALL offer three modes for turning a food into a logged meal:
barcode scan, photo capture, and a **food-search mode** (recent foods + name
search + quick-create-and-log). The app SHALL NOT include a chat surface or an
in-app recipe builder — those interactions remain in the agent — and the
food-search mode SHALL stay scoped to finding and quick-creating a single food;
it SHALL NOT grow into open-ended chat or composite-recipe building. The bottom
navigation SHALL reserve a slot for a future chat affordance so v2 can introduce
it without restructuring.

#### Scenario: Three screens, Camera hosts all add-food modes

- **WHEN** the user opens the app after pairing
- **THEN** the bottom navigation surfaces Today, Camera, and Recent as the three primary destinations
- **AND** the Camera screen offers scan, photo, and food-search modes
- **AND** Settings is reachable from the Today screen's top-right
- **AND** no chat surface or in-app recipe builder exists

#### Scenario: V2 chat slot is reserved

- **WHEN** the app is built for this version
- **THEN** the bottom navigation has a fourth slot wired but disabled
- **AND** the slot displays a placeholder (e.g. a `+` icon with a tooltip "coming soon") so the nav layout matches what v2 will ship

## ADDED Requirements

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
