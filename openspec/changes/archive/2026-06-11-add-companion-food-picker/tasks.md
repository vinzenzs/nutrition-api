All work is in the Flutter app (`apps/companion/`). No backend, migration, MCP,
or `task swag` changes — every endpoint this uses already exists.

## 1. Repository / data layer

- [x] 1.1 Add `searchProducts(String q)` to `Repository` → `GET /products/search?q=`, parse into `Product` models, write each through `products_cache` via `upsertFromApi`.
- [x] 1.2 Add `recentProducts({int limit, int offset})` to `Repository` → `GET /products`, parse `{products, total, limit, offset}`, write through `products_cache`.
- [x] 1.3 Add a `products_cache_dao` query `recentlyUsed(int limit)` ordered by `last_logged_at DESC NULLS LAST, name ASC` (distinct from the existing `recentlyScanned`, which orders by scan time).
- [x] 1.4 Add an offline substring fallback: `searchCached(String q)` filtering cached rows case-insensitively on name/brand.
- [x] 1.5 Extend `enqueueFreeformMeal` to forward `save_as_product: true` and any supplied micros (it currently sends macros only and omits `save_as_product`).

## 2. State (Riverpod)

- [x] 2.1 `foodSearchProvider`: holds the query, debounces (~250ms), returns recent foods when the query is empty and search results otherwise; SWR over the cache with background revalidation; offline → cache fallback.

## 3. Camera screen — food-search mode

- [x] 3.1 Add a mode switch to the Camera screen: **Scan / Photo / Search** (the first two already exist).
- [x] 3.2 Food-search mode UI: a search field plus a recent-first list; tapping a row opens the existing `product_card` (quantity pre-filled from `last_logged_quantity_g` → `serving_size_g` → 100; meal-type from time of day) and logs via `enqueueMeal` (`POST /meals`).
- [x] 3.3 "＋ Create new food" row at the bottom of the list, always present and emphasized when a query returns no matches; pre-fills the name with the current query.
- [x] 3.4 Quick-create form: name + macros (kcal/protein/carbs/fat) + quantity + meal-type → `enqueueFreeformMeal(..., saveAsProduct: true)`; optimistic insert into Today/Recent.
- [x] 3.5 Optionally relabel the "Camera" nav destination to an add-food framing (icon/label only — the three-destination nav and the reserved chat slot stay as-is).

## 4. Tests

- [x] 4.1 Repository tests: `searchProducts`/`recentProducts` parse responses and write through the cache; `enqueueFreeformMeal` emits a `pending_writes` row carrying `save_as_product: true`.
- [x] 4.2 DAO test: `recentlyUsed` orders by `last_logged_at DESC NULLS LAST`.
- [x] 4.3 Provider test: empty query → recent; non-empty → debounced search; offline → cache substring fallback.
- [x] 4.4 Widget test: tap a food → product card pre-fills last-logged quantity → Log enqueues `POST /meals`; quick-create with no match enqueues freeform with `save_as_product: true`.
- [x] 4.5 `flutter analyze` clean; `flutter test` green.

## 5. Spec sync

- [x] 5.1 On archive, fold the MODIFIED + ADDED requirements into `openspec/specs/mobile-companion/spec.md`.
