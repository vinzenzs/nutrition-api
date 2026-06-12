## Why

The "mirror everything" Garmin arc set out to bring Garmin's whole surface under
the backend's control. The headline slices are done or sequenced — workouts (B),
daily energy (A), recovery/fitness (C), gear + PRs (D), the control-plane
write/blob tools for the workout library (E), CI (F). What is left is the **tail**:
the parts of Garmin the arc deliberately deferred because none of them feed the
fueling, energy-availability, or hydration math — device inventory, daily health
vitals (blood pressure, all-day HR, all-day stress detail), and earned
badges/challenges, plus a handful of activity-level control operations
(activity↔gear link, structured-workout download, FIT upload, rename, delete).

**Be honest about why this exists:** this is **completeness-driven, LOW
PRIORITY**, and it SHOULD apply **LAST** in the arc. The user chose to mirror
*everything*, and these are the "everything else" that nothing downstream
consumes — they are coaching/reference context for the chat agent, not inputs to
any computation. If a slice here turns out genuinely worthless even for
completeness, it is listed under **Deliberately still excluded** below rather
than built. No fueling, EA, hydration, or summary math reads any field this
change adds, and nothing here may leak into the nutrition Totals struct.

This is change **G** of the arc — the explicit catch-all "tail." Siblings (out of
scope here): A `add-garmin-daily-energy`, B `add-garmin-workout-detail`, C
`extend-recovery-fitness`, D `add-garmin-gear-and-prs`, E
`garmin-workout-library-mgmt`, F (CI). Personal records already live in sibling D
and are NOT duplicated here; menstrual/pregnancy health metrics are N/A for this
single user and excluded.

## What Changes

