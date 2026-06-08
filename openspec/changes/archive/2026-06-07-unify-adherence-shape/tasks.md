## 1. Migration

- [x] 1.1 New migration `NNN_unify_kcal_to_range.up.sql` / `.down.sql`. Up: add `kcal_min NUMERIC(10,3)` and `kcal_max NUMERIC(10,3)` columns to `nutrition_goals`; backfill `kcal_min = kcal_target * 0.95`, `kcal_max = kcal_target * 1.05` where `kcal_target IS NOT NULL`; drop `kcal_target`.
- [x] 1.2 Down: add `kcal_target NUMERIC(10,3)`; set `kcal_target = (kcal_min + kcal_max) / 2` where both bounds are non-null; drop `kcal_min` and `kcal_max`. The lossy nature is documented in `design.md`.
- [x] 1.3 Confirm the up + down migrations are idempotent against a freshly-migrated `task dev` database.

## 2. Backend: types and repo

- [x] 2.1 In `internal/goals/types.go`, collapse `MinOnly` and `MaxOnly` types into the existing `Range`. Replace every pointer field across `Goals` with `*Range`. Replace `KcalTarget *float64` with `Kcal *Range`. Update JSON tags so the field serializes as `kcal`, not `kcal_target`.
- [x] 2.2 In `internal/goals/repo.go`, update `SELECT` projection, `INSERT`, and `UPDATE` to round-trip both columns per nutrient. Specifically: `kcal_min, kcal_max` replace `kcal_target` in the column list and scan/exec arg slice.
- [x] 2.3 Update unit tests in `internal/goals/repo_test.go` for the new column layout (insert + read-back round-trips).

## 3. Backend: handler validation

- [x] 3.1 `internal/goals/handlers.go` `PUT /goals` body struct: rename `KcalTarget` to `Kcal *Range` (or whatever the existing field shape is named). Use `Range` everywhere.
- [x] 3.2 Reject the legacy `kcal_target` field with `400 goal_value_invalid, field: kcal_target`. Implementation hint: decode with `json.NewDecoder(r.Body).DisallowUnknownFields()` so a stray `kcal_target` key triggers a decode error you can catch and translate. Verify no other validation paths rely on extra fields slipping through.
- [x] 3.3 Reject empty range objects (`{}`) with `400 goal_value_invalid, field: <which>`.
- [x] 3.4 Reject `min > max` with `400 goal_range_invalid, field: <which>`.
- [x] 3.5 Reject negative or NaN values per the existing rule.
- [x] 3.6 Update swag annotations on `PUT /goals` to reflect the new field shapes and the rejection codes.
- [x] 3.7 Update `internal/goals/handlers_test.go` to cover: the new shape acceptance, single-bound acceptance, empty `{}` rejection, inverted range rejection, legacy `kcal_target` rejection.

## 4. Backend: adherence builder

- [x] 4.1 In `internal/summary/service.go`, update `AdherenceEntry`: `Actual` becomes `*float64`, `Target` becomes the unified `goals.Range` type (not `any`), `Status` keeps its string type but now includes `"no_data"` as a fourth valid value.
- [x] 4.2 Rewrite the adherence-build helper(s) to iterate **every configured goal field** (i.e. every non-nil pointer in `goals.Goals`) and produce exactly one entry. For each entry, compute `Actual` from the corresponding total (a `*float64` for micros, a `float64` from macros wrapped as a pointer). Compute `Status` per the rules: nil actual → `no_data`; else compare against `Target.Min` / `Target.Max`.
- [x] 4.3 Compute `DeltaPct` only when `Actual != nil` AND the target has at least one bound. Reference point: the midpoint of `min` and `max` if both present; the single bound otherwise. Omit when `no_data`.
- [x] 4.4 Use the same builder from both `DailyFor` and the per-day loop in `RangeFor`. Delete any range-side adherence code that diverged.
- [x] 4.5 Update `internal/summary/service_test.go` (or wherever tests live) to cover: 15 goals + empty day → 15 adherence rows all `no_data`; 15 goals + meal-logged day → 15 rows with correct statuses; daily and range produce structurally identical adherence for the same `(goals, day)`.

## 5. Backend: rounding helper at the response boundary

- [x] 5.1 Add a helper in `internal/summary/` (or a shared `internal/numfmt` package) implementing `round1(f float64) float64 { return math.Round(f*10) / 10 }` and `round1p(p *float64) *float64` (nil-passthrough variant).
- [x] 5.2 Call `round1` / `round1p` on every nutrient field of every response-building path:
  - `summary.Totals.{Kcal, ProteinG, CarbsG, FatG, FiberG, SugarG, SaltG}` and the eight micro pointer fields.
  - `summary.AdherenceEntry.{Actual, Target.Min, Target.Max, DeltaPct}`.
  - `goals.Goals.<each nutrient>.{Min, Max}` on read in `internal/goals/handlers.go`.
  - Product and meal response paths that expose stored nutriment values (review `internal/products/handlers.go`, `internal/meals/handlers.go`).
- [x] 5.3 Storage and computation must NOT be rounded — verify by inspection that `meals.Service.Create*`, `products.Service.*`, and the recipe recompute path do not call `round1` before persisting.
- [x] 5.4 Add a test using the documented reproducer (or equivalent: a 240 g meal contributing 70.4496999… → assert the daily summary's JSON shows `70.4`).

## 6. MCP wrapper

- [x] 6.1 In `internal/mcpserver/tools_goals.go` (or current set_goals registration), rename the `KcalTarget` field on the input struct to `Kcal` and change its type to `*Range` matching the backend shape. Make sure the jsonschema tag reads `the kcal range: {min?, max?}`.
- [x] 6.2 Update the tool's `Description` to include one sentence on how to construct ranges from a user-stated "I want N kcal a day" (suggest ±5% as a default the agent may use unless the user states a tighter or wider tolerance).
- [x] 6.3 Verify the MCP integration test exercises the new shape — at minimum, a `tools/list` should include a `set_goals` whose schema has `kcal: {type: object, properties: {min, max}}`.

## 7. Documentation

- [x] 7.1 `task swag` to regenerate `docs/` so OpenAPI reflects the new shapes for `PUT /goals`, `GET /goals`, `GET /summary/daily`, and `GET /summary/range`.
- [x] 7.2 Update `README.md` API examples that show goals or adherence bodies — the curl snippet in the Goals section and the example response shapes in the Summary section.
- [x] 7.3 Update `RUN_LOCAL.md` "Recipe + goals walkthrough" so step 3 uses the new `kcal: {min, max}` shape and step 5's expected adherence response shows the unified row format.

## 8. Pre-merge checks

- [x] 8.1 `task vet` clean.
- [x] 8.2 `task test` green.
- [x] 8.3 Manual end-to-end: `task dev`; `PUT /goals` with the full new-shape body; log a meal; `GET /summary/daily` → confirm adherence rows include every goal, rounding is 1 dp, status fields are sensible.
- [x] 8.4 Manual e2e: with full goals set, `GET /summary/range?from=<empty-day>&to=<empty-day>` and confirm adherence still emits all configured-goal rows with `actual: null, status: "no_data"`.
- [x] 8.5 OpenSpec validation: `openspec status --change "unify-adherence-shape"` shows 4/4 artifacts done.
