## Continuity

_Forward plan for OpenSpec changes. Tracks **what's next**, **what's in flight**, and **what's queued**._
_Companion to `openspec/priorities.md` (tier/triage framing) — this file is the operational queue._
_Last refreshed: 2026-06-12 by the `continuity` skill (add-chat-sessions, add-companion-session-list, and add-plan-slot-targets all archived + merged to `main`; only `add-workout-reconciliation` remains planned)._

## In progress

| Change | Branch | Started | Owner | Notes |
|---|---|---|---|---|
| _(none)_ | | | | |

_Nothing in flight. `feat/add-chat-sessions` was fast-forward-merged into `main` (it held chat-sessions + plan-slot-targets + companion-session-list); the tree is clean. `main` is ahead of `origin/main` and unpushed._

## Up next

Ordered queue — top is next to pick up.

1. **add-workout-reconciliation** — Merge a completed Garmin import into its matching planned workout (planned→completed in place, keeping the prescription via `template_id`/`plan_slot_id`), with fulfill/unfulfill escape hatches. _Why now: the **last** open Option B follow-up; the plan + the bridge it needs are both shipped. Closes the inbound loop so a planned session and its actual stop double-listing._

## Backlog

Planned changes not yet prioritized.

- _Empty._

## Notes

- **The active arc is the Garmin integration + Option B training plan.** Designed across two explore sessions. Four planes of the old `garmin.py` script move into api/mcp: ① auth (`add-garmin-auth-token` ✓ archived), ② read-import (`add-garmin-bridge` ✓ archived), ③ login (`add-garmin-mcp-login` ✓ archived), and the new program — workout library → plan-as-system-of-record → write-to-watch → reconcile. The Garmin **foundation, the workout library, the plan, the write-to-watch edge, and per-slot pace progression are all shipped/archived**; the **only remaining** piece is reconciliation (`add-workout-reconciliation`). Coaching synthesis (the old `coach` command) deliberately stays the **chat agent's** job, not an API endpoint.
  - **Option B was chosen** (backend owns the plan, not thin-control): the plan becomes a queryable, per-day-editable structure with structured (interval/zone) templates, so the watch gets real guided workouts and the fueling math can see upcoming load.
  - **`add-plan-slot-targets` shipped** (`1a7ace2`): per-slot `target_overrides` let one template progress across the 18 weeks (e.g. interval at `7:15` = `435 sec/km`); `GET /workouts/{id}/program` resolves the effective steps, and the Garmin compile path was retrofitted to use them. Deliberately excluded: a `pace_zone` kind + time-varying threshold profile (a larger separate model) and duration overrides.
  - **One deferred primitive captured separately** (`add-workout-reconciliation`): matching completed↔planned. Its own follow-ons remain future: reverse-direction matching (activity imported before the plan existed), a ±1-day tolerance window, and full **plan-adherence analytics** (a capability that would sit on top of reconciliation).
- **Drift to clean up:**
  - **`main` is ahead of `origin/main` and unpushed** — push when ready (today's whole Garmin + Option B + chat-sessions arc is local-only).
  - **`add-workout-reconciliation` proposal is committed, awaiting apply** (validates `--strict`).
  - **Stale branches to prune:** `feat/add-chat-sessions` (now == `main` after the FF merge) and `feat/add-recommend-workout-fuel` (leftover from an archived change) — both safe to delete when convenient (this skill never prunes branches).
- **Previously shipped (now historical):** the chat + meal-planning + recipes arc is complete and archived (`add-recipe-ingredients`, `add-meal-plan`, `add-shopping-list`, `add-chat-backend`, `add-companion-chat`, `add-companion-food-picker`) — end-to-end "what should I eat → plan → one shopping list", Cookidoo recipes included. Two open follow-ups from it (not yet proposed): a manual companion-chat e2e on a device, and a real-Anthropic `/chat` smoke once `ANTHROPIC_API_KEY` is set (endpoint returns 503 `chat_unavailable` until then).
- **Still-open priorities-flagged work** (in `openspec/priorities.md`, independent of this arc): T2 #6E (retroactive freeform→product correction), #6F (`coach_recommendation` persistence — the rationale channel the in-app chat would read), #9 (supplement log); a derived sweat-rate (ml/hr) endpoint completing T2 #6C now its inputs exist; per-metric trend endpoints.
- **Pattern notes (carried):** MODIFIED spec deltas are full-replace — copy prior scenarios into the MODIFIED block, prefer ADDED requirements for additive intent (the Garmin-scheduling deltas use ADDED against the not-yet-archived bridge/control specs precisely to stay decoupled from archive order). OpenSpec requirement bodies must lead with a SHALL/MUST sentence or `validate --strict` rejects them. Spec sync + archive runs cleanly via `openspec archive <slug> --yes`. Migration head is now `034` on disk (`030` workout-templates, `031` training-plan, `032` workout garmin-ids, `033` chat-sessions, `034` plan-slot target_overrides). Remaining: `add-workout-reconciliation` → optional `needs_link` (would be `035`). Always verify the head before `migrate:new`, since out-of-band work can take the next slot — a `032` collision already happened this session (garmin-scheduling vs chat-sessions) and had to be renumbered.

---
_To update: ask Claude "update continuity", "queue X next", or "start work on X"._
_For tier/triage and "why does this matter" framing, see [`openspec/priorities.md`](openspec/priorities.md)._
_For the historical record of implemented changes, see [`roadmap.md`](roadmap.md)._
