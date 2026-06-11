## Continuity

_Forward plan for OpenSpec changes. Tracks **what's next**, **what's in flight**, and **what's queued**._
_Companion to `openspec/priorities.md` (tier/triage framing) — this file is the operational queue._
_Last refreshed: 2026-06-10 by the `continuity` skill (the Garmin-ingestion arc landed: `widen-workout-ingestion`, `add-garmin-daily-metrics`, `add-hydration-balance-metrics`, plus `add-deployment-pipeline` all archived. `add-flutter-companion-app` is now nearly complete and the only remaining planned change)._

## In progress

| Change | Branch | Started | Owner | Notes |
|---|---|---|---|---|
| add-flutter-companion-app | `main` (no feat branch) | 2026-06-10 | Vinzenz Stadtmueller | **86/87 tasks**; ~88 uncommitted files under `apps/companion/` on `main`. Near-complete — needs the last task + a commit. Was deliberately kept out of this session's backend commits. |

_Note: this change is being worked directly on `main` rather than a `feat/` branch — fine for a solo nearly-finished change, but means the ~88 uncommitted files share the tree with backend work. Commit them under their own `feat(companion): …` commit when ready._

## Up next

Ordered queue — top is next to pick up.

_Up next is empty — see Backlog / `openspec/priorities.md` for the next move. The clear strategic next step is out-of-repo (see Notes: the `garmin.py` importer)._

## Backlog

Planned changes not yet prioritized.

_Backlog is empty — every proposed change is either archived or in progress. New ideas live in `openspec/priorities.md` until proposed._

## Notes

- **The Garmin-ingestion arc shipped (2026-06-10).** Five changes across the session gave Garmin's data homes: `widen-workout-ingestion` (per-activity distance/power/temperature/sweat-loss + brick `session_group`), `add-garmin-daily-metrics` (recovery + fitness daily snapshots, richer weigh-in biometrics, planned/completed workout status), and `add-hydration-balance-metrics` (daily sweat-out / activity-intake / goal). Plus `add-deployment-pipeline`. Delivers priorities T2 #6A (sleep/HRV "morning metrics", via `recovery-metrics`) and most of the data side of #6C (sweat-rate).
- **The real bottleneck is now out-of-repo: `garmin.py`.** The backend can *store* recovery, fitness, biometrics, planned workouts, and hydration balance — but nothing *fills* them yet. CORRECTION to earlier session notes: `garmin.py` is NOT read-only; it already has a `_push_hydration` that POSTs the daily hydration rollup, and `cmd_coach` already fetches everything else (sleep/HRV/RHR/stress/readiness, VO2max/race-predictions/load, full weigh-ins, calendar). Wiring its existing fetches to the new endpoints is the highest-leverage next move — and it's a separate repo (`…/Orga/.scripts/garmin.py`).
- **Follow-ups surfaced this session (not yet proposed)** — candidates for `priorities.md`: a **race entity + per-leg race-day fueling plan** (the biggest remaining backend gap; worth proposing before taper, ~mid-July); EA consuming **measured muscle mass** (FFM resolver tier); **training-day template auto-apply** from planned workouts; **per-metric trend endpoints** (`/recovery-metrics/trend` etc.); a derived **sweat-rate (ml/hr)** endpoint (completes T2 #6C now that the inputs all exist).
- **Still-open priorities-flagged work**: T2 #6E (retroactive freeform→product correction), #6F (`coach_recommendation` persistence — deliberate-discussion-first), #8 (caffeine — likely subsumed by workout-fuel's `caffeine_mg`; 5-min audit), #9 (supplement log).
- **Stale branch to prune**: `feat/add-recommend-workout-fuel` is a leftover from an already-archived change — safe to delete when convenient (this skill never prunes branches itself).
- **Pattern note — MODIFIED spec deltas are full-replace.** A MODIFIED requirement that lists only new scenarios silently drops the prior ones at sync time. This session's changes handled it by copying full prior content into MODIFIED blocks and using ADDED requirements for genuinely-new behavior (the workouts `status` lifecycle, the daily-context blocks). Keep preferring ADDED for additive intent.
- **Process caveat**: the `openspec-sync-specs` skill named by the archive flow doesn't exist in this environment — the archive agent did the spec sync via direct file edits + `openspec validate` each time. Results are correct; just not via a dedicated skill.

---
_To update: ask Claude "update continuity", "queue X next", or "start work on X"._
_For tier/triage and "why does this matter" framing, see [`openspec/priorities.md`](openspec/priorities.md)._
_For historical record of implemented changes, see [`roadmap.md`](roadmap.md)._
