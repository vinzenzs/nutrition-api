## 1. Schema migration

- [x] 1.1 Migration `008_add_last_logged_quantity` (up): `ALTER TABLE products ADD COLUMN last_logged_quantity_g NUMERIC(10, 3);`
- [x] 1.2 Same migration: optional one-shot backfill `UPDATE products SET last_logged_quantity_g = me.quantity_g FROM (SELECT DISTINCT ON (product_id) product_id, quantity_g FROM meal_entries WHERE product_id IS NOT NULL ORDER BY product_id, logged_at DESC) me WHERE products.id = me.product_id;` (skip if rows are large enough to matter at migration time — wrap in a comment block and let the operator decide).
- [x] 1.3 Down migration: `ALTER TABLE products DROP COLUMN IF EXISTS last_logged_quantity_g;`
- [x] 1.4 Extend `migrate_test.go` cycle assertions to include the new column existing after Up and absent after Down.

## 2. Store layer

- [x] 2.1 Add `LastLoggedQuantityG *float64` field to `products.Product` struct with JSON tag `json:"last_logged_quantity_g,omitempty"`.
- [x] 2.2 Extend `selectAllColumns` to include the new column.
- [x] 2.3 Extend `scanProduct` to scan into the new field.
- [x] 2.4 Extend `Insert` and `UpdateFromOFF` to write the column (initially null on create paths). For composite recipe creation (`UpdateRecipeNutriments`) the column stays null until the first meal log.
- [x] 2.5 Change `TouchLastLoggedAt(ctx, id, ts)` to `TouchLastLoggedAt(ctx, id, ts, quantityG)`. Same atomic conditional update: advance both columns when `ts > last_logged_at`.
- [x] 2.6 Repo unit test: round-trip a product with a non-nil `last_logged_quantity_g` through Insert / GetByID; verify the field is preserved and the JSON serialization omits it when null.

## 3. Meals service wiring

- [x] 3.1 In `internal/meals/service.go` `Create`: pass `in.QuantityG` to the renamed `TouchLastLoggedAt(*productID, loggedAt, quantityG)` call.
- [x] 3.2 In `Patch`: when the patch includes `quantity_g` AND the linked product's `last_logged_at` equals the patched meal's `logged_at`, propagate the new quantity. Otherwise no-op on the product. (The conditional UPDATE in step 2.5 handles this automatically — verify it via a focused test.)
- [x] 3.3 In `CreateFreeform` save_as_product branch: pass `in.QuantityG` to the touch call so the newly-created product starts with the right default.

## 4. Tests: tracking semantics

- [x] 4.1 Handler-level test: log a meal at 200g → GET product → `last_logged_quantity_g == 200`.
- [x] 4.2 Handler-level test: log meal A at `t1` with 200g, then log meal B at `t0 < t1` (backdated) with 50g → product reflects 200, not 50.
- [x] 4.3 Handler-level test: PATCH meal A's `quantity_g` to 300 → product's `last_logged_quantity_g` becomes 300.
- [x] 4.4 Handler-level test: PATCH older meal B's `quantity_g` → product's `last_logged_quantity_g` is unchanged.
- [x] 4.5 Handler-level test: delete meal A → product's `last_logged_quantity_g` is still 300.
- [x] 4.6 Handler-level test: freeform with save_as_product, supplying `quantity_g = 120` → new product row has `last_logged_quantity_g = 120`.

## 5. JSON surface verification

- [x] 5.1 Verify the new field appears in the response of `GET /products/{id}`, `GET /products/search`, `POST /products/lookup/{barcode}`, `POST /products`, `POST /products/recipes`, and the recompute endpoint — all via existing tests' assertions (just confirm none of them assert the field is absent; if they do, update to ignore it).
- [x] 5.2 Confirm null serialization is omitted (`omitempty`).

## 6. Docs

- [x] 6.1 Regenerate swagger (`task swag`).
- [x] 6.2 Update the Products section of README with a one-line note that GET responses now include `last_logged_quantity_g` for previously-logged products.

## 7. Pre-merge

- [x] 7.1 `task vet` clean.
- [x] 7.2 `task test` green.
- [x] 7.3 `openspec validate add-last-logged-quantity --strict` passes.
