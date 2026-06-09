## 1. Service types + entry point

- [x] 1.1 `internal/summary/rolling.go`: response types — `Rolling{AnchorDate, WindowDays, TZ; Averages Totals; DaysWithData, TotalDays int; Days []RollingDay; Adherence Adherence; GoalSource string}` and `RollingDay{Date string; Totals Totals; HasData bool}`. `Totals` reuses the existing `summary.Totals` shape — same field set as `daily_summary` / `range_summary` so downstream tooling parsing those JSON shapes does not fork.
- [x] 1.2 `RollingParams{AnchorDate civil-date (use `time.Time` at local midnight in `loc`); WindowDays int; Loc *time.Location}` — validated by the handler; the service trusts them in-range.
- [x] 1.3 Add `func (s *Service) RollingFor(ctx context.Context, p RollingParams) (*Rolling, error)` next to the existing `DailyFor` / `RangeFor`.

## 2. Day-bucket aggregation

- [x] 2.1 Compute the window: `startDate = anchor − (window_days − 1) days` in `loc`. Enumerate the `window_days` calendar dates from `startDate` through `anchor` inclusive; the in-memory loop mirrors the energy package's `buildDays`.
- [x] 2.2 Pull every meal in the unioned UTC range `[localMidnight(startDate, loc), localMidnight(anchor + 1d, loc))` via `mealsRepo.List(ctx, meals.ListParams{From, To})`. One query.
- [x] 2.3 Bucket each meal by `LoggedAt.In(loc).Format("2006-01-02")` into a map; each bucket holds the meals for that local-calendar date.
- [x] 2.4 For each enumerated date, fold via `summary.SumEntries(bucket)` into a `RollingDay`. `HasData = len(bucket) > 0`. Empty days emit a zero-totals row with `HasData: false`.

## 3. Window averages + adherence

- [x] 3.1 `Averages`: for each field on `Totals`, accumulate values from days with `HasData: true` only; final value = `sum / DaysWithData`. Nullable micros: sum only the non-nil contributions; if zero non-nil contributors across the whole window, the field stays nil.
- [x] 3.2 When `DaysWithData == 0`: `Averages` is the zero `Totals` (all macros `0.0`, all nullable micros `nil`).
- [x] 3.3 Adherence: resolve the goal at `anchor_date` via `goals.Resolver.ResolveAt(ctx, anchor_date)` — same path `daily_summary` uses. Build the `Adherence` map per the existing `unify-adherence-shape` convention: one entry per configured nutrient, `Actual = pointer to window-average value`, `Target` from the resolved goal, `Status` via the existing thresholding helper. When `DaysWithData == 0`, every entry has `Status: "no_data"` and `Actual: nil`.
- [x] 3.4 `GoalSource` echoes the resolver's result (`"default"` or `"override"`).

## 4. Rounding at the response boundary

- [x] 4.1 Round `Averages` via the existing `rounded()` / `roundTotals()` helper from `summary` (or `numfmt.Round1` / `numfmt.Round1Ptr` directly, matching whichever pattern `RangeFor` already uses).
- [x] 4.2 Round each per-day `RollingDay.Totals` the same way.
- [x] 4.3 Round adherence `Actual` and `DeltaPct` per the existing daily/range pattern.

## 5. HTTP handler

- [x] 5.1 `internal/summary/handlers.go`: add `rg.GET("/summary/rolling", h.rolling)` next to the existing `daily` and `rng` registrations.
- [x] 5.2 Parse query params:
  - `anchor_date` (required, `YYYY-MM-DD`) — return `400 anchor_date_required` if missing, `400 anchor_date_invalid` if malformed.
  - `window_days` (required, integer) — return `400 window_days_required` if missing, `400 window_days_invalid` (with `{"range":{"min":2,"max":30}}`) if outside `[2, 30]` or unparseable.
  - `tz` (optional) — default to the handler's `defaultTZ`; `400 tz_invalid` on `time.LoadLocation` failure.
- [x] 5.3 Materialise `RollingParams` and call `svc.RollingFor`. Map any service errors to `500 compute_failed`.
- [x] 5.4 Swag annotations: list every documented error code, reference `Rolling` as the success type.

## 6. Wiring

- [x] 6.1 `internal/httpserver/server.go`: no constructor change — `summary.NewHandlers(...)` already gets the existing service; the new handler routes off the same `Service`. Confirm `RollingFor` is reachable via the existing API group.

## 7. Backend tests

