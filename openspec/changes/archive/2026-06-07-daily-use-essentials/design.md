## Context

The nutrition API today is a single-user macro tracker that ingests via barcode, OFF cache, freeform LLM estimate, or manual entry, and answers daily/range summary questions over macros. It's plumbed end-to-end (REST + MCP) and works, but four gaps stop it from being daily-driver useful: composite meals, goals, micros, and meal-type queries.

This change layers all four onto the existing data model without breaking the wire. The composite-products approach reuses the existing `products` row + per-100g columns rather than introducing a parallel "recipes" table, so meal-logging math doesn't fork. Goals are a single fixed-id row (no users table to extend). Micros are nullable additions to existing per-100g column sets. Meal-type queries reuse the existing `meal_type` column with new query params.

Pre-existing changes `add-mcp-server` and `streamline-local-dev` are still in flight. The mcp-server spec delta in this change ADDS new requirements alongside the ones add-mcp-server introduces; the two are mergeable in either order, and any tool count / description language that needs reconciling at archive time stays local to the mcp-server spec.

## Goals / Non-Goals

**Goals:**
- One log call per real-world meal when the meal is repeated regularly (recipes).
- "Did I hit my day?" answerable from the summary endpoint alone, no client-side math.
- Vegetarian-relevant micros (B12, iron, vit D) flow from OFF → product → meal entry → daily total → adherence.
- Meal-type slicing of summaries ("avg breakfast this week", "what did I have for breakfast today").
- Zero breaking changes on the wire: existing clients keep working with their existing parameter set and response shape.

**Non-Goals:**
- Recipe editing in place (delete + recreate is acceptable for v1).
- Hydration tracking (separate concern, separate change).
- Estimated-vs-weighed metadata on entries (small, defer).
- Multi-user goals (system is single-user).
- Per-meal-type goal targets (only per-day in v1).
- Trends/coaching surface beyond what daily+range adherence already gives.
- Carbohydrate sub-breakdowns (added sugar already covered as `sugar_g`; no need for "of which sugar" subtype today).

## Decisions

### Recipes as composite products, not a separate table

A recipe is a `products` row with `source = "recipe"` plus a `product_components` join table that holds the component product_ids and their per-100g grams. The recipe's typed nutriment columns are computed at creation time as the gram-weighted average of components' effective nutriments. Meal-logging code path stays identical: `POST /meals` with a recipe's `product_id` works exactly like any other product log.

**Alternatives considered:**
- *Separate recipes table with on-the-fly nutriment computation at meal-log time*: rejected. Forks the meal-logging math, requires every summary query to know about recipes, multiplies code paths.
- *Inline component-list on the meal entry itself (no recipe row)*: rejected. Defeats the "I eat this every morning, log it once" purpose — every meal log would re-name and re-quantify the components.

**Trade-off:** if a component's nutriments change later (OFF refresh), recipes become stale. Mitigated by `POST /products/recipes/{id}/recompute`. We deliberately do NOT recompute automatically on every component change because (a) it's surprising — meal entries shouldn't shift retroactively, (b) cross-row triggers would be a perf trap on large catalogs.

### Goals as a single fixed-id row

The system is single-user; goals are a single row in `nutrition_goals` with a fixed sentinel id (`'00000000-0000-0000-0000-000000000001'::uuid` or a dedicated singleton primary key). `GET /goals` returns `{"goals": null}` until first write; `PUT /goals` upserts.

**Alternative considered:** key by user_id once a users table exists. Rejected as YAGNI; the change is single-table and the migration to per-user goals is trivial when a users table appears (`ALTER TABLE nutrition_goals ADD COLUMN user_id UUID NOT NULL DEFAULT '<sentinel>'`).

**Trade-off:** no goal history (when did the user change kcal_target?). Mitigated by `created_at`/`updated_at` columns on the row. A goals-audit-log is out of scope; the row's `updated_at` is enough for now.

### Adherence is computed at query time, not stored

`adherence` is derived in the summary handler by joining daily totals against the goals row. Nothing is persisted.

**Alternative considered:** materialized daily adherence rows. Rejected — small cardinality, query is cheap, materialization adds invalidation complexity (changing a goal would have to backfill).

### Micros are typed columns on existing tables, not a generic key-value bag

We add eight specific micro columns to `products` and `meal_entries` (`iron_mg`, `calcium_mg`, `vitamin_d_mcg`, `vitamin_b12_mcg`, `vitamin_c_mg`, `magnesium_mg`, `potassium_mg`, `zinc_mg`). Same nullable pattern as existing macros.

**Alternative considered:** generic `nutriments JSONB` column. Rejected — loses type safety, breaks the consistent `coalesce(snapshot_x, product.x)` summary math, and makes index strategies impossible.

