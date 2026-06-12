## Context

`/chat` (capability `nutrition-chat`, package `internal/chat/`) runs a server-side Anthropic agent loop and streams SSE, but holds **no** state: the client POSTs `{messages:[...]}` (its own transcript), the loop truncates it to `CHAT_MAX_HISTORY_MESSAGES`, runs, streams, and forgets. The mobile companion keeps transcripts locally in Drift and never syncs them.

The user wants conversations to be durable and resumable across devices, with **full-fidelity** turn storage (the complete content-block array, including `tool_use`/`tool_result`), and `/chat` to be **always session-backed** (no stateless mode retained). This is a backend change; the desktop agent does not call `/chat`, so the only affected client contract is the companion's chat screen.

Constraints from the codebase:
- One package per capability: `types.go` / `repo.go` / `service.go` / `handlers.go` against `store.Querier`; sentinel errors mapping 1:1 to error codes; `Register(rg *gin.RouterGroup)`; integration tests against testcontainers Postgres.
- `numfmt.Round1` does not apply here (no nutrient floats).
- Migrations are append-only; head is `031`, `032` likely reserved by the queued Garmin program — verify before `migrate:new`.
- The chat loop dispatches tools as loopback HTTP under the caller's bearer; that is unchanged.

## Goals / Non-Goals

**Goals:**
- Durable `chat_sessions` + `chat_messages` store with full content-block fidelity per turn.
- A `/chat/sessions` management surface: create, list (most-recent-first), get-with-transcript, rename, delete (cascade).
- `/chat` reworked to `{session_id, message}`: load prior turns from the store, append the new user turn, run the loop, persist every assistant/tool turn produced.
- Preserve the unchanged parts of `nutrition-chat`: SSE event contract, tool allowlist, system prompt, round/timeout caps.

**Non-Goals:**
- No MCP tools for sessions (the chat loop is the sole consumer).
- No per-user scoping or sharing — single-user project; a session is owned by the one bearer identity.
- No server-side summarization/compaction of long histories beyond the existing message-count truncation.
- No search over session contents, no titles-from-LLM; title is client-supplied or derived from the first user message.
- No streaming-resume of a half-finished turn; resume granularity is a completed turn.

## Decisions

### D1 — Two tables: `chat_sessions` (header) + `chat_messages` (turns)
`chat_sessions(id uuid pk, title text null, created_at, updated_at, last_message_at)`. `chat_messages(id uuid pk, session_id uuid references chat_sessions on delete cascade, seq bigint generated always as identity, role text, content jsonb not null, created_at)` with index `(session_id, seq)`.

- **Why `content jsonb`, not split columns**: a turn's content is the Anthropic content-block array (string for plain user text, or an array of `text`/`tool_use`/`tool_result` blocks). Storing the raw array verbatim is exactly the full-fidelity requirement and means the loader hands the store value straight back to the upstream `messages` field — no re-serialization, no lossy mapping. (Mirrors how `internal/chat/types.go` already keeps `Content json.RawMessage`.)
- **Why a `seq` identity tiebreaker**: turns within one request share a transaction `created_at`; `seq` preserves intra-request order (same pattern as `shopping_items.seq`).
- **`last_message_at`** drives list ordering (most-recent-first) and is bumped on every persisted turn; `updated_at` covers rename.
- Alternative considered: a single `chat_sessions` row with a JSONB array of all turns. Rejected — unbounded row growth, rewrite-the-world on every turn, and no clean per-turn ordering.

### D2 — New package `internal/chatsessions/`, distinct from `internal/chat/`
The CRUD surface (store + REST handlers) lives in `internal/chatsessions/`; the agent loop stays in `internal/chat/`. `chat` depends on `chatsessions`' repo to load/persist turns. This keeps the loop (Anthropic protocol, SSE, tool dispatch) separate from persistence, and avoids bloating the already-large `chat` package. Wiring in `internal/httpserver/server.go`: construct the `chatsessions` repo/service/handlers always (persistence works without an API key), and inject the repo into `chat.Service` so the loop can read/write turns. When `ANTHROPIC_API_KEY` is unset, `/chat` still returns `503 chat_unavailable`, but `/chat/sessions` CRUD remains available (sessions are just data).

