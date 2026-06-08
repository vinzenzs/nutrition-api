## Why

For an endurance athlete in a deficit, Energy Availability (EA) is the single most important number — it predicts both performance ceiling and longer-term hormonal/bone health. The Loucks bands are concrete and well-published: `< 30 kcal/kg FFM/day` is "low" (real physiological risk), `30–45` is "sub-optimal", `> 45` is "adequate". Today every input EA needs already lives in the API: meals (kcal intake), workouts (kcal_burned), body weight (with optional body-fat % for FFM). The only missing piece is the composition — a single tool that pulls them together over a date window.

This closes T1 #4 in `openspec/priorities.md`. The note has been on hold waiting for `add-workouts-capability` + `add-weight-log` to land; both shipped 2026-06-08 so the inputs are now in place. Importantly, this is *pure composition over existing primitives* — no new tables, no new schema, no migration. The cheapest possible follow-up that closes a Tier-1 gap.

## What Changes

- **New `GET /energy/availability` endpoint**: query params `from` / `to` (RFC 3339 window, max 92 days), optional `lean_mass_kg` (overrides the auto-computed FFM), optional `body_fat_pct` (alternative override). Returns per-day EA values + a window average + the Loucks band classification.
  - Daily computation: `EA = (intake_kcal - workout_kcal_burned) / FFM_kg`
  - `intake_kcal` = SUM of meal entries' kcal over the calendar day (resolved in the requested TZ)
  - `workout_kcal_burned` = SUM of `workouts.kcal_burned` for workouts that *started* on that calendar day. Workouts without `kcal_burned` set are flagged in the response (lossy days are surfaced, not silently zeroed).
  - `FFM_kg` resolution order: explicit `lean_mass_kg` param → explicit `body_fat_pct` param + rolling-avg body weight → `body_fat_pct` from the most recent weight entry in-window + rolling-avg body weight → fallback `body_weight × 0.85` with a `composition_estimated: true` flag in the response.
- **One MCP tool**: `weekly_energy_summary(from, to, lean_mass_kg?, body_fat_pct?, tz?)` wrapping the endpoint. Per the priorities-doc shape but with the optional-overrides honest about the FFM resolution.
- **Response shape** clearly tells the caller what was inferred vs supplied (so a low EA reading on missing-burn-data days reads as "incomplete data" not "you're starving"):
  - `days: [{date, intake_kcal, burned_kcal, ea, ffm_kg, band, missing_burn_workout_ids: [...]}]`
  - `window: {avg_ea, band, days_with_complete_data, total_days}`
  - `composition: {ffm_kg, source: "explicit_lean_mass" | "explicit_body_fat" | "stored_body_fat" | "estimated_85pct", body_weight_kg, body_weight_source: "rolling_7d_avg" | "explicit"}`

## Capabilities

### New Capabilities

- `energy-availability`: A pure-computation read endpoint that derives per-day Energy Availability from existing intake, workout-burn, and body-composition primitives. Surfaces the Loucks bands honestly and flags days with incomplete data so missing-burn-kcal doesn't masquerade as healthy EA.

### Modified Capabilities

- `mcp-server`: Adds one new tool `weekly_energy_summary` wrapping the new REST endpoint.

## Impact

- **Prerequisites (all already shipped)**: `add-workouts-capability` (the `workouts.kcal_burned` column), `add-weight-log` (`body_weight_entries` with optional `body_fat_pct`), `add-meal-workout-link` is NOT a prerequisite — EA aggregates meals by calendar-day, not by workout window.
- **No schema migration**. Pure aggregation across three existing tables.
- **New code**:
  - `internal/energy/` package: `types.go` (response shapes), `service.go` (composition orchestration + FFM resolver), `handlers.go` (one GET handler).
  - `internal/mcpserver/tools_energy.go` — one tool.
- **Modified code**:
  - `internal/httpserver/server.go`: instantiate the service + handlers; pass `meals.Repo`, `workouts.Repo`, `bodyweight.Repo` (or `bodyweight.Service` for the rolling-avg trend) into the constructor.
- **Tests**:
  - Per-day computation: explicit-lean-mass path, body-fat-pct-on-weight-entry path, fallback-to-85pct path, missing-burn-flagging, calendar-day boundary handling in a non-UTC TZ.
  - Loucks band classification at the exact `30` and `45` boundaries.
  - Window aggregation: `avg_ea` weighting (simple mean across days vs SUM-intake / SUM-FFM-days — picked in design).
  - `range_too_large` and `window_invalid` rejection consistent with the other window endpoints.
- **Documentation**: `task swag`; README "Energy availability" subsection placed near "Body weight" (both derive from the same composition data); RUN_LOCAL.md gets a quick "log a few days, query EA, observe the band" walkthrough.
- **No idempotency middleware impact** — read-only endpoint.

### Out of scope (explicit non-goals)

- **Persisting EA snapshots.** It's a pure computation over primitives; storing snapshots would make the agent's "recompute with my updated body fat %" workflow lossy. Same reasoning as `plan_carb_load` staying stateless.
- **Anomaly flagging** ("you've been low for 3 weeks → talk to a coach"). That's agent-side synthesis from the EA series. The API surfaces honest numbers; the agent reasons about trajectory.
- **Per-meal-type breakdown of intake_kcal**. EA is a daily/weekly aggregate; per-meal slicing would invite micro-tuning that doesn't change the EA conclusion.
- **Non-Loucks bands** (e.g. sport-specific guidelines for runners vs cyclists). Loucks is the well-validated baseline; sport-specific tuning is a separate change if real use shows it matters.
- **Auto-imputing `kcal_burned` from sport + duration + body weight.** Estimation is the writer's job (Garmin already does it for synced sessions); a missing field is a signal, not a defect — surfaced via the `missing_burn_workout_ids` list rather than silently filled.
- **Macro-level EA components** (carbs vs fat contribution to intake). Out of scope; macro adherence is covered by `daily_summary`.
- **Goal-weight trajectory projection** (deficit per week → projected race-day weight). That's a separate, larger downstream computation.
