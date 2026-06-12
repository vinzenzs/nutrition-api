## Context

`recovery-metrics` and `fitness-metrics` are two of the existing date-keyed snapshot capabilities (sister to `hydration-balance` and the sibling-A `daily-summary`): one row per calendar date, source-agnostic, written by the garmin-bridge "POSTing every day it sees" via a full-replace upsert, with every metric a nullable pointer so absent stays distinct from a real zero. They already capture a slice of Garmin's daily wellness/fitness signals. This change widens that slice with the remaining cheap-to-fetch daily signals the coaching agent wants, without changing the capabilities' shape contract (still snapshot-by-date, still unit-isolated, still nullable-means-not-measured).

Constraints that shape the design:
- **One package per capability** — this extends `internal/recoverymetrics/` and `internal/fitnessmetrics/`; it is not a new capability and adds no new package.
- **Unit isolation** — recovery signals (SpO2 %, respiration bpm, sleep-stage seconds) stay on the recovery shape; fitness signals (scores, fitness age, status label) stay on the fitness shape; neither leaks into `summary`'s Totals struct or each other.
- **`numfmt.Round1` at the response boundary** for the new measurement floats.
- **Append-only sequential migrations** embedded in the binary; the arc assigns B=`036`, A=`037`, so this is `038` — but verify the head on disk before scaffolding.
- **garmin-bridge tolerates per-capability failure** — each new Garmin fetch is individually `safe()`-guarded; one bad endpoint must never abort a day's sync.
- The sleep-stage seconds are read from the **sleep DTO the bridge already fetches** for `sleep_seconds` — zero new Garmin call. SpO2, respiration, endurance/hill/fitness-age each need a per-day Garmin call; `training_status` is already fetched today (the fitness mapper reads `acuteTrainingLoad`/`chronicTrainingLoad` from it) so its label is also free.

## Goals / Non-Goals

**Goals:**
- Capture the remaining daily Garmin recovery signals (SpO2 trend, respiration, sleep-stage breakdown) on the existing recovery snapshot.
- Capture the remaining daily Garmin fitness signals (endurance score, hill score, fitness age, training-status label) on the existing fitness snapshot.
- Keep the snapshot contract intact: nullable, omitempty, full-replace upsert by date, unit-isolated.
- Surface the richer fields through the existing REST + MCP read path with no new endpoint and no new MCP tool.

