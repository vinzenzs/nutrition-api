## Why

Garmin tracks two inventory-shaped datasets the backend mirrors nowhere: **gear** (shoe/bike mileage, retirement state) and **personal records** (5k/10k/longest-ride PRs). Neither feeds fueling math — this is the **most tangential-to-nutrition** slice of the "mirror everything" Garmin arc, and we are building it precisely because the user chose to mirror *everything*, not because carb-oxidation or hydration math needs it.

The justification is **coaching context for the chat agent**, not nutrition computation:
- **Gear retirement reminders** — when a shoe crosses its mileage budget the agent can surface "your daily-trainers are at 780 km, time to rotate" without the user pulling up Garmin. Total distance + a `retired` flag is enough for that nudge.
- **"You're PR-fit right now" context** — fresh personal records let the agent frame race-prep advice ("your 10k PR is three weeks old — your top-end is sharp going into taper") instead of guessing fitness from training load alone.

Both are **slowly-changing inventory**, not date-keyed snapshots: gear and PRs change rarely and are identified by a stable Garmin id, so the natural shape is an idempotent **upsert by external id** rather than the per-date upsert the recovery/fitness snapshots use.

This is change **D** of the "mirror everything" Garmin arc. Siblings: B `add-garmin-workout-detail` (the headline workouts slice), A `add-garmin-daily-energy`, C `extend-recovery-fitness`, E `garmin-workout-library-mgmt` (write/blob MCP tools). This change is read/import only — two new capabilities, no fueling-math coupling.

## What Changes

- **New capability `gear`** (`internal/gear/`, table `gear`): mirrors Garmin's gear inventory from `get_gear` + `get_gear_stats`. Keyed by the Garmin gear `uuid` (external id); upserted, not date-keyed. Fields: `gear_type` (shoes/bike/other), `display_name`, `total_distance_m`, `total_activities`, `retired`, `date_begin`/`date_end` (nullable).
- **New capability `personal-records`** (`internal/personalrecords/`, table `personal_records`): mirrors Garmin's PRs from `get_personal_records`. Keyed by the Garmin PR `id` (external id); upserted. Fields: `pr_type` (e.g. `5k`/`10k`/`longest-ride`), `value` (numeric, with a unit note), `activity_id` (nullable link), `achieved_at`.
- **REST surface** (read + upsert, no PATCH/DELETE — Garmin owns the truth):
  - `GET /gear` (list), `GET /gear/{id}`, `POST /gear` (upsert by external id).
  - `GET /personal-records` (list), `POST /personal-records` (upsert by external id).
  - `numfmt.Round1` on distance/value floats at the response boundary.
- **MCP** gains two read tools, `gear_list` and `personal_records_list`, mirroring the REST list endpoints 1:1 — the `mcp_integration_test` expected-tools list grows by **two**.
- **garmin-bridge** adds guarded `get_gear` / `get_gear_stats` / `get_personal_records` fetches and maps + POSTs them as part of the daily sync (refresh-via-upsert; see design D1). One bad inventory fetch must never abort the day.

## Capabilities

### New Capabilities
- `gear`: slowly-changing gear inventory (shoe/bike mileage, retirement), upserted by Garmin gear uuid; list + single-get + upsert REST, `gear_list` MCP tool.
- `personal-records`: Garmin personal records, upserted by Garmin PR id; list + upsert REST, `personal_records_list` MCP tool.

### Modified Capabilities
- `garmin-bridge`: the daily sync additionally refreshes gear + PR inventory via guarded `get_gear`/`get_gear_stats`/`get_personal_records` fetches and upsert POSTs; per-capability failure tolerance extends to the two new fetches. (Delta is an ADDED inventory-refresh requirement plus a MODIFIED "Headless daily sync" mapping clause.)

## Impact

- **Schema**: one migration `039_add_gear_and_personal_records` creating **two** tables (`gear`, `personal_records`), each with a UNIQUE external-id column for upsert. Verify head on disk before scaffolding (arc order B=036, A=037, C=038 ⇒ this is `039`).
- **Code**: two new packages `internal/gear/` and `internal/personalrecords/` (each `types.go`/`repo.go`/`service.go`/`handlers.go` + tests); wiring in `internal/httpserver/server.go`; two new MCP tools in `internal/mcpserver/`; `apps/garmin-bridge/garmin_bridge/{garmin_client,mapping,sync}.py` + fixtures/tests.
- **Docs/tests**: `task swag` after handlers; per-handler integration tests against testcontainers Postgres; bridge mapping tests against an expanded fixture; `mcp_integration_test` expected-tools list bumped by two.
- **Conventions honored**: one package per capability (two here); repo against `store.Querier`; sentinel errors → API codes; unit isolation (gear distance / PR value never merged into nutrition Totals); `numfmt.Round1` at the boundary; append-only sequential migration.
- **Accepted limitation**: an upsert never deletes gear/PRs removed on Garmin's side; stale/retired handling is best-effort via the `retired` flag (see design D3).
