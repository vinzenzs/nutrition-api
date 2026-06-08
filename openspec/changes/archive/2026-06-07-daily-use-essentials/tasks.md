## 1. Schema migrations

- [x] 1.1 Migration `005_add_micronutrients`: add `iron_mg_per_100g`, `calcium_mg_per_100g`, `vitamin_d_mcg_per_100g`, `vitamin_b12_mcg_per_100g`, `vitamin_c_mg_per_100g`, `magnesium_mg_per_100g`, `potassium_mg_per_100g`, `zinc_mg_per_100g` columns to `products` (all `NUMERIC(10,3)`, nullable).
- [x] 1.2 Same migration: add matching `snapshot_*_per_100g` columns to `meal_entries`.
- [x] 1.3 Migration `006_add_recipes`: extend `products.source` CHECK to include `'recipe'`. Add `nutriment_computed_at TIMESTAMPTZ` column to `products`.
- [x] 1.4 Same migration: create `product_components(id UUID PK, product_id UUID REFERENCES products(id) ON DELETE CASCADE, component_product_id UUID REFERENCES products(id) ON DELETE RESTRICT, quantity_g NUMERIC(10,3) NOT NULL CHECK (quantity_g > 0), position INT NOT NULL DEFAULT 0)`. Index on `(product_id)`.
- [x] 1.5 Migration `007_add_nutrition_goals`: create `nutrition_goals` table with one row sentinel pattern. Columns: `id UUID PK DEFAULT '00000000-0000-0000-0000-000000000001'::uuid`, `kcal_target NUMERIC(10,3)`, then `<nutrient>_min` and `<nutrient>_max` columns for protein_g, carbs_g, fat_g, fiber_g (min only), sugar_g (max only), salt_g (max only), plus `<micro>_min` columns for the eight micros. `created_at`, `updated_at`. CHECK constraint enforces singleton id.
- [x] 1.6 Write down migrations for 005, 006, 007 (drop columns / drop tables in reverse). Verify down + up cycles cleanly in `migrate_test.go` style.

## 2. Store layer (sqlc / querier)

- [x] 2.1 Update product row struct + scan helpers in `internal/products/repo.go` and `internal/store/querier.go` to include the eight micro columns and `nutriment_computed_at`.
- [x] 2.2 Update meal_entry row struct to include the eight snapshot micro columns.
- [x] 2.3 Add CRUD helpers for `product_components`: `InsertComponents(tx, productID, []Component)`, `ListComponents(productID)`, `DeleteComponents(productID)`.
- [x] 2.4 Add CRUD helpers for `nutrition_goals`: `GetGoals()`, `UpsertGoals(NutritionGoals)`. Both no-op safely when the singleton row is absent / present.
- [x] 2.5 Update existing SELECT queries that return product rows to project the new columns.

## 3. OFF parser: micros

- [x] 3.1 Extend the OFF parser in `internal/off/` to read `iron_100g`, `calcium_100g`, `vitamin-d_100g`, `vitamin-b12_100g`, `vitamin-c_100g`, `magnesium_100g`, `potassium_100g`, `zinc_100g` from `product.nutriments`. Map to the typed columns with unit-correctness verified per the off-integration spec scenario.
- [x] 3.2 Add `testdata/off/<barcode>.json` fixtures: one fully-populated product with all macros + all eight micros; one macros-only product (micros absent); update an existing fixture if it already covers a partial micro set.
- [x] 3.3 Extend parser unit tests: each new field is read when present, left null when absent; existing tests for kJ fallback / serving_size still pass.

## 4. Products REST handlers: micros

- [x] 4.1 Extend `POST /products` handler to accept and validate (`>= 0`, numeric) the eight micro fields in `nutriments_per_100g`. Persist via the updated store layer.
- [x] 4.2 Extend `GET /products/{id}` and `GET /products/search` responses to include micros under `nutriments_per_100g`, omitting null fields.
- [x] 4.3 Extend `POST /products/lookup/{barcode}` flow: the OFF parser now writes the micros — verify the handler returns them in the response.
- [x] 4.4 Handler tests: each new micro round-trips on create + retrieve; null micros are absent (not zero) in responses; OFF lookup populates micros when fixture has them.

