## Context

The companion app is offline-first (a `pending_writes` outbox in front of every
mutation, a `products_cache` for SWR reads) and deliberately minimal. The
backend already exposes everything this feature needs; the design problem is
purely *how to surface it on the phone without breaking the app's invariants*
(outbox-before-network, no offline banner, reserved chat slot, agent owns chat).

## Decisions

### D1 — Recent and search are one mode, not two

Recent foods and name search are the same list with different default ordering:
recent = no query, ordered by `last_logged_at DESC`; search = the same list
filtered by `GET /products/search?q=`. They render as **one food-search mode**
(recent on open, search-as-you-type, debounced) — a single surface and a single
repository method pair. A "＋ Create new food" row sits at the bottom, always
available and emphasized when a query returns no matches.

### D2 — Fold into the Camera screen, not a separate screen

The Camera screen is already the app's "turn a food into a logged meal" surface:
it owns barcode scan and photo capture. Food search/recent/create is a **third
mode of the same screen**, not a new destination. This keeps the app at three
screens, leaves the reserved chat slot untouched, and puts every manual
food-entry path in one place — which is also where the existing escape hatches
already point (a barcode 404 offers "Describe it"/"photo"; low-confidence photo
falls back to freeform). Scan → no match → search-or-create becomes a single
fluid surface instead of a hop to another screen.

The Camera screen accordingly presents three modes — **Scan / Photo / Search** —
and may be relabeled from "Camera" to an add-food framing in the nav; the icon
and label are implementation details, the three-destination nav is the
invariant.

Alternatives considered: (a) its own full-screen destination reached by a FAB —
rejected by the "combine with photo" decision; it split the manual-entry paths
across two surfaces and pressured the reserved chat slot. (b) a 4th "Food" tab —
rejected, collides with the reserved chat slot.

### D3 — Quick-create logs **and** saves in one call via freeform+save_as_product

"Create a food" could be `POST /products` (create the product) followed by
`POST /meals` (log it referencing the new id). That needs the create round-trip
to *complete online* before the log can be queued — it breaks offline-first,
because the log call has no `product_id` until the server answers.

`POST /meals/freeform {save_as_product:true}` does both in **one request**: it
logs the meal now and, if the name doesn't match an existing product, creates a
`source=manual` product server-side with `last_logged_at`/`last_logged_quantity_g`
set. That single call drops cleanly into the existing outbox and replays exactly
once. It is therefore the create path — satisfying "log it now" and "saved for
reuse" simultaneously, offline-safe.

Consequence: the new product's server id isn't known locally until the next
sync, so it won't be individually addressable offline immediately after
creation — acceptable, because the picker re-reads recent/search (which the sync
refreshes) rather than holding a local id.

### D4 — Reuse the existing product card for the quantity → log step

`product_card.dart` already renders "quantity (pre-filled from
`last_logged_quantity_g` → `serving_size_g` → 100) + meal-type + Log" and the
two-tap contract is specced for the scan flow. Every picker path (tap a recent
food, tap a search result) funnels into that same card. Only the *picking* is
new; the *logging* is reused verbatim, including its outbox enqueue.

### D5 — Picker reads are SWR over `products_cache`

Recent and search render from `products_cache` first, then revalidate against
the network — same contract as Today/Recent, no offline banner. This requires:
(a) a cache query ordered by `last_logged_at DESC NULLS LAST` (today's
`recentlyScanned` orders by scan time, which is the wrong axis for "previously
*used* food"); (b) write-through of `GET /products` and `GET /products/search`
responses via the existing `upsertFromApi`. Offline search degrades to a
case-insensitive substring filter over the cached rows.

### D6 — Quick-create captures macros in v1; micros deferred

The create form takes name + the four headline macros (kcal, protein, carbs,
fat) — the set the freeform enqueue already (almost) carries. Micros
(`iron_mg`, `calcium_mg`, …) are supported by the backend but omitted from the
v1 form to keep manual entry fast; they can be added later or supplied via the
agent. The freeform enqueue is extended to forward `save_as_product` and any
micros that are present, so this is a form-scope choice, not a wire limitation.

## Non-Goals

- **Pure "save without logging"** (a food-library manager). v1 create always
  logs; a food enters the library *by being logged*. A no-log library add can
  come later if the need is real.
- **Editing or deleting existing foods** from the picker (`PATCH`/`DELETE
  /products`). The picker reads and logs; product mutation stays with the agent.
- **Barcode entry** inside the picker — that remains the Camera screen's job.
- **Recipe building / composite products** — explicitly still agent-only.
- **Backend changes** — none. If the offline-search experience proves too thin,
  a future change might add a server-side fuzzy match, but not here.

## Risks

- **Scope creep toward a second app.** Mitigated by the non-goals above and by
  reusing the product card rather than inventing new logging UI.
- **Cache staleness on `last_logged_at`.** Recent ordering is only as fresh as
  the last sync; acceptable under the SWR contract, and the live revalidation
  corrects it on screen open.
- **Duplicate products by name.** The freeform `save_as_product` path matches on
  existing name before creating, so re-creating "banana" re-uses the row rather
  than spawning duplicates — relied upon, not re-implemented client-side.
