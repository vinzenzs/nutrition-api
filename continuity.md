## Continuity

_Forward plan for OpenSpec changes. Tracks **what's next**, **what's in flight**, and **what's queued**._
_Companion to `openspec/priorities.md` (tier/triage framing) — this file is the operational queue._
_Last refreshed: 2026-06-12 by the `continuity` skill (workout-templates + training-plan + garmin-scheduling all archived; `add-chat-sessions` now in implementation → In progress; queue is the two remaining Option B follow-ups)._

## In progress

| Change | Branch | Started | Owner | Notes |
|---|---|---|---|---|
| add-chat-sessions | `feat/add-chat-sessions` | 2026-06-12 | Vinzenz | Implemented — `internal/chatsessions/`, migration `033` (`032` was taken by garmin-scheduling), `internal/chat/` session-backed rework. Artifacts complete, tests green, docs regenerated. `/opsx:archive` when committed. |

## Up next

Ordered queue — top is next to pick up. The remaining **Option B training-plan program** (workout library + plan + the write-to-watch edge are all shipped), ordered by dependency.

1. **add-plan-slot-targets** — Per-slot target overrides so one template progresses across the plan (e.g. the same tempo run at 7:30 → 7:15 → 7:00) without authoring a template per pace; `GET /workouts/{id}/program` exposes the resolved steps. _Why now: depends only on shipped changes. ⚠️ `add-garmin-scheduling` already shipped reading **raw** template steps, so this change's task 6.1 must **retrofit** that compile path to use effective (override-resolved) steps — the clean seam was missed by the fast concurrent build._
2. **add-workout-reconciliation** — Merge a completed Garmin import into its matching planned workout (planned→completed in place, keeping the prescription), with fulfill/unfulfill escape hatches. _Why now: needs the plan + the shipped bridge; closes the inbound loop so plan and actual stop double-listing._

## Backlog

Planned changes not yet prioritized.

- _Empty — `add-chat-sessions` moved to In progress; the two Option B follow-ups are queued above._

## Notes

- **The active arc is the Garmin integration + Option B training plan.** Designed across two explore sessions. Four planes of the old `garmin.py` script move into api/mcp: ① auth (`add-garmin-auth-token` ✓ archived), ② read-import (`add-garmin-bridge` ✓ archived), ③ login (`add-garmin-mcp-login` ✓ archived), and the new program — workout library → plan-as-system-of-record → write-to-watch → reconcile. The Garmin **foundation, the workout library, the plan, and the write-to-watch edge are all shipped/archived**; what remains is per-slot pace progression (`add-plan-slot-targets`) and reconciliation (`add-workout-reconciliation`). Coaching synthesis (the old `coach` command) deliberately stays the **chat agent's** job, not an API endpoint.
  - **Option B was chosen** (backend owns the plan, not thin-control): the plan becomes a queryable, per-day-editable structure with structured (interval/zone) templates, so the watch gets real guided workouts and the fueling math can see upcoming load.
  - **`add-plan-slot-targets` is the pace-progression follow-up**: pace is already a template-step target (e.g. `7:15` = `435 sec/km`); the slot override lets one template progress across the 18 weeks. Deliberately excludes a `pace_zone` kind + time-varying threshold profile (a larger separate model) and duration overrides.
  - **One deferred primitive captured separately** (`add-workout-reconciliation`): matching completed↔planned. Its own follow-ons remain future: reverse-direction matching (activity imported before the plan existed), a ±1-day tolerance window, and full **plan-adherence analytics** (a capability that would sit on top of reconciliation).
- **Drift to clean up:**
  - **`add-chat-sessions` is mid-implementation, uncommitted** (In progress) — `internal/chatsessions/`, migration `032`, `internal/chat/` edits on the tree. Its OpenSpec proposal artifacts are still thin (commit the proposal too when committing the feature).
  - **Proposals committed, awaiting apply:** `add-plan-slot-targets`, `add-workout-reconciliation` (both validate `--strict`).
  - **`roadmap.md` resynced** alongside this refresh — workout-templates/training-plan/garmin-scheduling now under Implemented.
  - **Stale branch to prune:** `feat/add-recommend-workout-fuel`, a leftover from an already-archived change — safe to delete when convenient (this skill never prunes branches).
- **Previously shipped (now historical):** the chat + meal-planning + recipes arc is complete and archived (`add-recipe-ingredients`, `add-meal-plan`, `add-shopping-list`, `add-chat-backend`, `add-companion-chat`, `add-companion-food-picker`) — end-to-end "what should I eat → plan → one shopping list", Cookidoo recipes included. Two open follow-ups from it (not yet proposed): a manual companion-chat e2e on a device, and a real-Anthropic `/chat` smoke once `ANTHROPIC_API_KEY` is set (endpoint returns 503 `chat_unavailable` until then).
- **Still-open priorities-flagged work** (in `openspec/priorities.md`, independent of this arc): T2 #6E (retroactive freeform→product correction), #6F (`coach_recommendation` persistence — the rationale channel the in-app chat would read), #9 (supplement log); a derived sweat-rate (ml/hr) endpoint completing T2 #6C now its inputs exist; per-metric trend endpoints.
- **Pattern notes (carried):** MODIFIED spec deltas are full-replace — copy prior scenarios into the MODIFIED block, prefer ADDED requirements for additive intent (the Garmin-scheduling deltas use ADDED against the not-yet-archived bridge/control specs precisely to stay decoupled from archive order). OpenSpec requirement bodies must lead with a SHALL/MUST sentence or `validate --strict` rejects them. Spec sync + archive runs cleanly via `openspec archive <slug> --yes`. Migration head is now `032` on disk (`030` workout-templates, `031` training-plan + workouts cols, `032` taken — `add-garmin-scheduling` shipped its garmin-ids migration AND `add-chat-sessions` is adding chat-sessions; **two changes contended for `032`** — verify/renumber). Remaining migrations: `add-plan-slot-targets` → `plan_slots.target_overrides`; `add-workout-reconciliation` → optional `needs_link`. Always verify the head before `migrate:new`, since out-of-band work can take the next slot.

---
_To update: ask Claude "update continuity", "queue X next", or "start work on X"._
_For tier/triage and "why does this matter" framing, see [`openspec/priorities.md`](openspec/priorities.md)._
_For the historical record of implemented changes, see [`roadmap.md`](roadmap.md)._
