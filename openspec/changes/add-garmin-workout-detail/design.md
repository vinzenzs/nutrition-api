## Context

Completed workouts are imported by the garmin-bridge as a flat summary and stored one-row-per-activity in `workouts`. The workouts spec deliberately scoped out "performance analysis (laps, splits, GPS, streams)" — that exclusion was right when the only consumer was "what was the athlete doing in window X?". It is now too tight: the fueling math (`raceprep`, `workoutfuel`, the planned derived sweat-rate endpoint) needs *duration-at-intensity*, and strength sessions land with no content at all.

Constraints that shape the design:
- **One package per capability** — this extends `internal/workouts/`, it is not a new capability.
- **Unit isolation** — workout detail stays on the workouts shape; it must never leak into `summary`'s Totals struct.
- **`numfmt.Round1` at the response boundary** for nutrient/measurement floats.
- **Append-only sequential migrations** embedded in the binary; head is `035`, so this is `036`.
- **garmin-bridge tolerates per-capability failure** — one bad Garmin endpoint must never abort a day's sync.
- The bridge already fetches `get_activities_by_date`; the richer **scalar** fields ride along in that same payload (no extra calls), while zones/splits/sets each need a per-activity Garmin call.

## Goals / Non-Goals

**Goals:**
- Capture the per-activity detail that drives fueling math: time-in-HR-zone, elevation, normalized power / IF, per-lap splits, strength sets.
- Keep re-sync idempotent: re-importing an activity fully replaces its detail.
- Surface the detail through the existing REST + MCP get-workout path with no new MCP tool.
- Leave the structured-workout *write* path (`workout_builder`, scheduling) untouched.

**Non-Goals:**
- Per-second time-series **streams** and **GPS polylines** remain explicitly out of scope (heavy, no fueling use). The amended scope brings in laps/zones/sets only.
- No new standalone capability — no `workout-detail` spec folder; splits/sets are child tables of `workouts`.
- No changes to the daily snapshot capabilities (recovery/fitness/hydration-balance) — those are sibling changes A/C.
- No back-fill of historical activities' detail (only forward syncs and explicit re-syncs gain detail).

## Decisions

### D1 — HR zones as fixed columns, splits/sets as child tables
HR zones are fixed-cardinality (always 5 buckets), so `secs_in_zone_1..5` live as columns directly on `workouts`. This avoids a join for the single most-queried fueling signal (time-in-zone) and keeps it inline on list responses. Splits and sets are genuinely 1:N (variable lap/set counts) → child tables `workout_splits` and `workout_sets`, `ON DELETE CASCADE` on `workout_id`.
*Alternative considered:* a `workout_zones` child table for symmetry — rejected; fixed cardinality makes columns simpler and join-free, and the count never varies.

### D2 — Scalar fields come free from the existing activities payload
`elevation_gain_m`, `elevation_loss_m`, `normalized_power_w`, `intensity_factor`, `avg_cadence`, `avg_stride_m`, `max_hr`, `aerobic_te`, `anaerobic_te` are already present in the `get_activities_by_date` summary the bridge fetches today. Mapping them adds **zero** Garmin calls — only `map_workouts` lines + columns. This is why the change is worth doing as one slice rather than gating the cheap fields behind the expensive fan-out.

### D3 — Nested write on POST and bulk, children replaced on re-sync
`POST /workouts` and `POST /workouts/bulk` items accept optional `splits[]` and `sets[]` arrays. The handler writes parent + children in **one transaction** via the repo running against a `pgx.Tx`. On an `external_id` UPSERT that matches an existing row, the children are fully **replaced** (`DELETE … WHERE workout_id = $1` then re-insert) so a re-sync never accumulates duplicate laps. Zone columns are part of the flat workout body (no nesting).
*Alternative considered:* follow-up `POST /workouts/{id}/splits` calls — rejected; it forces the bridge to thread returned UUIDs and breaks the single-activity atomicity. The bridge is not bound by the MCP "one HTTP call per tool" rule, so nesting is clean on the import side.

### D4 — Detail returned on single-get only; scalars+zones on list
`GET /workouts/{id}` returns scalar + zone fields inline **and** nested `splits[]` / `sets[]`. `GET /workouts` (list) returns the scalar + zone fields but **not** the nested arrays, to keep list payloads lean (a brick week could be dozens of activities × dozens of laps). MCP mirrors this 1:1 — the existing get-workout tool forwards the enriched body verbatim; no new tool, so the `mcp_integration_test` expected-tools list is unchanged.

