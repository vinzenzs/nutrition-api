## Why

The MCP test session that produced `harden-write-paths` and `unify-adherence-shape` left two product-hygiene findings unaddressed:

1. **Products accumulate forever.** Test sessions create products (`MCP test apple`, `MCP duplicate-component recipe`, etc.) with no way to remove them. After two sessions there are four leftover rows in the cache. There's no `delete_product` or `list_products` tool today — `search_products` requires a query, so the agent can't even enumerate the cache to find what's there.
2. **`POST /products/recipes` silently accepts duplicate component product_ids.** Supplying the same `product_id` twice with different quantities produces two component rows, doubling the math contribution from that ingredient. The agent's mistake (or copy-paste footgun) becomes a silently wrong recipe.

This change adds the two missing CRUD tools and tightens the recipe-creation validator to reject duplicate components. Scope deliberately stays narrow: management surface for products, not redesigns of the products model.

Explicit non-goals: soft-delete / archive semantics; bulk delete; recipe nutrient-gap flagging (the "recipe inherits component nutriment gaps silently" finding from the test report is design work that deserves an `/opsx:explore`, not a fix here).

## What Changes

- **`DELETE /products/{id}`** — hard-delete a product.
  - Returns `204 No Content` on success.
  - Returns `404 product_not_found` when the id doesn't exist.
  - Returns `409 product_in_use_as_component` when the product is referenced by any `product_components` row, with the list of using recipes in the body: `{"error":"product_in_use_as_component","recipes":[{"id":"…","name":"…"}],"hint":"delete the listed recipes first, or replace this product within them"}`. The pre-check is explicit; the existing `ON DELETE RESTRICT` FK is the final safety net.
  - **Preserves historical meal entries.** Before deleting, the handler materialises the product's current name + nutriments into every `meal_entries` row whose `product_id` equals the target and whose `snapshot_*` columns are null. The existing FK then nulls `meal_entries.product_id` on cascade, but the snapshot keeps the meal's effective name and nutriments stable — daily/range summaries computed in the future still see the meal's contribution.
- **`GET /products`** — paginated list with optional filters.
  - Query params: `source` (optional, one of `off | manual | recipe`), `limit` (default 50, max 200), `offset` (default 0).
  - Response: `{"products": [...], "total": N, "limit": L, "offset": O}`.
  - Order: same as `/products/search` — `last_logged_at DESC NULLS LAST, name ASC`. This means recently-used products surface first and stale leftovers sink to the bottom, which is exactly the cleanup signal the user wants.
- **`POST /products/recipes` rejects duplicate component product_ids** with `400 component_duplicate, product_id: <which>, occurrences: <N>, hint: "sum the quantities and supply one entry per product"`. The error is loud (agent learns the constraint fast) and the hint suggests the fix.
- **MCP `delete_product` tool** wrapping `DELETE /products/{id}`. Tool description spells out the `409` semantics ("delete recipes first").
- **MCP `list_products` tool** wrapping `GET /products`. Tool description emphasises the recency ordering and the source filter so the agent has a natural "show me what's in the cache" path.

## Capabilities

### Modified Capabilities
- `products`: Adds `DELETE /products/{id}` with the snapshot-materialisation guarantee, adds `GET /products` paginated list, and tightens the composite-recipe requirement with a duplicate-component rejection scenario.
- `mcp-server`: Adds a new requirement covering the two product-management tools. The existing "eight tools" / "recipe and goals tools" requirements remain unchanged.

## Impact

- **Backend code**:
  - `internal/products/repo.go` gains `Delete(ctx, id)` and `List(ctx, source string?, limit, offset int)` methods. `Delete` runs inside `WithTx`: pre-check componentry, materialise snapshots into `meal_entries`, delete the row.
  - `internal/products/service.go` gains the `RecipesInUse(productID)` pre-check and the dedup check on recipe creation.
  - `internal/products/handlers.go` registers `DELETE /products/:id` and `GET /products`. Swag annotations document the new error codes.
  - `internal/products/recipes_handlers.go` (or wherever `POST /products/recipes` lives) adds the duplicate-component check before the existing validation.
- **MCP wrapper**:
  - `internal/mcpserver/tools_products.go` registers `delete_product` and `list_products`, wired through the auto-derive idempotency machinery the existing write tools use (delete is a POST-style mutation; list is read-only).
- **No schema changes.** The FK rules already match what we need: `product_components.component_product_id ON DELETE RESTRICT` enforces the in-use guard; `meal_entries.product_id ON DELETE SET NULL` lets us null the link after snapshotting.
- **Tests**:
  - Handler tests for delete: happy path, not-found, in-use-as-component, snapshot materialisation correctness.
  - Handler tests for list: pagination, source filter, ordering.
  - Recipe-creation test for duplicate-component rejection.
  - MCP tool tests for the two new tools (capture-the-request-body style, like the existing product tools).
  - One e2e flow addition: create a recipe → try to delete a component → expect 409 with the recipe listed → delete the recipe → retry delete → expect 204.
- **Documentation**: `task swag` regenerates OpenAPI; `README.md` Products section gains the two new endpoint examples; the MCP tools table gains two rows.

### Out of scope (explicit non-goals)
- Soft-delete / undo-delete / archive flag. Hard delete with the snapshot guarantee is enough for a single-user personal app.
- Bulk delete (`DELETE /products?ids=…`). Iteration via the MCP tool is fine.
- A recipe-replacement flow ("swap product A for product B inside all recipes that use A"). Deferred.
- Recipe nutrient-gap flagging — that's a presentation-shape concern the user surfaced in the test report; needs an `/opsx:explore` pass before it's pinned down.
- Pagination cursors (`next_cursor`). Offset/limit is fine for the single-user inventory size; cursor pagination is a future change if the cache ever grows past ~thousand rows.
- Permission scoping. The single-user model means any authenticated client can delete any product; that's by design for now.
