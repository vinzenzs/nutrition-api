## Continuity

_Forward plan for OpenSpec changes. Tracks **what's next**, **what's in flight**, and **what's queued**._
_Companion to `openspec/priorities.md` (tier/triage framing) — this file is the operational queue._
_Last refreshed: 2026-06-09 by the `continuity` skill (no openspec/changes/ drift; `add-protein-distribution` ticked all 36 tasks since the last refresh — now alongside `add-rolling-window-summaries` in the "implemented, pending commit + archive" state)._

## In progress

This repo currently commits implementation work directly to `main` (single-user
project, no feature-branch flow yet). The Branch column reflects that — switch
to per-change branches when the cadence makes it worth it.

_Nothing in flight — pick from Up next._

## Up next

Ordered queue — top is next to pick up.

1. **add-meal-from-photo** — Backend-mediated Claude Vision integration so the Flutter app can log a meal from a photo. Mirrors the `off-integration` pattern (one API key, server-side; clients stay simple). _Why now: independent backend feature; the Flutter app's #2 killer interaction; touches different surfaces from the recent fueling + aggregator work._

## Backlog

Planned changes not yet prioritized.

- **add-protein-distribution** — `GET /summary/protein-distribution` returning per-meal `mps_effective: bool` (against the ~0.3 g × body_weight_kg per-meal MPS threshold) plus `gap_minutes_since_previous` and `logged_at_hour`. Closes T2 #7. _Implementation in-tree (`internal/summary/protein.go` + handler + MCP tool) with all 36 tasks ticked; uncommitted. Pending commit + `/opsx:archive add-protein-distribution`._
- **add-rolling-window-summaries** — `GET /summary/rolling?anchor_date=…&window_days=N`. Multi-day averages for the metrics that are actually multi-day phenomena: protein for MPS (~1.6–2.2 g/kg/day across a week), Energy Availability (5–14 day Loucks bands), 72-hour carb-load window, weekly sodium baseline. One bad day is noise; the rolling view is the signal. _Implemented + committed (8612f56) but not yet archived — pending `/opsx:archive add-rolling-window-summaries` and the §11.3 manual e2e._
- **add-flutter-companion-app** — Three-screen Flutter app (barcode / photo / hydration widget) as a focused supplement to the agent. _Caveat: predates the endurance-training pivot — see Meta #3 in `openspec/priorities.md`. Worth a short explore session on whether the three killer interactions still match today's most-pressing use._

## Notes

- **T1 list fully delivered**: #1, #1A, #1B, #2, #3, #4, #5 all shipped by 2026-06-09. Tier-2 work is the active surface.
- **T2 closures landing fast**: #6B (`daily_context` aggregator) shipped + archived 2026-06-09. #1B (`rolling_summary`) and #7 (`protein_distribution`) both implemented but sitting in the "pending commit + archive" Backlog row — process choreography only, no implementation work left.
- **Pending-archive backlog**: two changes (`add-protein-distribution`, `add-rolling-window-summaries`) are coded + tasks-ticked but their openspec directories haven't moved to `archive/` yet. A consolidated cleanup commit + archive run would close them out in one pass. Worth a quick pass before queuing `add-meal-from-photo` for implementation, so the queue mental model matches the file system.
- **Decisions pending** (do not queue yet): T2 #6F (`coach_recommendation` persistence) — tests the synthesis principle, deliberate discussion first.
- **Remaining priorities-flagged work**: T2 #10 (`recommend_workout_fuel`), T2 #6A (sleep/HRV log), T2 #6C (sweat-rate test workflow), T2 #6D (GI distress / RPE on workout fueling), T2 #6E (retroactive freeform→product correction), T2 #8 (caffeine), T2 #9 (supplement log). T2 #6C is the cheapest now that workouts + weight + workout-fuel all exist.

---
_To update: ask Claude "update continuity", "queue X next", or "start work on X"._
_For tier/triage and "why does this matter" framing, see [`openspec/priorities.md`](openspec/priorities.md)._
_For historical record of implemented changes, see [`roadmap.md`](roadmap.md)._
