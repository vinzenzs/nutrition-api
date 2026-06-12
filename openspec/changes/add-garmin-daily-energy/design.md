## Context

`energy-availability` is a pure-computation endpoint: `EA = (intake_kcal − exercise_energy_kcal) / FFM_kg`, where `exercise_energy_kcal` is `Σ workouts.kcal_burned` for the day. That denominator-numerator pair is the published Loucks definition — "energy availability is dietary energy intake minus *exercise* energy expenditure, normalized to fat-free mass." It deliberately does NOT subtract total daily expenditure; EA is about energy left for bodily function after *training*, not after *living*.

The consequence is that non-workout movement (NEAT) is completely invisible to the system today. Garmin measures it: `get_user_summary(date)` returns active vs resting kcal, total kcal, steps, floors, intensity minutes, and distance for the whole day. The bridge already syncs the same day, so capturing this is cheap. The question this change answers is *where* that data lives and *how* it relates to EA — without corrupting the Loucks metric.

Constraints that shape the design:
- **One package per capability** — this is a new capability `daily-summary` → new `internal/dailysummary/`, not an extension of `summary` or `energy`.
- **Unit isolation** — daily kcal/steps/distance stay on the `daily-summary` shape; they must never leak into `summary`'s Totals struct (which carries nutrition kcal+g+mg) nor into EA's response as a substituted denominator. This is the same reasoning that keeps hydration ml out of `/summary/daily` (archived `add-hydration-tracking`).
- **Date-keyed snapshot pattern** — exactly like `recovery-metrics`/`fitness-metrics`/`hydration-balance`: `date` PRIMARY KEY, upsert-by-date, source-agnostic, NULL-is-meaningful, 92-day list cap.
- **`numfmt.Round1` at the response boundary** for every float.
- **Append-only sequential migrations** embedded in the binary; head on disk is `035`, B `add-garmin-workout-detail` claims `036`, so this is `037` — but the head MUST be re-verified before scaffolding (out-of-band collisions have happened).
- **garmin-bridge tolerates per-capability failure** — a failing `get_user_summary` must yield an absent snapshot, never an aborted day.

## Goals / Non-Goals

**Goals:**
- Make Garmin's whole-day energy/activity totals a first-class, queryable, date-keyed signal (active/resting/total kcal, steps, floors, intensity minutes, distance).
- Keep re-sync idempotent via upsert-by-date, like the other snapshot capabilities.
- Surface the snapshot through REST + a single MCP read tool so the chat coaching agent can read total daily expenditure as context.
- Preserve the Loucks EA metric exactly as published.

**Non-Goals:**
- **No change to the EA formula.** EA keeps `exercise_energy_kcal = Σ workouts.kcal_burned` as its subtrahend; total daily expenditure does NOT replace it (see D3).
- **No merge into `summary`.** `/summary/daily` does not gain `total_kcal`/`steps`; daily-summary keeps its own shape (D2).
- **No EA enrichment in this change.** An EA variant or context block that *reads* `daily-summary` (e.g. reporting NEAT alongside EA, or a "total expenditure" context line) is a deliberate follow-up — see Open Questions / the non-goal note below.
- **No intraday/time-series** (e.g. per-15-minute step buckets). One whole-day snapshot per date.
- **No back-fill** of historical days; only forward syncs and explicit re-syncs populate rows.

## Decisions

### D1 — New `daily-summary` capability, date-keyed upsert table
`daily_summary` is its own table with `date` as PRIMARY KEY (no surrogate id), one row per calendar day, upserted in place — identical to `recovery_metrics`/`fitness_metrics`/`hydration_balance`. The package is `internal/dailysummary/` with the standard `types`/`repo`/`service`/`handlers` shape; `repo.go` runs against `store.Querier`; `service.go` sentinel errors map 1:1 to API error codes. REST surface mirrors the sibling snapshot capabilities: `POST` (upsert), `GET /{date}` (single, 404 when absent), `GET ?from=&to=` (inclusive window, 92-day cap).
*Alternative considered:* folding the fields onto `summary` or `energy` — rejected; both carry different units and a different cardinality (summary is computed per-window, energy is pure-computation), and merging would break unit isolation. A standalone snapshot capability matches the established Garmin-import pattern and is the lowest-surprise home.

### D2 — Daily kcal is NOT merged into `summary`'s Totals struct
`/summary/daily` returns nutrition totals (kcal from food, plus g/mg of macros/micros). Garmin's `total_kcal`/`active_kcal`/`resting_kcal` are *expenditure*, a different sign and a different source. Per the unit-isolation convention they stay in the `daily-summary` response shape and are never added to the `summary` Totals struct. A consumer that wants both intake and expenditure for a day reads both endpoints.
*Alternative considered:* adding `expenditure_kcal` to the summary Totals for one-call convenience — rejected; it conflates intake and expenditure in a struct whose every other field is intake, and tests across the codebase assert "summary carries no foreign units" (`assert.NotContains`).

