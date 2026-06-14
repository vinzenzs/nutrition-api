## Continuity

_Forward plan for OpenSpec changes. Tracks **what's next**, **what's in flight**, and **what's queued**._
_Companion to `openspec/priorities.md` (tier/triage framing) — this file is the operational queue._
_Last refreshed: 2026-06-14 by the `continuity` skill (`expand-chat-to-coach` is in flight on `main`; `rebrand-to-kazper` added to the queue)._

## In progress

| Change | Branch | Started | Owner | Notes |
|---|---|---|---|---|
| expand-chat-to-coach | `main` | 2026-06-14 | Vinzenz | 24/28 tasks; **uncommitted on `main`, no feature branch** (`tasks.md` + companion app files) |

_Root of the chat→coach arc — introduces `internal/agenttools` as the shared tool surface the other three changes build on. Work is happening directly on `main` rather than a `feat/…` branch._

## Up next

Ordered queue — top is next to pick up. Order here is **dependency-derived, not a priority call** — reorder freely. All three hang off `expand-chat-to-coach` (in progress).

1. **add-coach-context-endpoints** — `Why:` the in-app coach needs to **ground** training/recovery advice the way `get_daily_context` grounds nutrition, but no aggregate read exists — data is spread across ~30 granular Garmin-mirror tools, costly to put in front of the model each round. _Depends on `expand-chat-to-coach` phase 3._
2. **unify-mcp-tool-registry** — `Why:` `expand-chat-to-coach` makes `internal/agenttools` the single source of truth but only `internal/chat` consumes it; the MCP server still hand-maintains its 123 tool registrations behind a name-level drift-guard. Generate the MCP surface from the shared registry so chat + desktop coach are the same tools by construction. _Depends on `agenttools` (from `expand-chat-to-coach`)._
3. **rebrand-to-kazper** — `Why:` the project name `nutrition-api` describes a backend, not the product; the product is **Kazper**, an endurance-fueling + training coach the app embodies. Rebrands the user-facing identity (and the coach persona) to Kazper without disturbing internal plumbing. _Depends on `expand-chat-to-coach` phase 3 (the coach persona it renames)._

## Backlog

Planned changes not yet prioritized.

- _Empty — all planned changes are queued or in progress above._

## Notes

- **New arc — chat→coach unification (4 changes, 0 shipped; root in flight).** Deepens the prior chat-sessions + Garmin foundation rather than re-plumbing it. `expand-chat-to-coach` is the dependency root (introduces `internal/agenttools`, 24/28 tasks, **uncommitted on `main`**); `add-coach-context-endpoints`, `unify-mcp-tool-registry`, and `rebrand-to-kazper` all hang off it. The first two can land in either order once it ships; `rebrand-to-kazper` renames the coach persona phase 3 introduces. Per-change decision provenance lives in each `proposal.md` / `design.md`.
- **The "mirror everything" Garmin arc: COMPLETE — 8/8 shipped + archived** (`add-garmin-{workout-detail,daily-energy,gear-and-prs,athlete-config,misc-mirror,history-backfill,sync-rolling-lookback}` + `garmin-workout-library-mgmt`, plus `extend-recovery-fitness`). Garminconnect's whole surface is mirrored into the API/MCP. Migrations 036–041 landed in order; head is `041` on disk. Re-verify the head before any future `task migrate:new`.
- **The PRIOR Garmin + Option B training-plan arc is COMPLETE and archived** — auth, read-import, login, workout-templates → training-plan → garmin-scheduling → plan-slot-targets → workout-reconciliation, plus `fix-chat-tool-status-chips`. Coaching synthesis stays the chat agent's job, not an API endpoint.
- **Drift to clean up (carried):**
  - **`main` is well ahead of `origin/main` and unpushed** — the whole prior Garmin + Option B + chat-sessions arc is local-only. Push when ready.
  - **Stale branches to prune:** `feat/add-chat-sessions` (now == `main`) and `feat/add-recommend-workout-fuel` (leftover) — both safe to delete when convenient (this skill never prunes branches).
- **Open follow-ups from prior arcs (not proposed):** manual on-device smoke for `fix-chat-tool-status-chips`; reverse-direction workout reconciliation + ±1-day tolerance + plan-adherence analytics; a real-Anthropic `/chat` smoke once `ANTHROPIC_API_KEY` is set (503 `chat_unavailable` until then); the derived sweat-rate (ml/hr) endpoint (T2 #6C) — now buildable since `add-garmin-workout-detail` supplies time-in-zone, elevation, weather.
- **Still-open priorities-flagged work** (in `openspec/priorities.md`, independent of this arc): T2 #6E (retroactive freeform→product correction), #6F (`coach_recommendation` persistence), #9 (supplement log).
- **Pattern notes (carried):** MODIFIED spec deltas are full-replace — copy prior scenarios into the MODIFIED block; prefer ADDED requirements for additive intent. OpenSpec requirement bodies must lead with a SHALL/MUST sentence or `validate --strict` rejects them. The `openspec instructions … --json` command prints a progress line before the JSON — strip with `sed -n '/^{/,$p'` before parsing.

---
_To update: ask Claude "update continuity", "queue X next", or "start work on X"._
_For tier/triage and "why does this matter" framing, see [`openspec/priorities.md`](openspec/priorities.md)._
_For the historical record of implemented changes, see [`roadmap.md`](roadmap.md)._
