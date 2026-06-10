# add-hydration-balance-metrics

## Why

Garmin's daily hydration response (`get_hydration_data(date)`) carries `sweatLossInML` and `activityIntakeInML` alongside the `valueInML`/`goalInML` the importer already pushes. These two are the daily **water-balance** signals — how much fluid the body lost to sweat, and how much was taken *during activity* — and they have no home in the API today. Without them the agent can't answer "did I rehydrate enough relative to what I sweated?", which is the daily counterpart to the per-activity sweat picture.

Note on grain: the per-activity `workouts.sweat_loss_ml` column already exists (shipped in `widen-workout-ingestion`). Garmin's `sweatLossInML`/`activityIntakeInML` are **daily** aggregates — a different grain — so they belong in a daily snapshot, not on a workout row. This change adds that snapshot, following the exact date-keyed pattern of `recovery-metrics` and `fitness-metrics`.

## What Changes

- **New capability `hydration-balance`** — a `hydration_balance_metrics` table keyed by `date` (one daily snapshot), all metric columns nullable: `sweat_loss_ml` (estimated daily sweat loss), `activity_intake_ml` (fluid taken during activities), `goal_ml` (Garmin's daily hydration goal). REST: `POST /hydration-balance` (upsert-by-date — "push every day you see"), `GET /hydration-balance?from=&to=`, `GET /hydration-balance/{date}`, `DELETE /hydration-balance/{date}`.
- **MODIFIED `daily-context`** — the `GET /context/daily` bundle gains a `hydration_balance` block (same-day-or-null, read composition over the new repo), sitting alongside the existing `hydration` block (total_ml + entries_count). The two stay distinct: `hydration` is the user's logged intake; `hydration_balance` is Garmin's daily sweat/intake estimate.
- **MODIFIED `mcp-server`** — new tool group `log_hydration_balance`/`list_hydration_balance`/`get_hydration_balance`/`delete_hydration_balance`. Integration-test expected-tools list bumped (+4).
- **swag** regenerates `docs/` for the new endpoints.

## Capabilities

### New Capabilities

- `hydration-balance`: daily water-balance snapshot (sweat loss out, activity intake in, daily goal) stored one row per date, upsert-by-date, with list/get/delete reads. Distinct from the `hydration` capability (per-entry logged intake) by grain and source.

### Modified Capabilities

- `daily-context`: the daily bundle gains a `hydration_balance` block (same-day-or-null).
- `mcp-server`: one new tool group (4 tools); expected-tools list bumped.

## Impact

- **Schema**: one append-only migration (verify next free slot — `024` expected, head is `023`): `CREATE TABLE hydration_balance_metrics` (`date` DATE PK, three nullable NUMERIC columns + CHECKs, audit timestamps).
- **Code**: a new `internal/hydrationbalance/` package (types/repo/service/handlers/tests, mirroring `recoverymetrics`); `internal/httpserver/server.go` wiring; `internal/dailycontext/` (one new same-day read + block); `internal/mcpserver/` (one new tool file + expected-tools bump).
- **Tests**: handler/repo integration tests (upsert-by-date round-trip, full-replace NULL, validation, window caps, get/delete 404, unit-isolation); daily-context composition with/without the block; MCP forwarding + tool-list.
- **Docs**: README section + RUN_LOCAL push example; `task swag`.
- **Out-of-repo coordination** (not implemented here): `garmin.py`'s existing `_push_hydration` already fetches `get_hydration_data(day)` — it gains a second POST mapping `sweatLossInML`/`activityIntakeInML`/`goalInML` onto `/hydration-balance` for the same day.

### Out of scope (explicit non-goals)

- **Duplicating the daily intake total.** Garmin's `valueInML` (daily total intake) is already pushed to `/hydration` as a per-day entry; this capability does NOT restore it — the daily balance reads sweat-out from here and total-in from the existing hydration summary.
- **Per-activity sweat loss.** That is `workouts.sweat_loss_ml`, already shipped; this is the daily aggregate, a separate grain.
- **A computed "balance" field** (sweat_loss − intake). The capability stores primitives; the agent (or a future analytics surface) computes the deficit/ratio.
- **`garmin.py` push implementation** — out-of-repo; this change only makes the data storable.
- **PATCH on the snapshot table** — machine-written daily snapshots are corrected by re-POST (full-replace upsert), matching recovery/fitness; no PATCH endpoint.