### D3 — EA keeps exercise-burn as its denominator subtrahend; total expenditure is an *independent context signal*
This is the subtle one. It is tempting to "improve" EA by subtracting Garmin's total daily expenditure instead of `Σ workouts.kcal_burned`. **Rejected.** EA is a published metric (Loucks) with concrete clinical bands (`<30` low, `30–45` sub-optimal, `≥45` adequate). Those bands were validated against *exercise* energy expenditure, not total expenditure. Swapping the denominator changes what the number *means* — it would no longer be comparable to the literature bands the `energy-availability` spec cites, and a "low EA" flag would fire for reasons (a high-NEAT day) the bands were never calibrated against. So:
- The EA formula and the `energy-availability` spec are untouched by this change.
- Garmin's whole-day expenditure lives only in `daily-summary`, as an *independent* signal the chat agent (or a future EA-context block) MAY read alongside EA, but never as a substitute term inside the EA computation.
*Follow-up (non-goal here):* an EA enrichment that consumes `daily-summary` — e.g. reporting "EA 31 kcal/kg FFM; total expenditure 2,950 kcal of which 410 from logged workouts → 'EA looks adequate but NEAT was high, watch tomorrow'" as a *context annotation* next to the unchanged EA number. Explicitly deferred.

### D4 — All fields nullable, NULL is meaningful
Every metric column is nullable; a NULL means "Garmin did not report it for that day," not a data bug — same convention as the other snapshot capabilities. The bridge POSTs whatever `get_user_summary` returned; absent fields are simply omitted from the body and stored NULL. Responses use omitempty.

### D5 — Bridge fetch guarded, mapped, posted as a date-keyed snapshot
`fetch_day` gains a single `safe(get_user_summary, date)` sub-fetch; a failure yields a missing key, not an aborted day. `map_daily_summary(raw)` extracts the documented fields defensively (absent → omitted). `sync.py` posts the mapped body to `/daily-summary` in the same date-keyed flow as the other snapshot metrics (the `_SNAPSHOT_ROUTES`-style path). Re-running the day upserts in place.

### D6 — One MCP read tool, expected-tools list bumped
`daily_summary_get` mirrors `GET /daily-summary/{date}` 1:1 and forwards the body verbatim (`toToolResult`), registered via a `registerDailySummaryTools` group like the sibling snapshot tools. No write tool is added (the bridge writes via REST directly, like the other snapshot capabilities). The `mcp_integration_test` expected-tools list grows by exactly one.

## Risks / Trade-offs

- **Temptation to "fix" EA with total expenditure** → mitigated by D3: documented rejection, EA spec untouched, the signal lives in a separate capability. The reviewer should reject any future PR that swaps EA's denominator without re-validating against the literature bands.
- **Garmin field-name drift** (`bmrKilocalories` vs `restingKilocalories`, etc.) → mapping is defensive and field-by-field; an unexpected/renamed key stores NULL rather than crashing. Locked field names are pinned in the spec scenarios and the bridge fixture.
- **Another date-keyed table** → consistent with the established snapshot pattern; no new query shape, no join, additive migration, clean rollback.
- **MCP surface grows by one tool** → expected and accounted for in the expected-tools list bump; the tool is read-only, no idempotency concern.

## Migration Plan

1. **Verify the migration head on disk before scaffolding** (`ls internal/store/migrations | sort | tail`). The arc expects `035` head → B claims `036` → this is `037`, but an out-of-band slot collision has happened before; confirm `037` is free, then `task migrate:new NAME=add_daily_summary`.
2. `037_add_daily_summary.up.sql`: `CREATE TABLE daily_summary` with `date DATE PRIMARY KEY`, the eight nullable metric columns with sane CHECKs (`>= 0`), and `created_at`/`updated_at` TIMESTAMPTZ defaults — mirroring `recovery_metrics`.
3. `.down.sql`: `DROP TABLE daily_summary`.
4. Rollback is clean — a single additive table, no data transform; existing capabilities read back unchanged.

## Open Questions

- **`distance_m` precision** — Garmin reports `totalDistanceMeters` as a float; store `NUMERIC(10, 1)` and round to 1dp at the boundary like the other measurement floats. **Resolved in spec: `NUMERIC(10, 1)`.**
- **Intensity minutes type** — `moderate_intensity_minutes`/`vigorous_intensity_minutes` are integers in Garmin's payload; stored as INTEGER. **Resolved in spec: INTEGER.**
- **Whether `total_kcal` should be derived (`active + resting`) or stored as Garmin reports it** — store as reported (`totalKilocalories`); do not derive, since Garmin's total may include adjustments and a stored value preserves the source of truth. A NULL active/resting with a non-NULL total is therefore valid. **Resolved: store all three independently.**
