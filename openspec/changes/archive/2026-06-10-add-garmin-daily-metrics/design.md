# add-garmin-daily-metrics — design

## Context

The API has rich per-capability primitives (meals, workouts, hydration, body-weight, EA, training-phases) but no home for Garmin's daily wellness/fitness stream. `garmin.py`'s `cmd_coach` already fetches all of it (sleep, HRV, RHR, stress, body battery, training readiness, VO2max, race predictions, acute/chronic load, full biometric weigh-ins) and computes throwaway 7-day averages each run. This change adds the storage + read surface so the importer can push it and the LLM coach (and `daily-context`) can read it.

Inherited conventions this change must follow:

- **One package per capability** with the `types.go / repo.go / service.go / handlers.go / *_test.go` shape; wiring in `internal/httpserver/server.go`'s `Run()`.
- **Unit isolation** — distinct response shapes per capability; never merge into shared Totals structs.
- **`numfmt.Round1` at the response boundary** for floats; DB `NUMERIC(p,s)` precision also bounds stored values.
- **MCP mirrors REST 1:1**, write tools auto-derive idempotency keys, expected-tools list in the integration test must be bumped when tools are added.
- **Migrations append-only**, head is `019`.
- **PATCH tri-state** (value sets / JSON-null clears / absent leaves) where field-level edits matter.

## Goals / Non-Goals

**Goals:**

- Storable, source-agnostic homes for recovery + fitness daily snapshots, richer weigh-ins, and planned workouts.
- Idempotent "push every day you see" ergonomics for the daily snapshots (date is the dedup key).
- Surface the new daily data in `daily-context` so one read grounds a coaching session.
- Zero behavior change for existing rows/callers; existing workouts back-fill to `completed`.

**Non-Goals:**

- Consuming the data (EA muscle-mass tier, sleep-aware deficit advice, template auto-apply) — follow-ups.
- The `garmin.py` push path itself (out-of-repo).
- Trend endpoints, derived flags, sleep-stage breakdown, PATCH on metrics tables (see proposal non-goals).

## Decisions

### 1. Recovery and fitness are two capabilities, each upsert-by-date

