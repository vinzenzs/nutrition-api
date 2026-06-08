## Why

Today the API has no concept of a workout. That single gap blocks at least six tier-1/tier-2 needs that have surfaced from real triathlon-training use: attaching meals or fuel entries to a session, computing energy availability, recommending pre/intra/post fueling for a planned ride, anchoring GI/RPE notes, anchoring sweat-rate tests, and pulling burn data into adherence math. Every one of those is currently impossible because the system cannot answer "what was I doing yesterday between 6:00 and 7:30?"

This change introduces a `workouts` capability as a standalone primitive. The design choice is deliberate: the backend exposes a minimal write surface; the writer (today `garmin.py`, tomorrow potentially Apple Health, Strava, or a manual REST call) lives outside. This mirrors the agent-side architecture: backend stores primitives, external systems do the heavy domain work.

This proposal is intentionally narrow. It ships only the workout primitive — the new entities, endpoints, MCP tools, and the data shape that a Garmin-style importer can target. Wiring workouts into meal/hydration FK relationships, the EA tool, `recommend_workout_fuel`, and the rest of the cluster are explicit follow-ups, each a separate small change.

## What Changes

- **New `workouts` table** with the minimum fields nutrition tools need: `id`, `external_id` (nullable; UNIQUE — e.g. `"garmin:1234567"`), `source` (enum: `garmin | manual | other`), `sport` (enum: `run | bike | swim | strength | other`), `name`, `started_at`, `ended_at`, `kcal_burned` (nullable), `avg_hr` (nullable), `tss` (nullable), `notes` (nullable), audit timestamps. No laps, no splits, no GPS, no streams — nutrition tools only need start/end/sport/burn/TSS. No `intensity` enum column: TSS is the intensity signal, and downstream tools derive bands from it when they need a classification.
- **Six REST endpoints** (no Idempotency-Key contract on single writes — `external_id` is the deduplication mechanism):
  - `POST /workouts` — upsert on `external_id` if present; insert if absent. Returns 201 (insert) or 200 (update).
  - `POST /workouts/bulk` — upsert an array of workouts in one call. Each item upserts independently with the same `external_id` semantics as `POST /workouts`. Per-item results so the writer learns which items failed; partial failure is allowed. Capped at 100 items per request.
  - `GET /workouts?from=…&to=…` — list workouts whose `started_at` falls in the inclusive window, ordered ascending. 92-day cap (matches `/meals`).
  - `GET /workouts/{id}` — single workout.
  - `PATCH /workouts/{id}` — partial update of `name`, `notes`, `kcal_burned`, `avg_hr`, `tss` (manual corrections; sport / started_at / ended_at / source / external_id are immutable post-create).
  - `DELETE /workouts/{id}` — remove a workout. 204 / 404.
- **Five MCP tools** wrapping the single-item endpoints: `log_workout`, `list_workouts`, `get_workout`, `patch_workout`, `delete_workout`. Standard POST-style auto-derive idempotency keys on writes. No MCP wrapper for `/workouts/bulk` — bulk is a writer-side concern (garmin.py), the agent works one workout at a time.
- **No Garmin client in the backend.** No OAuth, no token storage, no scheduled sync. The external writer (today `garmin.py`) reads from Garmin and POSTs to the API.

## Capabilities

### New Capabilities
- `workouts`: A persisted catalogue of training sessions with the minimum metadata nutrition tools need (sport, time window, intensity, burn). Storage-only — performance analysis lives elsewhere. Designed as a comfortable target for an external Garmin-style importer without coupling the backend to any specific source.

### Modified Capabilities
- `mcp-server`: Adds a requirement covering the five new workout tools. The existing tool requirements are unchanged.

## Impact

- **Schema migration** `internal/store/migrations/012_add_workouts.{up,down}.sql`. One table, two indexes (`(started_at)` for window queries; partial UNIQUE on `external_id WHERE external_id IS NOT NULL`).
- **New code**:
  - `internal/workouts/` package: `types.go`, `repo.go`, `service.go`, `handlers.go` (single-item + bulk endpoints).
  - `internal/mcpserver/tools_workouts.go` — five single-item tools, follows the existing patterns (`tools_hydration.go` is the closest template).
- **Wiring** in `internal/httpserver/server.go` (one repo, one service, one handlers registration) and `internal/mcpserver/server.go` (one `registerWorkoutsTools` call).
- **No changes to** `meals`, `hydration`, `products`, `summary`, `goals`, `race-prep`, `off-integration`, `auth`. Workouts are genuinely standalone in this change.
- **Tests**: handler-level tests for single-item and bulk endpoints, MCP unit tests for each of the five tools, one integration test addition so the tools-list grows by five (20 → 25).
- **Documentation**: `task swag` regenerates OpenAPI; README gains a "Workouts" subsection + five MCP-table rows; RUN_LOCAL.md gains a short example of logging a workout and listing today's sessions.

### Out of scope (explicit non-goals)

- **`workout_id` foreign key on meals / hydration / fuel entries.** A follow-up change. Shipping workouts standalone keeps the blast radius small and lets us use the table before deciding which intake rows should reference it.
- **Energy availability tool, `recommend_workout_fuel`, sweat-rate workflow.** All depend on `workouts` existing; each is a separate small change after this lands.
- **In-session fueling (`workout_fuel_entries`).** A sibling capability for during-workout carbs / sodium / caffeine; separate change.
- **Garmin OAuth / scheduled sync in the backend.** Out of architecture — `garmin.py` is the writer. The API has no Garmin code.
- **Multisport parent / child relationships.** Each leg of a brick imports as its own workout row (decision detailed in design.md). If "tell me about the whole brick as one unit" becomes a real need, a `parent_external_id` column is a cheap follow-up.
- **Snapshot semantics for Garmin re-syncs.** If Garmin updates kcal after the fact, the API tracks the new truth (UPSERT overwrites). Preserving original-at-time-of-sync values is a v2 concern, deferred per the existing precedent of meal_entries snapshots only when needed.
- **HR streams, lap / split data, GPS, power curves, training-load metrics beyond TSS.** Nutrition tools don't need them. Performance analysis stays in Garmin / WKO / dedicated tooling.
- **Race-day classification.** No `intensity: "race"` enum, no `is_race` flag. Race-aware fueling adherence is a follow-up concern; until then the agent can tag a workout via `notes` or a future change adds an explicit shape.
- **MCP `bulk_log_workouts` tool.** Bulk is a writer-side concern (garmin.py); the agent operates one workout at a time. If a future use case wants bulk-via-agent, adding the wrapper is trivial.
