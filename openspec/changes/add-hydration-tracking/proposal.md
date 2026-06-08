## Why

The system can answer "what did I eat?" but not "what did I drink?" For an endurance-training user, that's a real blind spot: a 2-hour ride can easily produce ≥2 L of sweat loss, and intake without that data is half the picture. The MCP-driven test loop has surfaced this as a daily-flow gap.

Hydration is structurally different from food: volume not mass, no nutriments, no recipes, no Open Food Facts lookup. The smallest honest thing is a parallel `hydration_entries` table with its own endpoints and MCP tools — mirroring the meals shape but keeping units cleanly separate. The agent (in Claude Desktop / Claude Code / garmin.py coach) reasons about context — workout type, weather, sweat rate — using the captured volume and the user's training data; the API just provides the primitive.

## What Changes

- **New `hydration_entries` table** keyed on `id, logged_at, quantity_ml, note, created_at, updated_at`. No FK to meals, no beverage-type enum (free-text `note` carries that), no workout-window tagging (agent's job).
- **Five REST endpoints** mirroring the meals shape:
  - `POST /hydration` — log an entry (`{quantity_ml, logged_at, note?}`). Accepts the standard `Idempotency-Key` header.
  - `GET /hydration?from=…&to=…` — list entries in a half-open RFC 3339 window, ordered by `logged_at ASC`. Same 92-day window cap as `/meals`.
  - `PATCH /hydration/{id}` — partial update of `quantity_ml`, `logged_at`, `note`.
  - `DELETE /hydration/{id}` — remove an entry.
  - `GET /summary/hydration/daily?date=YYYY-MM-DD&tz=…` — daily total + per-entry list. Computes the day window in the supplied TZ exactly like `/summary/daily`.
- **Five MCP tools** wrapping each endpoint (`log_hydration`, `list_hydration`, `patch_hydration`, `delete_hydration`, `daily_hydration_summary`). Auto-derive idempotency keys on writes per the existing POST-style rule.
- **No changes to `/summary/daily`** (the nutrition one). Hydration stays in its own response — mixing g and ml in one Totals struct is a footgun.
- **No changes to the `nutrition-goals` capability.** Hydration target is too context-sensitive for a single static range, and the agent can compare today's total against whatever target it deems appropriate from the user's training context. If real-world use shows we want a stored target, that's a tiny follow-up.

## Capabilities

### New Capabilities
- `hydration`: Logged-fluid-intake events with CRUD + daily summary, parallel to the meal-logging surface but unit-isolated (ml only, never mixed with grams).

### Modified Capabilities
- `mcp-server`: Adds a new requirement covering the five hydration MCP tools. The existing tool requirements are unchanged.

## Impact

- **Schema migration** at `internal/store/migrations/010_add_hydration.up.sql` / `.down.sql`. One table, one index on `logged_at`, no FKs.
- **New code**:
  - `internal/hydration/` package: `types.go` (Entry struct), `repo.go` (Insert / GetByID / Patch / Delete / List), `service.go` (validation), `handlers.go` (HTTP + swag), `summary.go` (daily aggregation).
  - The `cmd/nutrition-api/serve.go` wiring grows two registrations (the entries handler and the summary handler).
- **MCP wrapper** at `internal/mcpserver/tools_hydration.go` — five tools, follows the existing patterns (`tools_meals.go` is the closest template).
- **No changes to** `meals`, `products`, `summary`, `nutrition-goals`, `auth`, `idempotency`, `off-integration`. Hydration is genuinely independent.
- **Tests**: handler-level tests for each endpoint, MCP unit tests for each tool, one integration test addition in `internal/mcpserver/` so the tools-list now expects 17 entries instead of 12.
- **Documentation**: `task swag` regenerates OpenAPI; README MCP table grows five rows; RUN_LOCAL.md walkthrough gains a tiny "log a glass of water" example.

### Out of scope (explicit non-goals)
- **Beverage type enum.** The `note` field is enough for "espresso" / "Skratch electrolyte" / "tap water." Don't tax the schema with a fixed taxonomy.
- **Workout-window tagging.** The agent decides what counts as during-workout from its own data; this API just records when each volume was drunk.
- **Hydration goal in `nutrition_goals`.** Real hydration targets vary with workout volume, temperature, and individual sweat rate — encoding a fixed daily target hides the context. Agent reasons; API records.
- **Integration with `/summary/daily`** (the nutrition one). Mixing units in one response is a footgun; two endpoints is cleaner.
- **Caffeine, alcohol, or other non-water context tracking.** All can live in `note` for now. If it becomes a real signal we'll revisit.
- **Hydration adherence in summary responses.** No goal stored, no adherence to compute. Agent compares total vs context.
