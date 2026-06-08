## Context

The API records what goes into the body (meals, hydration, supplements when those land) and — after `add-workouts-capability` — what the body did (workouts, kcal burn). It records nothing about what the body *is*: weight, composition, the slow-moving signals that determine whether a deficit is sustainable or a fuelling plan is calibrated. The Garmin coach reads body weight from Garmin Connect, but any measurement taken outside that loop (travel, hotel scale, in-laws', smart scale that doesn't sync to Garmin) is invisible to the API.

The shape this change targets is the simplest one that makes weight a first-class primitive: a measurement event, indexed by time, with a rolling-average analytic on top. The same shape pattern as hydration — capture an event with a few fields, expose CRUD, expose one aggregation. Where hydration's aggregation is a daily *total*, weight's aggregation is a rolling *trend*, because that's the question athletes actually ask.

This change deliberately stops at "log + trend." Goal weight, projections, body-composition fields (lean mass etc.), Garmin smart-scale ingestion — all sit one logical step above and ship as follow-ups when use cases demand them.

## Goals / Non-Goals

**Goals:**

- Capture: log a body-weight measurement (optionally with body-fat %) at a specific time, regardless of source.
- Retrieve: list within a window, fetch a single entry, edit, delete — the same hygiene as meals and hydration.
- Smooth: a rolling-average endpoint that suppresses daily noise (1–2 kg swings from hydration, glycogen, food in gut) — the signal athletes actually care about.
- Honest about gaps: the trend endpoint surfaces per-day `sample_count` so callers can tell a real trend from a "we had data for 2 of 7 days" mirage.
- Cheap: one table, one index, no FKs, no cross-capability changes.

**Non-Goals:**

- Goal weight, projected date, deficit calculator. These are second-order math over the data this change captures.
- Body-composition columns beyond body-fat % (lean mass, hydration %, bone mass). Smart-scale data isn't flowing yet; add columns when a tool actually reads them.
- Source / external_id tagging (Garmin smart-scale sync). Weight's primary write path today is manual; the workouts pattern can be retrofitted if needed.
- Bulk endpoint. Year-of-history is ~365 entries — N POSTs is fine.
- A daily-summary-style endpoint (`/summary/weight/daily`). Days have 0 or 1 entries; the list and trend endpoints cover the read shape better.
- Exponential moving average / weighted smoothing algorithms. SMA over a configurable window is the standard "kill daily noise" fix; sophisticated smoothing is YAGNI.
- Cross-capability integration (e.g. auto-flowing weight into `plan_carb_load`). The agent passes the current trend value as a parameter; auto-inject is a footgun.

## Decisions

### 1. One event per measurement, multiple per day allowed

Each row is one "I stepped on the scale at time T and it read W" event. Multiple measurements per day are allowed and aggregated by the trend endpoint.

```
body_weight_entries
  id              UUID PRIMARY KEY
  logged_at       TIMESTAMPTZ NOT NULL
  weight_kg       NUMERIC(5, 2) NOT NULL CHECK (weight_kg > 0)
  body_fat_pct    NUMERIC(4, 2) NULL CHECK (body_fat_pct IS NULL OR (body_fat_pct >= 0 AND body_fat_pct <= 100))
  note            TEXT NULL
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()

INDEX body_weight_entries_logged_at_idx ON (logged_at)
```

Notably absent: any composition column beyond body-fat %, any source / external_id, any per-day uniqueness constraint. Each is a deliberate omission.

**Alternatives considered:**

- *One row per day with `UNIQUE(date)`.* Rejected — humans don't always weigh once; on a day with two measurements the API would either reject the second (annoying) or overwrite (lossy). One-event-per-row is the cleanest model and matches every other event table in this project.
- *Combined "morning metrics" table with weight + sleep + HRV columns.* Considered, since these tend to be measured together each morning. Rejected for v1: the data sources are different (sleep + HRV come from Garmin; weight comes from a scale or manual input), the shapes are different (sleep has stages, HRV has a single ms number, weight has the body-fat % sibling), and pre-bundling forces every weight entry to also have a sleep column even when the data isn't available. If real use shows the bundling helps, sleep/HRV land as siblings (separate tables) and a `daily_context()` aggregator (priorities Tier 2) composes them at read time.
- *Smart-scale composition columns (lean mass, water %, bone mass).* Rejected for v1 — no tool reads them yet. NUMERIC nullables are cheap to add but the discipline of "only store what a tool needs" matters more than future-proofing.

### 2. Rolling-average trend, not raw daily averages

The trend endpoint returns, for each calendar date in the window, the average weight over the trailing `window_days` days:

