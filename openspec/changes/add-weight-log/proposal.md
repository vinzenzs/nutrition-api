## Why

The Garmin coach reads body-weight data from Garmin Connect, but the API itself has nowhere to record a weight measurement taken any other way — a kitchen scale at the in-laws', a hotel gym, a smart scale that's not Garmin-linked, a "just stepped on" reading from a friend's flat. As a result, the system's view of the user's weight is gappy and Garmin-dependent. That's a problem for three first-order use cases:

1. **Energy Availability** (`weekly_energy_summary`, a planned tool) needs body weight to compute `kcal / kg FFM / day`. Missing weight days mean missing EA days mean a useless trend.
2. **Race-day fuelling math** (`plan_carb_load`) takes body weight as a parameter. Today the agent has to ask each time. With a stored weight + simple trend, the agent can read the right number for race week.
3. **Trend signal**: athletes in a weight-loss block care about the *trajectory*, not any single day's reading. Daily noise (hydration, glycogen, post-meal water retention) routinely swings 1–2 kg. A rolling 7-day average is the standard fix; today there's no place to even compute it from.

This change adds the simplest capable shape: a `body_weight_entries` table, CRUD endpoints, and one trend endpoint that returns the rolling average over a window. No goal-weight, no projections, no smart-scale-specific composition columns — those are explicit follow-ups when real use shows they're earning their weight.

## What Changes

- **New `body_weight_entries` table**: `id`, `logged_at`, `weight_kg`, `body_fat_pct?`, `note?`, audit timestamps. One row per measurement (multiple per day allowed — the trend handles smoothing).
- **Five REST endpoints** mirroring the hydration shape:
  - `POST /weight` — log a measurement. Body: `{weight_kg, body_fat_pct?, logged_at, note?}`. Accepts the standard `Idempotency-Key` header.
  - `GET /weight?from=…&to=…` — list entries in a half-open RFC 3339 window, ordered by `logged_at` ascending. 92-day cap (matches `/meals`, `/hydration`).
  - `PATCH /weight/{id}` — partial update of `weight_kg`, `body_fat_pct`, `logged_at`, `note`.
  - `DELETE /weight/{id}` — remove an entry.
  - `GET /weight/trend?from=YYYY-MM-DD&to=YYYY-MM-DD&window_days=7` — daily rolling-average curve. Returns per-date `{rolling_avg_kg, sample_count}` so callers can render the trend and see data-quality (sparse-window) honestly.
- **Five MCP tools** wrapping each endpoint: `log_weight`, `list_weights`, `patch_weight`, `delete_weight`, `weight_trend`. Auto-derive idempotency on writes per the existing POST-style rule.
- **No changes to** `/summary/daily`, `/summary/range`, `nutrition_goals`, or any other capability. Weight is unit-isolated (kg ≠ g ≠ ml) and lives in its own response shape, just like hydration.

## Capabilities

### New Capabilities
- `body-weight`: A persisted log of body-weight measurements plus a rolling-average trend endpoint that suppresses daily noise. Forward-compatible with body-fat-percentage on the same row; further composition fields (lean mass, hydration %, bone mass) are explicit follow-ups when smart-scale data shows up.

### Modified Capabilities
- `mcp-server`: Adds a requirement covering the five new weight tools. The existing tool requirements are unchanged.

## Impact

- **Schema migration** at `internal/store/migrations/013_add_body_weight.{up,down}.sql` (next available number — `012_add_workouts` is in place). One table, one index on `(logged_at)`. No FKs.
- **New code**:
  - `internal/bodyweight/` package: `types.go`, `repo.go`, `service.go`, `handlers.go`, `trend.go` (rolling-average computation kept in its own file for clarity).
  - `internal/mcpserver/tools_weight.go` — five tools; `tools_hydration.go` is the closest template.
- **Wiring** in `internal/httpserver/server.go` (one repo, one service, one handlers registration) and `internal/mcpserver/server.go` (one `registerWeightTools` call).
- **No changes to** `meals`, `hydration`, `workouts`, `products`, `summary`, `goals`, `race-prep`, `off-integration`, `auth`. Body weight is genuinely standalone.
- **Tests**: handler-level tests for each endpoint, dedicated trend tests (sparse windows, single-entry windows, gap days), MCP unit tests for each tool, one integration test addition so the tools-list grows by five (25 → 30, assuming `add-workouts-capability`'s five tools are now in).
- **Documentation**: `task swag` regenerates OpenAPI; README gains a "Body weight" subsection + five MCP-table rows; RUN_LOCAL.md gains a short example of logging a weight and fetching the trend.

### Out of scope (explicit non-goals)

- **Goal weight + projected-date calculation.** "How long until I'm at 70 kg at current trend" is pure math over the data this change captures; ships as a follow-up tool (or alongside an EA tool that already needs body weight). Excluded here to keep the change small.
- **Lean mass / muscle mass / hydration % / bone mass** (smart-scale composition fields). Smart scales report them; nutrition tools don't need them today. Add columns when a real tool wants them.
- **Per-meal-type / per-time-of-day weighing buckets.** Morning weighing is the convention; the agent can filter by hour from the entries if it cares.
- **Garmin / source tagging on weight rows.** Workouts gained `source` + `external_id` because Garmin is the primary writer of activities. Weight's primary write path today is manual REST/agent; if a Garmin smart-scale sync surfaces, adding the columns is a small follow-up.
- **Bulk endpoint.** Weight backfill is at most ~365 entries per year of history; N POSTs at a polite rate is fine. If real backfill scenarios surface, `/weight/bulk` is a one-line follow-up (and the workouts proposal already pioneered the shape).
- **`/summary/weight/daily`-style endpoint.** Weight isn't an "aggregation over events in a day" — there are typically 0 or 1 entries per day. The trend endpoint covers the analytics use case; list + single-entry suffice for raw reads.
- **Exponential-weighted moving averages** or other smoothing algorithms. Simple moving average is the standard fix for daily-weighing noise; "give me a half-life parameter" is a real but second-order need. Add a `smoothing` query param later if needed.
- **Composition trend.** `weight_trend` covers `weight_kg` only in v1. Body-fat trend is the same algorithm over a different column; ships as a `weight_trend` query param (`metric=body_fat_pct`) or a sibling endpoint when first needed.
