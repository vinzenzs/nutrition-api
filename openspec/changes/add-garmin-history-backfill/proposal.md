## Why

The garmin-bridge runs a daily-sync CronJob over a *rolling* `backfillDays` window (`deploy/helm/nutrition-api/templates/garmin-bridge.yaml` — `cron.schedule`, `cron.backfillDays`, the per-day `POST /sync` loop). That window only ever re-touches the last N days, so any activity OLDER than it never gains the new detail the rest of the arc adds — workout splits/zones/sets (change B), the daily energy/recovery snapshots (A), recovery/fitness extensions (C), gear/PRs (D). The user is mid-season (18-week plan, race **2026-07-24**): the early-block sessions that anchor the season are already outside the rolling window and stay detail-less forever.

This is change **F** of the "mirror everything" Garmin arc — the **history backfill** slice. It adds a *one-shot, bounded, idempotent* full-range re-sync that replays the (now-enriched) per-day sync over an arbitrary historical date range, so a single trigger backfills whatever detail B/A/C/D added with **zero per-field work here** — it reuses the exact same `sync_day` mapping path. Siblings (out of scope here): A `add-garmin-daily-energy`, B `add-garmin-workout-detail`, C `extend-recovery-fitness`, D `add-garmin-gear-and-prs`, E `garmin-workout-library-mgmt`.

**Sequencing:** this change is mechanically independent (it just replays sync over a range), but it is only *fully useful* after B/A/C/D land — it backfills their detail. Recommend applying it **last** among the data changes.

## What Changes

- **garmin-bridge** gains `POST /sync/backfill` accepting `from`/`to` (`YYYY-MM-DD`, inclusive). It iterates day-by-day from `from` to `to` calling the **existing `sync_day` path** per day, so it AUTOMATICALLY picks up whatever enrichment the sibling changes added — no per-field mapping work in this change. Each day is independent: a single bad day records its error and the range continues. The response is a per-day summary (`{date, ok, results|error}`) plus a roll-up (`days_total`, `days_ok`, `days_failed`).
- **Idempotency by construction.** Re-running over the same range is safe: snapshots upsert by date and activities dedup by `external_id = "garmin:<activity_id>"` via the existing `/workouts` UPSERT (and B's replace-on-resync for nested splits/sets). No new dedup mechanism, no migration.
- **Rate-limit pacing — the design substance.** A multi-month backfill multiplies the arc's per-day Garmin-call budget (B ≈ 3 calls/activity, C ≈ 5/day, plus A/D) into potentially thousands of calls. The bridge therefore: (1) **sleeps a configurable delay between days** (`BACKFILL_DAY_DELAY_SECONDS`, default a few seconds) to stay friendly to Garmin; (2) **caps the range** (`BACKFILL_MAX_DAYS`, default ~120) so a typo can't launch a year-long crawl; (3) iterates **oldest→newest** so a partial/interrupted run leaves a contiguous backfilled prefix and is **resumable** by re-issuing from the first failed/unreached date (re-runs are free thanks to idempotency).
- **garmin-control** gains `POST /garmin/backfill` proxying the bridge verbatim (status + body), `503 garmin_disabled` when `GARMIN_BRIDGE_URL` is unset, auth required — same posture as the existing `/garmin/login`, `/garmin/calendar` proxies.
- **MCP** gains one tool `garmin_backfill` (one HTTP call to `POST /garmin/backfill`, idempotency key auto-derived per the existing `tools_garmin.go` write pattern, body forwarded verbatim via `toToolResult`). The `mcp_integration_test` expected-tools list grows by **+1**.

## Capabilities

### New Capabilities
<!-- None — backfill is a new endpoint on the existing garmin-bridge / garmin-control / mcp-server capabilities, not a standalone capability. -->

### Modified Capabilities
- `garmin-bridge`: new `POST /sync/backfill` requirement — bounded, paced, per-day-tolerant range replay over the existing `sync_day` path, idempotent by construction.
- `garmin-control`: new `POST /garmin/backfill` proxy requirement — verbatim forward, `503 garmin_disabled` when the bridge URL is unset, auth required.
- `mcp-server`: new `garmin_backfill` tool requirement — one HTTP call, auto-derived idempotency key, verbatim pass-through.

## Impact

- **Schema**: **no migration.** The backfill reuses the existing tables and upsert/UPSERT paths (date-keyed snapshots, `external_id` workout dedup, B's child-row replace) — it writes nothing the daily sync doesn't already write. (If one were ever needed it would take the next free slot after F — but this change deliberately introduces none.)
- **Code**: `apps/garmin-bridge/garmin_bridge/{app.py,sync.py,config.py}` (new route + range loop + pacing config); `internal/garmincontrol/{handlers.go,scheduling.go-style proxy}` (new proxy handler + route + swag); `internal/mcpserver/tools_garmin.go` (new tool + registration) and `mcp_integration_test.go` (+1 expected tool). No `internal/workouts` or other capability package changes — the enrichment mapping is owned by B/A/C/D.
- **Deploy**: the Helm CronJob's existing rolling-window loop is left as-is for steady-state; backfill is an explicit operator/agent-triggered action, not a scheduled one. New bridge env (`BACKFILL_DAY_DELAY_SECONDS`, `BACKFILL_MAX_DAYS`) surfaced through `values.yaml` with safe defaults.
- **Docs/tests**: `task swag` after the new garmin-control handler; bridge `pytest` for the range loop (happy path, one bad day continues, range-cap rejection, idempotent re-run); garmin-control proxy test (forward + `503` disabled); MCP integration expected-tools bump.
- **Conventions honored**: MCP mirrors REST 1:1 (one HTTP call, verbatim body), idempotent re-runs are success, `503 garmin_disabled` when the bridge is off, no merging of units (backfill posts nothing new — it replays existing shapes).
