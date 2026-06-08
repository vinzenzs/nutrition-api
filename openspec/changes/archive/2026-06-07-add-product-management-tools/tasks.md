## 1. Backend: list endpoint

- [x] 1.1 Add `List(ctx, source string, limit, offset int)` to `internal/products/repo.go`. SQL: `SELECT … FROM products WHERE ($1::text IS NULL OR source = $1::text) ORDER BY last_logged_at DESC NULLS LAST, name ASC LIMIT $2 OFFSET $3`. Re-use `selectAllColumns`.
- [x] 1.2 Add `Count(ctx, source string)` returning `(int64, error)`. SQL mirrors the WHERE clause from List, replaces SELECT/ORDER/LIMIT with `SELECT COUNT(*)`.
- [x] 1.3 In `internal/products/handlers.go`, register `GET /products` ahead of the parametrised `:id` route so it doesn't get swallowed. Validate `source` against the allow-list (`off`, `manual`, `recipe`) returning `400 source_invalid`. Validate `limit` and `offset` per the spec (negative → `pagination_invalid`, limit > 200 → `limit_too_large`).
- [x] 1.4 Build response struct `{Products []*Product, Total int64, Limit, Offset int}`. Default `limit=50`, `offset=0` when omitted.
- [x] 1.5 Update swag annotations with the new query params and error codes.

## 2. Backend: delete endpoint

- [x] 2.1 Add `RecipesUsing(ctx, productID uuid.UUID)` in `internal/products/repo.go` returning `[]struct{ID uuid.UUID; Name string}`. SQL joins `product_components` to `products` to surface the recipe name.
- [x] 2.2 Add `MaterialiseSnapshot(ctx tx, productID uuid.UUID)` in `internal/products/repo.go`. SQL: `UPDATE meal_entries SET snapshot_name = COALESCE(snapshot_name, p.name), snapshot_kcal_per_100g = COALESCE(snapshot_kcal_per_100g, p.kcal_per_100g), ... (every nutriment column) FROM products p WHERE meal_entries.product_id = $1 AND p.id = $1 AND meal_entries.snapshot_name IS NULL`.
- [x] 2.3 Add `Delete(ctx tx, productID uuid.UUID)` performing the bare DELETE — FK cascade handles `meal_entries.product_id`.
- [x] 2.4 In `internal/products/service.go`, add `Delete(ctx, productID)` that runs inside `store.WithTx`:
  - Check existence via `repo.GetByID`; map `ErrNotFound` to a service-level sentinel.
  - Call `repo.RecipesUsing`; if non-empty, return `*ErrProductInUseAsComponent{Recipes: [...]}`.
  - Call `repo.MaterialiseSnapshot`.
  - Call `repo.Delete`.
- [x] 2.5 In `internal/products/handlers.go`, register `DELETE /products/:id`. Map error types:
  - `ErrNotFound` → `404 product_not_found`.
  - `*ErrProductInUseAsComponent` → `409` with `{"error":"product_in_use_as_component","recipes":[...],"hint":"delete the listed recipes first, or replace this product within them"}`.
  - pgx unique-violation / foreign-key error (`23503`) as a belt-and-suspenders fallback → `409 product_in_use_as_component` with an empty `recipes` array.
  - Success → `204 No Content`.
- [x] 2.6 Update swag annotations for `DELETE /products/{id}`.

## 3. Backend: recipe duplicate-component rejection

- [x] 3.1 In `internal/products/service.go` (or `recipe_compute.go`, wherever recipe creation lives), add a `DuplicateComponentError{ProductID uuid.UUID; Occurrences int}` type.
- [x] 3.2 Before the existing component validation in `CreateRecipe`, iterate the input components and count occurrences per `product_id`; if any count > 1, return the duplicate error with the offending id and count.
- [x] 3.3 In `internal/products/recipes_handlers.go`, map `*DuplicateComponentError` to `400` with body `{"error":"component_duplicate","product_id":"<id>","occurrences":<N>,"hint":"sum the quantities and supply one entry per product"}`.