Per the scoping decision, recovery and fitness stay separate packages/tables/specs rather than one `daily_metrics` table — matching the unit-isolation discipline (a recovery query shouldn't drag VO2max columns; the two domains evolve independently).

Each table uses **`date DATE` as the natural identity** (one Garmin snapshot per calendar day), with a `UNIQUE` constraint and `POST … ON CONFLICT (date) DO UPDATE` (full-replace of the mutable columns). This gives the importer the same "POST every day, re-syncs land in place" idempotency that `workouts.external_id` gives, without needing a surrogate dedup key. Alternatives rejected:

- *PUT /{date} full-replace* (the goal-overrides pattern) — would reject `Idempotency-Key` per the PUT rule and reads less naturally for a machine writer that re-pushes; POST-upsert is the workouts ergonomic and fits "push every day."
- *Surrogate `id` + INSERT* (the body-weight pattern) — body-weight is legitimately multi-per-day (you can weigh twice); recovery/fitness are one-per-day by Garmin's own model, so date-keyed upsert prevents duplicate-day rows by construction.

Read surface per capability: `GET /…?from=&to=` (window list, ordered by date asc, 92-day cap like workouts), `GET /…/{date}` (single), `DELETE /…/{date}`. **No PATCH** — re-POST replaces; these are machine-written, not hand-curated.

`date` is parsed/validated as `YYYY-MM-DD` (not a timestamp — these are whole-day aggregates). Out-of-range/garbage date → `date_invalid`.

### 2. Column sets are deliberately small and Garmin-grounded

Only fields `garmin.py` already reads, kept flat and nullable (NULL = "not measured / device didn't report"):

- **recovery_metrics**: `sleep_seconds INT`, `sleep_score INT` (0–100), `hrv_ms NUMERIC(6,1)` (last-night avg), `resting_hr INT`, `stress_avg INT` (0–100), `body_battery_charged INT`, `body_battery_drained INT`, `training_readiness INT` (0–100). Bounded-range fields get CHECK constraints; counts/durations get `> 0` (or `>= 0`) checks.
- **fitness_metrics**: `vo2max_running NUMERIC(4,1)`, `vo2max_cycling NUMERIC(4,1)`, `race_predictor_5k_seconds INT`, `race_predictor_10k_seconds INT`, `race_predictor_half_seconds INT`, `race_predictor_full_seconds INT`, `acute_load NUMERIC(6,1)`, `chronic_load NUMERIC(6,1)`. Race predictors stored as **seconds** (the agent formats `h:mm:ss`), matching the project's metres-not-km, seconds-not-formatted primitive convention.

Derived values (HRV weekly trend, ACWR = acute/chronic, sleep debt) are NOT stored — composed by the agent or a future analytics capability.

### 3. Planned workouts: a `status` enum that conditions the future-date guard

`workouts` gains `status TEXT NOT NULL DEFAULT 'completed' CHECK (status IN ('planned','completed'))`. The migration back-fills existing rows to `completed` (they are all completed activities).

The critical interaction: today `buildWorkout` rejects `started_at` more than 24h in the future (`started_at_too_far_future`). A planned session for next week MUST be allowed. Decision: **the 24h guard applies only to `status='completed'`; `status='planned'` allows a future `started_at` up to +1 year** (a sanity bound, error `started_at_too_far_future` still fires beyond it). A `planned` workout with a *past* start is allowed (you can pre-load a plan that's already underway). `ended_at > started_at` still holds for both.

Reconciliation (planned → completed when the real activity lands) is the **importer's** responsibility — Garmin's scheduled-workout id and the eventual activity id differ, so the API cannot auto-link them. The API provides the mechanism: `status` is a mutable PATCH field, and `DELETE` + a fresh completed POST is equally valid. `GET /workouts?status=planned` lists the plan; the existing `from/to` window still applies. Documented as the importer's job; not modeled here.

`status` is echoed on every workout response (always present — it's `NOT NULL`, no omitempty). `POST` defaults it to `completed` when omitted, so existing Garmin-activity pushes are unaffected.

### 4. body-weight gains four nullable biometrics; EA stays unchanged

`muscle_mass_kg NUMERIC(5,2)` (`> 0`), `body_water_pct NUMERIC(4,1)` (0–100), `bone_mass_kg NUMERIC(4,2)` (`> 0`), `bmi NUMERIC(4,1)` (`> 0`) — all nullable, through POST/PATCH/GET like the existing `body_fat_pct`. The EA FFM resolver's four-tier rule is **untouched**: it keeps preferring explicit lean-mass param > body_fat_pct. Using `muscle_mass_kg` as a higher-confidence tier is a deliberate follow-up (changing EA composition silently here would alter a computed metric mid-change).

### 5. daily-context composes the two new snapshots (read-only)

`GET /context/daily` gains `recovery` and `fitness` blocks, each the row for the requested `date` or `null` when absent (no carryover — unlike weight, a stale recovery snapshot is misleading; same-day-or-null). The existing `weight` block echoes the four new biometric fields when present. Pure read composition over the new repos' `GetByDate` — no new tables, consistent with the capability's "no writes, no tables" contract. Adds two repo reads to the existing parallel fan-out.

### 6. MCP: two new tool groups, expected-tools list bumped

`registerRecoveryMetricsTools` + `registerFitnessMetricsTools` mirror the bodyweight tool file. Each: `log_*` (POST upsert), `list_*` (window GET), `get_*` (by date), `delete_*`. Write tools auto-derive idempotency keys. `log_weight`/`patch_weight` gain the four biometric args; `log_workout`/`patch_workout` gain `status`, `list_workouts` gains a `status` filter. The `mcp_integration_test.go` expected-tools list grows by 8 tools — **must be updated** (unlike `widen-workout-ingestion`, this change adds tools).

## Risks / Trade-offs

- **[Date-keyed upsert silently overwrites a hand-corrected day on the next push]** → Acceptable: these are machine-authoritative daily aggregates; the importer is the source of truth. If hand-correction ever matters, a future PATCH path can add it. Documented.
- **[Planned-workout future bound is a heuristic (+1 year)]** → Prevents absurd dates while allowing any real training plan; the bound is a constant, easily tuned. Past-dated planned rows are allowed by design (mid-plan loads).
- **[`status` default + back-fill must not break existing fueling/EA/daily-context reads]** → `completed` default means every existing query path sees unchanged data; EA and fueling don't filter on status (a planned workout has no `kcal_burned`, so it contributes nothing to EA burn anyway — but to be safe, EA/fueling windows should be reviewed to confirm planned rows don't distort aggregates). **Open question below.**
- **[Scope is large — 4 schema changes, 2 new packages, 4 modified]** → Tasks are grouped by capability slice so apply can pause cleanly between slices; the two new capabilities are independent and can land before the modifications.
- **[Two more parallel reads in daily-context]** → Negligible; already a parallel fan-out, two more `GetByDate` calls by indexed date.

## Migration Plan

1. Verify next free slots (expected `020`–`023`; head is `019`).
2. `020` recovery_metrics CREATE TABLE (`date` UNIQUE/PK, nullable metric columns + CHECKs, `created_at`/`updated_at`).
3. `021` fitness_metrics CREATE TABLE (same shape).
4. `022` body-weight ALTER: four nullable biometric columns + CHECKs.
5. `023` workouts ALTER: `status` column `NOT NULL DEFAULT 'completed'` + CHECK; existing rows take the default (no explicit back-fill statement needed — the DEFAULT applies); add `workouts_status_idx` partial-or-plain index if list-by-status warrants it.
6. Each migration has a `.down.sql` (DROP TABLE / DROP COLUMN). No data transforms; rollback drops the new structures.

## Open Questions

- **Does EA or `/workouts/{id}/fueling` need to exclude `status='planned'` rows?** A planned workout has null `kcal_burned` so it adds nothing to EA's burn sum, and fueling windows are time-anchored (a future planned workout won't overlap logged intake). Leaning **no filter needed**, but the apply step should add a test asserting a planned workout doesn't appear in EA burn or distort `ListWindow` consumers — and if it does, filter `status='completed'` in those read paths. Resolve during implementation with a test, not a guess.
- Whether `daily-context` should show *tomorrow's* planned workout (look-ahead) — deferred; `daily-context` stays same-day. A look-ahead is a follow-up if the coach wants it.