- **New capability `devices`** (`internal/devices/`, table `devices`): mirrors
  Garmin's device inventory from `get_devices` + `get_device_last_used`. Keyed by
  the stable Garmin device id (`external_id`); upserted in place, not date-keyed
  (slowly-changing inventory, like sibling D's gear). Fields: `display_name`,
  `model`, `last_sync_at`, plus cheap-to-read `battery_pct`, `firmware_version`
  (all nullable). Device state is reference context (battery/firmware nudges) and
  feeds no nutrition computation.
- **New capability `health-vitals`** (`internal/healthvitals/`, table
  `health_vitals`): a date-keyed daily snapshot (one row per calendar day, upsert
  by `date`, like recovery-metrics) of the daily health metrics not yet captured
  elsewhere — blood pressure (`get_blood_pressure`: systolic / diastolic /
  pulse), all-day resting/min/max heart rate (`get_heart_rates`), and an all-day
  average/max stress reading (`get_all_day_stress`) when cheap. All columns
  nullable; NULL means "the device did not report it that day." Distinct from
  `recovery-metrics` (sleep/HRV/readiness/body-battery) — this is the
  blood-pressure + all-day-HR/stress detail that snapshot does not carry.
- **New capability `achievements`** (`internal/achievements/`, table
  `achievements`): mirrors Garmin's earned badges and ad-hoc challenges from
  `get_earned_badges` + `get_adhoc_challenges`. Keyed by the Garmin badge/challenge
  id (`external_id`); upserted. Fields: `kind` (`badge`/`challenge`), `name`,
  `earned_at` (nullable for in-progress challenges), `progress_pct` (nullable).
  Personal records are NOT here (sibling D owns them).
- **Control-plane activity operations** (no table — change E's MCP-tool pattern,
  one HTTP call each, `503 garmin_disabled` when the bridge is off):
  - `GET /garmin/activity/{activity_id}/gear` (read the gear linked to an
    activity, from `get_activity_gear`).
  - `GET /garmin/workout/{garmin_workout_id}/download` (export a structured
    workout as a FIT blob via `download_workout`, returned as a base64 envelope —
    the blob analogue of E's activity export).
  - `POST /garmin/activity/upload` (FIT upload via `upload_activity`, a write).
  - `PATCH /garmin/activity/{activity_id}` accepting `{name}` (rename via
    `set_activity_name`, a write).
  - `DELETE /garmin/activity/{activity_id}` (delete via `delete_activity`, a
    write; an already-absent activity is a no-op success).
- **REST surface** for the three new capabilities (read + upsert, no PATCH/DELETE
  — Garmin owns the truth): `GET`/`POST` list + upsert for `devices` and
  `achievements`; date-keyed `GET`/`POST` + single-get for `health-vitals`.
  `numfmt.Round1` on every float at the response boundary.
- **MCP** gains **eight** new tools: three read tools mirroring the new list
  endpoints (`devices_list`, `health_vitals_list`, `achievements_list`) and five
  control-plane tools (`garmin_get_activity_gear`, `garmin_download_workout`,
  `garmin_upload_activity`, `garmin_rename_activity`, `garmin_delete_activity`).
  Write tools (`garmin_upload_activity`, `garmin_rename_activity`,
  `garmin_delete_activity`) auto-derive an idempotency key. The
  `mcp_integration_test` expected-tools list grows by **eight** (state it in
  tasks).
- **garmin-bridge** adds guarded `safe()` fetches for each new source
  (`get_devices`/`get_device_last_used`, `get_blood_pressure`/`get_heart_rates`/
  `get_all_day_stress`, `get_earned_badges`/`get_adhoc_challenges`), maps them, and
  POSTs them in the daily sync; plus the five activity control bridge ops
  (`get_activity_gear`, `download_workout`, `upload_activity`, `set_activity_name`,
  `delete_activity`). One bad fetch must never abort the day. The fixture + mapper
  tests expand to cover the new sources.

## Capabilities

### New Capabilities
- `devices`: slowly-changing Garmin device inventory (name, model, last sync,
  battery, firmware), upserted by Garmin device id; list + upsert REST,
  `devices_list` MCP tool.
- `health-vitals`: date-keyed daily snapshot of blood pressure + all-day HR /
  stress detail, upserted by date; list + single-get + upsert REST,
  `health_vitals_list` MCP tool.
- `achievements`: Garmin earned badges + ad-hoc challenges, upserted by Garmin id;
  list + upsert REST, `achievements_list` MCP tool.

### Modified Capabilities
- `garmin-control`: new activity-level control endpoints (read activity gear,
  download a structured workout blob, upload a FIT activity, rename an activity,
  delete an activity).
- `garmin-bridge`: the daily sync additionally refreshes device, health-vitals,
  and achievement data via guarded fetches and upsert POSTs; plus five new bridge
  operations backing the activity control endpoints. Per-capability failure
  tolerance extends to all new fetches.
- `mcp-server`: eight new tools (three read + five control-plane); expected-tools
  list bumped by eight.

## Impact

- **Schema**: ONE migration `041_add_garmin_misc_mirror` creating **three** tables
  (`devices`, `health_vitals`, `achievements`). `devices` and `achievements` each
  carry a UNIQUE `external_id` for upsert; `health_vitals` is keyed by `date`
  (PRIMARY KEY). Arc order: B=036, A=037, C=038, D=039, E=none, F=040,
  backfill=none ⇒ this is `041`. Verify the head on disk before scaffolding.
- **Code**: three new packages `internal/devices/`, `internal/healthvitals/`,
  `internal/achievements/` (each `types.go`/`repo.go`/`service.go`/`handlers.go` +
  tests); wiring in `internal/httpserver/server.go`; the five new control handlers
  + bridge client functions in `internal/garmincontrol/`; eight new MCP tools in
  `internal/mcpserver/` (`tools_garmin.go` for the control-plane ones) +
  `mcp_integration_test.go` expected list;
  `apps/garmin-bridge/garmin_bridge/{garmin_client,mapping,sync,app}.py` +
  fixtures/tests.
- **Docs/tests**: `task swag` after handlers; per-handler integration tests
  against testcontainers Postgres; bridge mapping tests against an expanded
  fixture; `mcp_integration_test` expected-tools list bumped by eight.
- **Conventions honored**: one package per capability (three here); repo against
  `store.Querier`; sentinel errors → API codes; unit isolation (device/vitals/
  achievement fields never merged into nutrition Totals); `numfmt.Round1` at the
  boundary; append-only sequential migration; MCP mirrors REST/control 1:1 (one
  HTTP call per tool, verbatim body via `toToolResult`); `503 garmin_disabled`
  when the bridge URL is unset; idempotent delete (an already-gone Garmin activity
  is success); write tools auto-derive an idempotency key.
- **Accepted limitation**: an upsert never deletes devices/achievements removed on
  Garmin's side; staleness is best-effort (a device's `last_sync_at` ages, a
  challenge's `progress_pct` updates on re-sync).

## Deliberately still excluded

These were considered and left out — built nowhere, even for completeness:

- **Menstrual cycle / pregnancy** (`get_menstrual_data`, `get_pregnancy_summary`)
  — N/A for this user; no value at any priority.
- **Personal records** (`get_personal_records`) — already mirrored by sibling D
  (`personal-records`); duplicating them here would create two sources of truth.
- **Per-second / GPS streams and time-series** — the same heavy, no-consumer data
  that sibling B explicitly kept out of scope; nothing here reverses that.
- **Social / Connections / leaderboard data** (`get_*_connections`,
  `get_gear_defaults`, social feeds) — multi-user/social surface with no
  single-user reference value.
- **Activity FIT/GPX *export*** (download an *activity* file) — already shipped by
  sibling E as `GET /garmin/activity/{id}/export`; this change exports a
  *structured workout* (`download_workout`), which E did not cover, and reuses the
  same base64-envelope shape.
