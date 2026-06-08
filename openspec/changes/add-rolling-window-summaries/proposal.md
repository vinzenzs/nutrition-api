## Why

Almost every nutrition-science recommendation an endurance athlete cares about is a multi-day average, not a single-day total:

- **Protein for MPS**: ~1.6–2.2 g/kg/day averaged across a week — daily spikes don't save a chronically-low week.
- **Energy Availability**: Loucks bands are diagnosed over 5–14 days; one bad day is noise, two bad weeks is the signal.
- **Carb-load window**: 72-hour rolling carbs/kg leading into a race.
- **Sodium baseline**: weekly average matters more than any individual ride.

Today the API answers "what did I eat **today**" (`daily_summary`) and "what did I eat **across these specific days**" (`range_summary`). It does NOT answer "what's my **trailing 7-day average** as of today" without the agent doing `range_summary` + division client-side. Every agent conversation that opens with "how am I doing this week?" pays that round-trip + math tax — and gets it slightly wrong when days are missing (the agent divides by `to - from` rather than the count of days with data, treating a missing log day as zero).

Closes T1 #1B in `openspec/priorities.md` — the only remaining Tier-1 item explicitly tagged "cheap pure-function add; ship after Tier 1 mechanical items." The Tier-1 mechanical items (workouts, meal-workout-link, workout-fuel, EA, weight-log) shipped 2026-06-08. This is the natural next step.

The shape is a pure read endpoint over existing `meal_entries` — no new schema, no migration, no new write paths. Mirrors the `weight_trend` shape (which already does the rolling-window thing for body weight) — same pattern, applied to nutrition.

## What Changes

- **New `GET /summary/rolling` endpoint**: query params `anchor_date` (YYYY-MM-DD), `window_days` (int, 2..30), `tz` (IANA, defaults to `DEFAULT_USER_TZ`). Returns the trailing-window aggregate plus per-day breakdown.
  - Window semantics: `[anchor_date − (window_days − 1), anchor_date]`, both inclusive, calendar-day buckets in the requested `tz`. `anchor_date` itself IS in the window — "7-day rolling as of June 8" means June 2 through June 8.
  - Response shape:
    ```json
    {
      "anchor_date": "2026-06-08",
      "window_days": 7,
      "tz": "Europe/Berlin",
      "averages": {
        "kcal":      2280.5,
        "protein_g": 128.0,
        "carbs_g":   285.5,
        "fat_g":     78.0,
        "fiber_g":   31.0,
        "sugar_g":   55.0,
        "salt_g":    8.5,
        "iron_mg":      null,
        "calcium_mg":   null,
        "vitamin_d_mcg": null,
        "vitamin_b12_mcg": null,
        "vitamin_c_mg": null,
        "magnesium_mg": null,
        "potassium_mg": null,
        "zinc_mg":      null
      },
      "days_with_data": 6,
      "total_days": 7,
      "days": [
        { "date": "2026-06-02", "totals": { ... }, "has_data": true },
        { "date": "2026-06-03", "totals": { ... }, "has_data": true },
        ...
        { "date": "2026-06-05", "totals": { ... }, "has_data": false },
        ...
      ],
      "adherence": {
        "protein_g": { "actual": 128.0, "target": {"min": 120, "max": 150},
                       "delta_pct": null, "status": "on" },
        ...
      },
      "goal_source": "default"
    }
    ```
  - **Averaging rule (the load-bearing decision)**: averages are over **days with data** — not over `total_days`. A user logging 6 of 7 days returns the 6-day mean, with `days_with_data: 6` exposing the divisor. Treating missing days as zero would let an agent congratulate the user on a 75 g/day protein "average" that's really 87.5 g/day across the days they actually ate. Honest-data-first beats convenient-arithmetic.
  - Per-day `totals` use the same shape as `daily_summary` totals (so the response is structurally homogeneous with `range_summary` for any downstream tooling that already parses `range_summary`).
  - Per-day `has_data: false` distinguishes "logged zero" from "didn't log" — same NULL-vs-zero hygiene as the workout-fuel response.
  - Adherence is computed against the resolved goal at `anchor_date` (most-recent goal that applies, honoring `add-date-varying-goals` overrides at the anchor). Adherence `actual` is the window average; `target` is the goal range. `status` follows the existing `under` / `on` / `over` / `no_data` semantics from `daily_summary`.

