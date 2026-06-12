## Context

The bridge already has everything needed to import one day: `POST /sync` reads the stored token, fetches the day via `gc.fetch_day`, and hands the raw payload to `sync.sync_day`, which maps and POSTs each capability tolerantly (one failing capability never aborts the day). The Helm CronJob bootstraps history with a *rolling* window — a shell loop that issues `POST /sync` for each of the last `backfillDays` days (`garmin-bridge.yaml`, lines ~150–168). That loop is fine for steady-state but structurally cannot reach activities older than its window, and the sibling enrichment changes (B/A/C/D) add detail only on forward sync or explicit re-sync.

What's missing is an *explicit, bounded* way to replay sync over an arbitrary past range — so a mid-season athlete can backfill the whole training block's new detail in one trigger. Because every day flows through the unchanged `sync_day`, the backfill inherits all enrichment automatically: this change adds the *driver*, not the mapping.

Constraints that shape the design:
- **The bridge is stateless except during the login window** — backfill must not introduce durable local state; resumability is achieved by idempotent re-runs, not a job table.
- **garmin-bridge tolerates per-capability failure** — extend that to *per-day* failure: one bad day must not abort the range.
- **MCP mirrors REST 1:1** — one HTTP call per tool, body forwarded verbatim, `503 garmin_disabled` when the bridge is off.
- **Idempotent re-runs are success** — date-keyed upserts + `external_id` workout dedup + B's child replace make replay free.
- **Garmin call budget** — the arc's per-day cost (B ≈ 3/activity, C ≈ 5/day, plus A/D) times a multi-month range is the real risk; pacing and a cap are mandatory.

## Goals / Non-Goals

**Goals:**
- A single bounded trigger that re-syncs an arbitrary `[from, to]` historical range through the existing `sync_day` path, inheriting B/A/C/D enrichment with no per-field work.
- Per-day failure tolerance with a per-day + roll-up summary.
- Stay friendly to Garmin under a large range: inter-day pacing, a hard range cap, and resumability via idempotent re-run.
- Expose the trigger through garmin-control and one MCP tool, matching the existing proxy/tool posture exactly.

**Non-Goals:**
- **No new enrichment mapping.** The fields backfilled are whatever B/A/C/D already emit; this change touches no `map_*` logic beyond calling the existing path.
- **No durable job/queue/progress store.** No background worker, no job-id polling. The call is synchronous (paced) and resumable by re-issuing the range.
- **No migration.** Reuses existing tables and upsert/UPSERT/child-replace paths (see Migration Plan).
- **No change to the steady-state daily CronJob.** Its rolling window stays; backfill is an explicit, operator/agent-triggered one-shot.
- **No parallel day fetches.** Sequential by design — pacing and a contiguous-prefix resume story both depend on ordered, throttled iteration.

## Decisions

### D1 — Backfill is a thin range-loop over the unchanged `sync_day`
`POST /sync/backfill {from, to}` parses the inclusive date range, then for each date calls the *same* token-load + `gc.fetch_day` + `sync.sync_day` sequence `do_sync` uses today. No mapping is duplicated or special-cased. This is the whole point: enrichment added by siblings rides along for free because the per-day path is identical to the daily sync.
*Alternative considered:* a dedicated "bulk fetch + bulk post" path that batches activities across days — rejected; it would re-implement (and drift from) the per-day mapping and lose the per-day tolerance/summary that mirrors the existing sync contract.

### D2 — Per-day isolation, per-day + roll-up summary
Each day is wrapped so its failure (token refresh hiccup, a Garmin 500, a backend 502) is caught, recorded as `{date, ok:false, error}`, and the loop proceeds. Successful days carry the day's existing `sync_day` summary verbatim (`{date, ok, results, errors}`). The top-level response adds a roll-up: `days_total`, `days_ok`, `days_failed`, and a `days[]` array. HTTP status is `200` when all days are ok, `207` when some days failed but the range completed, mirroring the single-sync `200/207` convention.

### D3 — Inter-day pacing via `BACKFILL_DAY_DELAY_SECONDS`
Between consecutive day-syncs the bridge sleeps a configurable delay (default a few seconds, e.g. `3`). This is the primary throttle: it spreads the arc's amplified call volume (B's per-activity fan-out, C's per-day fetches) so a multi-month backfill trickles rather than bursts against Garmin's undocumented rate limits. The delay is *between* days, not within a day, so a single day's existing fan-out is unchanged. `0` disables the sleep (useful for tests). The sync handlers are plain `def` run in FastAPI's threadpool, so a blocking `time.sleep` between days does not stall the event loop.

### D4 — Hard range cap via `BACKFILL_MAX_DAYS`
If `(to − from) + 1 > BACKFILL_MAX_DAYS` (default ~120, comfortably covers an 18-week block) the bridge rejects the request up front with `400 range_too_large` and the cap in the body, writing nothing. This bounds both the worst-case Garmin spend and the worst-case synchronous request duration, and turns a fat-fingered `from=2010-01-01` into an instant, harmless error instead of a multi-thousand-call crawl.

