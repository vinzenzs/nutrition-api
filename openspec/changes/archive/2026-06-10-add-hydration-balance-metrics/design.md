# add-hydration-balance-metrics — design

## Context

`recovery-metrics` and `fitness-metrics` (both archived 2026-06-10) established a date-keyed daily-snapshot pattern: a `date`-primary-key table, `POST … ON CONFLICT (date) DO UPDATE` full-replace upsert, `GET` list (92-day cap) / `GET /{date}` / `DELETE /{date}`, no PATCH, surfaced same-day-or-null in `daily-context`, mirrored 1:1 by an MCP tool group. This change is a third instance of that pattern for Garmin's daily hydration-balance signals. The work is mechanical; the only real decisions are which columns to store and how it relates to the existing `hydration` capability.

## Goals / Non-Goals

**Goals:**

- A storable home for Garmin's daily `sweatLossInML` / `activityIntakeInML` (and the daily `goalInML`).
- Idempotent push-by-date ergonomics; same-day-or-null surfacing in `daily-context`.
- Keep it cleanly separate from the per-entry `hydration` capability — no unit/grain overlap.

**Non-Goals:**

- Duplicating the daily intake total (`valueInML`), per-activity sweat loss, a computed balance field, PATCH, or the `garmin.py` push itself (all per proposal).

## Decisions

### 1. New capability, not an extension of `hydration`

`hydration` stores per-entry logged intake (many rows/day, ml + optional workout link) and a summary that sums those entries. Garmin's daily sweat/intake estimates are a different grain (one value/day) and a different source (device model output, not user logs). Folding them into the hydration summary would force the summary to mix "sum of entries" with "stored device estimate" — exactly the kind of shape-muddying the unit-isolation discipline avoids. A separate date-keyed snapshot, identical in shape to recovery/fitness, keeps each concept clean. (This was the option chosen over "extend hydration summary".)

### 2. Columns: the signals with no home, plus the daily goal

`sweat_loss_ml NUMERIC(10,1)` (`> 0`), `activity_intake_ml NUMERIC(10,1)` (`>= 0` — zero is meaningful: you did activity but drank nothing), `goal_ml NUMERIC(10,1)` (`> 0`). All nullable; NULL = not reported. Deliberately **no** `total_intake_ml` column — Garmin's `valueInML` is already pushed to `/hydration` as a daily entry, and the daily total-in is read from the existing hydration summary. The agent computes the balance (`sweat_loss_ml` vs total intake) at read time.

`activity_intake_ml` uses `>= 0` (not `> 0`) because "sweated, drank nothing during the session" is a real, meaningful zero; sweat loss and goal use `> 0` (a zero there means "not measured", better expressed as NULL).

### 3. Same date-keyed mechanics as recovery/fitness

`date DATE PRIMARY KEY`; `POST` upserts (201 insert / 200 update, full-replace of metric columns); `GET ?from=&to=` inclusive window, 92-day cap, ordered by date asc; `GET /{date}` and `DELETE /{date}` with `hydration_balance_not_found`. `date` validated as `YYYY-MM-DD` → `date_invalid`. Floats rounded at the response boundary via `numfmt.Round1Ptr`. Date carried as a string end-to-end (`to_char` on read, `$1::date` on write), exactly as recovery/fitness do.

### 4. daily-context block is same-day-or-null, alongside (not merged into) `hydration`

`GET /context/daily` gains `hydration_balance` = the snapshot for the requested date or `null` (no carryover — a stale sweat estimate misleads). It sits beside the existing `hydration` block; the two are never merged. Pure read composition over the new repo's `GetByDate`.

## Risks / Trade-offs

- **[Two hydration-ish blocks in daily-context could confuse]** → Mitigated by distinct names + the field shapes: `hydration` = `{total_ml, entries_count}` (logged intake); `hydration_balance` = `{date, sweat_loss_ml, activity_intake_ml, goal_ml}` (Garmin daily estimate). Documented.
- **[Thin capability — only 3 fields]** → Accepted: the pattern is cheap to instantiate, consistency with recovery/fitness outweighs the small surface, and the data is genuinely homeless today.
- **[MCP expected-tools list +4]** → Must bump the integration test (this adds tools, unlike a fields-only change).

## Migration Plan

1. Verify next free slot (expected `024`; head is `023`).
2. `024` `CREATE TABLE hydration_balance_metrics` (`date` PK, `sweat_loss_ml`/`activity_intake_ml`/`goal_ml` nullable with CHECKs, `created_at`/`updated_at`).
3. Down: `DROP TABLE`. No back-fill; no data transform.

## Open Questions

- None. (Whether to also store Garmin's daily `valueInML`/`goalInML` redundantly was resolved: goal yes — no other home; total no — already in `/hydration`.)