- **New MCP tool `rolling_summary`** wrapping the endpoint. Same param shape as the REST query. Tool description should call out (a) the trailing-window-includes-anchor convention, (b) the "averages over days-with-data" rule, and (c) the typical windows for endurance use (3, 7, 14, 30 days).

## Capabilities

### Modified Capabilities

- `meals`: Adds the rolling-window summary requirement alongside the existing daily and range requirements.
- `mcp-server`: Adds one new tool requirement for `rolling_summary`.

## Impact

- **Prerequisites (all already shipped)**: `add-date-varying-goals` (for goal resolution at the anchor), the existing daily-summary pipeline (`internal/summary/`). Nothing else needed.
- **No schema migration**. Pure aggregation over `meal_entries` + `nutrition_goals` + `daily_goal_overrides`.
- **New code**:
  - `internal/summary/rolling.go`: new types (`Rolling`, `RollingParams`) + service method `RollingFor(ctx, RollingParams) (*Rolling, error)`. Sits next to the existing `DailyFor` / `RangeFor`.
  - `internal/summary/handlers.go`: third route `GET /summary/rolling` next to the existing two.
  - `internal/mcpserver/tools_summary.go`: add `RollingSummaryArgs` + `handleRollingSummary` + register the tool. (`tools_summary.go` already holds `daily_summary` and `range_summary`; this is a third wrapper.)
- **Tests**:
  - `internal/summary/rolling_test.go`: 7-day window happy path, sparse-data divisor honesty (averages over `days_with_data` not `total_days`), zero-data day flagged as `has_data: false`, calendar-day boundaries in non-UTC TZ, anchor at start of DST, `window_days` boundary values (2 → 2-day window, 30 → 30-day window, 1 / 31 → `400 window_days_invalid`), goal resolution at anchor including override day.
  - MCP recorder tests for `rolling_summary` — endpoint URL, query string, no `Idempotency-Key` (read-only).
- **Documentation**: `task swag`; README "Summaries" subsection gains the rolling example placed next to `/summary/daily` and `/summary/range`; MCP tools table gains `rolling_summary`. RUN_LOCAL.md gets a one-liner showing the 7-day rolling pattern.
- **No idempotency middleware impact** — read-only.

### Out of scope (explicit non-goals)

- **`group_by=meal_type` on the rolling endpoint.** `range_summary` supports it for "breakfast average across May"; the rolling endpoint is opinionated as a single-window-at-a-time tool. Add later if real use shows the friction.
- **Per-nutrient rolling windows of different lengths.** ("Protein 7d, sodium 3d, EA 14d in one call.") Composer over the existing endpoint; agent can call three times.
- **Hydration rolling-summary** (ml/day average). Same pattern but separate endpoint under `/summary/hydration/`. If real use shows it earns the surface, follow-up — but the daily hydration question is rarely "what's my 7-day average ml" in practice.
- **Streak / consecutive-days-on-target metrics.** That's a different shape (Tier-3 idea). The window-average answers "trend" honestly; streaks are a behavioural-nudge surface that's a separate proposal.
- **Range-with-rolling-overlay** (e.g. `range_summary` returning a rolling-avg column alongside per-day). The two endpoints stay separate to keep response shapes simple.
- **Rolling-window EA.** The EA endpoint's `window.avg_ea` is already a window-mean; adding rolling adds complexity without new signal. The agent composes if needed.
- **Per-meal-type adherence inside the rolling response.** Adherence stays goal-level (one entry per nutrient), matching `daily_summary`.
- **Caching.** Window cap is small (≤ 30 days); pgx pool + the existing meals/goals query path is fast enough. Premature optimization.