```
GET /weight/trend?from=2026-05-01&to=2026-06-07&window_days=7

Response:
{
  "from": "2026-05-01",
  "to": "2026-06-07",
  "window_days": 7,
  "points": [
    {"date": "2026-05-01", "rolling_avg_kg": 73.4, "sample_count": 5},
    {"date": "2026-05-02", "rolling_avg_kg": 73.3, "sample_count": 6},
    ...
    {"date": "2026-06-05", "rolling_avg_kg": null, "sample_count": 0},   -- no data in trailing 7d
    {"date": "2026-06-06", "rolling_avg_kg": 71.2, "sample_count": 1},   -- single sample
    {"date": "2026-06-07", "rolling_avg_kg": 71.3, "sample_count": 2}
  ]
}
```

Algorithm per date `d`:
- Find every `body_weight_entries.weight_kg` where `logged_at >= d - window_days + 1` (start of that day) and `logged_at <= d` (end of that day), in the user's local timezone (uses `DEFAULT_USER_TZ` or the `tz` query param).
- `rolling_avg_kg = mean(those weights)`, rounded to 1 dp via `numfmt.Round1`.
- `sample_count = len(those weights)`.
- If `sample_count == 0`, `rolling_avg_kg = null`.

**Why expose `sample_count`:** a 7-day rolling average computed from 1 sample is qualitatively different from one computed from 7. Hiding that means callers (and the agent) confidently read a "trend" that's actually noise. This is the project's general data-quality-signaling principle made concrete; the cost is one int per point.

**Why simple moving average over EWMA:** SMA is easy to explain ("average of the last 7 readings"), easy to verify by eye, and addresses the specific problem (1–2 kg daily noise) adequately. Exponential weighting is a real second-order improvement; defer until SMA proves inadequate for some specific decision.

**Alternatives considered:**

- *Return a single trend value at `to` instead of a per-date series.* Rejected — the series is the more general shape; a single value is `points[-1]` for callers that only want today.
- *Return raw daily means (no rolling).* Rejected — that's what the existing list endpoint plus client-side `GROUP BY date` already gives you. Trend's whole reason to exist is the noise suppression.
- *Use median instead of mean to reduce outlier sensitivity.* Considered. Mean of 7 daily weights is robust enough in practice — one freak reading shifts a 7-day mean by ~14% of itself, well within physiological noise. Median has rougher behaviour on small samples (single-sample windows have median = that sample, not different from mean). Mean is simpler.

### 3. Window-days parameter, with bounds

`window_days` is optional, defaults to 7, bounded `[1, 30]`:

- `1` is degenerate (no smoothing) but allowed — useful for raw-daily charts via the trend endpoint without falling back to list+aggregation client-side.
- `30` is the upper end of realistically-useful weight smoothing — beyond that you're looking at slower signals than weight noise warrants.
- Out of range → `400 window_days_invalid` with the documented range.