### D5 — Bridge fan-out guarded per activity, per endpoint
`fetch_day` gains, for each activity, four guarded sub-fetches (`get_activity_hr_in_timezones`, `get_activity_splits`, `get_activity_exercise_sets`, `get_activity_weather`) using the existing `safe()` wrapper — a failing or unavailable endpoint yields a missing key, not an aborted day. `map_workouts` reads whatever detail is present and attaches it; absent detail simply omits the field/array. Fan-out is ~4N extra calls for N activities/day — negligible for a single-user daily sync.

### D7 — Weather (humidity + wind) belongs here, not a sibling
`temperature_c` already lands today; humidity is the *other* primary sweat-rate driver and wind affects evaporative cooling, so `humidity_pct` + `wind_speed_mps` (from `get_activity_weather`) are the directly fueling-relevant weather fields. They are per-activity, so they ride B's existing per-activity fan-out rather than a daily snapshot. Indoor activities legitimately have no weather → the fields stay NULL. Fuller weather (conditions text, dew point, pressure) is out of scope — only the sweat-rate inputs earn columns.

### D8 — Detail must attach to the reconciled row
The shipped reconciliation merges a Garmin import into a matching planned workout (planned→completed in place, preserving `template_id`/`plan_slot_id`). The nested splits/sets and scalar/weather columns must land on that *surviving* row, and the `external_id` must end up on it so subsequent imports keep matching — otherwise a reconciled day would either lose its detail or spawn a duplicate. This is the one seam where B touches shipped behavior; it gets an explicit spec scenario and an integration test (task 6.4) rather than being left implicit. The child-replace-on-resync semantics then apply to the reconciled row like any other.

### D6 — Sport-aware detail: sets for strength, splits for endurance
Strength activities populate `sets[]` (and typically no splits); run/bike/swim populate `splits[]`. The mapper does not enforce this — it attaches whatever Garmin returns — but the typical shapes inform the test fixtures (a strength activity with sets, a run with splits + zones).

## Risks / Trade-offs

- **Garmin per-activity endpoint churn** → the three new fetches are isolated in `garmin_client.py` behind `safe()`, exactly like the existing day fetches; a Garmin break degrades to "no detail this sync", not a failed sync.
- **Bulk transaction scope grows** (parent + children per item) → each bulk item remains independent; a child-write failure fails only that item (its `results` entry carries the error), matching existing per-item partial-failure semantics.
- **List-response shape divergence** (list omits nested arrays, get includes them) → documented in the spec and swag; a consumer wanting detail calls the single-get. Keeps list payloads bounded.
- **Column creep on `workouts`** (9 scalar + 5 zone = 14 new columns) → all nullable, no back-fill, no new index except the child-table FKs; "not measured" stays a meaningful NULL, consistent with the existing ingestion-metric columns.
- **Re-sync replace deletes user-unowned child rows** → splits/sets are import-only (no manual UI writes them), so replace-on-resync has no user data to clobber, unlike `rpe`/`gi_distress_score` which are PATCH-only and untouched here.

## Migration Plan

1. `task migrate:new NAME=add_workout_detail` after confirming head is `035` (→ `036`).
2. `036_add_workout_detail.up.sql`: `ALTER TABLE workouts ADD COLUMN` the 9 scalar + 5 zone columns (all NULL, with sane CHECKs mirroring existing ones, e.g. `>= 0`); `CREATE TABLE workout_splits` and `workout_sets` with `workout_id UUID NOT NULL REFERENCES workouts(id) ON DELETE CASCADE` + an index on `workout_id`.
3. `.down.sql`: drop the two tables, then drop the columns.
4. Rollback is clean — additive columns/tables, no data transform; existing rows read back unchanged.

## Open Questions

- Exact split field set: lock to `{index, distance_m, duration_s, avg_hr, avg_power_w, avg_speed_mps, elevation_gain_m}`; confirm `avg_speed_mps` vs storing pace — leaning speed (sport-agnostic, pace derivable). **Resolved in spec: store `avg_speed_mps`.**
- Whether `intensity_factor` is in the activities summary or needs derivation from normalized power ÷ FTP — if Garmin omits it, the field simply stays NULL (no FTP lookup in this change).
