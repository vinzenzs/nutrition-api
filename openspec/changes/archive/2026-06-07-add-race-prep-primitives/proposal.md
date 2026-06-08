## Why

A race week has a specific carb-loading shape — roughly 8–12 g of carbohydrate per kg of body weight per day for the 3 days before, then ~2 g/kg as a pre-race meal on race morning — and it's exactly the kind of math an LLM agent should never compute from scratch. The numbers are simple but the consequences of getting them wrong (under-fuelling a 70.3, over-eating into GI distress on race morning) are real. With a sprint tri on 2026-07-24 and a 70.3 build behind it, the user needs a deterministic primitive to anchor the agent's planning conversations.

This change adds exactly one such primitive: a pure function that, given a race date and a body weight, returns the daily carbohydrate target for the carb-load window and race morning. Storage-free, side-effect-free, math-only. Race-day execution plans (during-race fuel, gut training, post-race recovery) stay in the agent's reasoning space because they depend on weather, course profile, individual gut tolerance, and other context the API has no access to.

The downstream loop is: agent calls `plan_carb_load`, gets the schedule, then calls `set_daily_goal_override` (from `add-date-varying-goals`) to put each day's carb target into the goals row. Adherence on those days then reflects the carb-load target. Two primitives, one workflow.

## What Changes

- **One REST endpoint**: `GET /race-prep/carb-load` with query params:
  - `race_date` (required, `YYYY-MM-DD`)
  - `body_weight_kg` (required, numeric, 30–200 sanity range)
  - `days_before` (optional, default `3`, range `0–7`) — number of carb-load days before race day
  - `carbs_per_kg_per_day` (optional, default `10`, range `1–20`) — load-day multiplier
  - `race_day_carbs_per_kg` (optional, default `2`, range `0–10`) — race-morning pre-race meal multiplier
- **Response shape**: `{race_date, body_weight_kg, params, schedule}` where `schedule` is an array of `{date, days_before, target_carbs_g, rationale}` entries, one per day in `[race_date - days_before, race_date]`, ordered ascending.
- **One MCP tool**: `plan_carb_load` wrapping the endpoint. Read-only, no idempotency, description points the agent at the natural follow-up (`set_daily_goal_override` from `add-date-varying-goals`).
- **No storage.** No table, no migration, no per-user race calendar. The agent passes the race date as a parameter; the agent's own calendar / context holds when races are. If a stored race calendar becomes useful, that's a separate change.

## Capabilities

### New Capabilities
- `race-prep`: Deterministic computation primitives for race-week nutrition planning. Starts with carb-load; can grow as other "agent should not hallucinate this" primitives surface (e.g. recovery-window macros, fuelling-rate during long efforts).

### Modified Capabilities
- `mcp-server`: Adds a requirement covering the one new tool. The existing tool requirements are unchanged.

## Impact

- **No schema change.** Stateless feature; no migration.
- **New code**:
  - `internal/raceprep/service.go` — `PlanCarbLoad(params CarbLoadParams) (*Schedule, error)` with validation and the math.
  - `internal/raceprep/handlers.go` — `GET /race-prep/carb-load` route + swag annotations.
  - `internal/mcpserver/tools_raceprep.go` — the one tool registration.
- **Wiring**:
  - `cmd/nutrition-api/serve.go` instantiates the handlers and registers the route under the existing API group (so auth applies).
  - `cmd/nutrition-api/mcp.go` (or wherever tools are registered) calls a new `registerRacePrepTools`.
- **Tests**:
  - Pure-function tests on the service: typical case, every boundary (race today, race 7 days out, body weight at both ends, every override parameter), exact-math assertions.
  - Handler tests: valid request returns the expected schedule, each validation error has the documented code, race date in the past is rejected.
  - MCP wrapper test: query construction, response passthrough.
  - One line added to the MCP integration test's expected-tools list.
- **Documentation**: `task swag`; README MCP table gets one new row; RUN_LOCAL.md gets a tiny race-prep example (`curl /race-prep/carb-load?race_date=2026-07-24&body_weight_kg=70`).

### Out of scope (explicit non-goals)
- **Stored race calendar.** Multi-race support (e.g. "show me all my upcoming races") is the agent's job — its calendar / chat history knows. If we ever want analytics across past races, revisit.
- **Race-day in-event fuelling plans** (gels, electrolytes, hourly carb rate). Too context-dependent: weather, course profile, gut tolerance. Agent reasons from training data + the user's stated preferences.
- **Recovery-window macros** (post-workout protein + carb targets within 30–60 min). Generally applicable but not race-specific; would broaden this change's scope. If real demand surfaces, ship as a separate `add-recovery-window-primitive` change.
- **Gut-training tracking** (carbs-per-hour during long sessions). Requires workout data we don't have without Garmin integration.
- **Carb-load + override auto-apply** (agent calls one tool and the goals override gets PUT automatically). Conflates planning with execution; the agent should make those decisions explicitly.
- **Race-type presets** (`race_type: "sprint" | "70.3" | "ironman"` → curated defaults). The three numeric parameters cover the variation cleanly; presets hide the assumptions in a string. If real use shows the agent picks the wrong defaults consistently, revisit.