The `from`/`to` range itself caps at 366 days (matches `add-date-varying-goals`'s `list_daily_goal_overrides`), since "trend over a year" is a reasonable analytics question for slow weight changes.

**Alternatives considered:**

- *Hard-code window_days = 7.* Rejected — different analyses want different smoothing (training-block trend = 14 or 21 day; daily noise check = 3 day). The parameter is cheap.
- *Allow unbounded windows.* Rejected — 365-day windows would compute a "rolling year" that's not actually useful and burns DB time. Bounds catch misuse early.

### 4. Body-fat % is nullable, single column, no separate trend in v1

`body_fat_pct` rides along on the same row because measurements typically come together (a smart scale reports both). It's nullable because manual scales don't measure it.

The trend endpoint computes `weight_kg` only in v1. Body-fat trend is the same algorithm against a different column; ships as a `metric=body_fat_pct` query param (or a sibling endpoint) when the first tool needs it. No reason to build the second-metric machinery before it's used.

**Alternatives considered:**

- *Separate `body_composition_entries` table with weight + body fat + lean mass + etc.* Rejected for v1 — adds JOIN complexity and table count for a single nullable column today. If real composition data starts flowing, additive columns on the existing table are cheaper than restructuring.
- *Trend endpoint accepts `metric=weight_kg|body_fat_pct` from day 1.* Rejected as premature surface area — easy to add when needed without breaking the contract.

### 5. No source/external_id; standard Idempotency-Key on POST

Weight's primary write path is manual (REST or agent). Garmin smart-scale data could flow in the future via `garmin.py`, but the user's stated framing is "if Garmin isn't synced or you weigh elsewhere, no record" — the manual path is what's missing. Building Garmin-shape upsert semantics now would be speculative.

`POST /weight` accepts the standard `Idempotency-Key` header. Two writes with the same key and body return the same row (existing middleware). The agent's auto-derived keys mean retries don't double-log.

When (if) Garmin smart-scale ingestion becomes real, the workouts pattern is a known-good template: add `source` + `external_id`, add UPSERT-on-conflict, document. Cheap follow-up.

**Alternatives considered:**

- *Mirror the workouts table shape (source + external_id + UPSERT) from day 1.* Rejected as forward-compatibility for a non-existent writer. The same forward-compatibility argument was made for workouts and turned out right *because Garmin was already the activity source*. For weight, Garmin isn't currently writing anything to the API.

### 6. Multiple-entry-per-day semantics are the trend's responsibility

The list endpoint returns rows as logged. The trend endpoint averages all entries that fall within a date's trailing window. This means:

- Two readings on the same day → both included in that day's average → smooths morning vs evening noise within the day.
- A late-evening reading bleeds into the next day's trailing window → that's correct; it's still a sample of that recent body state.
- "What was I at on a specific date?" is best answered via the trend endpoint with `window_days=1` (returns the per-date mean) or by listing the day and reading what's there.

**Alternatives considered:**

- *Materialize a `daily_weight` view that picks one canonical entry per day (e.g. the first).* Rejected — opinionated; loses information; another denormalization to maintain.

### 7. MCP tool descriptions stay terse

The five tools have no business-logic surprises beyond what other capture surfaces (meals, hydration, workouts) already established. Descriptions stick to the contract:

- `log_weight`: "Record a body-weight measurement, optionally with body-fat %. Multiple measurements per day are fine — the trend tool smooths them. Use the `note` field for context that affects readings (post-workout, post-meal, time of day if not morning)."
- `weight_trend`: "Return a rolling-average weight curve for a date range. `window_days` defaults to 7 (suppresses normal daily noise). Each point carries `sample_count` so you can tell a real trend from a sparse one — a `rolling_avg_kg` from `sample_count: 1` is not a trend."
- `list_weights` / `patch_weight` / `delete_weight`: standard CRUD.

The descriptions explicitly do NOT recommend a default time-of-day for weighing (that's coaching territory) or interpret what a trend means (also coaching).

## Risks / Trade-offs

- **No Garmin sync today.** A user whose only weight data lives in Garmin gets nothing from this change unless they also log manually or build `garmin.py` to push to the new endpoint. Mitigation: the manual + agent paths cover the primary "I'm somewhere a scale isn't talking to Garmin" use case; Garmin-push is a known small follow-up.
- **Rolling-average sparse windows can mislead.** A 7-day window with 1 sample reports a `rolling_avg_kg` that's just that sample. Mitigation: `sample_count` is in every point; callers and the agent are responsible for interpreting it; the MCP description calls this out explicitly.
- **`body_fat_pct` precision.** NUMERIC(4,2) stores 0.01% steps; smart scales typically report 0.1%. Storage is fine. The trend endpoint doesn't smooth body fat in v1, so single-day BF % readings are visible to clients via the list endpoint with whatever noise the scale produces.
- **No deduplication on identical bodies.** Two identical POSTs (same weight, same logged_at, same body_fat_pct, no idempotency key) create two rows. The trend would then count that one moment twice. Mitigation: agent uses idempotency keys (standard pattern); manual users won't typically log the same instant twice; PATCH/DELETE handle the cleanup case. Adding a `UNIQUE(logged_at, weight_kg)` constraint would over-fit (legitimate "two readings 30 seconds apart" would be rejected).
- **Timezone of the trend window.** Day boundaries are in the configured `DEFAULT_USER_TZ` (or `tz` query param, matching summary endpoints). A late-evening reading in UTC may roll into the next "day" depending on TZ, which is the right behaviour — the user's local "today" is what matters for weight context.
- **No goal weight here.** A user who wants "kg to goal" today has to compute it themselves. Mitigation: explicit non-goal; lands cleanly as a separate small change (`add-weight-goals`) when needed.

## Migration Plan

- Forward migration creates `body_weight_entries` + the `(logged_at)` index. No backfill.
- Rollback drops the table. No data outside the new table changes.
- Migration is numbered `013_add_body_weight` since `add-workouts-capability` claimed `012`. If applied before that change for any reason, the apply step renumbers (pattern previously used for `add-hydration-tracking` ↔ `add-date-varying-goals`).

## Open Questions

- Whether `body_fat_pct` should also accept "fat mass kg" as a sibling field. Tentative answer: no — percentage is what scales report; kg can be derived if needed (`weight_kg × body_fat_pct / 100`).
- Whether the trend endpoint should accept a `tz` query param like the summary endpoints. Tentative answer: yes, for consistency. Defaults to `DEFAULT_USER_TZ` with the usual warn-on-fallback log.
- Whether to expose a "current trend" convenience tool (just the latest `points[-1]` for clients that don't want a series). Tentative answer: no — `weight_trend(from=today-7d, to=today)` is one extra call; not worth a special tool.
- Whether `log_weight` should record a `weighed_in_unit: "lb"` and convert. Tentative answer: no — the API is kg-only consistent with grams-only for meals; clients convert at input. Document.
