## Context

The Garmin "mirror everything" arc captured the data that drives fueling and
energy math first (workouts B, daily energy A, recovery/fitness C) and the
coaching-adjacent inventory next (gear + PRs D), with the control-plane
write/blob tools for the workout library landing in E. What remains is the
**tail** — device inventory, daily health vitals, earned badges/challenges, and a
handful of activity-level control operations. None of it feeds any computation;
it is reference/coaching context the user asked for under "mirror *everything*."

This change is therefore explicitly LOW PRIORITY and applies LAST. The design
problem is not "how do we compute X" — nothing computes anything here — it is
"how do we keep the tail coherent and minimal." The governing constraint is to
**group the long Garmin tail into a SMALL number of coherent capabilities** (the
brief's pragmatic grouping), each CAPTURE-ONLY with no derived math, rather than
spawning one capability per Garmin endpoint.

Constraints that shape the design:
- **One package per capability** — three new packages (`devices`, `healthvitals`,
  `achievements`), not one-per-endpoint.
- **Unit isolation** — none of the new fields may merge into `summary`'s Totals
  struct; each capability keeps its own response shape.
- **`numfmt.Round1` at the response boundary** for every float.
- **Append-only sequential migrations** — head on disk is `035`; the arc reserves
  `036`–`040` (B=036, A=037, C=038, D=039, F=040), so this is `041`.
- **garmin-bridge tolerates per-capability failure** — every new fetch is guarded
  so one bad Garmin endpoint never aborts a day's sync.
- **MCP mirrors REST/control 1:1** — one HTTP call per tool, body forwarded
  verbatim via `toToolResult`; control endpoints return `503 garmin_disabled`
  when `GARMIN_BRIDGE_URL` is unset.

## Goals / Non-Goals

**Goals:**
- Complete the "mirror everything" promise by capturing the deferred Garmin tail
  in a small, coherent capability set (three tables + a handful of MCP tools).
- Keep every new capability CAPTURE-ONLY: store what Garmin returns, round floats
  at the boundary, no derived metrics.
- Reuse the established shapes — inventory upsert-by-external-id (D's gear/PRs) for
  devices and achievements; date-keyed upsert (recovery-metrics) for
  health-vitals; control-plane verbatim-forward + base64 blob envelope (E) for the
  activity operations.

**Non-Goals:**
- No fueling/EA/hydration/summary coupling — nothing downstream reads these
  fields; they never enter a Totals struct.
- No new derived math, no rolling averages, no aggregation across rows.
- No duplication of sibling D's personal records, no menstrual/pregnancy data, no
  streams/GPS, no social/connections data (see proposal's "Deliberately still
  excluded").
- No back-fill of historical devices/vitals/achievements (forward syncs and
  explicit re-syncs only).

## Decisions

### D1 — Three capabilities, grouped by shape, not by endpoint
The Garmin tail spans ~8 endpoints. Rather than one capability per endpoint, group
them by their natural storage shape:
- **`devices`** and **`achievements`** are *slowly-changing inventory* keyed by a
  stable Garmin id → upsert-by-`external_id`, exactly like sibling D's `gear` and
  `personal_records`. `get_devices` + `get_device_last_used` collapse into one
  device row; `get_earned_badges` + `get_adhoc_challenges` collapse into one
  `achievements` row each (discriminated by a `kind` column).
- **`health-vitals`** is a *daily snapshot* — blood pressure, all-day HR, all-day
  stress are all "one reading per calendar day" → date-keyed upsert, exactly like
  `recovery-metrics`. `get_blood_pressure` + `get_heart_rates` + `get_all_day_stress`
  collapse into one row per `date`.

*Alternative considered:* separate `blood-pressure`, `all-day-hr`, `stress-detail`
capabilities — rejected; they share the date-key and the "daily wellness reading"
shape, so one snapshot table with nullable columns is the recovery-metrics
precedent and avoids three near-empty packages.

### D2 — health-vitals is distinct from recovery-metrics, not a column add
The obvious-looking move is to add blood-pressure and all-day-HR columns to the
existing `recovery_metrics` table. Rejected: `recovery-metrics` is the
sleep/HRV/readiness/body-battery *recovery* snapshot, and sibling C is already
extending it; piling unrelated blood-pressure and all-day-HR detail onto it would
blur a settled capability and collide with C's in-flight columns. A separate
`health_vitals` table keeps the two daily snapshots independent — recovery for
"how recovered am I," vitals for "raw cardiovascular readings" — both date-keyed,
both upsert-on-POST, neither feeding nutrition.

### D3 — Activity control operations are MCP tools + control endpoints, no table
The activity↔gear read, structured-workout download, FIT upload, rename, and
delete are *operations on Garmin's side*, not periodic reads into a table — there
is nothing to persist. They follow change E's pattern exactly: a bridge op, a
backend control endpoint that forwards verbatim, and an MCP tool issuing one HTTP
call. The download is a blob → a base64-wrapped JSON envelope `{garmin_workout_id,
format, filename, content_base64}`, mirroring E's activity-export envelope. Reads
(`get_activity_gear`, `download_workout`) send no `Idempotency-Key`; writes
(`upload_activity`, `set_activity_name`, `delete_activity`) auto-derive one via
`effectiveIdempotencyKey`. Delete is idempotent: an activity Garmin reports as
already absent is a no-op success (404-is-success), matching E's
delete-workout-object semantics.

*Alternative considered:* a `garmin-activities` mirror table that imports every
activity's metadata — rejected; completed activities already land in `workouts`
(sibling B carries their detail), and these five are control operations, not a new
read surface.

### D4 — Upsert-by-external-id refreshes inventory; no delete reconciliation
Devices and achievements re-POST on every sync; `INSERT … ON CONFLICT
(external_id) DO UPDATE` refreshes them in place (a device's `last_sync_at` and
`battery_pct` move, a challenge's `progress_pct` advances). A row removed on
Garmin's side is never deleted locally — the same accepted limitation as sibling
D. This is fine: the agent reads "what Garmin last reported," and stale inventory
is harmless reference data, not a math input.

### D5 — Bridge fetches are guarded and additive
`fetch_day` gains guarded `safe()` sub-fetches for the new sources
(`get_devices`/`get_device_last_used`, `get_blood_pressure`/`get_heart_rates`/
`get_all_day_stress`, `get_earned_badges`/`get_adhoc_challenges`). `map_day` gains
`map_devices` / `map_health_vitals` / `map_achievements` extractors, each
defensive (absent → omitted, snapshot dropped when it carries no metric). `sync.py`
gains the new routes: date-keyed snapshot for health-vitals, per-item upsert lists
for devices and achievements (modelled on the existing `/weight` per-item loop).
The five activity control bridge ops live in `garmin_client.py` + `app.py`
alongside E's workout-library ops; they are not part of `fetch_day`/sync — they
are on-demand, driven by the control endpoints.

### D6 — Capture-only: no rounding-up into derived fields
Every stored value is a direct Garmin reading. The only transform is unit
normalization the existing mapper already does (e.g. epoch-ms → RFC3339 for
`last_sync_at`/`earned_at`) and `numfmt.Round1` at the serialization boundary for
floats (`battery_pct`, `progress_pct`, any decimal vital). No averages, no
trends, no cross-row aggregation — those would be follow-ups if a real consumer
ever appears.

## Risks / Trade-offs

- **Low-value scope creep** → the proposal frames this honestly as completeness
  work and lists everything genuinely worthless under "Deliberately still
  excluded," so the arc does not overclaim. Building it is opt-in completeness, not
  a fueling requirement.
- **Garmin endpoint churn across many new fetches** → each is isolated behind
  `safe()` in `garmin_client.py`, exactly like the existing day fetches; a break
  degrades to "no devices/vitals/achievements this sync," never a failed sync.
- **health-vitals vs recovery-metrics confusion** → kept as two distinct
  date-keyed tables with non-overlapping columns (D2); the spec Purpose for each
  states the boundary so a future reader does not merge them.
- **Three near-trivial packages** → accepted; one-package-per-capability is the
  repo convention, and the three are genuinely different shapes (two inventory,
  one snapshot). Collapsing them would break the convention without saving real
  code.
- **Upsert leaves stale rows** (D4) → accepted limitation, identical to sibling D;
  reference data only, no math depends on freshness.

## Migration Plan

1. Confirm the migration head on disk before scaffolding. The arc reserves
   `036`–`040`; with no out-of-band slot taken this change is `041`. Run
   `task migrate:new NAME=add_garmin_misc_mirror` only after verifying head.
2. `041_add_garmin_misc_mirror.up.sql` creates **three** tables in one migration:
   - `devices` — `id` UUID PK, `external_id` TEXT NOT NULL (UNIQUE), `display_name`
     TEXT NOT NULL, `model` TEXT NULL, `last_sync_at` TIMESTAMPTZ NULL,
     `battery_pct` NUMERIC(5,1) NULL (0–100 CHECK), `firmware_version` TEXT NULL,
     audit timestamps.
   - `health_vitals` — `date` DATE PRIMARY KEY, nullable vital columns
     (`bp_systolic`, `bp_diastolic`, `bp_pulse`, `resting_hr`, `min_hr`, `max_hr`,
     `stress_avg`, `stress_max`), audit timestamps.
   - `achievements` — `id` UUID PK, `external_id` TEXT NOT NULL (UNIQUE), `kind`
     TEXT NOT NULL (CHECK `kind IN ('badge','challenge')`), `name` TEXT NOT NULL,
     `earned_at` TIMESTAMPTZ NULL, `progress_pct` NUMERIC(5,1) NULL (0–100 CHECK),
     audit timestamps.
3. `.down.sql` drops the three tables (`achievements`, `health_vitals`, `devices`).
4. Rollback is clean — additive tables, no data transform, no change to existing
   rows.

## Open Questions

- Whether `get_heart_rates` exposes a usable all-day min HR or only resting/max —
  if a field is absent it simply stays NULL (no derivation in this change).
- Whether `download_workout` returns FIT only or also other formats — default to
  `fit` and pass through an optional `format` param like E's export; if Garmin
  rejects a format the bridge surfaces the error verbatim.
- Whether ad-hoc challenges expose a stable id distinct from earned badges — if the
  id space overlaps, prefix the `external_id` by `kind` in the mapper so the UNIQUE
  constraint never collides (resolved in the bridge mapper, not the schema).