### D3 — `/chat` request shape: `{session_id, message}` (BREAKING)
The handler validates `session_id` (must exist → `404 session_not_found`) and a non-empty `message` (`400 empty_message`). The system prompt and a client-supplied `system` role were already rejected; with no client transcript there is no `system_role_not_allowed` path anymore — that validation is removed. The loop:
1. Loads the session's stored turns (truncated to the most recent `CHAT_MAX_HISTORY_MESSAGES`) → the `messages` slice. `initialMessages(inbound)` is deleted.
2. Persists the new user `message` as a turn (role `user`, content the JSON-encoded string), bumping `last_message_at`.
3. Runs the existing loop. As each assistant turn (with its `tool_use` blocks) and each `tool_result` user turn is appended to the in-memory `messages`, it is **also** persisted as a `chat_messages` row in the same order.
4. On terminal `done`, the final assistant text turn is persisted.

Persisting incrementally (not just at the end) means a mid-stream failure leaves a coherent prefix: the user turn and any completed tool rounds are durable, so a retry resumes with full context rather than replaying. Tool side effects were already idempotent loopback calls, so a resumed turn that re-dispatches a tool replays rather than duplicates (existing `effectiveIdempotencyKey` behavior).

### D4 — Title handling
`POST /chat/sessions` accepts an optional `title`. If absent, the session is created untitled and the first `/chat` turn sets the title from a trimmed/truncated prefix of the first user message (server-side, deterministic — no LLM call). `PATCH /chat/sessions/{id}` renames. This keeps list views legible without a dedicated naming step.

### D5 — Endpoint surface and error codes
- `POST   /chat/sessions` → create. Body `{title?}`. `201` `{id,title,created_at,updated_at,last_message_at}`. Idempotency-Key honored (write path).
- `GET    /chat/sessions` → list, most-recent-first by `last_message_at` (then `created_at`). Returns headers only (no transcript) for a light list.
- `GET    /chat/sessions/{id}` → one session header **plus** its ordered turns at full fidelity (`role` + raw `content`). `404 session_not_found`.
- `PATCH  /chat/sessions/{id}` → rename. Body `{title}` (`""` clears to untitled). `404 session_not_found`.
- `DELETE /chat/sessions/{id}` → delete session + cascade messages. `204`. `404 session_not_found`.

PATCH-on-a-singleton rules and the empty-string-clear convention from elsewhere in the repo are reused for the title field (`""` clears). Idempotency middleware rejects `Idempotency-Key` on PUT only — these are POST/PATCH/DELETE, so unaffected.

## Risks / Trade-offs

- **[Breaking change to `/chat`]** → The companion chat screen and any direct caller must migrate to `{session_id, message}`. Mitigation: the companion is the only known client and is updated in lockstep (its spec is modified here); the desktop agent does not use `/chat`. No deprecation window is kept since the project is single-user pre-release.
- **[Partial turn on mid-stream failure]** → A failure between persisting the assistant `tool_use` turn and its `tool_result` turn could leave a session ending on an unanswered `tool_use`, which Anthropic rejects on the next call. Mitigation: persist the assistant turn and its corresponding `tool_result` turn together (one transaction per round) so a stored session never ends on a dangling `tool_use`; the loader can also defensively drop a trailing unmatched `tool_use` turn.
- **[Unbounded history growth]** → Long sessions accumulate rows and the loaded context is capped only by message count. Mitigation: reuse `CHAT_MAX_HISTORY_MESSAGES` to bound what is loaded per turn; full history is still retrievable via `GET /chat/sessions/{id}`. Server-side compaction is a deliberate Non-Goal for now.
- **[`tool_result` bodies persisted verbatim]** → Stored content includes tool response bodies, which may be large. Acceptable for single-user scale; revisit if row sizes become a problem.
- **[Sessions without an API key]** → CRUD works but `/chat` returns 503; a user could accumulate empty sessions. Acceptable and self-evident.

## Migration Plan

1. Add migration `0NN_add_chat_sessions.{up,down}.sql` (verify head; `031` taken, `032` likely reserved) creating both tables + index; `down` drops them.
2. Ship `internal/chatsessions/` and the `/chat` rework together (the new `/chat` shape depends on the store).
3. `task swag` to regenerate `docs/` for the changed `/chat` body and the five new routes.
4. Update the companion chat screen to the session-backed shape (separate client work; tracked by the modified `mobile-companion` spec).
- **Rollback**: `migrate down` one step drops the tables; reverting the `chat` package restores the stateless handler. No data migration is needed (no prior persisted chat state exists).

## Open Questions

- Should `GET /chat/sessions/{id}` project `tool_use`/`tool_result` blocks into a client-friendlier shape, or return raw Anthropic blocks? Leaning raw (full fidelity, lets a future client reconstruct exactly); the companion renders only `text` blocks and ignores the rest. Resolve during specs review.
- Cap on number of sessions or messages per session? Not enforced initially (single-user); revisit if it matters.
