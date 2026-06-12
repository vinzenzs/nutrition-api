## Continuity

_Forward plan for OpenSpec changes. Tracks **what's next**, **what's in flight**, and **what's queued**._
_Companion to `openspec/priorities.md` (tier/triage framing) — this file is the operational queue._
_Last refreshed: 2026-06-12 by the `continuity` skill (archived `add-shopping-list`, `add-chat-backend`, and `add-companion-chat` — the chat/meal-planning arc is fully shipped; the queue is now empty)._

## In progress

| Change | Branch | Started | Owner | Notes |
|---|---|---|---|---|
| _(none)_ | | | | |

_Nothing in flight._

## Up next

_Empty — every proposed change has shipped. The next pickup comes from `openspec/priorities.md`: propose a change with `/opsx:propose <slug>`, and it lands here._

## Backlog

_Empty — no proposed-but-unprioritized changes. New ideas live in `openspec/priorities.md` until proposed._

## Notes

- **The chat + meal-planning + recipes arc is COMPLETE.** All six proposals from the explore session are implemented and archived. End-to-end, the app now does "what should I eat today / the next 3 days → pick → plan + one consolidated shopping list", with Thermomix recipes pulled from Cookidoo. The pieces:
  - `add-recipe-ingredients` — server-side Cookidoo import + verbatim ingredient lists (`feat` `d3fc3da` / archive `78f547b`).
  - `add-meal-plan` — planned meals + the eaten→real-meal transition (`feat` `61bff4c` / archive `4686f10`).
  - `add-shopping-list` — the dumb-checklist primitive (`feat` `633b66d` / archive `1de88cd`).
  - `add-chat-backend` — server-side Anthropic SSE agent loop with loopback-HTTP tool dispatch, plus `PATCH /products/{id}` (`feat` `5c44ff7` / archive `9938907`).
  - `add-companion-chat` — Flutter chat screen + Today plan card + shopping-list screen (`feat` `6a99c30` / archive `c57c58e`).
  - `add-companion-food-picker` — camera recent/search/quick-create (code `81d00e3`, archive `127a935`).
- **Two follow-ups carried out of this arc** (not yet proposed; candidates for `openspec/priorities.md`):
  - **Manual e2e for the companion chat** (was task 5.2 of `add-companion-chat`, left open): on a device against a deployed backend — plan 3 dinners in chat → entries on Today → ate-it offline → replay → adherence updates; shopping check-off in airplane mode.
  - **Real-Anthropic smoke for `/chat`** (was task 4.4 of `add-chat-backend`): a live "plan 3 dinners" conversation once the server runs with `ANTHROPIC_API_KEY` set. The endpoint returns 503 `chat_unavailable` until the key is configured.
- **Everything sits on `main`.** No `feat/` branches this session — solo near-done work committed directly to `main` in the repo's two-commit-per-change rhythm (`feat(...)` then `chore(openspec): archive ...`). The working tree still carries the generated `apps/companion/devtools_options.yaml` (untracked tooling artifact, deliberately not committed) and the `continuity.md`/`roadmap.md` derived docs.
- **`roadmap.md` is stale** — five changes archived since its last refresh and it does not yet list them as implemented. Run the `roadmap` skill to resync the historical companion.
- **Stale branch to prune:** `feat/add-recommend-workout-fuel` is a leftover from an already-archived change — safe to delete when convenient (this skill never prunes branches itself).
- **Still-open priorities-flagged work** (in `openspec/priorities.md`): T2 #6E (retroactive freeform→product correction), #6F (`coach_recommendation` persistence — now especially relevant, as it's the rationale channel the in-app chat would read to ground "why this target today"), #9 (supplement log); a derived sweat-rate (ml/hr) endpoint that completes T2 #6C now its inputs exist; per-metric trend endpoints. The `garmin.py` importer (separate repo) wiring its existing fetches to the recovery / fitness / hydration-balance endpoints remains the highest-leverage out-of-repo move — the backend can store that data but nothing fills it yet.
- **Pattern notes (carried):** MODIFIED spec deltas are full-replace — copy prior scenarios into the MODIFIED block, prefer ADDED requirements for additive intent. Spec sync + archive runs cleanly via the `openspec archive <slug> --yes` CLI (auto-applies delta specs into `openspec/specs/` and moves the change dir; renames stage as `git mv`); no separate sync skill is needed. The chat loop's loopback dispatch (in-process `ServeHTTP` through the real middleware) is the pattern to reuse if a second agent surface is ever added.

---
_To update: ask Claude "update continuity", "queue X next", or "start work on X"._
_For tier/triage and "why does this matter" framing, see [`openspec/priorities.md`](openspec/priorities.md)._
_For the historical record of implemented changes, see [`roadmap.md`](roadmap.md)._
