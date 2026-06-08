## Context

`/summary/daily` answers "today" and `/summary/range` answers "this exact interval." The whole-window-average shape — "trailing 7d as of today" — is the one nutrition science talks in (protein/MPS, EA, carb-load), and it's the shape both `weight_trend` (`add-weight-log`) and EA's `window.avg_ea` (`add-energy-availability`) already use. Bringing it to nutrition makes the trio coherent and removes a recurring client-side bug pattern (dividing by `to - from` rather than the count of days with data).

The endpoint is small. Most of the design weight sits in (a) the averaging-divisor decision, (b) calendar-day boundaries in TZ, and (c) goal/adherence resolution at the anchor.

## Goals / Non-Goals

**Goals:**

- One stateless read endpoint returning per-day rows + window averages over the trailing `[anchor − (window_days − 1), anchor]`.
- Honest divisor: averages use `days_with_data`, not `total_days`. Both fields are exposed.
- Loud "did-the-user-actually-log" signal per day via `has_data: bool`, separate from "logged zero on this day."
- Adherence vs the goal that applies *at the anchor* (honoring date-varying overrides) — same `under` / `on` / `over` / `no_data` semantics already used by `daily_summary`.
- Identical totals shape to `daily_summary` / `range_summary` per-day so downstream tooling that already parses those JSON shapes doesn't fork.

**Non-Goals:**

- `group_by=meal_type` (range_summary owns that).
- Per-nutrient window-length differentiation (3d sodium + 7d protein in one call).
- Hydration / weight rolling shapes (each capability owns its own roll-up; this endpoint is for nutrition).
- Caching, indexes, persistence of anything derived.
- Streak / consecutive-day-on-target metrics.
- Multi-window comparisons in one call (e.g. "this week vs last week" — two calls).

## Decisions

### 1. Averaging divisor = days_with_data (NOT total_days)

For a 7-day window with logs on 6 days totaling 12 600 kcal:

```
avg_kcal = 12_600 / 6 = 2_100   (days_with_data divisor — chosen)
avg_kcal = 12_600 / 7 = 1_800   (total_days divisor — rejected)
```

The 7-day divisor would imply the user "averaged" 1 800 kcal/day, suggesting a deficit larger than the data supports. The 6-day divisor reports what the user actually ate on the days they logged. The response exposes both divisors so the caller can spot sparse windows:

```json
"days_with_data": 6,
"total_days": 7
```

A response with `days_with_data: 2` out of `total_days: 7` is structurally fine but the agent should treat its averages as low-confidence and surface the sparsity explicitly to the user. The endpoint refuses neither case — it reports honestly and lets the caller reason.

**Alternatives considered:**

- *Total-days divisor.* Rejected per the example above.
- *Refuse when `days_with_data < 4` (or similar threshold).* Rejected — thresholds belong agent-side where the user's context matters; the API stays loud-and-honest.
- *Return both averages.* Rejected — two number sets invite mis-reading; pick the right one, expose the divisor.

### 2. Window semantics — anchor is IN the window, half-open at the early end

```
window = [ anchor − (window_days − 1) days , anchor ]   (both inclusive)
```

A 7-day window anchored at `2026-06-08` covers `2026-06-02` through `2026-06-08` — seven calendar days in the requested TZ. Anchor is end-of-window because "as of today, the trailing 7d" is the natural framing.

`days[]` is ordered ascending by date (matches `range_summary` ordering). Anchor is always the last entry.

**Alternatives considered:**

- *Anchor centred (3d before + 3d after).* Rejected — "what does my data say *now*" is the dominant question.
- *Half-open `[anchor − window_days, anchor)`.* Considered. The inclusive-both form is more intuitive ("7-day average ending today includes today"); off-by-one risk is local to this endpoint and tested explicitly.

### 3. Per-day calendar bucketing in requested TZ

Same rule as `daily_summary` / `range_summary` / EA: `meal_entries.logged_at.In(loc).Date()` defines the day. `tz` parameter defaults to `DEFAULT_USER_TZ`. A meal at `22:30Z` in `Europe/Berlin` (UTC+2) lands on the local-day *after* the UTC date.

Already-tested pattern; this endpoint reuses the day-bucket helper rather than duplicating it.

### 4. `has_data: bool` per day — distinct from "totals are zero"

```json
{ "date": "2026-06-05", "totals": { "kcal": 0, ... }, "has_data": false }
```

A user who logs zero meals on Sunday differs from a user who logs a meal whose kcal evaluates to zero (unusual but allowed). The agent might say "you missed Sunday" in one case and "you logged a salad with zero calories on Sunday" in the other. The Boolean carries that distinction.

Implementation: `has_data = bool(count of meals on the day > 0)`. Cheap, computed during the bucket pass.

### 5. Adherence is computed against the goal resolved AT the anchor

Adherence semantics already exist (`unify-adherence-shape`): for each configured nutrient, the response has `{actual, target, delta_pct?, status}`.

For this endpoint:
- `target` = the goal range at `anchor_date` (honoring `add-date-varying-goals` overrides for that exact date — not a window average of overrides).
- `actual` = the window-average value for that nutrient (already computed for the `averages` field).
- `status` follows `under` / `on` / `over` / `no_data` with the same threshold rules as `daily_summary`. `no_data` triggers when `days_with_data == 0` for the whole window.
- `goal_source` echo (`default` or `override`) carries the resolution at the anchor.

**Alternatives considered:**

