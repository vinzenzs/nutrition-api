## Context

`GET /context/training` (`internal/coachcontext`) is a composition-only aggregate the coach grounds on. Its `Service.BuildTraining` fans out over read repos in parallel (`errgroup`) and returns one `TrainingContext` with no partial result on error. It already surfaces phase (+ methodology), latest fitness snapshot, derived ACWR, recent load, recent + upcoming workouts.

Two physiology inputs are stored but unreachable from this bundle:
- `athlete_config` — singleton FTP/threshold/HR-zone/power-zone row (`internal/athleteconfig`, `Repo.Get`). Garmin is source-of-truth; nothing consumes it today.
- power-to-weight (W/kg) — FTP and bodyweight are both stored, but the ratio is computed nowhere. Bodyweight has `Repo.LatestBefore(ctx, before)`.

The coach can't anchor intensity prescriptions to the athlete's zones or W/kg without a second round-trip it doesn't make.

## Goals / Non-Goals

**Goals:**
- Surface the `athlete_config` block (or null) in `GET /context/training`.
- Add a derived `watts_per_kg` (FTP ÷ latest bodyweight kg), null when either input is missing or bodyweight is zero — same derive-or-null discipline as the existing `acwr`.
- Keep the additions inside the existing parallel fan-out with no partial-bundle-on-error behavior.

**Non-Goals:**
- ACWR population. The server already derives ACWR; supplying `acute_load`/`chronic_load` is Garmin-bridge ingestion, out of scope. No TSS-derived fallback is added (decided).
- Phase `methodology` — already shipped (`add-coach-methodology`).
- Any write path / new MCP write tool for `athlete_config` (only `athlete_config_get` exists; out of scope here).
- Surfacing athlete config on the nutrition `daily_context` bundle — training context only.

## Decisions

- **Embed the full `athlete_config` block, not a flattened subset.** The decision (W/kg + full athlete_config) is to give the coach zones and thresholds too. Reuse `athleteconfig.AthleteConfig` as the field type (`*athleteconfig.AthleteConfig`, `json:"athlete_config"`) rather than minting a `*Lite` — the config is already compact, all-nullable, and omit-empty, mirroring how `Fitness` reuses `*fitnessmetrics.Snapshot`. Alternative (a coachcontext-local lite struct) rejected: no field-trimming benefit, more code to drift.
- **Derive `watts_per_kg` in coachcontext, never store it.** `wattsPerKg(cfg, bw)` returns `*float64`, non-nil only when `cfg.FtpWatts != nil`, a bodyweight entry exists, and its kg > 0; rounded via `numfmt.Round1`. Mirrors the existing `acwr(...)` helper exactly. Serialized as `watts_per_kg` (top-level, alongside `acwr`), null when underivable.
- **Bodyweight anchor = latest on/before `dayEnd`.** Use `bodyWeightRepo.LatestBefore(ctx, dayEnd)` so a same-day weigh-in counts, consistent with how the bundle treats the anchor date as inclusive.
- **One additional errgroup goroutine** fetches athlete config + latest bodyweight and computes W/kg, writing `out.AthleteConfig` and `out.WattsPerKg`. Both reads are cheap singleton/indexed lookups; co-locating them keeps the derivation atomic. A missing athlete_config row (`Get` returns a zero/empty or not-found) leaves `AthleteConfig` nil and `WattsPerKg` nil — not an error, matching the "quiet history is not an error" rule.
- **Wiring:** `coachcontext.NewService` gains `athleteConfigRepo *athleteconfig.Repo` and `bodyWeightRepo *bodyweight.Repo` params; `httpserver/server.go` already constructs both (`athleteConfigRepo`, `bodyWeightRepo`) before the `coachCtxSvc` line, so it's a call-site edit.

## Risks / Trade-offs

- [Garmin overwrites manually-set FTP on next sync] → Accepted; documented already on `PUT /athlete-config`. Surfacing is read-only and reflects whatever is current.
- [W/kg looks stale if bodyweight logging lapses] → `LatestBefore` returns the most recent known weight; acceptable for grounding. Null only when there is no weight at all.
- [Constructor signature change ripples to tests/callers] → Only `httpserver` and coachcontext tests construct the service; both updated in the same change.
- [`athleteconfig.Repo.Get` semantics on empty singleton] → Verify whether it returns a zero-value config or a sentinel; the goroutine must map "unset" to nil `AthleteConfig` so the null-serialization scenarios hold.
