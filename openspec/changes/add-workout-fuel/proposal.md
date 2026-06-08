## Why

Sodium targets during endurance work are 300–800 mg/hr — entirely invisible to the system today. `log_hydration` records ml and a free-text `note`; it cannot tell you sodium intake during a 90-minute Z2 ride, what carbs/hour rate you sustained on a long ride, or how much caffeine you took 60 minutes before the gun went off. This is the missing surface for the "did this fuelling strategy work?" loop that race-fuelling rehearsal depends on.

The cleanest shape is a sibling capability to hydration — `workout_fuel_entries` — explicitly NOT a column extension on `hydration_entries`. When `add-hydration-tracking` shipped, the design rationale was clear: keep ml-only out of structures that carry grams or milligrams, because mixing units in one Totals struct is the canonical footgun. Extending hydration with `sodium_mg` / `carbs_g` / `caffeine_mg` re-introduces exactly that footgun. A separate table keeps both capabilities honest: hydration is "did I drink enough water"; workout-fuel is "did I take the right gels / electrolytes / caffeine during the session."

This closes T1 #2 in `openspec/priorities.md`. It also extends the workout-anchored fueling summary (introduced by `add-meal-workout-link`) to compose the new entries in — that's the future-compat note both proposals already wrote down, now cashed in.

## What Changes

- **New `workout_fuel_entries` table**: `id`, `logged_at`, `name` (required free text — *what* you took matters for rehearsal data), `quantity_ml` (nullable), `carbs_g` / `sodium_mg` / `potassium_mg` / `caffeine_mg` (all nullable), `note`, `workout_id` (nullable FK `ON DELETE SET NULL`), audit timestamps. At least one of `quantity_ml`/`carbs_g`/`sodium_mg`/`potassium_mg`/`caffeine_mg` MUST be set — an entry with no measurable intake is rejected.
- **Four REST endpoints** mirroring the hydration shape:
  - `POST /workout-fuel` — log an entry. Accepts the standard `Idempotency-Key` header. `workout_id` optional (you can log a gel before Garmin has synced the ride; tag it later).
  - `GET /workout-fuel?from=…&to=…` — list entries in a half-open RFC 3339 window, ordered by `logged_at` ASC. 92-day cap.
  - `PATCH /workout-fuel/{id}` — partial update of any field; `workout_id` supports the `""` empty-string clear semantic established by `add-meal-workout-link`.
  - `DELETE /workout-fuel/{id}` — remove.
- **Four MCP tools** wrapping each endpoint: `log_workout_fuel`, `list_workout_fuel`, `patch_workout_fuel`, `delete_workout_fuel`. Standard POST-style auto-derive idempotency on writes.
- **Extension to `/workouts/{id}/fueling`** (introduced by `add-meal-workout-link`): each window's response object gains a third sub-object `workout_fuel: {totals: {carbs_g, sodium_mg, potassium_mg, caffeine_mg, quantity_ml}, entry_count}`. The unit-isolation pattern continues: `nutrition` carries kcal/g from meals; `hydration` carries ml from hydration_entries; `workout_fuel` carries its own field shape with carbs/mg/ml from this new table. No mixing.

## Capabilities

### New Capabilities
- `workout-fuel`: A persisted log of in-session fueling events (gels, electrolyte drinks, salt tabs, caffeine pills, pre-race espresso). Carries carbs/sodium/potassium/caffeine in their natural units alongside optional volume. Sister to `hydration` and `body-weight` — capture-only; deliberately unit-isolated.

### Modified Capabilities
- `workouts`: The `/workouts/{id}/fueling` endpoint (added by `add-meal-workout-link`) extends its per-window response shape to include a third sub-object for workout-fuel contributions. The shape promise made by `add-meal-workout-link` — that workout_fuel composes in cleanly when it exists — is fulfilled here.
- `mcp-server`: Adds a requirement for four new tools wrapping the workout-fuel CRUD endpoints.