## 5. Products REST handlers: recipes

- [x] 5.1 New handler `POST /products/recipes`. Validate body (`name` required, `components` non-empty array, each `{product_id, quantity_g>0}`), reject components whose `source = 'recipe'` (no nested recipes in v1), reject unknown component product ids with 404 `component_not_found`. Compute gram-weighted-average nutriments across components for each macro and micro. Persist product row (source=`recipe`), product_components rows, and `nutriment_computed_at = now()` in one transaction. Return 201 with product + echoed `components`.
- [x] 5.2 New handler `POST /products/recipes/{id}/recompute`. Reject if product not found (404) or `source != 'recipe'` (400 `not_a_recipe`). Recompute nutriments from current component effective values, update product row + `nutriment_computed_at`, return 200.
- [x] 5.3 Extend `GET /products/{id}` to honor `?expand=components`: when the product is a recipe and `expand=components` is set, include a `components: [{product_id, name, quantity_g, effective_nutriments_per_100g}]` array; for non-recipes with `expand=components`, return `components: []`.
- [x] 5.4 Helper function `ComputeRecipeNutriments(components []ComponentWithProduct) NutrimentsPer100g` shared between create and recompute paths. Unit-test gram-weighted averaging and null propagation directly.
- [x] 5.5 Handler tests for `POST /products/recipes`: happy path, missing components, unknown component, zero quantity, recipe-as-component rejection, gram-weighted math against a known fixture.
- [x] 5.6 Handler tests for recompute: happy path updates nutriments after a simulated component change; 400 on non-recipe; 404 on missing recipe.
- [x] 5.7 Handler tests for `GET /products/{id}?expand=components`: recipe returns scaled components, non-recipe returns empty array, omitting the flag returns no components field.

## 6. Meals REST handlers: micros + components

- [x] 6.1 Extend `POST /meals/freeform` to accept the eight micros inside `nutriments_per_100g`, validate `>= 0`, and write to the matching `snapshot_*` columns. Same validation error shape (`nutriments_invalid` with `field`).
- [x] 6.2 Extend the freeform `save_as_product=true` branch to persist micros on the created manual product.
- [x] 6.3 Extend `GET /meals/{id}` to support `?expand=components`. When the linked product is a recipe, compute scaled component breakdown `quantity_g_component = recipe_component_qty * meal.quantity_g / (recipe.serving_size_g OR 100)` per component and return as a `components` array; otherwise return `components: []`.
- [x] 6.4 Handler tests: freeform with micros round-trips; freeform save_as_product propagates micros to the product; meal expand returns scaled components for recipe-backed meals and empty array otherwise.

## 7. Summary handlers: micros + meal_type + adherence

- [x] 7.1 Update the summary computation in `internal/summary/service.go` to include each of the eight micros in the per-day totals. Apply the "no fake-zero" rule: omit a micro from `totals` when no contributing entry has a non-null effective value for that day.
- [x] 7.2 Add `meal_type` query parameter to `GET /summary/daily`. Validate against the enum (reuse meals validation), reject invalid values with `meal_type_invalid`. When set, filter entries to that meal type only, omit `adherence`, include `meal_type` in the response echo.
- [x] 7.3 Add `group_by` query parameter to `GET /summary/range` (only `meal_type` is valid in v1, reject others with `group_by_invalid`). When set, replace each day's top-level `totals` with `by_meal_type: {<type>: {totals: {...}}, ...}`. Omit meal types with no entries on a day. Omit `adherence` in this mode.
- [x] 7.4 Implement adherence computation: read goals row, for each goal-targeted nutrient compute `{actual, target, delta_pct, status}` per the rules in `specs/nutrition-goals/spec.md`. kcal uses ±5% tolerance; min/max ranges use boundary checks; min-only never "over"; max-only never "under". Omit the entry when no goal is set; omit when no contributing entry has a non-null effective value for that nutrient.
- [x] 7.5 Wire adherence into `GET /summary/daily` and per-day entries of `GET /summary/range`. Skip adherence when filter/group mode is active (per the spec).
- [x] 7.6 Handler tests covering each adherence status branch (under/on/over per kcal, per range, per min-only, per max-only); meal_type filter on daily; group_by=meal_type on range; no-fake-zero rule for micros; goals-unset case omits adherence cleanly.

