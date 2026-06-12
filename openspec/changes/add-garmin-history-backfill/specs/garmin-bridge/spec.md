# garmin-bridge Specification (delta)

## ADDED Requirements

### Requirement: Bounded, paced, idempotent history backfill over a date range

The bridge SHALL expose `POST /sync/backfill` accepting an inclusive `from` and `to` date (`YYYY-MM-DD`) and SHALL replay the existing per-day sync over every date in `[from, to]`, calling the same token-load + day-fetch + `sync_day` path used by `POST /sync` so that any per-activity or per-day enrichment added elsewhere is picked up with no backfill-specific mapping. The range SHALL be capped: when the requested span exceeds `BACKFILL_MAX_DAYS`, the bridge SHALL reject the request with `400 range_too_large` (including the cap) and write nothing. Days SHALL be processed oldest-first, with a configurable `BACKFILL_DAY_DELAY_SECONDS` pause between consecutive days to pace Garmin calls. Each day SHALL be isolated: a failing day SHALL be recorded and the range SHALL continue. The response SHALL carry a per-day summary plus a roll-up (`days_total`, `days_ok`, `days_failed`), returning `200` when every day succeeded and `207` when the range completed with one or more failed days. Re-running the same range SHALL be idempotent via the existing date-keyed upserts and `external_id` activity dedup — no new dedup mechanism and no migration.

#### Scenario: Backfilling a range replays each day through the existing sync path

- **WHEN** `POST /sync/backfill` is called with `{"from":"2026-03-01","to":"2026-03-05"}` and a valid stored token
- **THEN** the bridge syncs each date from `2026-03-01` through `2026-03-05` inclusive using the same per-day mapping as `POST /sync`
- **AND** whatever enrichment the per-day path produces (scalar/zone/split/set detail, daily snapshots, recovery/fitness extensions) is written for each day with no backfill-specific field handling
- **AND** the response lists one entry per date plus `days_total`, `days_ok`, and `days_failed`

#### Scenario: One bad day does not abort the range

- **WHEN** a backfill spans several days and the Garmin fetch or a backend write fails for exactly one of them
- **THEN** that day's entry records its error (`{date, ok:false, error}`) and the remaining days still sync
- **AND** the roll-up reports `days_failed` ≥ 1 and the HTTP status is `207`
- **AND** re-issuing the backfill for the failed date alone re-attempts only that day

#### Scenario: Pacing inserts a delay between days

- **WHEN** a multi-day backfill runs with `BACKFILL_DAY_DELAY_SECONDS` set to a positive value
- **THEN** the bridge pauses that many seconds between consecutive day-syncs to spread Garmin calls
- **AND** a value of `0` disables the pause (no sleep between days)

#### Scenario: An over-large range is rejected before any sync

- **WHEN** `POST /sync/backfill` is called with a span larger than `BACKFILL_MAX_DAYS`
- **THEN** the bridge responds `400 range_too_large` including the configured cap
- **AND** no day is synced and nothing is written

#### Scenario: Re-running a backfill range is idempotent

- **WHEN** `POST /sync/backfill` is run twice over the same range
- **THEN** date-keyed snapshots are upserted (not duplicated) and activities are deduped by `external_id = "garmin:<activity_id>"` via the existing `/workouts` UPSERT
- **AND** any nested per-activity detail is replaced (not duplicated) on the second run
- **AND** the second run requires no new field and no migration

#### Scenario: Backfill with no stored token fails clearly

- **WHEN** `POST /sync/backfill` runs and the backend has no stored token
- **THEN** the bridge returns an error indicating a login is required
- **AND** writes nothing