## Impact

- **Prerequisite**: `add-meal-workout-link` MUST be applied before this change (the `/workouts/{id}/fueling` endpoint is the integration point for the extension). If it hasn't shipped yet, the CRUD pieces of this change can land standalone, with the fueling-endpoint extension following — but the cleanest path is to apply in order.
- **Schema migration** at `internal/store/migrations/015_add_workout_fuel.{up,down}.sql`: one table, one index on `(logged_at)`, one partial index on `(workout_id) WHERE workout_id IS NOT NULL`.
- **New code**:
  - `internal/workoutfuel/` package: `types.go`, `repo.go`, `service.go`, `handlers.go`. Closely mirrors the `internal/hydration/` shape.
  - `internal/mcpserver/tools_workout_fuel.go` — four tools.
- **Modified code**:
  - `internal/workouts/fueling.go` (or wherever the aggregation introduced by `add-meal-workout-link` lives): take a `*workoutfuel.Repo` as an additional constructor dependency; sum entries per window into the new `WorkoutFuel` sub-object of each window.
  - `internal/httpserver/server.go`: instantiate the new repo + service + handlers; pass the repo into the workouts fueling constructor.
- **Tests**:
  - Per-endpoint handler tests + idempotency replay (the standard set used for hydration / weight / workouts).
  - Validation: at-least-one-field-required, quantity_ml > 0 when supplied, nutriment fields >= 0 when supplied, name required.
  - One integration test asserting `/workouts/{id}/fueling` now includes workout_fuel data in the right windows.
  - One assertion confirming `/summary/hydration/daily` does NOT include workout_fuel ml (capabilities stay separate; the unit-isolation rule extends across these too).
  - MCP wrapper tests; integration tools-list grows by 4 (31 → 35, assuming `add-meal-workout-link`'s +1 has landed).
- **Documentation**: `task swag`; README "Workout fuel" subsection + four new MCP-table rows; RUN_LOCAL.md gets a "log a gel during the ride" example with subsequent `/workouts/{id}/fueling` showing the contribution.

### Out of scope (explicit non-goals)

- **Daily summary endpoint** for workout fuel (`/summary/workout-fuel/daily`). The workout-anchored summary covers the primary "did this session get the right fueling" question; for a day-wide rollup the agent composes from the list endpoint. Cheap to add later if real use surfaces the need.
- **Aggregation into `/summary/hydration/daily`.** A 500 ml electrolyte drink during a ride lands in workout_fuel; it does NOT bump the daily hydration total. The two capabilities are deliberately separate; the agent composes when the user asks "total ml today including in-session." Documented; revisit only if the friction is real.
- **Aggregation into nutrition `/summary/daily`.** Workout-fuel carbs do not count toward the daily macro adherence. Different mental model: macro adherence is about food choices; in-session fueling is its own protocol. The agent composes if a "total carbs including gels" question surfaces.
- **Product catalog for gels** (a `workout_fuel_products` table the entries reference). v1 keeps `name` as free text — sufficient for rehearsal data (the goal is "what worked in training"). A catalog is a separate, larger change.
- **Garmin / smart-bottle ingestion.** No mainstream sport-tracker pushes in-session intake data to the API. If a future bottle / device does, the `workout_id` link plus the standard write endpoint accommodates it via an external pusher (same pattern as `garmin.py` for workouts).
- **Bulk endpoint.** No historical source to backfill from; manual entry only. N POSTs is fine.
- **Composition into the EA tool** (future). EA needs `kcal_burned` (workouts) and dietary kcal (meals); workout-fuel kcal is a smaller signal that the agent can pull separately if needed.
- **Snapshot semantics.** No product link, no need to snapshot.
- **Per-hour rate computation** (e.g. "average sodium per hour during this workout"). Derivable from `/workouts/{id}/fueling` + workout duration; if real use shows the agent wants it pre-computed, a small `?include=rates` flag adds it without a contract change.
