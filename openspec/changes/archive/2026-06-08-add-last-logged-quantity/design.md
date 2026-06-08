## Context

The API today tracks per-product recency via a single nullable column `products.last_logged_at`. The convention is: on meal-entry create (and PATCH of `logged_at`), if the new value is strictly greater than the current, advance; otherwise no-op. The same touch happens for freeform `save_as_product` flows. This works for ranking products by recency in search results but is silent about *how much* the user logged.

The companion Flutter app's barcode-scan flow needs a default quantity good enough that most scans become 2 taps. Three candidates:

- `100g` — wrong far too often (cereals, condiments, sauces, drinks)
- `products.serving_size_g` from OFF — sometimes set, often null, sometimes garbage (we already log warnings on unparseable values)
- *last quantity logged for this product* — the actually-correct default in the overwhelming majority of cases

The third option is the right answer but requires storing the value. This change adds it.

## Goals / Non-Goals

**Goals:**

- A single new column on `products` carrying the most recent meal entry's `quantity_g`.
- Update semantics that exactly mirror the existing `last_logged_at` advancement rule (so the two columns move in lockstep).
- Zero behaviour change for clients that ignore the field.
- The new column flows through every product-returning endpoint with no per-endpoint code.

**Non-Goals:**

- Per-meal-type defaults (breakfast vs snack quantity). The data is there if a future change wants this (aggregate over `meal_entries`), but the single most-recent value is enough for the phone's killer interaction today.
- Backfilling historical products' `last_logged_quantity_g` from existing meal history. The phone gains the default for *future* logs of a product; historical products will see the value populated the next time they're logged. Backfill would be a one-shot pgSQL query that we can add later if the gap turns out to bother anyone.
- A separate quantity-history table. The existing meal entries already serve as the history.

## Decisions

### 1. The new column moves in lockstep with `last_logged_at`

Same advance rule, same atomic update path. When `last_logged_at` advances, write `last_logged_quantity_g` to the new meal's `quantity_g`. When it doesn't advance (a backdated meal landing late), neither column changes.

**Alternatives considered:**

- *Always write the new quantity, regardless of recency.* Rejected — would mean a backdated correction overwrites the current "I had 200g this morning" with "I had 50g last Tuesday." Confusing.
- *Compute on demand: query the most-recent meal entry for this product whenever the phone asks.* Rejected — the read path becomes an index scan per product instead of a column read, and the phone reads products in search results in batches.

### 2. Extend the existing repo method rather than add a parallel one

`TouchLastLoggedAt(id, ts)` becomes `TouchLastLoggedAt(id, ts, quantity_g *float64)` (pointer because freeform-save-as-product creates the product and immediately touches with the meal's quantity; both come from the same call). Callers that don't have a quantity (only the freeform-creation initial-touch case) pass nil and the column stays null.

Actually, both current call sites — `Service.Create` and `Service.Patch` in meals — DO have the quantity at hand. The freeform `save_as_product` path also has it (it just logged the meal). So in practice no caller passes nil.

**Alternative considered:** a separate `TouchLastLoggedQuantity` method. Rejected — two SQL statements where one would do, and the lockstep semantics get harder to enforce.

### 3. Patch semantics

When a client PATCHes a meal's `quantity_g`, should that update the product's `last_logged_quantity_g`?

- Yes, *if* this meal is currently the most-recent (its `logged_at` equals the product's `last_logged_at`).
- No otherwise.

Same logic as for `last_logged_at` (which can be advanced via PATCH today): only the most-recent log's values propagate.

This is one more clause in the PATCH path, but it's the consistent behaviour. Implementation detail: the simplest way is the same conditional UPDATE — `WHERE id = $1 AND last_logged_at = $2` — which only writes when the patched meal IS the most-recent.

### 4. Delete semantics

Deleting a meal entry does not touch `last_logged_at` today (the existing spec scenario "Deleting the most recent entry for a product does not revert last_logged_at"). Same applies here: deleting a meal does not revert `last_logged_quantity_g`. The product still "remembers" the last logged amount even after that log is deleted.

This is mildly weird but matches the existing pattern. Reverting would mean an extra read on every delete (find the next-most-recent meal for this product) and a subtle race window. The cost of "the phone defaults to 200g but I deleted that meal" is small and self-correcting — the next log overwrites it.

### 5. Field naming

`last_logged_quantity_g` (mirror of `last_logged_at`, with the `_g` suffix consistent with the rest of the schema). JSON: `last_logged_quantity_g`, omitempty.

## Risks / Trade-offs

- **The defaults can lie when consumption patterns change.** "I usually have 200g of skyr but today I'm having a small portion of 50g" — the phone defaults to 200, user re-types 50. Acceptable; this is a default, not a constraint. Worst case: 3 taps instead of 2 for the rare case of an unusual portion.
- **Patch race.** Two clients PATCH the same meal's `quantity_g` near-simultaneously; both compute `last_logged_at = current product value` and both update. Postgres MVCC serializes the writes; the *second* commit wins. Same race window as the current `last_logged_at` patch path, no new exposure.
- **Recipes.** A recipe IS a product; logging a 250g serving of "Morning skyr bowl" sets `last_logged_quantity_g = 250` on the recipe product. Reasonable — the phone defaults to "your usual portion of the bowl."

## Migration Plan

Migration 008 (up):

```sql
ALTER TABLE products
    ADD COLUMN last_logged_quantity_g NUMERIC(10, 3);
```

Down: `ALTER TABLE products DROP COLUMN IF EXISTS last_logged_quantity_g;`

No data backfill. The column populates organically as users log new meals after deployment.

Rollback safe: old binary against new schema works (ignores the column). New binary against old schema fails at startup (column missing). Standard order.

## Open Questions

- Worth a one-shot backfill query in the migration to populate `last_logged_quantity_g` from the most-recent meal entry per product, for products that already have `last_logged_at` set? It's a single `UPDATE … FROM (SELECT DISTINCT ON …)` and would mean the phone benefits immediately rather than waiting for each product to be re-logged. **Default: yes, include the backfill** — it's cheap, deterministic, and removes the "why does my default still say nothing" complaint window.