## 4. Backend: tests

- [x] 4.1 List handler tests: default pagination, source filter, large offset returns empty, limit_too_large, pagination_invalid, source_invalid, ordering recency-then-name.
- [x] 4.2 Delete handler tests: 204 on simple delete, 404 on unknown id, 409 with recipes list when product is a component, retried-delete idempotence (204 then 404).
- [x] 4.3 Delete snapshot-materialisation test: create product → log meal against it (no freeform snapshot) → delete product → fetch the meal → assert `snapshot_name` and every nutriment snapshot column are populated from the product's last value.
- [x] 4.4 Delete + freeform interaction test: log a freeform meal that also linked to a product → delete the product → assert the freeform snapshot was NOT overwritten.
- [x] 4.5 Recipe duplicate-component test: POST `/products/recipes` with two components sharing the same `product_id` → assert `400 component_duplicate`, no rows created.
- [x] 4.6 e2e flow test in `internal/e2e/`: create two products → create recipe using both → try delete one component product → assert `409` with the recipe listed → delete the recipe → retry delete on the component product → assert `204`.

## 5. MCP wrapper

- [x] 5.1 In `internal/mcpserver/tools_products.go`, add `ListProductsArgs{Source *string; Limit *int; Offset *int; IdempotencyKey string}` and `DeleteProductArgs{ProductID string; IdempotencyKey string}`. Use jsonschema tags to document the field semantics; the agent reads the schema as docs.
- [x] 5.2 Add `handleListProducts(ctx, c, args)` that builds the query string from non-nil args and calls `c.Get(ctx, "/products", q)`. Return the body through `toToolResult`.
- [x] 5.3 Add `handleDeleteProduct(ctx, c, args)` that calls `c.Delete(ctx, "/products/"+args.ProductID, key)` with the effective idempotency key. On `204` from the backend, return a clean `{IsError:false, Content:[]}` rather than passing the empty body straight through; this avoids the agent seeing an empty string and getting confused.
- [x] 5.4 Register both tools in `registerProductsTools`. Descriptions per the spec scenarios (recency ordering, source filter, 409-on-component-use, cleanup pattern). `delete_product` description ends with "If you get a 409, delete the listed recipes first or replace this product within them; then retry."
- [x] 5.5 Wire `list_products` to NOT auto-derive an idempotency key (read-only); `delete_product` to use the standard auto-derive path for POST-style writes.

## 6. MCP tests

- [x] 6.1 Test `handleListProducts`: query construction (with and without each filter), response passthrough.
- [x] 6.2 Test `handleDeleteProduct`: happy path 204, 404 passthrough, 409 in-use body forwarded verbatim including the `recipes` array.
- [x] 6.3 Update the MCP integration test (`mcp_integration_test.go`) so the expected tools-list now includes `list_products` and `delete_product`.

## 7. Documentation

- [x] 7.1 `task swag` to regenerate OpenAPI for the new routes.
- [x] 7.2 README.md Products section gains a `list` example (`GET /products`) and a `delete` example (`DELETE /products/<uuid>`); MCP tools table gains the two new rows.
- [x] 7.3 Update `internal/mcpserver/README.md` (if it exists) with the same two new tools.
- [x] 7.4 Add a one-paragraph note to `RUN_LOCAL.md`'s API walkthrough showing how to clean up leftover products: `list_products` to enumerate, `delete_product` to remove.

## 8. Pre-merge checks

- [x] 8.1 `task vet` clean.
- [x] 8.2 `task test` green (including the new e2e flow).
- [x] 8.3 Manual: with `task dev` running, create a recipe whose components include a test product, try to delete the component product → confirm 409 JSON; delete the recipe → retry the delete → confirm 204. Then `list_products?source=manual` confirms the cache state.
- [x] 8.4 OpenSpec validation: `openspec status --change "add-product-management-tools"` shows 4/4 artifacts done.