**Trade-off:** adding a new micro later is a migration. Accepted — micros aren't volatile; eight covers vegetarian-relevant nutrients without bloating the schema, and the OFF payload is preserved verbatim in `off_payload` for any future re-extraction.

### "No fake-zero" rule for micros in summaries

If no contributing meal entry has a non-null effective value for a micro on a given day, the daily summary OMITS that micro from `totals` rather than reporting zero. Rationale: zero implies "you had no iron today" which is wrong; the truth is "we don't know". Macros keep the existing zero-default behavior because they're present on essentially every logged food.

### meal_type query parameter on daily, group_by on range

- Daily takes `?meal_type=<type>` because the use case is "slice this day".
- Range takes `?group_by=meal_type` because the use case is "compare meal types across days" — filtering would lose the comparative angle.

Adherence is omitted in both filtered/grouped modes — per-day goals don't apply to a single meal type. We deliberately don't invent per-meal-type goal targets here (kept as non-goal).

### MCP tool count grows from 8 → 12, no renames

Four new tools (`create_recipe`, `recompute_recipe`, `get_goals`, `set_goals`). No existing tool gets renamed or its input shape narrowed; new optional fields are additive. This keeps Claude Desktop and Claude Code configs valid through the change.

## Risks / Trade-offs

- **Recipe nutriments go stale silently when components change.** → Mitigation: `recompute` endpoint + we'll surface `nutriment_computed_at` on recipe products so the agent / future UI can flag stale recipes. (No automatic re-compute by design — see Decisions above.)
- **OFF coverage of micros is uneven.** Many products list none of the micro fields. → Acceptance: that's why the spec is null-tolerant and adherence omits empty micros. Users with goals for unsupported micros will see those goals as "no data" rather than "missed" in the agent's response.
- **Composite recipes can be created with circular references** if we let a recipe component be another recipe. → Mitigation: v1 rejects recipe components whose `source = "recipe"` (`{"error":"recipe_as_component_not_supported"}`). Nested recipes are out of scope; revisit if real users hit the limit.
- **Single-row goals model is fragile under concurrent writes.** → Mitigation: PUT is an upsert with a fixed primary key; no race produces a duplicate row. Last-write-wins is the right semantics for "set my targets".
- **Increased response payload size on summary endpoints** from micros + adherence. → Acceptance: payloads are still well under any practical limit (8 extra numeric fields, a flat adherence object). No pagination needed.
- **Spec delta against the still-unarchived mcp-server capability.** → Mitigation: this change's mcp-server delta uses ADDED requirements only and references new tool names (no MODIFIED collisions with add-mcp-server's spec). Either change can archive first.

## Migration Plan

1. **Migration 005** — `ALTER TABLE products ADD COLUMN iron_mg_per_100g NUMERIC(10,3), ...` (eight columns); same on `meal_entries.snapshot_*`. Backfill not required (nulls are correct).
2. **Migration 006** — `ALTER TABLE products` extend the source CHECK to include `'recipe'`. Create `product_components(id UUID PK, product_id UUID FK products NOT NULL, component_product_id UUID FK products NOT NULL, quantity_g NUMERIC(10,3) NOT NULL CHECK (quantity_g > 0))`. Add `nutriment_computed_at TIMESTAMPTZ` to products.
3. **Migration 007** — `CREATE TABLE nutrition_goals` with the singleton id pattern and one nullable column per goal field (or a JSONB column — flat columns are clearer for the small fixed set).
4. **REST handlers** — new endpoints (`/products/recipes*`, `/goals`), extended params on daily / range / freeform. All previous endpoints stay shape-stable.
5. **MCP wrapper** — register four new tools, extend three input schemas. Tool descriptions updated.
6. **OFF parser** — extract eight additional fields. Fixture set extended (one fully-populated, one macros-only).
7. **Tests** — schema migration tests, handler tests, OFF parsing tests, MCP tool tests, summary adherence + filter tests.

**Rollback:** down migrations exist for 005/006/007 (drop columns / table). REST changes are additive — old binaries serving the new schema work; new binaries serving the old schema fail at startup because handlers reference missing columns. So rollback order: revert binaries first, then run down migrations. Standard order.

## Open Questions

- Should `kcal_target` be on/off ±5% or a configurable tolerance? Going with ±5% in v1 to keep the spec deterministic; revisit if it feels wrong in practice.
- Should `set_goals` be PATCH or PUT semantics? Spec'd PUT (replace-all) for simplicity. If users complain "I cleared a goal by accident", switch to PATCH.
- Recipe nutriment recomputation: should it auto-fire when a component is updated via `POST /products/lookup/<barcode>?refresh=true`? Spec'd NO for v1; revisit once users report stale-recipe pain.
- Should the daily summary include a `meal_types_breakdown` view (always-on per-meal-type totals) instead of requiring the filter? Considered, deferred; keeps the daily response slim and the filter approach is what the agent will use anyway.
