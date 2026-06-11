## Why

The companion app can only turn a food into a logged meal three ways: **scan a
barcode**, **snap a photo**, or **describe it to the agent**. There is no way to
*type a food's name and log it*, no way to *re-log something you ate yesterday*
without re-scanning it, and no way to *create a food the app remembers*. Every
"I know what this is, just let me log it" interaction falls back to the agent —
slow for exactly the high-frequency case the app exists to make instant.

The backend already solves all of this. `GET /products/search?q=` returns foods
ranked by recency of use, `GET /products` lists them most-recently-used first,
`products.last_logged_quantity_g` remembers the last portion, and
`POST /meals/freeform {save_as_product:true}` both logs a meal and creates a
reusable product in one idempotent call. **None of it is reachable from the
phone.** This change surfaces it.

Doing so **reverses a recorded non-goal**. The `mobile-companion` spec
explicitly excludes "a generalized product search experience," deferring all
search to the agent. That bet has proven too strict: manual logging without a
barcode is a daily need, not a power-user edge. We narrow the carve-out rather
than abandon it — a *scoped* recent/search/quick-create capability joins the
app; open-ended chat and in-app recipe building still belong to the agent. The
app stays at three screens: this folds into the existing Camera screen rather
than adding a surface.

## What Changes

- **The Camera screen becomes the single "add food" surface.** It already turns
  a food into a logged meal two ways (barcode scan, photo); this adds a third
  **food-search mode** — open it to see **previously-used foods** (recent-first),
  type to **search by name**. No new screen, no new nav slot; the reserved chat
  slot is untouched.
- **Tap a food → log it** through the existing product card (quantity +
  meal-type, pre-filled from `last_logged_quantity_g`), committed through the
  same offline outbox every other mutation uses.
- **Quick-create a food** when nothing matches: a name + macros form that **logs
  the food now and saves it for reuse** in a single outbox-queued
  `POST /meals/freeform {save_as_product:true}` call — the same freeform endpoint
  the photo flow's low-confidence and barcode-404 escape hatches already use, so
  every manual path converges on one mechanism. The created product then appears
  in recent/search next time — satisfying both "log it now" and "save for later."
- **Offline-first reads**: recent and search render from the local
  `products_cache` when the network is unreachable; live responses write through
  the cache. No "offline" banner, consistent with the existing SWR contract.
- **No backend changes.** Every endpoint already exists; the work is entirely in
  the Flutter app (one new screen, three new repository methods, a cache query
  ordered by `last_logged_at`, and `save_as_product` on the freeform enqueue).

## Capabilities

### Modified Capabilities

- `mobile-companion`: Narrows the "focused supplement, no generalized product
  search" requirement to admit a scoped recent/search/quick-create capability
  **within the Camera screen**, and adds requirements for recent-first browsing,
  name search, quick-create-and-save, and the offline-read behavior of that
  mode. The app stays at three screens; the reserved chat slot and the
  agent-owns-chat/recipe-building invariants are preserved.
