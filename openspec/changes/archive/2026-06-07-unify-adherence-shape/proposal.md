## Why

The same MCP test session that produced `harden-write-paths` also flagged three API-shape inconsistencies. None corrupts data; all annoy clients and make the API feel under-baked:

1. **`kcal_target` is a plain number, every other goal is an object.** The current shape pivots on the field type for a reason that no longer holds: kcal used to bake in a `±5%` tolerance that the rest of the goals exposed as explicit `{min, max}`. The asymmetry forces every adherence-row consumer to special-case kcal.
2. **Adherence rows leak inconsistently between daily and range summaries.** The existing nutrition-goals spec says "Goal set but no contributing entries omits the nutrient" — that's the documented rule for `GET /summary/daily`. The range endpoint, in the implementation that shipped after this spec was written, includes adherence rows on empty days. The two endpoints disagree on the same goals + same data.
3. **Float precision leaks through serialization** as `70.44969999999999`. Standard floating-point math artefact. Nothing harmful, but the API looks careless.

This change unifies all three with one coherent move: a single goal target shape (`{min?, max?}`), a single adherence-row shape (always present per configured goal, with a `no_data` status when nothing was logged), and consistent 1-dp rounding for every nutrient value on the way out. The change is breaking but pre-1.0 personal-app scope makes that acceptable.

## What Changes

- **`kcal_target` (scalar) becomes `kcal` (Range).** All targets across the goals model now have the shape `{min?: number, max?: number}` with both bounds optional. The implicit `±5%` rule for kcal is removed; the user sets explicit min/max. Field name drops the `_target` suffix for symmetry with macros (`protein_g`, `carbs_g`).
- **All goals normalize to the Range shape internally.** `MinOnly`, `MaxOnly`, and `Range` collapse to a single Go type `Range{Min, Max *float64}`. The semantic "min-only means no upper bound" is now expressed by `Max == nil`. The semantic "max-only means no lower bound" is now expressed by `Min == nil`. Stored representation in `nutrition_goals` is two columns per nutrient (`<nutrient>_min`, `<nutrient>_max`); `kcal_target` migrates to `kcal_min` and `kcal_max` (backfilled from the prior single value via ±5%).
- **Adherence rows are always present per configured goal.** `GET /summary/daily` and `GET /summary/range` produce one adherence entry for every goal the user has set, regardless of whether the day's meals contributed data. The "omit when no contributing entries" rule is replaced with an explicit `status: "no_data"` entry. `actual` becomes nullable (`*float64`) to honestly represent "no data" without writing fake zeros.
- **Adherence row target is one shape: `{min?, max?}`.** No more `Target: any` polymorphism. Clients always parse the same object.
- **Status enum extended to four values.** `under | on | over | no_data`. The first three keep their existing semantics; `no_data` fires when the day produced no contribution (null actual). For days with logged meals but the meals contained no relevant data, the rule depends on the nutrient: macros (always present in any meal) report the numeric actual with normal status; micros (legitimately absent in many products) report `no_data` when the column was null across every contributing meal.
- **All nutrient floats round to 1 decimal place at serialization.** Applies to `totals.*` in summary responses, `adherence.*.actual`, `adherence.*.target.{min,max}`, the goals row on read, and any product/meal response that exposes a stored nutriment value. Storage stays at full precision; the rounding is presentation-only. Implementation via a single helper called from response-building paths.
- **`PUT /goals` accepts the new shape only.** No backward-compat shim for `kcal_target`. A request that supplies the old field is rejected with `400 goal_value_invalid` (since `kcal_target` is now an unknown field). Documented in the migration notes so any existing scripts adapt in lockstep with the deploy.
- **MCP `set_goals` tool input mirrors the new shape.** The tool's input struct renames `kcal_target` to `kcal` and changes its type to the unified Range. The agent learns the new shape from the tool description.

## Capabilities

### Modified Capabilities
- `nutrition-goals`: Replaces `kcal_target` with `kcal` (Range); collapses `MinOnly` / `MaxOnly` / `Range` into a single `{min?, max?}` shape applied to every nutrient; adds the "always include configured-goal rows" rule with the new `no_data` status; introduces the 1-dp rounding rule for nutrient values in responses.
- `mcp-server`: The `set_goals` tool input schema renames `kcal_target` → `kcal` and changes its type to `{min?, max?}`. Tool description carries the new shape.

(`meals/spec.md` is not modified — the summary endpoints are owned there but their adherence block is owned by nutrition-goals. The 1-dp rule for `totals.*` rides along under the nutrition-goals spec since that's where the response-building rule is documented.)

## Impact

- **Migration** at `internal/store/migrations/`:
  - Add `kcal_min NUMERIC(10,3)` and `kcal_max NUMERIC(10,3)` columns to `nutrition_goals`.
  - Backfill: `UPDATE nutrition_goals SET kcal_min = kcal_target * 0.95, kcal_max = kcal_target * 1.05 WHERE kcal_target IS NOT NULL`.
  - Drop `kcal_target`.
- **Backend code**:
  - `internal/goals/types.go`: collapse `MinOnly`, `MaxOnly` into `Range`; replace `KcalTarget *float64` with `Kcal *Range`; update field tags accordingly.
  - `internal/goals/repo.go`: update SELECT / INSERT / UPDATE to use the new column set (two columns per nutrient).
  - `internal/goals/handlers.go`: validate `min <= max` when both supplied; reject the old `kcal_target` field.
  - `internal/summary/service.go`: `AdherenceEntry.Target` becomes the unified Range type (not `any`); `AdherenceEntry.Actual` becomes `*float64`; status set extended; build adherence rows for every configured goal regardless of data.
  - New helper `roundNutrient(*float64) *float64` (or per-type) called by every response-building path. Storage and computation unchanged.
- **MCP wrapper**: `internal/mcpserver/tools_goals.go` (or current location of `set_goals`) input struct renamed and re-typed.
- **Tests**:
  - Goals handler tests for the new shape (kcal Range; partial Range with only min or only max; rejection of `kcal_target`).
  - Summary tests for the four adherence statuses including `no_data`.
  - Summary tests confirming daily and range produce identical adherence shape for the same goals + data.
  - Rounding test against the documented 70.4496999... reproducer (or equivalent: build a recipe with deliberate float drift, fetch a summary, assert the JSON shows `70.4` or `70.5`).
- **Documentation**:
  - `task swag` regenerates the OpenAPI annotations for `PUT /goals` and the summary endpoints.
  - `RUN_LOCAL.md` "Recipe + goals walkthrough" updates the goal example body to use the new `kcal` shape.

### Out of scope (explicit non-goals)
- Backward compatibility for the old `kcal_target` field. Hard break; pre-1.0 scope.
- ETag / If-Match retry-safety on `PUT /goals` (still forward-pointed from `harden-write-paths`).
- A goals history / versioning model. The row is still a singleton with one timestamp.
- Recipe quality / nutrient-gap indicators (a separate explore topic).
- `list_products` / `delete_product` MCP tools (lives in the planned `add-product-management-tools` change).
- Changing the rounding rule per-nutrient (kcal at 0 dp, mcg at 2 dp). Uniform 1 dp keeps the rule reviewable in one line; if real ergonomic problems surface, we tune later.
