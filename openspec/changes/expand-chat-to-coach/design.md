## Context

`/chat` (capability `nutrition-chat`, package `internal/chat/`) runs a server-side Anthropic agent loop, streams SSE, and persists durable sessions (`chat_sessions`/`chat_messages`, full content-block fidelity per turn). Its inner loop dispatches every requested tool **inline and synchronously** within one SSE turn (`service.go` ~L174–L181): agent decides → `dispatcher.execute` fires → `tool_result` feeds straight back to Anthropic. The tool allowlist (`tools.go` `registry()`) is a hand-written ~14-tool nutrition-planning set; the system prompt (`prompt.go`) restricts scope to food and redirects everything else to "the desktop coaching agent."

Separately, `internal/mcpserver/` exposes ~50 tools (the full coach surface) to an external MCP client over stdio. Each MCP tool also does exactly one loopback HTTP call under the caller's bearer (`apiClient` + `toToolResult`). The two registries are structurally identical but maintained independently.

The user wants: one unified in-app coach (planner subsumed), the full write surface from mobile, and a human-in-the-loop confirm on consequential writes — but the existing fast meal-planning writes should keep firing without a tap.

Constraints from the codebase:
- One package per capability; sentinel errors map 1:1 to error codes; `Register(rg *gin.RouterGroup)`; integration tests against testcontainers Postgres.
- Tools dispatch as loopback HTTP under the caller's bearer; write tools derive an idempotency key **from conversation position** (`deriveIdempotencyKey`) so a retried turn replays rather than duplicates.
- `sanitizeHistory`/`opensCleanly` currently **drop** any turn that opens with a dangling `tool_use`, treating it as truncation garbage.
- The four SSE event types and `done.stop_reason` values are a client contract.

## Goals / Non-Goals

**Goals:**
- One shared tool-spec registry consumed by both `internal/mcpserver` and `internal/chat`; no second hand-written allowlist.
- `/chat` exposes the full coach surface and a coaching system prompt; the planner flow is a retained subset.
- A per-tool **tier** (`read` | `write-auto` | `write-confirm`) that drives whether a write pauses for confirmation.
- Pause-and-resume confirmation: a `proposal` SSE event + `awaiting_confirmation` stop reason + a `POST /chat/sessions/{id}/confirm` resume endpoint, all reusing the existing session store.
- The companion Chat screen renders the proposal as an approve/reject card and resumes via the endpoint.

**Non-Goals:**
- No held-open connection / side-channel approval (rejected below).
- No second chat surface, no per-session mode toggle.
- No new tables / migration (paused state is derivable from the trailing turn).
- No change to MCP server external behavior; no change to EA/summary unit isolation.
- No streaming-resume of a half-finished *upstream* turn; resume granularity is "a completed assistant turn that proposed writes."

## Decisions

### D1 — Confirmation mechanism: pause-and-resume across stateless turns (not a held connection)

When the loop reaches an assistant turn that contains a `write-confirm` call, it ends the turn early: emit `proposal`, persist the assistant turn (its `tool_use` blocks, **no** `tool_result` yet), and send `done` with `stop_reason: "awaiting_confirmation"`. The HTTP request completes. The session now sits with a trailing assistant turn whose `tool_use` blocks are unanswered.

The alternative — hold the SSE stream open and block the server goroutine on an approval channel fed by a side-channel request — was rejected: it pins a connection open for human-think-time (minutes), breaks when the phone backgrounds or loses signal, and needs a cross-request registry of pending approvals. Pause-and-resume is a near-perfect fit because `/chat` is **already** stateless-per-turn with fully persisted sessions; "wait for a human" becomes "end the turn, start the next one."

### D2 — Confirmation is decided at the **turn** level, not the individual-call level

Anthropic requires every `tool_use` block in an assistant turn to be answered (by a `tool_result`) before the next user message. So the loop cannot dispatch some calls in a turn and leave others pending across an HTTP boundary. Rule: **a turn auto-executes only if *all* its tool calls are `read`/`write-auto`; if any call is `write-confirm`, the entire turn pauses** and the `proposal` lists every write in it. On resume, the approved calls execute in order, declined calls get a synthetic `tool_result` ("the user declined this action"), and the loop continues. This also yields the nicer UX of one batch card ("I'm about to: schedule 3 workouts, update your protein goal — Approve / Reject per item").

Mixed turns where an auto write shares a turn with a confirm write are rare; the auto write simply waits for the same confirmation. Acceptable and safe.

### D3 — Tool **tier** lives on the shared spec; classification is explicit

