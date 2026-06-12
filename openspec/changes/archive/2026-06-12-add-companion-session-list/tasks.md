## 1. Client + domain layer

- [x] 1.1 In `lib/domain/chat.dart`, add `ChatSessionSummary` (`id`, `title?`, `lastMessageAt`, `createdAt`) and a pure `reconstructTranscript(List<rawTurn>) -> List<ChatMessage>` that maps user-string turns to user bubbles, assistant text-block turns to assistant bubbles, and drops `tool_use`-only / `tool_result` turns.
- [x] 1.2 In `lib/data/net/chat_client.dart`, add `listSessions()`, `getSession(id)` (returns title + reconstructed messages), `renameSession(id, title)` (empty clears), `deleteSession(id)` — raw `http`, same bearer/baseUrl pattern as `createSession`, online-only, mapping failures to a small typed error.

## 2. State

- [x] 2.1 Add `lib/state/sessions_provider.dart`: an async-notifier holding the list with `refresh()`, `delete(id)` (optimistic + active-session reset hook), and `rename(id, title)`; loading/empty/error states.
- [x] 2.2 In `lib/state/chat_provider.dart`, add `Future<void> openSession(ChatSessionSummary)`: fetch detail, set `state` to the reconstructed messages, adopt `_sessionId`, mint a fresh `_conversationId`; guard while streaming. Expose the active `_sessionId` so delete can detect it.

## 3. UI

- [x] 3.1 Add `lib/ui/chat/sessions_page.dart`: a `MaterialPageRoute` page (like `ShoppingPage`) listing sessions newest-first (title/Untitled + relative time), with loading/empty/error; swipe-or-menu delete (confirm dialog) and a rename dialog.
- [x] 3.2 Add a history `IconButton` to the Chat app bar in `lib/ui/chat/chat_page.dart` (left of "New chat") that pushes `SessionsPage`; tapping a row calls `openSession(...)` and pops back to Chat.
- [x] 3.3 Wire delete-of-active-session to `ChatNotifier.newChat()` so the Chat screen falls back to a fresh conversation.

## 4. Tests + checks

- [x] 4.1 Unit test `reconstructTranscript` over mixed turns (user string, assistant text, assistant tool_use-only, user tool_result) → text-only bubbles in order.
- [x] 4.2 Provider test for `sessions_provider` (list/refresh/delete/rename happy paths against a fake client) and `openSession` adopting the session id.
- [x] 4.3 `flutter analyze` clean; `flutter test` green.
