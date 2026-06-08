## Why

The Flutter companion app's killer interaction is **barcode-scan → log a meal in 2 taps**. The "log" tap is the moment that decides whether scanning is fast enough to use, and the *default quantity* at that step is the single biggest determinant of tap count. Today the API tracks `last_logged_at` per product but not the gram amount of that last log, so the phone has no useful default to pre-fill — it has to either guess (100g, almost always wrong), pull `serving_size_g` from OFF (often null), or make the user type.

Adding `last_logged_quantity_g` to the `products` row lets the phone default to "what I had last time" — the right default in the overwhelming majority of cases (you keep eating the same portions of the same foods). The MCP agent benefits too: a freshly logged meal can be reflected back as "logging your usual 200g of skyr" without the agent having to remember.

This is a small, surgical change: one column, one extra clause on the existing `TouchLastLoggedAt` path, no migration of existing data needed.

## What Changes

- Add nullable `last_logged_quantity_g NUMERIC(10, 3)` column to `products`.
- Extend the existing "advance `last_logged_at` if the new value is greater" semantics: when the advance happens, also write the new meal entry's `quantity_g` to `last_logged_quantity_g`. When the advance does NOT happen (older meal landing late), leave `last_logged_quantity_g` unchanged.
- Surface the new field on every endpoint that returns a product row: `GET /products/{id}`, `GET /products/search`, `POST /products/lookup/{barcode}`, `POST /products` (initially null), `POST /products/recipes` (initially null), the recipe-recompute response.
- The field is omitted from JSON when null (existing `omitempty` pattern).
- No MCP wrapper changes needed — the tools already return the product row verbatim, so the field flows through for free.

## Capabilities

### Modified Capabilities

- `products`: The "Last-logged tracking updates on meal entry creation" requirement extends to also track quantity in lockstep with `last_logged_at`. No other behaviour changes.

## Impact

- **Schema**: migration `008_add_last_logged_quantity` adds one nullable column to `products`. No backfill — null is the correct value for products with no log history.
- **Service layer**: `internal/products/repo.go` gains a `TouchLastLoggedAtAndQuantity(id, ts, quantity_g)` method (or extends the existing `TouchLastLoggedAt` signature). The meals service's create / patch / freeform-save-as-product paths call this instead.
- **Wire**: the new field surfaces on product JSON automatically once added to the struct + scan.
- **Tests**: extend the existing last-logged tracking tests with a "quantity follows the advance" scenario and a "quantity does NOT regress when an older entry lands late" scenario.
- **Breaking**: none. The column is nullable and the JSON tag is `omitempty`.
- **MCP**: no changes.
- **Docs**: small README/swagger refresh; godoc on the product struct.

## Out of scope

- Per-meal-type defaults (e.g. "200g for breakfast, 100g for snacks"). Considered and rejected as overscoped — the single most-recent quantity is enough to take the phone from 3 taps to 2 in the common case.
- Tracking *unit* of last log (g vs ml vs servings). The whole API is grams-only today; this change doesn't introduce a new unit.
- A separate "favourites" / "recent" table for the phone. The product table's `last_logged_at` already drives recency-ranked search; the new field is the natural extension, not a parallel structure.
