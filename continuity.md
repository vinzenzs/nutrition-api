## Continuity

_Forward plan for OpenSpec changes. Tracks **what's next**, **what's in flight**, and **what's queued**._
_Companion to `openspec/priorities.md` (tier/triage framing) — this file is the operational queue._
_Last refreshed: 2026-06-08 by the `continuity` skill (post-archive of `add-carb-load-auto-apply` — T1 #3 closed; `add-rolling-window-summaries` newly proposed and added to Backlog)._

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

- **add-rolling-window-summaries** — `GET /summary/rolling?anchor_date=…&window_days=N`. Multi-day averages for the metrics that are actually multi-day phenomena: protein for MPS (~1.6–2.2 g/kg/day across a week), Energy Availability (5–14 day Loucks bands), 72-hour carb-load window, weekly sodium baseline. One bad day is noise; the rolling view is the signal. _Newly proposed — read the proposal before queuing; sizing not yet pinned._
- **add-flutter-companion-app** — Three-screen Flutter app (barcode / photo / hydration widget) as a focused supplement to the agent. _Caveat: predates the endurance-training pivot — see Meta #3 in `openspec/priorities.md`. Worth a short explore session on whether the three killer interactions still match today's most-pressing use._

## Notes

- **T1 closed today**: T1 #3 (`plan_carb_load` auto-apply) shipped via `add-carb-load-auto-apply` and was archived 2026-06-08. The Tier-1 gap list in `openspec/priorities.md` is now empty as far as I can see — worth a re-read to confirm before queuing the next tier-1 item.
- **Decisions pending** (do not queue yet): T1 #5 (templates) vs T1 #1A (training-phase) — same question, two answers. T2 #6F (`coach_recommendation` persistence) — tests the synthesis principle, deliberate discussion first.
- **Waiting on usage data**: T1 #5 (templates) revisits after first multi-week training block planned.
- **Uncommitted prior work**: roadmap.md still shows several 2026-06-08 archives plus the new `internal/energy/` and `internal/workoutfuel/` packages as `_uncommitted_`. A cleanup commit before more drift accumulates would unblock clean roadmap entries.

---
_To update: ask Claude "update continuity", "queue X next", or "start work on X"._
_For tier/triage and "why does this matter" framing, see [`openspec/priorities.md`](openspec/priorities.md)._
_For historical record of implemented changes, see [`roadmap.md`](roadmap.md)._
