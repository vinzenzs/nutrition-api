# add-garmin-daily-metrics

## Why

Garmin Connect produces a rich daily stream the nutrition API has nowhere to store, so the `garmin.py` coach script computes 7-day recovery/fitness averages on every run and throws them away — none of it survives into the API the LLM coach reasons over. Four data domains have no home:

- **Recovery** — sleep, HRV, resting HR, stress, body battery, training readiness. The single most relevant context for "is today's deficit tolerable / should fueling shift." No endpoint.
- **Fitness** — VO2max (run + bike), race predictions, acute/chronic training load. No endpoint.
- **Richer weigh-ins** — Garmin reports BMI, muscle mass, body water, bone mass, but the `body-weight` model only accepts kg + body-fat %, dropping the rest on push.
- **Planned workouts** — Garmin's training calendar holds scheduled sessions, but `workouts` has no `planned` lifecycle, so "fuel for tomorrow's long ride" can't be grounded and the `started_at_too_far_future` guard actively *rejects* a future session.

This change gives all four a home so the (out-of-repo) importer can push them. It deliberately changes no downstream computation — EA does not yet consume muscle mass, and training-day templates do not yet auto-apply from planned sessions; those are follow-ups once the data exists. (The per-activity sweat-loss/temperature and brick/multisport-grouping gaps from the same analysis already shipped in `widen-workout-ingestion`, archived 2026-06-10.)

## What Changes

- **New capability `recovery-metrics`** — a `recovery_metrics` table keyed by `date` (one daily snapshot), all metric columns nullable: `sleep_seconds`, `sleep_score`, `hrv_ms`, `resting_hr`, `stress_avg`, `body_battery_charged`, `body_battery_drained`, `training_readiness`. REST: `POST /recovery-metrics` (upsert-by-date — "push every day you see"), `GET /recovery-metrics?from=&to=`, `GET /recovery-metrics/{date}`, `DELETE /recovery-metrics/{date}`.
- **New capability `fitness-metrics`** — a `fitness_metrics` table keyed by `date`, nullable columns: `vo2max_running`, `vo2max_cycling`, `race_predictor_5k_seconds`, `race_predictor_10k_seconds`, `race_predictor_half_seconds`, `race_predictor_full_seconds`, `acute_load`, `chronic_load`. Same REST shape (`POST` upsert-by-date, `GET` list, `GET /{date}`, `DELETE /{date}`).
- **MODIFIED `body-weight`** — add nullable `muscle_mass_kg`, `body_water_pct`, `bone_mass_kg`, `bmi` to the entry; POST + PATCH accept them with validation; GET echoes them (omitempty).
- **MODIFIED `workouts`** — add `status TEXT CHECK IN ('planned','completed') DEFAULT 'completed'`; existing rows back-fill to `completed`. The `started_at_too_far_future` guard is relaxed for `status='planned'` (planned sessions are expected in the future, bounded at +1 year); `completed` keeps the 24h guard. `GET /workouts` gains a `?status=` filter. `status` is a mutable PATCH field.
- **MODIFIED `daily-context`** — the `GET /context/daily` bundle gains `recovery` and `fitness` sub-objects (read-side composition over the two new repos); the existing `weight` block echoes the new biometric fields when present.
- **MODIFIED `mcp-server`** — new tool groups `log_recovery_metrics`/`list_recovery_metrics`/`get_recovery_metrics`/`delete_recovery_metrics` and the `fitness_metrics` equivalents; `log_weight`/`patch_weight` gain the biometric fields; `log_workout`/`patch_workout`/`list_workouts` gain `status`. The integration-test expected-tools list is bumped (new tools added).
- **swag** regenerates `docs/` for all new/changed endpoints.

## Capabilities

### New Capabilities

- `recovery-metrics`: daily recovery snapshot (sleep, HRV, resting HR, stress, body battery, training readiness) stored one row per date, upsert-by-date, with list/get/delete reads.
- `fitness-metrics`: daily fitness snapshot (VO2max run/bike, race predictions, acute/chronic load) stored one row per date, upsert-by-date, with list/get/delete reads.

### Modified Capabilities

- `body-weight`: the entry shape gains four nullable biometric fields; POST/PATCH accept them, GET echoes them.
- `workouts`: the row gains a `status` lifecycle (`planned`|`completed`, default `completed`); the future-date guard is conditioned on status; list gains a `status` filter; PATCH can change status.
- `daily-context`: the daily bundle gains `recovery` + `fitness` sub-objects and richer `weight` fields.
- `mcp-server`: two new tool groups; weight + workout tools gain the new fields; expected-tools list bumped.

## Impact

- **Schema**: four append-only migrations (verify next free slot — `020`–`023` expected, head is `019`): two CREATE TABLE (recovery_metrics, fitness_metrics, each `date` UNIQUE), two ALTER TABLE (body-weight biometrics, workouts status with back-fill to `completed`).
- **Code**: two new `internal/recoverymetrics/` + `internal/fitnessmetrics/` packages (types/repo/service/handlers/tests, mirroring the bodyweight package shape); `internal/bodyweight/` (4 fields through types/repo/service/handlers); `internal/workouts/` (status field + future-guard conditional + list filter); `internal/dailycontext/` (two new read compositions + richer weight); `internal/httpserver/server.go` wiring for the two new capabilities; `internal/mcpserver/` (two new tool files + weight/workout tool edits + expected-tools bump).
- **Tests**: per-capability handler/repo integration tests; upsert-by-date round-trips; planned-workout future-date acceptance + completed-workout rejection; daily-context composition with/without the new data; MCP forwarding + tool-list.
- **Docs**: README sections for the two new capabilities + the weight/workout additions; RUN_LOCAL push examples; `task swag`.
- **Out-of-repo coordination** (not implemented here): `garmin.py` gains a push path mapping its already-fetched recovery/fitness/weigh-in/calendar data onto these endpoints, and a reconciliation rule for promoting/replacing `planned` rows when the real activity syncs.

### Out of scope (explicit non-goals)

- **`garmin.py` push implementation** — out-of-repo; this change only makes the data storable.
- **EA / FFM consuming muscle mass** — the EA resolver keeps its current four-tier rule; preferring measured lean mass is a follow-up.
- **Training-day template auto-application from planned workouts** — the periodization behavior is a separate, larger change; this only adds the `status` field + filter.
- **Derived analytics** — rolling HRV trend, ACWR flags, sleep-debt, race-readiness scoring stay agent-side or future; these capabilities store primitives.
- **Sleep-stage breakdown** (deep/light/REM/awake) — store total `sleep_seconds` + `sleep_score`; stage detail is not fueling-relevant enough to model now.
- **Per-metric trend endpoints** (a `/recovery-metrics/trend` like `/weight/trend`) — range-list + agent-side math for now.
- **PATCH on the metrics tables** — machine-written daily snapshots are corrected by re-POST (full-replace upsert), not field-by-field PATCH; no PATCH endpoint on recovery/fitness.
- **A combined single metrics table** — recovery and fitness are deliberately separate capabilities (per the scoping decision), matching the unit-isolation discipline.