### D5 — Oldest→newest iteration for contiguous-prefix resumability
The loop runs `from` forward to `to`. With idempotent re-runs, an interrupted or partially-failed backfill is resumed simply by re-issuing the range (or a sub-range starting at the first failed date) — already-synced days re-upsert to the same rows at low marginal cost. No job table, no progress cursor: the *data* is the cursor. Oldest-first means a timeout/interruption leaves a contiguous backfilled prefix (the oldest, season-anchoring sessions land first), which is the more useful partial outcome for a mid-season athlete.
*Alternative considered:* newest→oldest (so recent days land first) — rejected; the rolling daily window already covers recent days, so the *value* of backfill is in the older tail, which oldest-first reaches even on a truncated run.

### D6 — Idempotency: reuse existing dedup, derive the MCP key from the range
No new dedup primitive. The backfill writes exactly what daily sync writes, so date-keyed snapshot upserts, the `/workouts` `external_id` UPSERT, and B's child-row replace-on-resync already make replay safe. On the MCP side, `garmin_backfill` is a POST-style write: it auto-derives `Idempotency-Key = sha256_hex("garmin_backfill|" + canonical_json(args_without_key))` per the existing `effectiveIdempotencyKey` pattern, so a retried identical trigger carries a stable key. Note the dedup that actually protects the data is the per-write upsert, not this header — the header just satisfies the write-tool convention and lets the backend short-circuit an immediate duplicate trigger.

### D7 — garmin-control proxy mirrors the existing forward pattern
`POST /garmin/backfill` reads the JSON body (`{from, to}`), forwards it to the bridge's `/sync/backfill` verbatim, and copies status + body back — structurally identical to `loginMFA`'s `forward(c, "/login/mfa", body)` and the `calendar` GET proxy. It adds no fields, parses nothing, returns `503 garmin_disabled` when `GARMIN_BRIDGE_URL` is unset, and requires auth. The proxy's `forwardTimeout` (currently 30s) must accommodate a paced backfill: because the call is synchronous and paced, a long range can exceed 30s. The proxy therefore uses a **longer, backfill-specific timeout** (or the bridge returns promptly and the cap keeps duration bounded) — see Open Questions.

## Risks / Trade-offs

- **Synchronous long-running request.** A capped, paced backfill can still take minutes (e.g. 120 days × a few seconds + per-day fetch latency). Mitigation: `BACKFILL_MAX_DAYS` bounds it; the proxy/MCP timeouts are widened for this path; if it proves too long in practice, a future change can move to a fire-and-forget job (explicitly out of scope now to avoid introducing durable state). The cap is the primary guard.
- **Garmin rate-limit / throttling under volume.** Even paced, a large backfill is the heaviest Garmin consumer in the arc. Mitigation: the inter-day delay, the per-day fan-out already guarded by B's `safe()` wrapper (a throttled endpoint degrades to absent detail, not a failed day), and the operator's ability to run smaller sub-ranges.
- **Partial completion semantics.** `207` with a `days[]` summary means "some days failed" — the caller must inspect, not assume success on non-`200`. Documented in the spec scenarios and mirrored in the MCP tool description. Because re-runs are free, the remedy is always "re-issue the failed dates."
- **Timeout vs. cap tension.** Set the cap low enough that a worst-case paced run fits the widened timeout; if they diverge, the cap wins (reject early) over a silent mid-stream cutoff.
- **No progress visibility during the run.** Synchronous + no job store means the caller waits with no incremental feedback. Acceptable for a single-user, occasional operation; the roll-up summary on completion is the feedback.

## Migration Plan

**No migration.** Confirmed: the backfill issues the same writes as the daily sync —
- date-keyed snapshots (`/recovery-metrics`, `/fitness-metrics`, `/hydration-balance`) upsert by date,
- weigh-ins create idempotently,
- activities go through `/workouts/bulk` and dedup on `external_id = "garmin:<activity_id>"` (plus B's nested-child replace-on-resync).

None of these introduce a new table, column, or constraint. The current migration head is around `040` (after F's siblings); were a migration ever required it would take the next free slot (`041`), but this change deliberately introduces none — state that explicitly in review. The only new persisted-config surfaces are bridge **environment variables** (`BACKFILL_DAY_DELAY_SECONDS`, `BACKFILL_MAX_DAYS`), wired through Helm `values.yaml`, not the database.

## Open Questions

- **Exact `BACKFILL_MAX_DAYS` default** — leaning ~120 (covers an 18-week block in one shot). If the synchronous-request duration at that cap exceeds a comfortable proxy timeout, lower the default or split the recommended workflow into month-sized sub-ranges. **Resolved in spec as a documented default with the cap enforced; tune at apply.**
- **Proxy/MCP timeout for the backfill path** — the existing 30s `forwardTimeout` is too short for a paced multi-day run. Options: (a) a dedicated longer timeout on the `/garmin/backfill` proxy and a matching MCP per-request timeout note, or (b) keep the cap low enough that the worst case fits 30s. Leaning (a) with the cap as a secondary guard. **To confirm at apply.**
- **`BACKFILL_DAY_DELAY_SECONDS` default** — a few seconds (e.g. `3`) balances friendliness against total runtime; `0` for tests. Exact value tunable via env without a code change.
