## Why

The agent currently makes 5–7 separate MCP calls to start every conversation: "what's today's adherence?", "what did I drink?", "what workouts are logged?", "any weight log?", "what training phase am I in?", "any goal override on today?" — each a separate tool round-trip. Every primitive shipped (hydration, workouts, weight, EA, training-phases) added another row to this morning ritual. The cost compounds: by the time the agent has the bundle, latency is 3-5 seconds and the cold context has crowded out the user's actual question.

A single `daily_context(date)` MCP tool collapses the bundle into one read. No new schema; pure composition over primitives that already exist. Tests the "one tool, many sources" pattern the codebase has been deferring — and a working example here will inform whether similar aggregators (e.g. `weekly_context`) become a thing.

## What Changes

- **New REST endpoint** `GET /context/daily?date=YYYY-MM-DD&tz=…` returning a single JSON bundle covering everything the agent would otherwise stitch together. Required params: `date`. Optional: `tz` (defaults to `DEFAULT_USER_TZ`).
- **Bundle shape** (see Impact section for the full field list):
  - **meta**: the resolved `date`, `tz`, the request's local-day window in UTC.
  - **adherence**: `goal_source`, `phase_name` (when applicable), and the same `adherence` object the `/summary/daily` endpoint produces — the existing shape, re-used verbatim.
  - **nutrition**: the day's `totals` block from `/summary/daily` (rounded), plus a one-line `entries_count`. Full meal entries are NOT included — too verbose for an aggregator; the agent can call `daily_summary` if it needs them.
  - **hydration**: total `ml` for the day and `entries_count`.
  - **workouts**: array of workouts whose `started_at` falls in the day's window (id, sport, started_at, ended_at, duration_min, kcal_burned, notes); empty array when none.
  - **workout_fuel**: array of workout-fuel entries logged on the day (id, logged_at, quantity_ml, carbs_g, sodium_mg, potassium_mg, caffeine_mg, workout_id when linked).
  - **weight**: the most-recent body-weight entry whose `logged_at` is on the day, OR — if none — the most recent before the day (with `is_carryover: true` so the agent can tell whether it's fresh).
  - **phase**: the phase covering the day (id, name, type, start_date, end_date, default_template_name), or null.
  - **goal_override**: a boolean `present` plus the override `goals` object when present (no override → `null`).
- **Composition-only**: no new tables, no new migration, no new write paths. Every datum comes from an existing repo's read method.
- **New MCP tool** `daily_context` wrapping the REST endpoint. Tool description names the latency tradeoff explicitly: "Use this as the first call of a session to load the bundle; pair with the dedicated tools (`daily_summary`, `list_workouts`, etc.) when you need a deeper view of one slice."
- **`internal/dailycontext/` package** following the per-capability layout — types + service + handlers; no repo of its own (consumes existing repos).

## Capabilities

### New Capabilities
- `daily-context`: Owns the aggregator endpoint + MCP tool. Sits as a thin composition layer over `meals`, `hydration`, `workouts`, `workoutfuel`, `bodyweight`, `goals`, and `trainingphases` repos; carries no persistence itself.

### Modified Capabilities
- `mcp-server`: 1 new tool (`daily_context`) added; existing tools unchanged.

## Impact

- **No schema migration** — pure read-side composition.
- **New `internal/dailycontext/` package**:
  - `types.go` — `DailyContext` response struct with sub-types (`AdherenceBlock`, `NutritionBlock`, `HydrationBlock`, `WorkoutsBlock`, `WeightBlock`, `PhaseBlock`, `GoalOverrideBlock`). JSON tags carry `omitempty` for nullable slices.
  - `service.go` — `Service` with constructor that takes every read-side dependency it needs (a long parameter list — `mealsRepo, hydrationRepo, workoutsRepo, workoutFuelRepo, bodyWeightRepo, goalOverridesRepo, phasesRepo, templatesRepo, goalsResolver, summarySvc`). One method: `BuildFor(ctx, date, loc) (*DailyContext, error)`. Internally calls each dependency in parallel (errgroup) — the cost of the aggregator is the slowest single read, not the sum.
  - `handlers.go` — `GET /context/daily?date=&tz=` Gin handler with swag annotations.
- **Wiring in `internal/httpserver/server.go`**: new constructor call with all the existing repos and the goals resolver passed in.
- **MCP wrapper** in `internal/mcpserver/tools_dailycontext.go`: `DailyContextArgs { Date, TZ? }`, handler issues `GET /context/daily?date=…&tz=…`, forwards verbatim. Read-only — no Idempotency-Key. Registered from `server.go` next to `registerSummaryTools`.
- **Tests**:
  - Service-level test exercising the parallel-read path with seeded data (every slice non-empty so the bundle's shape is fully covered).
  - Handler tests: `date` required → 400; invalid `date` format → 400; invalid `tz` → 400; happy path returns 200 with the full bundle; empty day (no data anywhere) returns the bundle with empty arrays and nulls where appropriate.
  - MCP wrapper test: GET path + query params correct; no idempotency key.
  - Integration test list bump: `daily_context` added to the expected-tools assertion.
- **Documentation**:
  - README "Summaries" subsection gains a "Daily context bundle" entry with curl + response shape.
  - RUN_LOCAL gets a short example showing the aggregator after a few logged entries.
  - `task swag` regenerates `docs/`.

### Out of scope (explicit non-goals)

- **Meal entries inlined into the bundle.** Full meal entries would bloat the response on heavy log days. The `nutrition.entries_count` field signals presence; the agent calls `daily_summary` when the breakdown is needed.
- **`weekly_context` / `range_context`.** Same composition principle but a different shape (per-day arrays everywhere); v2 if real use shows the daily one is heavily used.
- **Sleep / HRV.** T2 #6A isn't shipped yet; the bundle adds a `sleep` block when that primitive lands. v1's shape leaves room for the additive field.
- **Personalised recommendations.** This endpoint is data; "what should I do today?" stays agent-side. No advisory text in the response.
- **Goal templates resolved inline.** The bundle reports the phase's `default_template_name` (already present on phase responses) but does NOT include the template's bound details. The agent calls `get_goal_template` when it needs them. Reason: keeps the bundle's size bounded and the agent's reads explicit.
- **Etag / If-Modified-Since.** No caching layer in v1. Reads are cheap and the agent issues `daily_context` once per session start.
- **Multi-day batching.** No `?from=&to=` form. Each `daily_context` call is one day. Range queries are still served by `range_summary`, `list_workouts`, etc.
- **Computing tomorrow's plan.** The bundle is strictly *what happened / what's set today*. Tomorrow's planning is `recommend_workout_fuel` and `plan_carb_load` territory.
