## Continuity

_Forward plan for OpenSpec changes. Tracks **what's next**, **what's in flight**, and **what's queued**._
_Companion to `openspec/priorities.md` (tier/triage framing) — this file is the operational queue._
_Last refreshed: 2026-06-08 by the `continuity` skill._

## In progress

This repo currently commits implementation work directly to `main` (single-user
project, no feature-branch flow yet). The Branch column reflects that — switch
to per-change branches when the cadence makes it worth it.

| Change | Branch | Started | Owner | Notes |
|---|---|---|---|---|
| add-meal-workout-link | `main` | 2026-06-08 | Vinzenz | 16/36 tasks. Closes T1 #1 (`workout_ref` on meal/hydration logs + workout-anchored fueling summary). First leverage-cluster leaf after workouts-capability. |
| add-hydration-tracking | `main` | pre-2026-06-07 | Vinzenz | 34/35 tasks. Ready-to-archive pending manual e2e. |
| add-date-varying-goals | `main` | pre-2026-06-07 | Vinzenz | 32/33 tasks. Ready-to-archive pending manual e2e. |
| add-weight-log | `main` | pre-2026-06-08 | Vinzenz | 34/35 tasks. Delivers T2 #6. Ready-to-archive pending manual e2e — unblocks T1 #4 (EA) body-weight input. |

## Up next

Ordered queue — top is next to pick up.

1. **add-workout-fuel** — Workout fuel entries (carbs / sodium / caffeine in-session) as a sibling capability to hydration. Deliberately not a column extension, to avoid mixing ml + g + mg in one Totals struct. _Why now: closes T1 #2; composes into the workout-anchored summary `add-meal-workout-link` introduces — wait for that to archive so the future-compat note can cash in._
2. **add-meal-from-photo** — Backend-mediated Claude Vision integration for photo-of-meal logging. Mirrors the off-integration pattern (one API key, server-side). _Why now: independent backend feature; the Flutter app's #2 killer interaction; can ship in parallel with workout-fuel since they touch different surfaces._

## Backlog

Planned changes not yet prioritized.

- **add-flutter-companion-app** — Three-screen Flutter app (barcode / photo / hydration widget) as a focused supplement to the agent. _Caveat: predates the endurance-training pivot — see Meta #3 in `openspec/priorities.md`. Worth a short explore session on whether the three killer interactions still match today's most-pressing use._

## Notes

- **Tier-1 EA tool (priorities.md T1 #4)** is not yet a proposal but is now cheap to write: workouts shipped, weight-log is ready-to-archive, both inputs exist. Worth a `/opsx:propose` once the in-progress queue clears.
- **Decisions pending** (do not queue yet): T1 #5 (templates) vs T1 #1A (training-phase) — same question, two answers. T2 #6F (`coach_recommendation` persistence) — tests the synthesis principle, deliberate discussion first.
- **Waiting on usage data**: T1 #3 (`plan_carb_load` auto-apply) revisits after first real race-week use. T1 #5 (templates) revisits after first multi-week training block planned.

---
_To update: ask Claude "update continuity", "queue X next", or "start work on X"._
_For tier/triage and "why does this matter" framing, see [`openspec/priorities.md`](openspec/priorities.md)._
_For historical record of implemented changes, run the `roadmap` skill (no `roadmap.md` yet)._
