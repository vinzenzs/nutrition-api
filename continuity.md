## Continuity

_Forward plan for OpenSpec changes. Tracks **what's next**, **what's in flight**, and **what's queued**._
_Companion to `openspec/priorities.md` (tier/triage framing) — this file is the operational queue._
_Last refreshed: 2026-06-08 by the `continuity` skill (verified against `openspec/changes/` — no drift; queue unchanged since the `add-energy-availability` archive)._

## In progress

This repo currently commits implementation work directly to `main` (single-user
project, no feature-branch flow yet). The Branch column reflects that — switch
to per-change branches when the cadence makes it worth it.

_Nothing in flight — pick from Up next._

## Up next

Ordered queue — top is next to pick up.

1. **add-meal-from-photo** — Backend-mediated Claude Vision integration so the Flutter app can log a meal from a photo. Mirrors the `off-integration` pattern (one API key, server-side; clients stay simple). _Why now: independent backend feature; the Flutter app's #2 killer interaction; touches different surfaces from the just-shipped workout-fuel work._

## Backlog

Planned changes not yet prioritized.

- **add-carb-load-auto-apply** — Extend `plan_carb_load` with an `apply: true` flag that writes the computed per-date carb targets straight into `daily_goal_overrides` atomically, returning a per-date `applied` outcome alongside the schedule. Closes T1 #3 in priorities; the pure-compute default path is unchanged. _Why now: the original deferral was "pending usage data showing the friction is real" — after the recent archive cycle, every race-prep workflow runs compute-then-loop. The data exists._
- **add-flutter-companion-app** — Three-screen Flutter app (barcode / photo / hydration widget) as a focused supplement to the agent. _Caveat: predates the endurance-training pivot — see Meta #3 in `openspec/priorities.md`. Worth a short explore session on whether the three killer interactions still match today's most-pressing use._

## Notes

- **Decisions pending** (do not queue yet): T1 #5 (templates) vs T1 #1A (training-phase) — same question, two answers. T2 #6F (`coach_recommendation` persistence) — tests the synthesis principle, deliberate discussion first.
- **Waiting on usage data**: T1 #5 (templates) revisits after first multi-week training block planned. (T1 #3 carb-load auto-apply moved out of "waiting" — proposal `add-carb-load-auto-apply` is now in Backlog.)

---
_To update: ask Claude "update continuity", "queue X next", or "start work on X"._
_For tier/triage and "why does this matter" framing, see [`openspec/priorities.md`](openspec/priorities.md)._
_For historical record of implemented changes, see [`roadmap.md`](roadmap.md)._