The shared tool spec gains `tier: read | write-auto | write-confirm`. Reads are always `read`. `write-auto` = the existing planner writes (`import_cookidoo_recipe`, `update_product`, `create_planned_meal`, `update_planned_meal`, `mark_planned_meal_eaten`, `add_shopping_items`, `update_shopping_item`, `clear_checked_shopping_items`) — they already fire from mobile today and stay fast. `write-confirm` = everything newly added from the coach surface that changes training/goals or deletes anything (goal/override writes, training-plan and workout-template edits, Garmin scheduling, workout/meal/hydration logging mutations, all deletes). The MCP server ignores `tier` (the desktop coach has its own client-side trust model); only the chat loop reads it.

### D4 — Shared registry: extract, don't duplicate

Introduce a registry both packages consume (working name `internal/agenttools/`). Each entry carries: tool `name`, `description`, JSON-Schema `input`, `tier`, and the `build(input) -> httpCall` mapping (method/path/query/body). `internal/chat` builds Anthropic tool defs + dispatches from it (adding the tier gate); `internal/mcpserver` builds MCP tool registrations from the same entries. This is a pure refactor with **no external behavior change** — verified by the existing `mcp_integration_test.go` expected-tools list and the chat tool tests. Ship it first so the later changes ride on one list.

**D4 — amended at apply time (2026-06-14).** The design estimated `internal/mcpserver` at ~50 tools; the actual surface is **127 tool registrations backed by 201 unit tests**. A full port of mcpserver onto the shared registry — deleting ~25 typed handler files, invalidating/rewriting those 201 tests, and replacing the SDK's reflection-derived schemas with 127 hand-authored JSON-Schema strings — is the highest-risk, zero-user-value slice of the arc and would churn the *working* desktop coach for no functional gain. The chat-as-coach value (phases 2–4) only needs **chat** to consume the shared registry. Decision: in this change, introduce `internal/agenttools` as the single source of truth and have **`internal/chat`** consume it; **defer** mcpserver's consumption to a separate future change. Anti-drift is guarded by a test asserting every `agenttools` tool name exists in mcpserver's announced surface (`internal/mcpserver.AnnouncedToolNames`), modulo a documented allowlist of chat-bespoke convenience tools (`get_daily_context`, `get_race_fueling`, `get_product`, `update_product`) whose names intentionally differ from the MCP equivalents. The full bidirectional unification (mcpserver also consuming `agenttools`) remains the eventual goal expressed by the `nutrition-chat` spec's "One registry, no drift" requirement.

### D5 — Resume endpoint: `POST /chat/sessions/{id}/confirm`, streams SSE

Body: `{decisions: [{tool_id: string, approve: bool}]}` covering every `write-confirm` call in the paused turn (auto calls in the same turn need no decision; they execute on resume). Preconditions: session exists (`404 session_not_found`); its trailing turn is an unanswered assistant `tool_use` turn awaiting confirmation (else `409 nothing_to_confirm`); decisions cover exactly the pending confirm calls (else `400 invalid_confirmation`). On success it dispatches (approved calls + any auto calls in the turn, in original order), appends one user turn with all `tool_result` blocks, and **resumes the same agent loop**, streaming `text`/`tool`/`proposal`/`done`/`error` exactly like `/chat`. Idempotency keys are **content-derived** — `sha256(toolName | canonical-JSON(input))` (existing `deriveIdempotencyKey`), not positional — so a re-sent confirm re-dispatches the identical call, hits the same key, and replays rather than double-writes; no positional bookkeeping is needed.

**Resume is state-driven and idempotent at the turn level.** The endpoint inspects the session's trailing turn: (a) an awaiting-confirmation assistant `tool_use` turn → consume `decisions`, execute, append `tool_result`s, continue; (b) a trailing `tool_result` turn with no following assistant (a continuation owed because a prior resume's upstream call died mid-stream) → ignore `decisions` and just continue the loop; (c) a trailing assistant *text* turn → `409 nothing_to_confirm`. This means a confirm whose continuation stream dies after the writes already committed is recovered by **re-POSTing the same confirm** — it re-enters at (b) and finishes the answer, without a third "continue" endpoint and without re-executing the writes.

### D6 — `proposal` is a fifth SSE event (not an overloaded `tool` status)

`proposal` payload: `{turn_id, calls: [{tool_id, name, tier, preview}]}` where `preview` is a short human-readable description of the pending write (e.g. "Schedule a 90-min Z2 ride on 2026-06-20") — richer than the `tool` event's `summary`, and the only place the loop describes a write *before* it happens. Kept distinct from `tool` so the existing started/ok/error chip logic is untouched. `tool` events still never carry raw request/response bodies; `preview` is a server-composed human string, not the raw body.