## 8. Goals REST handlers

- [x] 8.1 New handler `GET /goals`. Returns `{"goals": null}` when the row is absent; otherwise returns the goals object with null fields omitted.
- [x] 8.2 New handler `PUT /goals`. Validate each field (`>= 0`, numeric, `min <= max` where both present); reject with `goal_value_invalid` / `goal_range_invalid` per spec. Upsert against the singleton id. Return 200 with the stored goals.
- [x] 8.3 Handler tests: empty initial state returns null; first PUT creates the row; subsequent PUT replaces fields (including clearing previously-set fields by omitting them); validation errors for each branch.

## 9. MCP server: new tools

- [x] 9.1 In `internal/mcpserver/tools_products.go`, register `create_recipe` with input schema mirroring `POST /products/recipes`. Forward an explicit `idempotency_key` when supplied, derive otherwise. Description mentions composite meals.
- [x] 9.2 Register `recompute_recipe` with input `{product_id: uuid (required)}`. Forward to `POST /products/recipes/{id}/recompute`.
- [x] 9.3 New file `internal/mcpserver/tools_goals.go`. Register `get_goals` (no input) → `GET /goals`. Register `set_goals` with input schema mirroring the PUT body. Apply idempotency forwarding/derivation.
- [x] 9.4 Update `internal/mcpserver/server.go` registration to include the four new tools. Tool count is now 12.
- [x] 9.5 Unit tests with stubbed apiclient covering each new tool: correct endpoint hit, idempotency-key behavior, error passthrough for the documented REST error shapes (`component_not_found`, `not_a_recipe`, `goal_value_invalid`).

## 10. MCP server: existing tool extensions

- [x] 10.1 Extend `daily_summary` input schema with optional `meal_type` enum. When supplied, append `&meal_type=` to the query string; otherwise omit.
- [x] 10.2 Extend `range_summary` input schema with optional `group_by` (only `"meal_type"` valid in v1). Forward to query string when supplied.
- [x] 10.3 Extend `log_meal_freeform` input schema: add the eight micros under `nutriments_per_100g` as optional. Forward the body verbatim — no transformation.
- [x] 10.4 Update tool descriptions:
  - `log_meal_freeform`: append guidance pointing to `create_recipe` for repeated multi-ingredient meals.
  - `search_products`: note that results include `source` of `off`, `manual`, or `recipe`.
- [x] 10.5 Tests covering the new optional fields: present-forwarded vs absent-omitted, schema accepts the micros, descriptions contain the new sentences.

## 11. Docs & developer ergonomics

- [x] 11.1 Update `README.md`'s API summary table with the four new endpoints and two new query parameters.
- [x] 11.2 Update OpenAPI/Swagger spec under `docs/` (or `internal/api-docs/`) to reflect new endpoints, micros in nutriment objects, adherence response shape, meal_type/group_by params, and goals endpoints.
- [x] 11.3 Update `RUN_LOCAL.md` to include a sample curl session: create two manual products → create a recipe from them → set goals → log the recipe → check daily summary including adherence and components.
- [x] 11.4 Update `Taskfile.yml` if any new tasks help (e.g. a `task seed:recipes` for local smoke tests). Optional — only if it speeds up the dev loop.

## 12. Validation & sign-off

- [x] 12.1 Run full test suite. All existing meals/products/off-integration/mcp tests still pass.
- [x] 12.2 End-to-end smoke: spin up `task dev`, run the RUN_LOCAL recipe scenario via curl, then via the MCP server (using the registered Claude Desktop config), confirm adherence + components show as expected.
- [x] 12.3 Verify migration up/down/up cycle on a clean local DB.
- [x] 12.4 Run `openspec validate daily-use-essentials --strict` (or equivalent) and resolve any spec lint errors.
- [x] 12.5 Open PR. Title: `daily-use-essentials: recipes, goals, micros, meal-type queries`. Link to this change directory.
