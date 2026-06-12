## Why

The backend now persists every chat conversation as a server-side session (`add-chat-sessions`), and the companion already creates one per conversation — but the app gives no way to *see* them. The only chat affordance is "New chat," which abandons the current thread with no path back. A single-user coach is most useful when yesterday's planning conversation can be reopened and continued, from any paired device.

## What Changes

- **New session-history screen in the companion**: a list of past conversations (most-recent-first, titled, with relative timestamps), reachable from a history icon in the Chat app bar — mirroring how the shopping list rides off the Today header.
- **Reopen-and-continue**: tapping a session fetches its transcript (`GET /chat/sessions/{id}`), reconstructs the visible text turns into the chat screen, sets it as the active server session, and lets the user keep talking — new turns append to that same server session.
- **Delete**: remove a session (`DELETE /chat/sessions/{id}`) from the list; deleting the active one falls back to a fresh conversation.
- **Rename**: edit a session's title (`PATCH /chat/sessions/{id}`); otherwise titles stay auto-derived from the opening message.
- **Client read layer**: extend the companion's chat client with `list / get / rename / delete` against the existing `/chat/sessions` surface. Online-only (the Chat screen already is) — no offline cache for the list.
- No backend changes — the `/chat/sessions` endpoints already exist.

## Capabilities

### New Capabilities
<!-- none — this rides on the existing mobile-companion capability -->

### Modified Capabilities

- `mobile-companion`: adds the session-history screen and reopen/delete/rename behavior. The existing "Chat screen streams the server conversation" requirement is extended (a history entry point and the notion that the active session may be an *reopened* one, not only a freshly-created one); the new browse/reopen/manage behavior is added as its own requirement.

## Impact

- **Code (companion only)**: `lib/data/net/chat_client.dart` gains `listSessions / getSession / renameSession / deleteSession`; new `lib/domain/chat.dart` types (`ChatSessionSummary`, transcript reconstruction); a new `lib/state/sessions_provider.dart`; a new `lib/ui/chat/sessions_page.dart`; the Chat app bar (`lib/ui/chat/chat_page.dart`) gets a history action; `lib/state/chat_provider.dart` gains an `openSession` path that loads a transcript and adopts its `session_id`.
- **Backend / API / docs**: none — read/manage endpoints already shipped under `add-chat-sessions`.
- **Tests**: companion widget/unit tests for transcript reconstruction (full-fidelity blocks → text-only bubbles) and the sessions provider; `flutter analyze` clean.
- **Offline**: the session list and reopen are online-only, consistent with the Chat screen; no Drift schema change.