**The preview is CODE-composed from the call's input, never the model's narration.** A confirmation card's value is that the user trusts it to decide, so it must reflect the actual bytes about to be sent — a card that can disagree with the request defeats its purpose. Each `write-confirm` tool gets a deterministic `previewFor(toolName, input) string` formatter (e.g. `schedule_workout{date:2026-06-21,type:ride,dur_min:90}` → "Schedule a 90-min ride · Sun Jun 21"). The model's own framing ("I'll move your ride to Sunday and lighten it") still appears in the streamed **text bubble**; the **card** shows only the deterministic render. Tools without a formatter fall back to a generic "`<verb> <resource>`" line and MUST NOT surface model-supplied prose as the authoritative preview.

### D7 — Preserve the intentionally-paused trailing turn

`sanitizeHistory`/`opensCleanly` must distinguish "truncation left a dangling `tool_use`" (drop it) from "this session is awaiting confirmation" (keep it — it is the resume anchor). Simplest signal: a paused turn is the session's **last** turn and its `tool_use` calls are all `write-confirm`; the sanitizer only trims dangling `tool_use` turns that are *not* the trailing awaiting-confirmation turn. The confirm endpoint is the only path that answers it.

### D8 — System prompt: coach persona, planner retained as a subset

This repo currently has **no coach persona anywhere** — the MCP server ships no instructions and the desktop coach's personality lives outside the codebase. So `buildSystemPrompt` is the first place a coach persona is codified. It stays a **hard-coded Go template** with the existing config injection (`CHAT_DIETARY_PREFERENCES`, timezone), server-assembled and non-overridable by the client. No new persona env var.

The prompt sets **policy, not a tool catalog** (the ~50 tool descriptions carry the catalog). It must hold four things at once:

