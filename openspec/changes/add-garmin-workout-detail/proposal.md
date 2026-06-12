## Why

Garmin-synced completed workouts currently land as a flat summary (sport, kcal, avg HR, TSS, distance, avg power, temperature). Garmin exposes far more per activity — time-in-HR-zone, elevation gain, normalized power, per-lap splits, and strength sets — and the fueling math wants it: carbohydrate-oxidation rate and total glycogen cost are driven by *duration-at-intensity*, not a single average HR. Today the still-open derived sweat-rate endpoint, the `raceprep`/`workoutfuel` carb math, and the chat coaching agent all see only a flat average where the signal lives in the distribution. Strength sessions arrive completely blank (no sets/reps), and per-lap pace-fade is invisible.

This is change **B** of the "mirror everything" Garmin arc — the headline "especially workouts" slice. Siblings (sequenced after, out of scope here): A `add-garmin-daily-energy`, C `extend-recovery-fitness`, D `add-garmin-gear-and-prs`, E `garmin-workout-library-mgmt` (which carries the write/blob MCP tools `delete_workout`, `add_hydration_data`, FIT export).

## What Changes

- **Amends the workouts capability's deliberate "no performance analysis" scope.** The current spec says laps/splits/streams are explicitly out of scope; this change narrows that exclusion to *streams/GPS only* and brings **per-lap splits, HR-zone distribution, and strength sets** in scope, because they feed nutrition fueling math (not because we want generic performance analytics).
- **New scalar columns on `workouts`** (all nullable, mapped from fields already present in the `get_activities_by_date` summary — **zero new Garmin calls**): `elevation_gain_m`, `elevation_loss_m`, `normalized_power_w`, `intensity_factor`, `avg_cadence`, `avg_stride_m`, `max_hr`, `aerobic_te`, `anaerobic_te`.
- **New weather columns** `humidity_pct` + `wind_speed_mps` (from `get_activity_weather`, complementing the existing `temperature_c`) — humidity is a primary sweat-rate driver, so this closes a fueling-relevant gap; indoor activities leave them NULL.
- **HR-zone time as fixed columns** `secs_in_zone_1`..`secs_in_zone_5` on `workouts` (fixed cardinality → columns, deliberately *not* a child table, so the most-queried fueling signal needs no join).
- **New child table `workout_splits`** (1:N, `ON DELETE CASCADE`): per-lap index, distance, duration, avg HR, avg power, avg speed, elevation gain.
- **New child table `workout_sets`** (1:N, `ON DELETE CASCADE`): per-set index, exercise name/category, reps, weight, duration — fills the blank strength sessions.
- **`POST /workouts` and `POST /workouts/bulk` accept nested `splits[]` / `sets[]` arrays**, written with the parent in a single transaction; on re-sync (same `external_id`) children are fully **replaced** (delete + reinsert) so re-imports stay idempotent.
- **`GET /workouts/{id}` returns the scalar + zone fields inline and the nested splits/sets detail.** List responses carry the scalar + zone fields; nested detail is returned on the single-get to keep list payloads lean.
- **MCP mirrors REST 1:1** — the existing get-workout tool forwards the enriched body verbatim; no new tool (no expected-tools bump expected).
- **garmin-bridge** extends `fetch_day` to pull per-activity zones/splits/sets (each guarded by the existing `safe()` pattern) and `map_workouts` to emit the new scalar fields + nested children; the trimmed test fixture grows to cover them.

## Capabilities

### New Capabilities
<!-- None — splits/sets are child tables of the existing workouts capability, not a standalone capability. -->

### Modified Capabilities
- `workouts`: new scalar + zone columns, two new child tables (`workout_splits`, `workout_sets`), nested-write on POST/bulk with replace-on-resync, enriched single-get response, and an amended Purpose (splits/zones/sets now in scope for fueling).
- `garmin-bridge`: `fetch_day` fans out to per-activity detail endpoints; `map_workouts` emits the new fields and nested children; per-capability failure tolerance extends to the new fetches.

## Impact

- **Schema**: migration `036_add_workout_detail` — new columns on `workouts` (nullable, no back-fill) + `workout_splits` + `workout_sets` tables. Verify head is `035` before scaffolding (`task migrate:new`).
- **Code**: `internal/workouts/` (types, repo cross-table reads, service validation, handlers, swag); `internal/httpserver` wiring unchanged (same package); `apps/garmin-bridge/garmin_bridge/{garmin_client,mapping,sync}.py` + fixtures/tests. `workout_builder` (structured-workout write side) is **unaffected** — this is purely the read/import path.
- **Docs/tests**: `task swag` after handler/struct changes; per-handler integration tests; bridge mapping tests against the expanded fixture; MCP integration test expected-tools list reviewed (no change expected).
- **Conventions honored**: unit isolation (detail stays on the workouts shape, never merged into `summary` Totals), `numfmt.Round1` at the response boundary for new floats, append-only sequential migration.
