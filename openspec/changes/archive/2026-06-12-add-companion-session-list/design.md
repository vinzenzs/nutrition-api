## Context

`add-chat-sessions` made the backend the source of truth for chat history: `/chat` is `{session_id, message}`, and `/chat/sessions` exposes list / get-with-transcript / rename / delete. The companion (`apps/companion/`) was migrated to create a session lazily on the first turn and reuse its id, but it surfaces no history — `ChatNotifier` holds one in-memory conversation, "New chat" resets it, and Drift is a local render cache keyed by a local `_conversationId`.

This change adds the missing UI: browse past sessions, reopen one and keep talking, delete, rename. It is companion-only.

Relevant existing shapes:
- `ChatClient` (`lib/data/net/chat_client.dart`) does raw `http` against `/chat` and `/chat/sessions` (it already has `createSession`). Chat is **online-only**.
- `ChatNotifier` (`lib/state/chat_provider.dart`) owns `_sessionId` (server) + `_conversationId` (Drift cache) + `state.messages` (rendered `ChatMessage`s).
- `GET /chat/sessions/{id}` returns `{id, title?, created_at, updated_at, last_message_at, messages: [{role, content}]}` where `content` is the verbatim Anthropic value — a JSON **string** (plain user text) or a content-block **array** (assistant text / `tool_use`; or a user `tool_result` reply).
- Navigation precedent: the shopping list is a full `MaterialPageRoute` page reached from a header button (`today_page.dart` → `ShoppingPage`).

## Goals / Non-Goals

**Goals:**
- A session-history screen, most-recent-first, reachable from the Chat app bar.
- Reopen-and-continue: load a session's visible transcript, adopt its `session_id`, append new turns to it server-side.
- Delete and rename from the list.
- A thin client read/manage layer on `ChatClient`; a `sessions_provider` for list state.

**Non-Goals:**
- No backend changes (endpoints exist).
- No offline cache / Drift schema for the session list — online-only, matching the Chat screen.
- No replay of historical tool-activity chips; reopened transcripts render text bubbles only.
- No search, pagination, multi-select, or pinning. The list is a single fetch.
- No change to the streaming turn flow itself (`send`/`retry` are untouched apart from the new `openSession` entry).

## Decisions

### D1 — Client methods live on `ChatClient`, online-only
Add `listSessions()`, `getSession(id)`, `renameSession(id, title)`, `deleteSession(id)` to `ChatClient` (raw `http`, same bearer/baseUrl pattern as `createSession`). Rationale: keeps all `/chat*` networking in one place; the Dio `ApiClient`/`Repository` layer is built around offline-first write-through caching, which the online-only session list does not want. Each method returns a typed result or throws a small typed failure the provider maps to an error state.

### D2 — Text-only transcript reconstruction (lossless for continuation)
`getSession` maps each stored turn to zero or one `ChatMessage`:
- `role: "user"`, content a JSON string → a user bubble with that text.
- `role: "assistant"`, content an array → an assistant bubble built from the `text` blocks (skip the turn if it has only `tool_use`).
- `role: "user"`, content an array (a `tool_result` reply) → skipped.

This is purely cosmetic: when the user continues, the client sends only `{session_id, message}` and the **server** replays the full-fidelity history, so dropping `tool_use`/`tool_result` from the *rendered* view loses nothing for the model. Tool chips are transient by design and are not reconstructed.

### D3 — `ChatNotifier.openSession(...)` adopts the server session
Add `Future<void> openSession(ChatSessionSummary s)`: fetch the detail, reconstruct messages, then set `state = ChatState(messages: reconstructed)`, `_sessionId = s.id`, and mint a fresh local `_conversationId` for Drift scrollback. Subsequent `send`/`retry` already key off `_sessionId`, so continuation "just works" and `_ensureSession` is a no-op (id already set). Guard against opening while a turn is streaming.

### D4 — Entry point + navigation
Add a history `IconButton` (e.g. `Icons.history`) to the Chat `AppBar`, left of "New chat", that pushes a full-page `SessionsPage` (`MaterialPageRoute`), mirroring `ShoppingPage`. Tapping a row calls `chatProvider.notifier.openSession(...)` then pops back to the Chat screen showing the loaded conversation. The list refetches on screen open (online-only; no cache).

### D5 — Delete / rename interactions
- **Delete**: swipe-to-dismiss (or row overflow menu) → `deleteSession(id)`, optimistic removal with re-fetch on failure. If the deleted id equals the active `_sessionId`, call `newChat()` so the Chat screen falls back to a fresh conversation.
- **Rename**: row overflow menu → a title text dialog → `renameSession(id, title)` (empty clears to untitled), then refresh. Titles otherwise stay backend-auto-derived from the opening message.

### D6 — States
The screen renders loading / empty ("No conversations yet") / error (retry) / list. No offline banner; if the device is offline the fetch fails into the error state, consistent with the Chat screen's online-only stance.

## Risks / Trade-offs

- **[Reopened transcript looks thinner than the live turn]** (no tool chips, no streaming animation) → Acceptable: chips are explicitly transient; the text content is complete. Documented in the spec scenario.
- **[List can go stale vs. server]** (new title after first turn, another device's edits) → Mitigation: online-only refetch on every screen open; no local cache to drift.
- **[Deleting the active session mid-use]** → Mitigation: detect `id == _sessionId` and reset to a new chat; the next send creates a fresh server session.
- **[Large transcript fetch]** for a very long session → Acceptable at single-user scale; the backend already caps what it *replays* to the model via `CHAT_MAX_HISTORY_MESSAGES`, and the list view itself is header-only.
- **[Reconstruction parsing of dynamic JSON]** (string vs array content) → Mitigation: a small, well-tested pure function (`reconstructTranscript`) with explicit cases and a unit test over mixed user/assistant/tool turns.

## Migration Plan

1. Extend `ChatClient` with the four methods + domain types (`ChatSessionSummary`, `reconstructTranscript`).
2. Add `sessions_provider.dart` (list/refresh/delete/rename) and `ChatNotifier.openSession`.
3. Build `SessionsPage` and wire the Chat app-bar history action.
4. `flutter analyze` + `flutter test` (new reconstruction/provider tests) green.
- **Rollback**: revert the companion commit; the backend is untouched, so nothing else is affected. No data migration.

## Open Questions

- Show a relative timestamp only, or also a one-line preview snippet (would need the first user message — available as the title when auto-derived, so a snippet is largely redundant)? Leaning timestamp + title only. Resolve in UI review.
- Confirm-on-delete dialog, or undo-snackbar? Leaning a lightweight confirm dialog for a destructive, non-undoable server delete.
