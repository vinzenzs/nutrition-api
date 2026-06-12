## Continuity

_Forward plan for OpenSpec changes. Tracks **what's next**, **what's in flight**, and **what's queued**._
_Companion to `openspec/priorities.md` (tier/triage framing) — this file is the operational queue._
_Last refreshed: 2026-06-12 by the `continuity` skill (Garmin foundation — auth-token, bridge, mcp-login — all archived; queue is now the four-change Option B training-plan program in dependency order)._

## In progress

| Change | Branch | Started | Owner | Notes |
|---|---|---|---|---|
| _(none)_ | | | | |

_Nothing in flight. The four queued proposals are drafted but uncommitted on `main` — see Drift._

## Up next

Ordered queue — top is next to pick up. This is the **Option B training-plan program** (the Garmin auth/bridge/login foundation is shipped), ordered by dependency.

1. **add-workout-templates** — The ~40-session workout library (`WORKOUT_DEFS`) as structured steps (intents · time/distance/lap/open durations · HR/power-zone/pace/RPE targets · repeat groups) in JSONB. _Why now: offline foundation of Option B — no Garmin dependency; everything below references it._
2. **add-training-plan** — The 18-week plan as plan→weeks→slots→template, race/phase-anchored, with an idempotent `materialize` that expands it into planned `workouts`. Retires `Plan.md`. _Why now: depends on (1); the `WHERE status='planned'` materialize guard is already folded in so it composes with reconciliation._
3. **add-garmin-scheduling** — The write-to-watch edge: compile template steps → structured Garmin workout → schedule on the calendar (push workout / push plan-week / unschedule / read calendar). _Why now: needs (1)+(2); the bridge + `garmin-control` foundation it extends is already shipped; closes the outbound loop._
4. **add-workout-reconciliation** — Merge a completed Garmin import into its matching planned workout (planned→completed in place, keeping the prescription), with fulfill/unfulfill escape hatches. _Why now: needs (2) + the shipped bridge; closes the inbound loop so plan and actual stop double-listing._

## Backlog

Planned changes not yet prioritized.

- _Empty — all six in-flight changes are sequenced in In progress / Up next above._

## Notes

- **The active arc is the Garmin integration + Option B training plan.** Designed across two explore sessions. Four planes of the old `garmin.py` script move into api/mcp: ① auth (`add-garmin-auth-token` ✓ archived), ② read-import (`add-garmin-bridge` ✓ archived), ③ login (`add-garmin-mcp-login` ✓ archived), and the new program — workout library → plan-as-system-of-record → write-to-watch → reconcile (the four queued changes). The Garmin **foundation is fully shipped**; what remains is the training-plan program. Coaching synthesis (the old `coach` command) deliberately stays the **chat agent's** job, not an API endpoint.
  - **Option B was chosen** (backend owns the plan, not thin-control): the plan becomes a queryable, per-day-editable structure with structured (interval/zone) templates, so the watch gets real guided workouts and the fueling math can see upcoming load.
  - **One deferred primitive captured separately** (`add-workout-reconciliation`): matching completed↔planned. Its own follow-ons remain future: reverse-direction matching (activity imported before the plan existed), a ±1-day tolerance window, and full **plan-adherence analytics** (a capability that would sit on top of reconciliation).
- **Drift to clean up:**
  - **Four proposals are drafted but uncommitted on `main`** — `add-workout-templates`, `add-training-plan`, `add-garmin-scheduling`, `add-workout-reconciliation` (all validate `--strict`). Commit them as a `docs(openspec): propose …` batch before/at first apply.
  - **`roadmap.md` is stale** — `add-garmin-auth-token`, `add-garmin-bridge`, and `add-garmin-mcp-login` all archived 2026-06-12 since its last refresh; run the `roadmap` skill to resync.
  - **Stale branch to prune:** `feat/add-recommend-workout-fuel`, a leftover from an already-archived change — safe to delete when convenient (this skill never prunes branches).
  - Working tree also carries unrelated `.gitignore` + `Taskfile.yml` edits (from the bridge work) — not part of the proposal batch.
- **Previously shipped (now historical):** the chat + meal-planning + recipes arc is complete and archived (`add-recipe-ingredients`, `add-meal-plan`, `add-shopping-list`, `add-chat-backend`, `add-companion-chat`, `add-companion-food-picker`) — end-to-end "what should I eat → plan → one shopping list", Cookidoo recipes included. Two open follow-ups from it (not yet proposed): a manual companion-chat e2e on a device, and a real-Anthropic `/chat` smoke once `ANTHROPIC_API_KEY` is set (endpoint returns 503 `chat_unavailable` until then).
- **Still-open priorities-flagged work** (in `openspec/priorities.md`, independent of this arc): T2 #6E (retroactive freeform→product correction), #6F (`coach_recommendation` persistence — the rationale channel the in-app chat would read), #9 (supplement log); a derived sweat-rate (ml/hr) endpoint completing T2 #6C now its inputs exist; per-metric trend endpoints.
- **Pattern notes (carried):** MODIFIED spec deltas are full-replace — copy prior scenarios into the MODIFIED block, prefer ADDED requirements for additive intent (the Garmin-scheduling deltas use ADDED against the not-yet-archived bridge/control specs precisely to stay decoupled from archive order). OpenSpec requirement bodies must lead with a SHALL/MUST sentence or `validate --strict` rejects them. Spec sync + archive runs cleanly via `openspec archive <slug> --yes`. Migration head is `029` (after `add-garmin-auth-token`); the program's migrations are `030` templates → `031` plan+workouts-cols → `032` workout garmin-ids → optional `needs_link` — verify the head before each `migrate:new`, since out-of-band work can take the next slot.

---
_To update: ask Claude "update continuity", "queue X next", or "start work on X"._
_For tier/triage and "why does this matter" framing, see [`openspec/priorities.md`](openspec/priorities.md)._
_For the historical record of implemented changes, see [`roadmap.md`](roadmap.md)._