- *Adherence against the mean of daily goals across the window* (i.e. if the window has 5 default days and 2 override days, average the targets). Rejected — overrides exist precisely to change *that day's* target; averaging them re-collapses the distinction. The anchor's goal is the one the user is "currently aimed at."
- *Per-day adherence array.* Rejected — that's what `range_summary` does. The rolling endpoint is a *window* tool.

### 6. `window_days` bounds: 2..30

```
window_days < 2   → 400 window_days_invalid
window_days > 30  → 400 window_days_invalid
```

- 1 would be `daily_summary` with extra ceremony.
- > 30 starts to lose the "trailing trend" character and overlap with `range_summary`. Three months is `range_summary` territory; the rolling endpoint stays in the "what's my actual recent pattern" zone.

The natural inputs from priorities.md are 3, 7, 14, 30 days — all covered.

**Alternatives considered:**

- *Cap at 92 days* (matches the other list endpoints). Rejected — the rolling endpoint has a different cost profile than a list, and a 90-day rolling window is not what athletes ask for ("am I trending healthy this week" vs "what did I eat across Q1").

### 7. Endpoint placement — `/summary/rolling`

Sits next to `/summary/daily` and `/summary/range`. Three coherent shapes covering "today / a specific interval / trailing-as-of-today." `/summary/hydration/rolling` and `/energy/availability/rolling` are NOT implied — those capabilities own their own rolling questions if they ever earn the surface.

**Alternatives considered:**

- *Extend `daily_summary` with `rolling_avg: 7d` param.* Rejected — `daily_summary` returns one day; conflating it with a window response shape is the kind of contract creep that hurts later. Separate endpoint, separate shape, both stay cheap to reason about.
- *Reuse `/summary/range` with a flag.* Rejected — `range_summary`'s per-day-row shape is appropriate when the caller wants per-day output; the rolling shape is appropriate when the caller wants the aggregate. Same data, different shape; different endpoints.

### 8. MCP tool description hammers the divisor rule

```
"Returns the trailing-window aggregate of nutrition totals as of `anchor_date`.
The window is `[anchor − (window_days − 1) days, anchor]`, both inclusive,
in the requested `tz` (defaults to DEFAULT_USER_TZ). IMPORTANT: averages
are computed across DAYS WITH DATA (logged meals), not total_days — a 7-day
window with 5 days logged returns the 5-day mean. The `days_with_data` and
`total_days` fields expose the divisor so callers can spot sparse windows.
Typical windows: 3d (acute), 7d (weekly trend), 14d (training-block trend),
30d (block-length trend)."
```

This rule trips client-side implementations more than any other; describing it in the tool surface itself reduces the chance the agent re-introduces the bug client-side after seeing the response.

### 9. No idempotency, no migration

Pure read. No schema. No middleware change.

## Risks / Trade-offs

- **The `days_with_data` divisor over-reports averages for users who log infrequently.** A user who logs 2 of 7 days will see a 2-day mean labelled as "the weekly average." → Mitigation: the response exposes both `days_with_data` and `total_days`, and the MCP tool description names the rule explicitly. Sparse windows are an agent-side concern (the agent should say "you logged 2 of 7 days, here's your average across those days").
- **Adherence against the anchor's goal can mis-report on weeks where overrides shifted mid-window.** A race-week build with overrides on Mon-Wed and defaults on Thu-Sun gets adherence vs the Sunday default. → Mitigation: documented; `goal_source` carries the resolution at the anchor; if it becomes a real problem, the follow-up is a `group_by=goal_source` mode (not a contract change to this endpoint).
- **Anchor-end window has a subtle DST handling.** A 7-day window across DST has either 167 or 169 hours, not 168. → Mitigation: calendar-day buckets in the requested TZ already handle this — DST is an hour shift, not a day shift; the day count stays at 7 either way. Tested explicitly with a DST-spanning anchor.
- **30-day cap is opinionated.** Some users might want 60d / 90d trends. → Mitigation: 30d is the cap because that's where the "recent trend" framing breaks down; beyond it, `range_summary` is the right tool. Re-evaluate if real use shows the cap is wrong.
- **No caching means every call re-runs the SQL aggregation.** Pool is local, query is straightforward, window is ≤ 30 days. → Negligible. Add caching only if usage data shows it matters.
- **Sparse rolling windows can encode privacy-sensitive patterns** (gaps in logging may reflect periods the user prefers not to share). → Single-user project, not a concern; flagged for future multi-user.

## Migration Plan

- Forward: no schema change. Deploy lands the endpoint behind the existing `BearerAuth`.
- Rollback: revert the binary.
- No feature flag — additive, read-only.

## Open Questions

- Whether to expose `min_days_with_data` as an optional query parameter the caller can use to opt into a hard refusal (`400 sparse_window` when `days_with_data < N`). Tentative: no for v1 — the agent does this kind of "should I trust this" check on its end; pushing it into the API forces every other caller to think about it. Reconsider if multiple callers want the same threshold.
- Whether the response should expose `count_of_meals` per day (not just `has_data`). Tentative: yes — cheap to compute, useful for "you logged one meal on Tuesday" framing. Decide during spec/impl.
- Whether the MCP tool should support `granularity: rolling` as an option on the existing `daily_summary` tool instead of a separate `rolling_summary` tool. Tentative: no — separate tool keeps descriptions and parameter schemas focused; the agent's selection between them is unambiguous.
- Whether to include the per-day adherence delta in the per-day rows (in addition to the window-level adherence). Tentative: no — that's `range_summary` territory; this endpoint is window-aggregate-first with per-day rows for transparency, not per-day analysis.
