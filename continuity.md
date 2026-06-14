## Continuity

_Forward plan for OpenSpec changes. Tracks **what's next**, **what's in flight**, and **what's queued**._
_Companion to `openspec/priorities.md` (tier/triage framing) — this file is the operational queue._
_Last refreshed: 2026-06-14 by the `continuity` skill (chat→coach arc 3/4 shipped + archived; only `rebrand-to-kazper` remains, now unblocked)._

## In progress

| Change | Branch | Started | Owner | Notes |
|---|---|---|---|---|
| _(none)_ | | | | |

_Nothing in flight; tree clean on `main`._

## Up next

Ordered queue — top is next to pick up.

1. **rebrand-to-kazper** — `Why:` the project name `kazper` describes a backend, not the product; the product is **Kazper**, an endurance-fueling + training coach the app embodies. Rebrands the user-facing identity (and the in-app coach persona) to Kazper without disturbing internal plumbing. _Why now: **unblocked** — its dependency (`expand-chat-to-coach` phase 3, the generic coach persona) has shipped; this is the last piece of the chat→coach arc._

## Backlog

Planned changes not yet prioritized.

- _Empty._

## Notes

- **The chat→coach unification arc is 3/4 shipped + archived** (all 2026-06-14): `expand-chat-to-coach` (the planner became the unified coach — shared `internal/agenttools` registry + tiered pause/resume write-confirm + companion proposal card), `add-coach-context-endpoints` (`/context/training` + `/context/recovery` grounding reads, dual-surface), and `unify-mcp-tool-registry` (the MCP server's entire 128-tool surface generated from `agenttools` via one generic dispatcher; name-level drift guard retired; announced surface registry-derived; only `log_meal_from_photo` stays bespoke for multipart). **`rebrand-to-kazper` is the only remaining piece** — it renames the coach persona phase 3 introduced.
- **The "mirror everything" Garmin arc: COMPLETE — archived** (`add-garmin-{workout-detail,daily-energy,gear-and-prs,athlete-config,misc-mirror,history-backfill,sync-rolling-lookback}` + `garmin-workout-library-mgmt`, plus `extend-recovery-fitness`). Migrations 036–041 landed; head is `041` on disk. Re-verify the head before any future `task migrate:new`.
- **The PRIOR Garmin + Option B training-plan arc is COMPLETE and archived** — auth, read-import, login, workout-templates → training-plan → garmin-scheduling → plan-slot-targets → workout-reconciliation, plus `fix-chat-tool-status-chips`.
- **Drift to clean up (carried):**
  - **`main` is well ahead of `origin/main` and unpushed** — the whole prior Garmin + Option B + chat-sessions + chat→coach arc is local-only. Push when ready.
  - **`roadmap.md` is stale** — the 3 chat→coach changes archived today (plus the earlier Garmin arc) aren't reflected; run the `roadmap` skill to refresh.
  - **Stale branches to prune:** `feat/add-chat-sessions` (now == `main`) and `feat/add-recommend-workout-fuel` (leftover) — both safe to delete when convenient (this skill never prunes branches).
- **Open follow-ups from prior arcs (not proposed):** the deferred mcpserver→registry full port is now DONE (`unify-mcp-tool-registry`); remaining: manual on-device smoke for `fix-chat-tool-status-chips` + `expand-chat-to-coach` phase 4 (4.6); reverse-direction workout reconciliation + ±1-day tolerance + plan-adherence analytics; a real-Anthropic `/chat` smoke once `ANTHROPIC_API_KEY` is set (503 `chat_unavailable` until then); the derived sweat-rate (ml/hr) endpoint (T2 #6C).
- **Still-open priorities-flagged work** (in `openspec/priorities.md`, independent of this arc): T2 #6E (retroactive freeform→product correction), #6F (`coach_recommendation` persistence), #9 (supplement log).
- **Pattern notes (carried):** MODIFIED spec deltas are full-replace — copy prior scenarios into the MODIFIED block; prefer ADDED requirements for additive intent. OpenSpec requirement bodies must lead with a SHALL/MUST sentence or `validate --strict` rejects them. The `openspec instructions … --json` command prints a progress line before the JSON — strip with `sed -n '/^{/,$p'` before parsing.

---
_To update: ask Claude "update continuity", "queue X next", or "start work on X"._
_For tier/triage and "why does this matter" framing, see [`openspec/priorities.md`](openspec/priorities.md)._
_For the historical record of implemented changes, see [`roadmap.md`](roadmap.md)._
