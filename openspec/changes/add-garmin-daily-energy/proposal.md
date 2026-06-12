## Why

The `energy-availability` endpoint computes Loucks EA from `intake_kcal − Σ workouts.kcal_burned`, so the only "energy out" it can see is the burn of explicitly-logged workouts. All non-workout movement — the walk to the pool, the commute, standing through a work day, the post-long-run fidget — is invisible. For an athlete in a deliberate deficit this NEAT (non-exercise activity thermogenesis) is exactly the term that decides whether a "30 kcal/kg FFM" day was actually a low-energy-availability day once total expenditure is counted. Garmin already measures the whole-day picture (`get_user_summary(date)`: active vs resting kcal, total kcal, steps, floors, intensity minutes, distance) and the bridge fetches the same day anyway — we just throw it away.

This is change **A** of the "mirror everything" Garmin arc and the highest-ROI sibling: it makes total daily expenditure a first-class, queryable signal for the first time. Siblings (sequenced independently, out of scope here): B `add-garmin-workout-detail` (per-activity zones/splits/sets), C `extend-recovery-fitness`, D `add-garmin-gear-and-prs`, E `garmin-workout-library-mgmt`.

## What Changes

- **New capability `daily-summary`** → new package `internal/dailysummary/` (the standard `types`/`repo`/`service`/`handlers` shape) and a new **date-keyed snapshot table `daily_summary`** (one row per calendar date, upsert-by-date), mirroring the existing snapshot capabilities (`recovery-metrics`, `fitness-metrics`, `hydration-balance`).
- **Fields mapped from `get_user_summary(date)`** (all nullable, omitempty, `numfmt.Round1` at the response boundary): `active_kcal` (activeKilocalories), `resting_kcal` (bmrKilocalories), `total_kcal` (totalKilocalories), `steps` (totalSteps), `floors` (floorsAscended), `moderate_intensity_minutes`, `vigorous_intensity_minutes`, `distance_m` (totalDistanceMeters).
- **REST**: `POST /daily-summary` (upsert by date, 201 on insert / 200 on update), `GET /daily-summary/{date}` (single), `GET /daily-summary?from=&to=` (inclusive window, 92-day cap), wired in `internal/httpserver/server.go` exactly like the sibling snapshot capabilities.
- **MCP**: one new read tool `daily_summary_get` mirroring `GET /daily-summary/{date}` 1:1 (verbatim body forward). The `mcp_integration_test` expected-tools list grows by exactly one.
- **garmin-bridge**: `fetch_day` gains a guarded `get_user_summary(date)` fetch (existing `safe()` pattern); `mapping.py` gains `map_daily_summary`; `sync.py` posts it as a date-keyed snapshot alongside the other date-keyed metrics; the test fixture and mapping tests grow to cover it.
- **EA stays unchanged.** Garmin's total/active daily expenditure is surfaced *only* in the `daily-summary` shape as an independent context signal; it is **not** merged into `summary`'s Totals struct and does **not** replace EA's exercise-burn denominator (unit isolation + preserving the Loucks metric's meaning — see design D2/D3). EA enrichment that *consumes* `daily-summary` is an explicit follow-up / non-goal.

## Capabilities

### New Capabilities
- `daily-summary`: a date-keyed snapshot of Garmin's whole-day energy/activity totals (active/resting/total kcal, steps, floors, intensity minutes, distance), source-agnostic, unit-isolated, upsert-by-date with the same REST surface as the other snapshot capabilities.

### Modified Capabilities
- `garmin-bridge`: `fetch_day` additionally fetches `get_user_summary(date)` under the per-capability `safe()` guard; `map_daily_summary` maps it; the daily-sync mapping requirement gains `/daily-summary` as a target.

## Impact

- **Schema**: migration `037_add_daily_summary` — one new date-keyed table, additive, no back-fill. (B `add-garmin-workout-detail` takes `036`; verify the head on disk before scaffolding — an out-of-band slot collision has happened before.)
- **Code**: new `internal/dailysummary/` package; `internal/httpserver/server.go` wiring (instantiate repo/service, register routes); `internal/mcpserver/` new tool + group registration; `apps/garmin-bridge/garmin_bridge/{garmin_client,mapping,sync}.py` + fixtures/tests.
- **Docs/tests**: `task swag` after handlers land; per-handler integration tests against testcontainers Postgres; bridge mapping tests against the expanded fixture; `mcp_integration_test` expected-tools list bumped by one.
- **Conventions honored**: unit isolation (daily kcal stays on the `daily-summary` shape, never in `summary` Totals or EA's denominator), `numfmt.Round1` at the response boundary for every float, append-only sequential migration, NULL-is-meaningful nullables.
