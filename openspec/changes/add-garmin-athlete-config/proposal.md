## Why

Change **B** (`add-garmin-workout-detail`) imports normalized power and time-in-HR-zone per activity, but flagged two gaps that make that data only half-interpretable: `intensity_factor` stays NULL because the backend has no FTP to divide normalized power by, and the `secs_in_zone_1..5` seconds are unlabeled — we store "240 seconds in zone 4" with no record of what heart rate zone 4 actually *is* for this athlete. The physiology that makes workout detail interpretable — FTP, threshold HR & pace, max HR, and the HR/power zone boundaries — lives in the Garmin user profile, and the bridge already authenticates against that profile every sync. We just never read it.

This is change **F** of the "mirror everything" Garmin arc — the "make the workout-detail data *interpretable*" slice. It captures the athlete's slowly-changing physiology configuration as a singleton so the zone seconds gain labeled boundaries and FTP becomes available for downstream IF/intensity math. Siblings (sequenced independently, out of scope here): A `add-garmin-daily-energy`, B `add-garmin-workout-detail`, C `extend-recovery-fitness`, D `add-garmin-gear-and-prs`, E `garmin-workout-library-mgmt`.

## What Changes

- **New capability `athlete-config`** → new package `internal/athleteconfig/` (the standard `types`/`repo`/`service`/`handlers` shape) and a new **singleton table `athlete_config`** (exactly one row, upserted in place — modeled on the existing `nutrition_goals` singleton: a fixed sentinel primary key, `INSERT … ON CONFLICT (id) DO UPDATE`, lazy-created on first write, `GET` returns `{"athlete_config": null}` until then).
- **Fields mapped from `get_user_profile` / `get_userprofile_settings` and `get_heart_rate_zones`** (all nullable, omitempty, `numfmt.Round1` at the response boundary): `ftp_watts`, `threshold_hr`, `threshold_pace_sec_per_km` (run), `threshold_swim_pace_sec_per_100m`, `max_hr`, `lactate_threshold_hr`, and the five HR-zone boundaries `hr_zone_1_max` … `hr_zone_5_max`; optionally the five power-zone boundaries `power_zone_1_max` … `power_zone_5_max`.
- **REST**: singleton `GET /athlete-config` (returns `{"athlete_config": <config> | null}`) and `PUT /athlete-config` (full-replace, absent fields cleared to NULL), wired in `internal/httpserver/server.go` exactly like the `goals` singleton. Per the PUT convention, `PUT /athlete-config` rejects an `Idempotency-Key` header with `400 idempotency_unsupported_for_put`.
- **MCP**: one new read tool `athlete_config_get` mirroring `GET /athlete-config` 1:1 (verbatim body forward). The `mcp_integration_test` expected-tools list grows by exactly one.
- **garmin-bridge**: `fetch_day` gains guarded `get_user_profile` (and/or `get_userprofile_settings`) and `get_heart_rate_zones` fetches (existing `safe()` pattern); `mapping.py` gains `map_athlete_config`; `sync.py` POSTs the mapped config to `PUT /athlete-config` on each daily sync. Because this config is **not date-keyed** — it is slowly-changing physiology, not a per-day snapshot — the daily sync refreshes the single row in place via the singleton upsert (the same inventory-refresh-on-each-sync stance D took for gear/PRs). The test fixture and mapping tests grow to cover it.

- **CAPTURE ONLY — no consumption.** This change *stores* FTP, thresholds, and zone boundaries. It does **not** derive `intensity_factor` from FTP, does **not** relate the stored `secs_in_zone_*` to the imported zone boundaries, and does **not** feed the raceprep intensity math. Consuming this config (back-filling B's `intensity_factor`, labeling zone seconds, or wiring FTP into raceprep) is an explicit follow-up / non-goal — it would touch B's mapper or the raceprep math and is deliberately out of scope (see design D2).

## Capabilities

### New Capabilities
- `athlete-config`: a single-row mirror of the athlete's Garmin physiology configuration (FTP, threshold HR/pace, max HR, lactate-threshold HR, HR-zone and power-zone boundaries), source-agnostic, unit-isolated, with the same `GET`/`PUT` singleton REST surface as `nutrition-goals`.

### Modified Capabilities
- `garmin-bridge`: `fetch_day` additionally fetches the user profile and heart-rate zones under the per-capability `safe()` guard; `map_athlete_config` maps them; the daily-sync mapping requirement gains `PUT /athlete-config` as a target (a non-date-keyed singleton refresh, not a date-keyed snapshot).

## Impact

- **Schema**: migration `040_add_athlete_config` — one new singleton table, additive, no back-fill. Arc order so far is B=036, A=037, C=038, D=039 (E=none), so this is `040`. **Verify the head on disk before scaffolding** (`task migrate:new`) — an out-of-band slot collision has happened before; the on-disk head may lag the arc order if siblings have not yet landed.
- **Code**: new `internal/athleteconfig/` package; `internal/httpserver/server.go` wiring (instantiate repo/service, register routes); `internal/mcpserver/` new tool + group registration; `apps/garmin-bridge/garmin_bridge/{garmin_client,mapping,sync}.py` + fixtures/tests.
- **Docs/tests**: `task swag` after handlers land; per-handler integration tests against testcontainers Postgres; bridge mapping tests against the expanded fixture; `mcp_integration_test` expected-tools list bumped by one.
- **Conventions honored**: singleton pattern (sentinel PK + upsert-in-place, like `nutrition_goals`); unit isolation (config stays on the `athlete-config` shape, never in `summary` Totals or any fueling-math input in this change); `numfmt.Round1` at the response boundary for every float; PUT rejects `Idempotency-Key`; append-only sequential migration; NULL-is-meaningful nullables.
