## Continuity

_Forward plan for OpenSpec changes. Tracks **what's next**, **what's in flight**, and **what's queued**._
_Companion to `openspec/priorities.md` (tier/triage framing) — this file is the operational queue._
_Last refreshed: 2026-06-09 by the `continuity` skill (four big closures since the last refresh — `add-recommend-workout-fuel` (T2 #10), `add-rolling-window-summaries` (T1 #1B), `add-meal-from-photo` (Flutter killer #2), and `add-workout-rpe-and-fueling` (T2 #6D) all archived. Queue is now effectively empty)._

## In progress

Currently back on `main` — no per-change branch active. The first branched
flow (`feat/add-recommend-workout-fuel` earlier today) merged cleanly; future
work can either branch again or commit directly, depending on the shape.

_Nothing in flight — pick from Up next._

## Up next

Ordered queue — top is next to pick up.

_Up next is empty — see Backlog or `openspec/priorities.md` for the next move._

## Backlog

Planned changes not yet prioritized.

- **add-flutter-companion-app** — Three-screen Flutter app (barcode / photo / hydration widget) as a focused supplement to the agent. _Caveat: predates the endurance-training pivot — see Meta #3 in `openspec/priorities.md`. Worth a short explore session on whether the three killer interactions still match today's most-pressing use, especially now that `add-meal-from-photo` has shipped and the workouts/fueling story is more complete than when the original Flutter proposal was written._

## Notes

- **T1 list fully delivered**: #1, #1A, #1B, #2, #3, #4, #5 all shipped by 2026-06-09. Tier-2 work has been the active surface today.
- **T2 closures (all 2026-06-09)**: #6B (`daily_context` aggregator), #7 (`protein_distribution`), #10 (`recommend_workout_fuel`), #6D (RPE + GI distress on workouts, via `add-workout-rpe-and-gi`). Plus `add-meal-from-photo` (the Flutter app's killer interaction #2) and `add-rolling-window-summaries` (the cheapest remaining T1 add) both archived. Today shipped roughly twice as many changes as any prior session.
- **Decisions pending** (do not queue yet): T2 #6F (`coach_recommendation` persistence) — tests the synthesis principle, deliberate discussion first. Still un-touched.
- **Remaining priorities-flagged work**: T2 #6A (sleep / HRV log — natural "morning metrics" pair with weight), T2 #6C (sweat-rate test workflow — cheap now that workouts + weight + workout-fuel all exist), T2 #6E (retroactive freeform→product correction — small primitive, data-quality story), T2 #8 (caffeine — might be subsumed by workout-fuel's existing `caffeine_mg` field; worth a 5-minute audit), T2 #9 (supplement log). With six T2 items already closed today, the remaining list is closer to "second-priority polish" than "blocking surface."
- **Uncommitted archive moves**: today's archives haven't all been bundled into cleanup commits yet. A consolidated commit of the archive directory moves + the synced main specs would close the loop and let `roadmap.md` stop showing them as `_uncommitted_`.
- **Pattern note on MODIFIED spec deltas**: today's `add-workout-rpe-and-gi` archive surfaced a real pitfall — the openspec MODIFIED requirement is full-replace, so any delta that lists only the new scenarios silently drops the prior ones at sync time. Caught + repaired before commit. Worth folding into the propose skill's design template next time it's edited: when the intent is *additive* (new scenarios only), prefer phrasing the delta as a new ADDED sub-requirement rather than a MODIFIED block, OR include the full prior content verbatim in the MODIFIED block.

---
_To update: ask Claude "update continuity", "queue X next", or "start work on X"._
_For tier/triage and "why does this matter" framing, see [`openspec/priorities.md`](openspec/priorities.md)._
_For historical record of implemented changes, see [`roadmap.md`](roadmap.md)._
