## Context

The product cache is the long-lived index of every food the system has touched: every OFF barcode lookup, every manual product the user typed, every composite recipe. Today the surface to manipulate that index is **create + read + update-via-OFF-refresh** — there's no enumerate-all and no delete. The MCP test loop exposes this: the agent creates products to demonstrate flows, and they all linger.

Two adjacent corners share the theme:

1. **Recipe creation accepts duplicate component references.** No DB constraint stops two component rows pointing at the same `component_product_id`, and the service-layer validator doesn't either. The math is mechanically correct (two rows × N grams each = 2N grams of that ingredient) but the user/agent rarely means it — usually it's a copy-paste mistake that doubles a recipe's protein contribution.
2. **Hard delete vs historical meals.** `meal_entries.product_id` uses `ON DELETE SET NULL`. If we just delete a product, every meal that referenced it loses its nutriment data — `effective_name` and `effective_nutriments` both go null because the meal didn't have a snapshot. Summaries past and future then silently misreport.

The change picks the least-invasive shape for each: pre-check FKs to translate violation into a useful 409, materialise the snapshot before deleting to preserve history, reject duplicate component_ids in the recipe validator.

## Goals / Non-Goals

**Goals:**

- The agent can enumerate the product cache and remove leftover rows in two MCP tool calls.
- Deleting a product never silently corrupts historical meal totals.
- Deleting a product that's used inside a recipe surfaces a clear, actionable 409.
- Creating a recipe with duplicate component references fails loudly at create time rather than producing a silently-wrong row.
- Backend change is small enough to land in one focused review pass.

**Non-Goals:**

- Soft delete / archive flag / undo. Hard delete with the snapshot guarantee is the entire mental model.
- Bulk delete. One product per call is fine for the volumes a single-user app sees.
- A "swap product A for B in every recipe" replacement flow. The 409 hint tells the user to delete the recipes first; that's enough.
- Permission scoping or audit columns. Single-user.
- Cursor pagination on the list endpoint. Offset/limit is right-sized for current scale.

## Decisions

### 1. `DELETE /products/{id}` materialises a snapshot before nulling

The handler runs inside a single transaction (`store.WithTx`):

```
1. SELECT id, name, nutriments_per_100g FROM products WHERE id = $1 → product row P
   (404 product_not_found if missing)
2. SELECT pc.product_id, p.name FROM product_components pc
     JOIN products p ON p.id = pc.product_id
   WHERE pc.component_product_id = $1
   → list of recipes using this product as a component
   (409 product_in_use_as_component with the list if non-empty)
3. UPDATE meal_entries SET
     snapshot_name = COALESCE(snapshot_name, P.name),
     snapshot_kcal_per_100g = COALESCE(snapshot_kcal_per_100g, P.kcal_per_100g),
     ... (every macro + micro column)
   WHERE product_id = $1 AND snapshot_name IS NULL
4. DELETE FROM products WHERE id = $1
   (FK ON DELETE SET NULL fires on meal_entries.product_id automatically)
5. Return 204 No Content
```

The `COALESCE(snapshot_*, P.*)` shape protects against the freeform case where a snapshot was already stored — we don't want to overwrite a freeform meal's hand-supplied nutriments with the product's, because they're semantically distinct (the freeform meal's nutriments are the agent's estimate at the time, not the product's current state). Only meals whose `snapshot_name IS NULL` get the materialisation.

**Alternatives considered:**

- *Hard delete with no snapshot materialisation.* Rejected — silently nulls meal history.
- *Refuse delete when any `meal_entries` row references the product.* Rejected — would mean you can never clean up any product you've ever logged, which is the opposite of the user's pain point.
- *Soft delete (`deleted_at` column).* Rejected — adds a new column and a "should this product surface in search?" predicate to every read path. The snapshot+hard-delete combo gives us the same effective outcome (history preserved) without the schema rule.
- *Cascade delete `meal_entries` along with the product.* Rejected — loses the user's intent (they logged the meal, that's a real event in time, independent of whether the product still exists).

### 2. Pre-check componentry; FK is the safety net

The pre-check in step 2 above produces the friendly 409 body:

```json
{
  "error": "product_in_use_as_component",
  "recipes": [
    {"id": "...", "name": "Morning skyr bowl"},
    {"id": "...", "name": "Oat & banana smoothie"}
  ],
  "hint": "delete the listed recipes first, or replace this product within them"
}
```