- [x] 7.1 `internal/summary/rolling_test.go` — table-driven over the day-bucket helper using `storetest.NewPool`:
  - Happy path: 7-day window with one meal/day → seven `Days` entries, `Averages` matches the per-day mean, `DaysWithData == TotalDays == 7`.
  - Sparse window: 7-day window, meals on 5 days only → `DaysWithData: 5`, divisor is 5, the empty days have `HasData: false`.
  - Empty window: 7 days, no meals → all-zero `Averages`, every per-day row has `HasData: false`, adherence statuses all `no_data`.
  - Zero-kcal logged day: a single meal with `0` computed kcal → `HasData: true`, the day's `totals.kcal == 0`, divisor includes the day.
  - TZ boundaries: meal at `22:30Z` with `tz=Europe/Berlin` (UTC+2) lands on the LOCAL next day; assert the per-day bucket attribution.
  - DST-spanning window: 7-day window across the spring-forward boundary → still seven `Days` rows; durations are calendar days, not 24h slices.
  - `window_days` boundaries: `window_days=2` returns two rows; `window_days=30` returns thirty rows; `window_days=1` and `window_days=31` rejected with the documented error and range payload.
  - Adherence at the anchor with a same-day override → `GoalSource == "override"`, `Target` reflects the override; default-day adherence → `GoalSource == "default"`.
  - Rounding: assemble per-day totals that produce a `.x666...` average; assert the response is rounded to 1dp.
- [x] 7.2 `internal/summary/handlers_test.go` (extension): end-to-end through Gin
  - Happy path response shape including `anchor_date`, `tz`, `days_with_data`, `total_days`, and ordered `days` array.
  - Every documented `400` reachable via query manipulation.
  - `tz` defaulting to `DEFAULT_USER_TZ` when omitted.

## 8. MCP wrapper

- [x] 8.1 `internal/mcpserver/tools_summary.go`: add `RollingSummaryArgs{AnchorDate string; WindowDays int; TZ string ",omitempty"}` next to the existing daily/range args.
- [x] 8.2 `handleRollingSummary`: build the query (`anchor_date`, `window_days`, optional `tz`), call `c.Get(ctx, "/summary/rolling", q)`. No `Idempotency-Key`. Forward via `toToolResult`.
- [x] 8.3 Add the `mcp.AddTool` registration inside `registerSummaryTools` (the function already groups `daily_summary` + `range_summary`; this slots in alongside).
- [x] 8.4 Tool description per the spec — call out the trailing-window-includes-anchor semantic, the days-with-data divisor rule, and the typical-windows hint (3 / 7 / 14 / 30 days).

## 9. MCP tests

- [x] 9.1 `internal/mcpserver/tools_summary_test.go` (extension) using the recorder pattern. Cover:
  - GET URL + method + endpoint path (`/summary/rolling`).
  - Required params always included; `tz` only when supplied.
  - No `Idempotency-Key` header.
  - REST `200` body forwarded verbatim.
  - REST `400 window_days_invalid` forwarded as `isError: true`.
- [x] 9.2 `internal/mcpserver/mcp_integration_test.go` expected-tools list: add `rolling_summary`. Tool count grows by 1 (was 36 after `add-energy-availability`; now 37).

## 10. Documentation

- [x] 10.1 `task swag` regenerates OpenAPI with the new endpoint + response shape.
- [x] 10.2 `README.md`:
  - "Summaries" subsection gains the rolling example, placed after `/summary/range` and before `/summary/hydration/daily`. Show: 7-day rolling at today, an example with a sparse window output (`days_with_data` != `total_days`), and the 14-day call with `tz=Europe/Berlin`.
  - Add `rolling_summary` to the MCP tools table next to `daily_summary` / `range_summary`.
- [x] 10.3 `RUN_LOCAL.md` walkthrough: append a one-liner showing `/summary/rolling?anchor_date=$(date +%Y-%m-%d)&window_days=7&tz=Europe/Berlin` next to the existing daily-summary example.

## 11. Pre-merge checks

- [x] 11.1 `task vet` clean.
- [x] 11.2 `task test` green per-package — `internal/summary/` and `internal/mcpserver/` both pass. Full-module `go test -p 1 ./...` flaked on the testcontainers pool ping in `summary` (the same flake pattern observed under add-workout-fuel and add-energy-availability); re-running `summary` in isolation passes.
- [x] 11.3 Manual e2e with `task dev`:
  - Log meals on 5 of the last 7 days (skip 2).
  - `GET /summary/rolling?anchor_date=$(date +%Y-%m-%d)&window_days=7&tz=Europe/Berlin` → assert `days_with_data == 5`, `total_days == 7`, `averages.kcal` matches manual SUM-divided-by-5.
  - `GET /summary/rolling?anchor_date=...&window_days=1` → assert `400 window_days_invalid` with the range payload.
- [x] 11.4 OpenSpec validation: `openspec status --change "add-rolling-window-summaries"` shows 4/4 artifacts done.
