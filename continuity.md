## Continuity

_Forward plan for OpenSpec changes. Tracks **what's next**, **what's in flight**, and **what's queued**._
_Companion to `openspec/priorities.md` (tier/triage framing) — this file is the operational queue._
_Last refreshed: 2026-06-12 by the `continuity` skill (the "mirror everything" Garmin arc — now 8 proposed changes after a gap-closure pass — recorded as the forward plan; `add-garmin-workout-detail` is next to pick up)._

## In progress

| Change | Branch | Started | Owner | Notes |
|---|---|---|---|---|
| _(none)_ | | | | |

_Nothing in flight; tree clean (`continuity.md` + `roadmap.md` carry uncommitted edits). All five arc changes are proposed + `validate --strict`-clean but unimplemented._

## Up next

Ordered queue — top is next to pick up. **This is the "mirror everything" Garmin arc** — bring every capability garminconnect exposes into the API/MCP, especially workouts. Apply **in order**: the migration numbers (036→041) only hold if the migration-bearing changes land in this sequence, and each `tasks.md` re-verifies the head on disk before scaffolding (an out-of-band slot collision has bitten this repo before). Items 6–8 came out of a gap-closure pass (weather + reconcile-seam folded into B; F/backfill/G added).

1. **add-garmin-workout-detail** (B, migration `036`) — Garmin-synced workouts land as a flat summary; this adds the per-activity detail the fueling math wants: time-in-HR-zone + elevation + normalized power as columns, per-lap splits and strength sets as child tables, **plus weather (humidity + wind) for sweat-rate**, nested-write on `/workouts/bulk` with replace-on-resync, and an explicit **reconcile-seam** guarantee (detail attaches to the merged planned→completed row, not a duplicate). _Why now: the headline "especially workouts" slice — feeds raceprep/workoutfuel carb math and the still-open derived sweat-rate endpoint (priorities T2 #6C); fills the blank strength sessions._
2. **add-garmin-daily-energy** (A, migration `037`) — EA only sees explicitly-logged workout burn, so all NEAT (commute, standing, non-workout movement) is invisible. New date-keyed `daily-summary` capability maps `get_user_summary` (active/resting/total kcal, steps, floors, intensity minutes, distance). _Why now: highest-ROI sibling — makes total daily expenditure visible without changing the Loucks EA formula (it's an independent context signal, not a new denominator)._
3. **extend-recovery-fitness** (C, migration `038`) — additive column extension to the existing `recovery-metrics` + `fitness-metrics` snapshots: SpO2, overnight respiration, deep/light/REM/awake sleep stages, plus endurance score / hill score / fitness age / the `training_status` label. _Why now: cheap per-day Garmin calls alongside ones we already make; richer coaching context._
4. **add-garmin-gear-and-prs** (D, migration `039`) — two new inventory-shaped capabilities: `gear` (shoe/bike mileage + retirement) and `personal-records`. _Why now: the most tangential-to-nutrition slice — built because the user chose to mirror **everything**; chat-agent coaching context (gear-retirement reminders, "PR-fit right now"), not fueling math._
5. **add-garmin-athlete-config** (F, migration `040`) — singleton `athlete-config` capability (goals-singleton pattern) capturing FTP / threshold HR & pace / max HR / HR-zone boundaries from `get_user_profile` + `get_heart_rate_zones`. _Why now: makes B's normalized-power and time-in-zone data interpretable (B's `intensity_factor` stays NULL without FTP); Garmin is source-of-truth, capture-only (deriving IF is a follow-up)._
6. **garmin-workout-library-mgmt** (E, no migration) — control-plane / write-and-blob sibling. Leads with the **orphan-bug fix** (today both unschedule and re-push leak the prior Garmin workout object); adds Garmin-library read tools, and the two write/blob MCP tools the user opted into — `add_hydration_data` (push hydration back to Garmin) and activity FIT export. _Why now: latent-bug fix is real; reuses the ids from migration 032, so no schema change._
7. **add-garmin-misc-mirror** (G, migration `041`) — the explicit catch-all tail completing "mirror everything": new `devices`, `health-vitals` (BP + all-day HR/stress), and `achievements` (badges/challenges) capabilities, plus control-plane tools (activity-gear link, structured-workout export, FIT upload, activity rename/delete). _Why now: completeness only — **LOW PRIORITY, apply near-last**; nothing feeds fueling/EA/hydration math._
8. **add-garmin-history-backfill** (no migration) — one-shot bounded, paced, idempotent `POST /sync/backfill` + `garmin_backfill` MCP tool, replaying the (now-enriched) sync over an arbitrary historical range so mid-season activities older than the rolling CronJob window gain the new detail. _Why now: **apply LAST** — only useful once B/A/C/D/F/G have enriched the sync path; also the home of the arc's Garmin call-budget / rate-limit pacing design._

## Backlog

Planned changes not yet prioritized.

- _Empty — every proposed change is in the Garmin arc above._

## Notes

- **The "mirror everything" Garmin arc is fully PROPOSED (this session), not yet implemented.** Eight changes, all `openspec validate --strict`-clean: B `add-garmin-workout-detail`, A `add-garmin-daily-energy`, C `extend-recovery-fitness`, D `add-garmin-gear-and-prs`, F `add-garmin-athlete-config`, E `garmin-workout-library-mgmt`, G `add-garmin-misc-mirror`, and `add-garmin-history-backfill`. New MCP surface across the arc: **~+18 tools** (A +1, D +2, F +1, E +5, G +8, backfill +1; B and C just return richer JSON). Decision provenance lives in each change's `design.md`.
  - **Gap-closure pass (after the initial 5):** weather (humidity/wind, sweat-rate) + the reconcile-seam guarantee were folded into **B**; **F** (athlete-config) makes B's IF/zone data interpretable; the **backfill** change sweeps mid-season history older than the rolling CronJob window and houses the Garmin call-budget pacing; **G** is the honest catch-all so "mirror everything" doesn't overclaim. The low-value tail G still excludes is listed in its proposal ("Deliberately still excluded": menstrual/pregnancy, social/leaderboard, per-second streams).
  - **Apply order is load-bearing for migrations:** 036/037/038/039/040/041 (E + backfill claim none) only hold if the migration-bearing changes land in the queue order above. Re-verify the head on disk before each `task migrate:new`.
  - **E carries two open items for the apply phase** (flagged in its `design.md`): the control proxy's **16 KB body cap is too small for a FIT export blob** (needs a per-route raise to ~8 MB on the export path only), and FIT transport is decided as **base64-in-JSON** since the bridge shares no filesystem with the agent. E's orphan fix has two wiring points — `unschedule` and re-push (`pushOne`) — both confirmed to leak today against `scheduling.go`.
  - **A's design boundary:** the Loucks EA formula is untouched (`(intake − exercise burn)/FFM`); Garmin TDEE is surfaced only as context in `daily-summary`, never merged into `summary` Totals (unit isolation). "EA NEAT-enrichment" is an explicit follow-up, not in scope.
- **The PRIOR Garmin integration + Option B training-plan arc is COMPLETE and archived** — auth, read-import, login, workout-templates → training-plan → garmin-scheduling → plan-slot-targets → workout-reconciliation, plus `fix-chat-tool-status-chips`. This new arc deepens that foundation (depth + breadth) rather than re-plumbing it. Coaching synthesis stays the chat agent's job, not an API endpoint.
- **Drift to clean up (carried):**
  - **`main` is well ahead of `origin/main` and unpushed** — the whole prior Garmin + Option B + chat-sessions arc is local-only. Push when ready.
  - **Stale branches to prune:** `feat/add-chat-sessions` (now == `main`) and `feat/add-recommend-workout-fuel` (leftover) — both safe to delete when convenient (this skill never prunes branches).
- **Open follow-ups from prior arcs (not proposed):** manual on-device smoke for `fix-chat-tool-status-chips` (task 4.3); reverse-direction workout reconciliation + ±1-day tolerance + plan-adherence analytics; manual companion-chat e2e; a real-Anthropic `/chat` smoke once `ANTHROPIC_API_KEY` is set (503 `chat_unavailable` until then).
- **Still-open priorities-flagged work** (in `openspec/priorities.md`, independent of this arc): T2 #6E (retroactive freeform→product correction), #6F (`coach_recommendation` persistence), #9 (supplement log); the derived sweat-rate (ml/hr) endpoint completing T2 #6C — note `add-garmin-workout-detail` supplies its missing inputs (time-in-zone, elevation), so #6C becomes buildable once B lands.
- **Pattern notes (carried):** MODIFIED spec deltas are full-replace — copy prior scenarios into the MODIFIED block; prefer ADDED requirements for additive intent. OpenSpec requirement bodies must lead with a SHALL/MUST sentence or `validate --strict` rejects them. Migration head is `035` on disk; this arc claims 036–039. The `openspec instructions … --json` command prints a progress line before the JSON — strip with `sed -n '/^{/,$p'` before parsing.

---
_To update: ask Claude "update continuity", "queue X next", or "start work on X"._
_For tier/triage and "why does this matter" framing, see [`openspec/priorities.md`](openspec/priorities.md)._
_For the historical record of implemented changes, see [`roadmap.md`](roadmap.md)._