1. **Grounding discipline (extends the planner's).** Read before advising: daily nutrition context before food advice; training/recovery/Garmin context before training advice. Never recommend or propose from an empty context.
2. **Propose-vs-answer policy — proactive, but card-gated.** The coach MAY turn its recommendations into proposed `write-confirm` actions readily ("your protein's been low all week — want me to bump the goal? *proposes the write*"), because nothing fires without the user's tap. Proactivity is safe *because* the card is the commit point. Bounded by anti-spam rules: **batch** related proposals into one turn rather than spraying single cards; **do not re-propose** something the user rejected earlier in the same conversation; keep proposals tied to what's actually being discussed, not drive-by audits.
3. **Retained planner contract.** Meal planning is a subset, not lost: 2–3 options fitting the remaining macro budget + dietary preference, prefer library recipes then Cookidoo import with an estimated serving mass, and on selection persist planned meals + one consolidated merged shopping list. These planner writes are `write-auto` (no card).
4. **Guardrails.** Never invent nutriment/metric values (import or state the gap). Refuse/defer medical questions — a fueling-and-training coach is not a doctor. State plainly in the text what a proposed write will do; the card shows the authoritative render (D6).

Illustrative skeleton (final wording at apply time):

```
You are <user>'s endurance-fueling and training coach inside their personal
app. You range across nutrition AND training — fueling, sessions, recovery,
race prep. Dietary preference: %s. Timezone: %s (interpret "today"/"tomorrow"
in it).

GROUND BEFORE YOU ADVISE. Read the relevant context first — daily nutrition
context before food advice; recovery/training/Garmin context before training
advice. Never advise or propose from nothing.

YOU CAN CHANGE THINGS — BUT THE USER CONFIRMS. Some tools change data
(schedule/modify workouts, edit goals, log, delete). When your advice implies
one, go ahead and PROPOSE it — the user sees a confirmation card and nothing
happens until they approve. Be willing to propose; you are not committing.
  · Batch related changes into one proposal, don't drip single cards.
  · Don't re-propose something the user just declined.
  · Propose around what you're actually discussing — no unprompted audits.

MEAL PLANNING (a subset of your job): <the existing planner contract —
2-3 grounded options, Cookidoo import with serving mass, persist picks + one
merged shopping list>. These writes happen without a card.

NEVER invent nutriment or metric values. Defer medical questions to a
professional. Keep replies short and skimmable.
```

Open sub-point: per-tool `previewFor` formatters (D6) are the bridge between the model's intent and an honest card — they live with the shared registry so both the proposal event and the cold-open `pending_confirmation` (D9) render identically.

### D9 — Cold-open: the card is reconstructed from the session, not held in client memory

The proposal must survive the app being killed, not just backgrounded. So `GET /chat/sessions/{id}` (capability `chat-sessions`) SHALL surface a `pending_confirmation: {turn_id, calls:[{tool_id, name, tier, preview}]}` field when the session's trailing turn awaits confirmation (null otherwise). The `preview` strings are composed **server-side** (same logic as the `proposal` SSE event) so the Dart client never re-derives them from raw `tool_use` blocks. On open, the client renders the card from this field exactly as it would from a live `proposal` event — one rendering path, two sources. The session **list** (`GET /chat/sessions`) SHALL also flag awaiting-confirmation sessions (a boolean) so the history view can badge them.

### D10 — The chat coach surface is curated and aggregate-first, not all 127

mcpserver exposes 127 tools; putting all of them in front of the model on every `/chat` round is costly and degrades tool selection. The in-app coach instead gets a **curated ~15–25 tool surface**, following the pattern `get_daily_context` already set: prefer a few **aggregate context reads** over many granular ones. Concretely:

- **Reads:** `get_daily_context` (exists) + a small number of aggregate coaching reads — e.g. `get_training_context` (recent load, upcoming sessions, plan phase) and `get_recovery_context` (sleep/HRV/body-battery/readiness) — instead of the ~30 granular Garmin-mirror read tools. Where no aggregate endpoint exists yet, **add one** (1–2 new `/context/*` endpoints) rather than exposing the granular set.
- **Writes:** only the actions the coach actually proposes — meal-planning `write-auto` writes (exist) + the `write-confirm` training/goal/log/delete actions worth proposing conversationally. Not every write the desktop surface has.

The 127 stay fully available to the **desktop** coach (a human drives tool selection there, no token anxiety). This keeps the grounding policy in the prompt (D8) trivial — "read the relevant context tool first" — because there are only a few context tools, and it makes the proactive propose-stance sharper (the model isn't hunting through 127 options). New aggregate `/context/*` endpoints are net-new backend work; they may be split into their own small change ahead of phase 3, since the desktop coach benefits from them too.

## Risks / Trade-offs

- **[Tool-surface blast radius]** ~14 → ~50 tools in one assistant context inflates token cost and the chance the agent picks a wrong tool. Mitigation: the shared registry keeps descriptions tight; `CHAT_MAX_TOOL_ROUNDS`/timeout caps still apply; tiering keeps the dangerous ones gated.
- **[Refactor regression]** Extracting the shared registry could subtly change a tool's schema/path. Mitigation: ship D4 alone first, gated by the MCP integration test's expected-tools list and the chat tool tests; assert zero diff in generated tool defs.
- **[Paused-session correctness]** A bug in D7 could either drop the resume anchor (confirm impossible) or feed Anthropic a dangling `tool_use` (upstream 400). Mitigation: targeted tests for sanitize behavior on a paused vs truncated trailing turn; the confirm endpoint validates the trailing-turn shape before acting.
- **[Mobile writes are now powerful]** The phone can drive goal/training/delete writes. Mitigation: that is exactly what the confirm gate is for; deletes are `write-confirm`; the preview names the consequence before the tap.
- **[Stale/abandoned confirmations]** A user may never answer a proposal. Mitigation: there is no stuck state — a new `/chat` message **implicitly rejects** the pending `write-confirm` calls (appends declined `tool_result`s, then proceeds), so simply typing resolves the dangling `tool_use`. The card offers explicit per-item Approve/Reject; ignoring it and typing = reject-all. The session is never wedged.

## Migration Plan

No DB migration. Rollout as an ordered sub-arc, each independently shippable:
1. **Shared registry** (`internal/agenttools/`) — pure refactor, behavior identical, both packages consume it.
2. **Tiering + confirm protocol** — add `tier`, the pause branch, `proposal` event, confirm endpoint, sanitize fix, `409 awaiting_confirmation` on `/chat`. Testable on the existing narrow surface by tagging one existing write as `write-confirm` in tests.
3. **Broaden allowlist + coach prompt** — flip the chat registry to the full coach surface and swap the system prompt; this is where `write-confirm` tools actually appear in production.
4. **Companion UI** — proposal card + resume call + scope copy.

## Open Questions

- Registry package name/location (`internal/agenttools/` vs `internal/mcpserver/toolspec/`) — bikeshed, settle at apply.
- Should `update_goal`/override be `write-confirm` or stay out-of-surface entirely? Leaning `write-confirm` (the user asked for full training writes), but it is the highest-stakes single write — confirm gate makes it acceptable.
- ~~Per-item approve/reject vs all-or-nothing~~ — **decided: per-item toggles** ("Apply selected"). The card composer is **not locked**: typing a new message is an implicit reject-all. D5's per-item decisions back the card; implicit-reject backs the typed path.
