## Why

Today `/chat` is stateless: the server holds no conversation state and every client must carry the full transcript itself, so a conversation lives and dies on one device with no way to list, resume, or continue it elsewhere. For a single-user coach that the user returns to across days and devices (mobile companion + desktop agent), the conversation history *is* the product — it should be durable, listable, and resumable, and it should preserve full tool-grounded context so a resumed session reconstructs exactly what the model saw.

## What Changes

- **New `chat-sessions` capability**: server-side persistence of chat conversations with a management surface — create a session, list sessions (most-recent-first), fetch one with its full transcript, rename it, and delete it (cascading its messages).
- **Full-fidelity turn storage**: each persisted turn stores the complete Anthropic content-block array (text, `tool_use`, `tool_result` blocks), not just visible text, so a resumed session feeds the model the exact prior context rather than re-grounding from scratch.
- **BREAKING — `/chat` becomes session-backed.** `POST /chat` no longer accepts a client-held transcript. It now requires a `session_id` plus the single new user `message`; the server loads that session's prior turns from the store, appends the new user turn, runs the agent loop, and persists every assistant/tool turn it produces. Omitting `session_id` is now a `400`.
- **BREAKING — request/response shape change.** `{messages:[{role,content}...]}` → `{session_id, message}`. The streaming SSE contract (`text`/`tool`/`done`/`error`) is unchanged; `done` still carries the final assistant text and usage.
- **Mobile companion chat is now server-synced**, superseding the prior "transcripts are never synced to the server" stance: the companion holds a session id and the new message, not the authoritative transcript.

## Capabilities

### New Capabilities

- `chat-sessions`: durable server-side chat conversations — the `chat_sessions` / `chat_messages` store, the `/chat/sessions` CRUD surface (create, list, get-with-transcript, rename, delete), full-fidelity content-block persistence, and the persistence lifecycle that the agent loop writes into as turns complete.

### Modified Capabilities

- `nutrition-chat`: `POST /chat` flips from stateless-with-client-transcript to session-backed. The request requires `session_id` + a single new `message`; the server reconstructs history from the store, persists new turns, and no longer truncates a client-supplied transcript (it truncates the *stored* history instead). The "server holds no conversation state" requirement is replaced; the tool allowlist, system prompt, round/timeout caps, and SSE event contract are otherwise unchanged.
- `mobile-companion`: the Chat-screen requirement's "transcripts SHALL persist locally (Drift)… never synced to the server" clause is replaced — the companion now sends `{session_id, message}` and treats the server session as the source of truth.

## Impact

- **Code**: new `internal/chatsessions/` package (types/repo/service/handlers, per the one-package-per-capability convention) for the CRUD surface; `internal/chat/` service + handler reworked to read `session_id`+`message`, load prior turns via the sessions repo, and persist turns as the loop runs (`initialMessages` is replaced by a store load); wiring in `internal/httpserver/server.go`.
- **Schema**: one new migration adding `chat_sessions` and `chat_messages` (cascade FK). Verify the head before `migrate:new` — `031` is taken and the queued Garmin program reserves `032`.
- **API/docs**: `/chat` request body changes (BREAKING) and five new `/chat/sessions` routes; `task swag` must regenerate `docs/`.
- **MCP**: out of scope — the chat loop is the only consumer; no new MCP tools.
- **Clients**: the mobile companion's chat screen and any `/chat` caller must migrate to the session-backed shape. The desktop agent is unaffected (it does not call `/chat`).
- **Config**: existing `CHAT_*` knobs unchanged; `CHAT_MAX_HISTORY_MESSAGES` now bounds the *stored* history loaded per turn.
