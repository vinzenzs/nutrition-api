## Why

The MVP ships single-ingredient logging, macro-only summaries, and no concept of "did I hit my day?" — fine for a demo, painful by day three. Real meals are 3–5 ingredients (skyr + oats + berries + honey is one breakfast, not four log calls). A vegetarian diet rises or falls on B12, iron, and vitamin D, none of which are surfaced today. The numbers you get back from the daily summary are raw totals with no targets to compare against, so every check-in turns into mental arithmetic. And `meal_type` is recorded on every entry but no endpoint queries by it — the data is there but unreachable. These four gaps together are what stop the system from being something you'd actually use every day.

## What Changes

### 1. Recipes / composite products (criticals #1, nice-to-have "bulk logging")

- Add a notion of a **composite product**: a product that is a fixed list of component products + grams per component. `source = "recipe"` joins `"off"` and `"manual"`.
- New endpoint `POST /products/recipes` to create one (`{"name": "Morning skyr bowl", "components": [{"product_id": "...", "quantity_g": 200}, ...], "serving_size_g": 350}`). Nutriments-per-100g are computed and stored from components at creation time so existing meal-logging math keeps working.
- `POST /meals` accepts a composite `product_id` like any other product; one meal entry per recipe log, but `GET /meals/{id}?expand=components` exposes the component breakdown for the agent or UI to render.
- Recipes are re-computable: `POST /products/recipes/{id}/recompute` refreshes nutriments from current component values (if a component's nutriments were updated since recipe creation).
- This subsumes the nice-to-have "bulk logging" — a recipe IS the bulk-log unit.

### 2. Goals / targets surfaced in summaries (critical #2)

- New capability `nutrition-goals`: single-user goals row (kcal target, protein min/max, carbs min/max, fat min/max, fiber min, sugar max, salt max, plus per-micro targets — see #3).
- `PUT /goals` / `GET /goals` to set and read. Goals are user-scoped (single-user system today; the row id is fixed for now).
- `GET /summary/daily` and `GET /summary/range` add an `adherence` object per day: `{kcal: {actual, target, delta_pct, status: "under|on|over"}, protein_g: {...}, ...}`. Each macro/micro for which a target exists gets a status badge. No target → field omitted, never a fake zero.

### 3. Micronutrients (critical #3)

- Extend the products schema with iron_mg, calcium_mg, vitamin_d_mcg, vitamin_b12_mcg, vitamin_c_mg, magnesium_mg, potassium_mg, zinc_mg per 100g. Same null-tolerant pattern as existing macros (missing in OFF → null, not zero).
- Extend OFF parser to extract these from `product.nutriments` (`iron_100g`, `calcium_100g`, `vitamin-d_100g`, `vitamin-b12_100g`, `vitamin-c_100g`, `magnesium_100g`, `potassium_100g`, `zinc_100g`).
- Extend meal_entries snapshot columns to mirror the new micros so freeform entries can record them too.
- Daily/range summary totals include the micros (only when at least one contributing entry has a non-null value, so we don't fake-zero a day).
- `POST /meals/freeform` accepts these in `nutriments_per_100g`.

### 4. Meal-type queries (critical #4)

- `GET /summary/daily` adds an optional `?meal_type=breakfast` filter that scopes totals + entries to that meal type only.
- `GET /summary/range` adds an optional `?group_by=meal_type` mode that returns totals grouped by meal type per day (so "what's my average breakfast this week" works).
- `GET /meals` already supports `meal_type` filtering per the current spec; no change there.

### 5. MCP tools (downstream of all the above)

- New tools: `create_recipe`, `recompute_recipe`, `get_goals`, `set_goals`.
- Existing tools (`daily_summary`, `range_summary`, `log_meal_freeform`) gain the new optional parameters and richer response shapes. Backwards-compatible: agents that don't pass new params keep working.

### Deferred (not in this change)

- **Hydration** (nice-to-have): separate concern from food logging; warrants its own change later.
- **Estimated-vs-weighed metadata** (nice-to-have): real signal but small; bundling here bloats the change. Track as a follow-up.
- Multi-user goals — current system is single-user.

## Capabilities

### New Capabilities

- `nutrition-goals`: User-set targets for macros and micros, with `GET`/`PUT` endpoints and adherence computation that the summary capability composes into its responses.

### Modified Capabilities

- `products`: Adds composite ("recipe") product type with components, micros columns, recipe endpoints.
- `meals`: Adds component-expansion view, micros in snapshot columns and freeform input, recipe-aware meal entries.
- `off-integration`: Parses micronutrient fields from OFF payloads into the new typed columns.
- `summary` (currently part of meals spec — extracting into its own delta): Adds `meal_type` filter on daily, `group_by=meal_type` on range, `adherence` block in both, micros in totals.
- `mcp-server`: Adds four new tools and accepts new optional parameters on three existing tools.

## Impact

- **Schema**: three new migrations.
  - `005`: add micros columns to `products` and `meal_entries`.
  - `006`: add `product_components` table and `source = 'recipe'` to the products check constraint.
  - `007`: add `nutrition_goals` table (single-row, fixed id).
- **REST API**: new endpoints (`/products/recipes*`, `/goals`); existing endpoints gain optional fields. No breakage.
- **OFF parser**: pulls eight additional nutriment fields. Fixture set extended.
- **MCP**: four new tools, three tools' input/output shapes extended. Tool count goes from 8 → 12.
- **Docs**: README's "what the API does" section needs updating; agent-facing tool descriptions get extended.
- **Out of scope**: hydration, estimated-vs-weighed metadata, multi-user goals, recipe editing (delete + recreate for v1), per-meal-type targets (only per-day targets in v1).
