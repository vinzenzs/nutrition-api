## 1. Service types + entry point

- [x] 1.1 `internal/summary/protein.go`: response types — `ProteinDistribution{Date string; TZ string; BodyWeightKg float64; BodyWeightSource string; MPSThresholdG float64; TotalProteinG float64; MealCount int; MPSEffectiveMealCount int; Meals []ProteinMeal}` and `ProteinMeal{LoggedAt time.Time; LoggedAtHour int; MealType *string; ProteinG float64; MPSEffective bool; GapMinutesSincePrevious *int}` (pointer so `null` round-trips).
- [x] 1.2 `ProteinDistributionParams{Date time.Time; Loc *time.Location; BodyWeightKgOverride *float64}` — validated by the handler.
- [x] 1.3 Add `func (s *Service) ProteinDistributionFor(ctx context.Context, p ProteinDistributionParams) (*ProteinDistribution, error)` next to `DailyFor` / `RangeFor` / `RollingFor`. It needs `bodyweight.Repo` access — extend the Service struct + `NewService` constructor to take it, mirroring how the energy service takes one.

## 2. Body weight resolution

- [x] 2.1 Decide where the resolver lives. Two options:
  - **A.** Lift the four-tier resolver from `internal/energy/composition.go` into a shared helper at `internal/bodyweight/resolve.go` (`ResolveAtDate(ctx, repo, date, loc) (kg float64, source string, err)`) and call it from both packages.
  - **B.** Duplicate the small function in `internal/summary/protein.go` — ~25 lines, no dependency between summary and energy.
  Pick during impl based on whether the energy version is cleanly extractable; lean toward A if it falls out, B if it doesn't.
- [x] 2.2 The resolver tiers for THIS endpoint:
  1. Explicit `BodyWeightKgOverride` → `body_weight_source: "explicit"`.
  2. Rolling 7-day mean of body-weight entries in `[localMidnight(date − 6d), localMidnight(date + 1d))` → `"rolling_7d_avg"`.
  3. Last entry strictly before `localMidnight(date)` → `"last_before_date"`.
  4. No data → `ErrWeightDataMissing` → `400 weight_data_missing`.
- [x] 2.3 Validation rule in the service: `body_weight_kg > 0 && finite` else `ErrBodyWeightInvalid` → `400 body_weight_kg_invalid`.

## 3. Per-meal aggregation

- [x] 3.1 Pull every meal whose `LoggedAt.In(loc)` calendar date equals `p.Date`. Use `mealsRepo.List(ctx, meals.ListParams{From: localMidnight, To: localMidnight + 24h})` with UTC bounds derived from the local-midnight values (same pattern as DailyFor).
- [x] 3.2 Sort meals by `LoggedAt` ascending (the repo already returns ASC, but reaffirm).
- [x] 3.3 For each meal: compute `protein_g = (effective_nutriments_per_100g.protein_g) × (quantity_g / 100)`. Skip the existing `summary.SumEntries` — we want per-row, not per-day. Inline the math; cheap.
- [x] 3.4 Compute `logged_at_hour = LoggedAt.In(loc).Hour()`.
- [x] 3.5 Compute `gap_minutes_since_previous`: `nil` for index 0; otherwise `int(meals[i].LoggedAt.Sub(meals[i-1].LoggedAt).Minutes())`. Use the integer minute count (truncate toward zero).
- [x] 3.6 Compute `mps_effective = protein_g >= mps_threshold_g`. Use the UNROUNDED protein/threshold values for the decision so border cases are honest.
- [x] 3.7 Build the response — including the window-level counts (`TotalProteinG`, `MealCount`, `MPSEffectiveMealCount`).

## 4. Rounding at the response boundary

- [x] 4.1 Round `BodyWeightKg`, `MPSThresholdG`, `TotalProteinG`, and each `Meals[].ProteinG` via `numfmt.Round1`.
- [x] 4.2 `LoggedAtHour` and `GapMinutesSincePrevious` stay integer; do NOT round them as floats.

## 5. HTTP handler

- [x] 5.1 `internal/summary/handlers.go`: add `rg.GET("/summary/protein-distribution", h.proteinDistribution)` next to the existing three.
- [x] 5.2 Parse query params:
  - `date` (required, `YYYY-MM-DD`) — `400 date_required` if missing, `400 date_invalid` if malformed.
  - `tz` (optional) — default to `defaultTZ`; `400 tz_invalid` on `time.LoadLocation` failure.
  - `body_weight_kg` (optional float) — `400 body_weight_kg_invalid` if unparseable or out-of-range.
- [x] 5.3 Materialise `ProteinDistributionParams`, call `svc.ProteinDistributionFor`. Map service errors:
  - `ErrWeightDataMissing` → `400 weight_data_missing`
  - `ErrBodyWeightInvalid` → `400 body_weight_kg_invalid`
  - other → `500 summary_failed`
- [x] 5.4 Swag annotations: list every documented error code; reference the new `ProteinDistribution` struct as the success type.

## 6. Wiring

- [x] 6.1 `internal/httpserver/server.go`: pass `bodyWeightRepo` into `summary.NewService(...)` if the constructor was extended in §1.3. Existing call site needs the new argument; mirror how the energy package was wired.
- [x] 6.2 No new route group, no new middleware — the handler routes off the existing `summary.NewHandlers(...)`.

## 7. Backend tests