If the pre-check passes but the DB FK still rejects (race window: another process inserts a `product_components` row referencing this product between the SELECT and the DELETE), the handler translates the pgx unique-violation-or-foreign-key error code (`23503`) into a `409 product_in_use_as_component` without the recipe list (since we don't know which one raced in). Single-user reality means this never fires in practice; it's belt-and-suspenders.

**Alternatives considered:**

- *Trust the FK only.* Rejected — the agent gets a useless string-parsed error instead of the actionable recipe list.
- *Take a row lock on the product during the check.* Overkill for single-user; the race window is theoretical.

### 3. `GET /products` paginated list with the same recency ordering as search

```
GET /products?source=manual&limit=50&offset=0
```

Response:

```json
{
  "products": [...],
  "total": 142,
  "limit": 50,
  "offset": 0
}
```

Ordering is `last_logged_at DESC NULLS LAST, name ASC` — matches `/products/search` so the agent's "show me the cache" call returns recent foods first. Stale leftovers naturally sink to the bottom, which is the cleanup affordance the user wants.

`total` is a `SELECT COUNT(*) FROM products WHERE ...` companion query; cheap at this scale. Pagination uses offset/limit (not cursor) because the cache is small and the typical UX is "show me everything in one or two pages, then ask me what to delete."

`source` is the only filter for v1 — it covers the common case ("show me my recipes" / "show me OFF-imported barcodes"). Other filters (name LIKE, brand, has-meals-logged) deferred.

**Alternatives considered:**

- *Reuse `GET /products/search?q=` with an empty `q`.* Rejected — search forbids empty queries today (400 q_required); changing that contract would surprise existing callers, and a dedicated list endpoint is clearer.
- *Cursor pagination.* Rejected — offset is right for the scale and gives the agent a `total` it can present.

### 4. Duplicate component product_ids fail with `400 component_duplicate`

Inside `service.CreateRecipe`, before any database write:

```go
seen := make(map[uuid.UUID]int, len(in.Components))
for _, c := range in.Components {
    seen[c.ProductID]++
}
for pid, n := range seen {
    if n > 1 {
        return nil, &DuplicateComponentError{ProductID: pid, Occurrences: n}
    }
}
```

The error maps to:

```json
{
  "error": "component_duplicate",
  "product_id": "<uuid>",
  "occurrences": 2,
  "hint": "sum the quantities and supply one entry per product"
}
```

This is the same loud-over-silent pattern `harden-write-paths` used for the PUT idempotency rejection: surfacing the constraint at the first integration attempt is cheaper than letting agents accumulate silently-doubled recipes.

**Alternatives considered:**

- *Collapse-and-sum.* Friendlier-feeling but hides the agent's mistake. Rejected.
- *Add a `UNIQUE (product_id, component_product_id)` DB constraint.* Belt-and-suspenders that would also reject this case; would force us to translate the unique-violation error anyway, so the service-layer check is just as good with a friendlier error body up front.

### 5. MCP wrapper: `delete_product` and `list_products`

Both follow the existing tool patterns established by `add-mcp-server` and `add-meal-logging-mvp`:

- `delete_product { product_id: uuid, idempotency_key?: string }` → `DELETE /products/{id}`. Auto-derive idempotency key from inputs per the existing rule for POST-style writes (delete is idempotent — same key replaying produces the same outcome, 204 or 404). Description: "Permanently delete a product. Recipes that reference this product as a component will block the delete with a 409 — delete the recipes first or replace the component within them. Historical meals that logged this product preserve their nutriment snapshot at the moment of deletion."
- `list_products { source?: "off" | "manual" | "recipe", limit?: int, offset?: int }` → `GET /products`. Read-only, no idempotency. Description: "List products in the cache, most recently used first. Combine with delete_product to clean up leftovers from old experimental sessions."

Both tools live in `internal/mcpserver/tools_products.go` alongside the existing `lookup_product_by_barcode` and `search_products`.

**Alternatives considered:**

- *Reject delete via idempotency replay.* Treated like every other POST-style write: the idempotency middleware does the right thing without per-tool plumbing.

## Risks / Trade-offs

- **Snapshot materialisation runs an UPDATE over `meal_entries.product_id = $1` at delete time.** With current indexes (`meal_entries_product_id_idx` exists per migration 004), the cost is bounded by the number of meals that referenced the product. *Mitigation:* index already in place; in practice the count is small for a personal app.
- **`total` query on `GET /products` doubles the round-trips.** *Mitigation:* trivial at current scale; can be removed later if we ever care.
- **Duplicate-component rejection is breaking** for any caller that today relies on the silent acceptance. *Mitigation:* the only known caller (the MCP agent) is the very thing complaining about the misbehaviour; no external clients are known to rely on the silent path.
- **The pre-check race is theoretical but real** in a multi-process deploy. *Mitigation:* belt-and-suspenders via FK error translation. Single-user means the window is essentially zero today.
- **Hard delete is irreversible.** *Mitigation:* the snapshot preserves history; the only true loss is the product row itself, which is fine for the test-leftover use case the user is solving.

## Migration Plan

- No schema migration. Existing FK rules suffice.
- Backend, MCP wrapper, swag regeneration, doc updates all ship in one commit so the API surface flips atomically.
- Rollback: revert the commit. The two new endpoints disappear; the existing routes are unchanged. No data shape change to undo.

## Open Questions

- Whether the `delete_product` MCP tool should accept the same product by `barcode` as a convenience (so the agent doesn't have to call `lookup_product_by_barcode` first to get the id). Tentative answer: no for v1; the tool surface stays narrow. The agent's "look up, then delete" sequence is clear and lets the user inspect the lookup before destruction.
- Whether the `list_products` `total` should be cheap to compute (it is now) but might warrant an `include_total=false` flag if the cache grows. Deferred.
- Whether the recipe duplicate-component check should also fire on `POST /products/recipes/{id}/recompute` if the existing component rows somehow contain duplicates. Tentative answer: no — recompute trusts the stored state; if you want to "fix" a recipe with duplicate components, the path is delete-and-recreate. (And after this change, you can't create a new one with duplicates.)