**Non-Goals:**
- No new capability, no new package, no new table — pure column extension of two existing tables in ONE migration.
- No intraday time-series (per-minute SpO2/respiration arrays) — only the daily avg/lowest aggregates Garmin already summarises.
- No derived metrics: the sleep-stage seconds are stored raw, not validated to sum to `sleep_seconds` (Garmin's own totals occasionally disagree at the margin, and "awake" overlaps differently across firmware); the agent reads them as reported.
- No back-fill of historical snapshots — only forward syncs (and explicit re-POSTs) gain the new fields.

## Decisions

### D1 — Extend the two existing snapshot tables, not a new capability
SpO2/respiration/sleep-stage are conceptually recovery; endurance/hill/fitness-age/training-status are conceptually fitness. They belong on the snapshots that already carry the kindred signals (sleep on recovery; VO2max/load on fitness), so the agent gets one coherent recovery row and one coherent fitness row per date rather than a third snapshot to join. This mirrors how `recovery-metrics` already co-locates sleep + HRV + body battery.
*Alternative considered:* a new `wellness-extras` snapshot — rejected; it fragments the daily picture and duplicates the date-keyed upsert plumbing for no contract benefit.

### D2 — ONE migration adds nullable columns to BOTH tables, no back-fill
A single `038_extend_recovery_fitness` migration `ALTER TABLE`s both `recovery_metrics` and `fitness_metrics`. All new columns are `NULL` with CHECKs mirroring the existing column conventions (percentages `BETWEEN 0 AND 100`, breaths/scores/ages `> 0` or `>= 0`, seconds `>= 0`). No back-fill: every existing row reads back NULL for the new columns, which is the meaningful "not measured" state — identical to how the original snapshot columns were introduced. Rollback drops the added columns only.
*Alternative considered:* two separate migrations (`038`, `039`) — rejected; the arc reserves one slot per change and both tables change for the same reason in the same sync, so one migration keeps the change atomic.

### D3 — Sleep-stage seconds come free from the already-fetched sleep DTO
`map_recovery` already digs `sleep.dailySleepDTO.sleepTimeSeconds` for `sleep_seconds`. The per-stage seconds live in the same DTO (`deepSleepSeconds`, `lightSleepSeconds`, `remSleepSeconds`, `awakeSleepSeconds`), so the four new recovery fields add **zero** Garmin calls — only `map_recovery` extractor lines + columns. SpO2 (`get_spo2_data`), respiration (`get_respiration_data`), endurance (`get_endurance_score`), hill (`get_hill_score`), and fitness age (`get_fitnessage_data`) each need a guarded per-day fetch added to `fetch_day`.

### D4 — `training_status` stored as a free-text label, validated only as a sane string
Garmin's `get_training_status` payload (already fetched for the load numbers) carries a `trainingStatus` / `latestTrainingStatusData[*].trainingStatus` phrase. We store it as TEXT verbatim (lower-cased phrase like "productive"), NOT as an enum, because Garmin's vocabulary drifts across firmware and we would rather store an unknown label than drop it. The service trims it and rejects only empty/oversized strings; it does not gate on a fixed set. It complements (does not replace) the numeric `acute_load`/`chronic_load`.
*Alternative considered:* a CHECK-constrained enum column — rejected; an unrecognised future status string would be silently dropped or 400 the whole snapshot.

### D5 — Floats rounded at the boundary; integers stored as-is
`spo2_avg`/`spo2_lowest`, `respiration_avg`/`respiration_lowest`, `endurance_score`, `hill_score`, `fitness_age` are the new numeric fields. Where stored as floats (HRV-style `NUMERIC`), `numfmt.Round1` rounds them at serialization, consistent with the rest of the snapshot shapes. Sleep-stage seconds are integers. `training_status` is text, untouched by rounding.

### D6 — Bridge fetches guarded per endpoint; absent detail simply omitted
Each of the five new `fetch_day` sub-fetches uses the existing `safe()` wrapper — a failing, throttled, or account-unavailable Garmin endpoint yields a missing key in the raw day, not an aborted sync. `map_recovery`/`map_fitness` read whatever is present and `_prune` drops the absent fields, so a day with no SpO2 watch simply omits `spo2_*`. The snapshot is still posted as long as it carries at least one metric beyond `date` (existing `_has_metrics` gate).

## Risks / Trade-offs

- **Garmin per-day endpoint churn** → the five new fetches are isolated behind `safe()`, exactly like the existing day fetches; a Garmin break degrades to "no SpO2/respiration/score this sync", not a failed sync.
- **Sleep-stage seconds may not sum to `sleep_seconds`** → by design we store raw, not derived (D2/Non-Goals); the agent treats minor mismatches as Garmin's own rounding, not a data bug.
- **`training_status` free-text drift** → accepted as TEXT (D4); an unknown phrase is stored, not dropped, and the agent can map vocabularies at read time.
- **Column creep on two snapshot tables** (8 recovery + 4 fitness = 12 new columns) → all nullable, no back-fill, no new index; "not measured" stays a meaningful NULL, consistent with the existing snapshot columns.

## Migration Plan

1. Verify the migration head on disk (`internal/store/migrations/`) is `037` from sibling A — or whatever the highest slot actually is — before scaffolding; an out-of-band slot collision has happened before. Then `task migrate:new NAME=extend_recovery_fitness` (→ `038`).
2. `038_extend_recovery_fitness.up.sql`: `ALTER TABLE recovery_metrics ADD COLUMN` the 8 recovery columns and `ALTER TABLE fitness_metrics ADD COLUMN` the 4 fitness columns, all NULL with CHECKs mirroring existing conventions.
3. `.down.sql`: drop the 4 fitness columns, then the 8 recovery columns.
4. Rollback is clean — additive columns only, no data transform; existing rows read back unchanged.

## Open Questions

- Exact SpO2 / respiration DTO paths in `get_spo2_data` / `get_respiration_data` (avg vs lowest keys) — to be confirmed against a recorded fixture during apply; if a key is absent the field stays NULL, no FTP-style lookup.
- Whether `fitness_age` arrives as an integer or a fractional year from `get_fitnessage_data` — store as `NUMERIC(4,1)` to be safe and round at the boundary; confirm during apply.
- Whether `training_status` is best read from `get_training_status.latestTrainingStatusData[*].trainingStatus` or a top-level `trainingStatus` — resolved at apply against the fixture; the spec only fixes the column, not the dig path.