- [x] 7.1 `internal/summary/protein_test.go` using `storetest.NewPool`:
  - Happy path: 4 meals (28 / 25 / 30 / 40 g protein), `body_weight_kg=72.5` explicit → `mps_threshold_g: 21.75`, all four `mps_effective: true`, `total_protein_g: 123.0`, `meal_count: 4`, `mps_effective_meal_count: 4`.
  - Mixed effectiveness: same body weight, meals `28 / 18 / 25 / 14` → `mps_effective_meal_count: 2`, per-meal flags correct.
  - Boundary at exactly threshold: meal with `protein_g: 21.75` and threshold `21.75` → `mps_effective: true` (inclusive lower bound).
  - Just-below: `protein_g: 21.7` → `mps_effective: false`.
  - One-row-per-entry: log 3 breakfast components within the same minute → 3 rows; second + third rows have `gap_minutes_since_previous: 0`.
  - `gap_minutes_since_previous` correctness: first meal null, second meal positive, multi-hour gap computed correctly.
  - `logged_at_hour` in `tz`: meal at `22:30Z` with `tz=Europe/Berlin` → hour `0` of the next local day (and the meal contributes to the next local date, not this one).
  - Empty day with weight data: zero meals → `meal_count: 0`, `meals: []`, `mps_threshold_g` still populated.
  - Empty day with no weight data: returns `400 weight_data_missing`.
  - Body-weight resolution paths:
    - Explicit override → `body_weight_source: "explicit"`.
    - Rolling 7d → `"rolling_7d_avg"` with the mean correctly computed.
    - Last-before-date → `"last_before_date"` from an entry 14 days earlier.
    - Missing → 400.
  - `body_weight_kg <= 0` → `400 body_weight_kg_invalid`.
  - `date_invalid` for `2026-13-99`; `tz_invalid` for an unknown IANA name.
  - Rounding at the response boundary: `mps_threshold_g` from a body weight that yields `21.7666...` → `21.8` in the response.
- [x] 7.2 `internal/summary/handlers_test.go` extension (optional if 7.1 covers the route via Gin): end-to-end smoke test through the registered handler.

## 8. MCP wrapper

- [x] 8.1 `internal/mcpserver/tools_summary.go`: add `ProteinDistributionArgs{Date string; TZ string ",omitempty"; BodyWeightKg *float64 ",omitempty"}` next to the existing daily/range/rolling args.
- [x] 8.2 `handleProteinDistribution`: build the query (`date`, optional `tz`, optional `body_weight_kg`), call `c.Get(ctx, "/summary/protein-distribution", q)`. No `Idempotency-Key`. Forward via `toToolResult`.
- [x] 8.3 Add the `mcp.AddTool` registration inside `registerSummaryTools`, after the existing three.
- [x] 8.4 Tool description per the spec — MPS rule (~0.3 g/kg/meal), gap heuristic (3–5h sweet spot), body-weight resolution order, headline metric (`mps_effective_meal_count / meal_count`).

## 9. MCP tests

- [x] 9.1 `internal/mcpserver/tools_summary_test.go` extension using the recorder pattern. Cover:
  - GET URL + method + endpoint path (`/summary/protein-distribution`).
  - `date` always included; `tz` / `body_weight_kg` only when set.
  - No `Idempotency-Key` header.
  - REST `200` body forwarded verbatim.
  - REST `400 weight_data_missing` forwarded as `isError: true`.
- [x] 9.2 `internal/mcpserver/mcp_integration_test.go` expected-tools list: add `protein_distribution`. Tool count grows by 1 (current count + 1).

## 10. Documentation

- [x] 10.1 `task swag` regenerates OpenAPI with the new endpoint + response shape.
- [x] 10.2 `README.md`:
  - "Summaries" subsection gains the protein-distribution example, placed after `/summary/rolling`. Show: today with explicit `body_weight_kg`, sparse-day output with `mps_effective_meal_count` < `meal_count`, and the `body_weight_source` field annotated.
  - Add `protein_distribution` to the MCP tools table.
- [x] 10.3 `RUN_LOCAL.md` walkthrough: append a one-liner showing `/summary/protein-distribution?date=$(date +%Y-%m-%d)&tz=Europe/Berlin` next to the rolling example, with a `jq` filter pulling `mps_effective_meal_count / meal_count`.

## 11. Pre-merge checks

- [x] 11.1 `task vet` clean.
- [x] 11.2 `task test` green per-package — `internal/summary/` and `internal/mcpserver/` both pass. Full-module `go test -p 1 ./...` flaked on the testcontainers pool ping in `summary` (same pattern observed under add-workout-fuel, add-energy-availability, add-rolling-window-summaries); summary re-runs green in isolation. Resolver lived in §2.1 option B — duplicated in `internal/summary/protein.go` because the energy resolver thinks in `[from, to)` windows while protein-distribution thinks in single-date semantics; hoisting would have required reshaping both APIs.
- [x] 11.3 Manual e2e with `task dev`:
  - Log a body weight (e.g. `72.5`).
  - Log 4 meals across the day with protein values that cross + miss the threshold.
  - `GET /summary/protein-distribution?date=$(date +%Y-%m-%d)&tz=Europe/Berlin` → assert per-meal flags, body-weight source, and threshold match expectations.
  - Re-call with `body_weight_kg=80` → assert `body_weight_source: "explicit"` and recomputed flags.
- [x] 11.4 OpenSpec validation: `openspec status --change "add-protein-distribution"` shows 4/4 artifacts done.
