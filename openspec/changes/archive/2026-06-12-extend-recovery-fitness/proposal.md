## Why

The `recovery-metrics` and `fitness-metrics` snapshot capabilities already mirror a slice of what Garmin measures each day — sleep duration, HRV, resting HR, stress, body battery, training readiness on the recovery side; VO2max, race predictors, acute/chronic load on the fitness side. But Garmin exposes several more daily wellness/fitness signals that the coaching agent wants and that we currently throw away: blood-oxygen (SpO2) trend, overnight respiration rate, the breakdown of sleep into deep/light/REM/awake stages, and the longitudinal fitness markers (endurance score, hill score, fitness age) plus the human-readable `training_status` label ("productive", "maintaining", "unproductive") that contextualises the acute/chronic load numbers we already store as bare ratios. The bridge already fetches the sleep DTO; the rest are cheap per-day Garmin calls alongside the ones we make anyway.

This is change **C** of the "mirror everything" Garmin arc — a pure additive column extension of two EXISTING snapshot capabilities, not a new capability. Siblings (sequenced independently, out of scope here): A `add-garmin-daily-energy` (whole-day NEAT/expenditure), B `add-garmin-workout-detail` (per-activity zones/splits/sets), D `add-garmin-gear-and-prs`, E `garmin-workout-library-mgmt`.

## What Changes

- **`recovery-metrics` (extend)** — new nullable columns mapped from Garmin (omitempty, `numfmt.Round1` for floats):
  - `spo2_avg`, `spo2_lowest` (from `get_spo2_data`) — blood-oxygen percentage, 0–100.
  - `respiration_avg`, `respiration_lowest` (from `get_respiration_data`) — breaths/min.
  - `deep_sleep_seconds`, `light_sleep_seconds`, `rem_sleep_seconds`, `awake_seconds` (from the sleep DTO the bridge **already** fetches) — the per-stage breakdown of the `sleep_seconds` total we already store.
- **`fitness-metrics` (extend)** — new nullable columns:
  - `endurance_score` (from `get_endurance_score`).
  - `hill_score` (from `get_hill_score`).
  - `fitness_age` (from `get_fitnessage_data`).
  - `training_status` (TEXT, from `get_training_status`) — the human-readable phase label ("productive" / "maintaining" / "unproductive" / "recovery" / …) that complements the already-stored `acute_load` / `chronic_load`.
- **REST**: the existing `POST /recovery-metrics`, `GET /recovery-metrics`, `GET /recovery-metrics/{date}`, and the matching `fitness-metrics` shapes gain the fields — same endpoints, no new routes, full-replace upsert semantics unchanged. `numfmt.Round1` at the response boundary for new floats; `task swag` after the struct changes.
- **MCP**: the existing read tools (`recovery_metrics_get`, `fitness_metrics_get`, and their list variants) forward the enriched body verbatim — **no new tool**, so the `mcp_integration_test` expected-tools list is reviewed but expected unchanged.
- **garmin-bridge**: `fetch_day` gains guarded `safe()` fetches for `get_spo2_data`, `get_respiration_data`, `get_endurance_score`, `get_hill_score`, `get_fitnessage_data` (the sleep DTO and `get_training_status` are already fetched); `map_recovery` and `map_fitness` in `mapping.py` extract the new fields defensively (absent → omitted); the test fixture and mapping tests grow to cover them.

## Capabilities

### New Capabilities
<!-- None — this is a column extension of two existing snapshot capabilities. -->

### Modified Capabilities
- `recovery-metrics`: new nullable columns (`spo2_avg`, `spo2_lowest`, `respiration_avg`, `respiration_lowest`, `deep_sleep_seconds`, `light_sleep_seconds`, `rem_sleep_seconds`, `awake_seconds`); enriched POST/GET response shape; unit isolation preserved.
- `fitness-metrics`: new nullable columns (`endurance_score`, `hill_score`, `fitness_age`, `training_status` text); enriched POST/GET response shape; unit isolation preserved.
- `garmin-bridge`: `fetch_day` additionally fetches `get_spo2_data`, `get_respiration_data`, `get_endurance_score`, `get_hill_score`, `get_fitnessage_data` under the per-capability `safe()` guard; `map_recovery` / `map_fitness` map the new fields onto the existing `/recovery-metrics` and `/fitness-metrics` targets (no new sync target endpoint).

## Impact

- **Schema**: ONE migration `038_extend_recovery_fitness` adds nullable columns to BOTH `recovery_metrics` and `fitness_metrics` — additive, no back-fill (every existing row reads back NULL for the new columns). Verify the migration head on disk before scaffolding — the arc assigns B=`036`, A=`037`, so this is `038`, but an out-of-band slot collision has happened before.
- **Code**: `internal/recoverymetrics/` and `internal/fitnessmetrics/` (types, repo INSERT/UPSERT + SELECT, service validation, handlers, swag); `internal/httpserver/server.go` wiring unchanged (same packages, same routes); `apps/garmin-bridge/garmin_bridge/{garmin_client,mapping,sync}.py` + fixtures/tests.
- **Docs/tests**: `task swag` after the struct changes; per-handler integration tests against testcontainers Postgres; bridge mapping tests against the expanded fixture; `mcp_integration_test` expected-tools list reviewed (no change expected).
- **Conventions honored**: unit isolation (SpO2/respiration/sleep-stage stay on the recovery shape, endurance/hill/fitness-age/training-status stay on the fitness shape — never merged into a shared Totals struct), `numfmt.Round1` at the response boundary for every new float, ONE append-only sequential migration, NULL-is-meaningful nullables.
