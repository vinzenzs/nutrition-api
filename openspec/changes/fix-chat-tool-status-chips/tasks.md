# Tasks: fix-chat-tool-status-chips

## 1. Backend: emit the tool-call id

- [x] 1.1 `internal/chat/service.go`: extend the `sseWriter.tool` helper to take an `id` and include it in the `tool` event JSON (`{id, name, status, summary}`)
- [x] 1.2 In the agent loop, pass `call.ID` to both `sse.tool(...)` calls (the `started` event and the post-dispatch `ok`/`error` event) so the pair shares one id; on success `summary` MAY be empty, error keeps its `"failed (status N)"` summary
- [x] 1.3 Update the `POST /chat` swag `@Description` to document the `tool` event as `{id, name, status, summary}` with the shared-id pairing
- [x] 1.4 Backend test: a tool round emits a `started` then a terminal event for the same call with an identical, non-empty `id`; two calls to the same tool name yield distinct ids

## 2. Companion: model + parse the id

- [x] 2.1 `apps/companion/lib/domain/chat.dart`: add `id` to `ChatToolEvent`
- [x] 2.2 `apps/companion/lib/data/net/chat_client.dart`: parse `id` from the `tool` SSE event payload

## 3. Companion: coalesce + label

- [x] 3.1 `apps/companion/lib/state/chat_provider.dart`: replace `tools.add(ev)` with an upsert-by-`id` (replace the existing chip with the same id, else append); preserve first-seen order; keep the per-turn reset
- [x] 3.2 `apps/companion/lib/ui/chat/chat_page.dart` `_ToolChip`: label by the (humanized) tool `name`; keep the avatar icon as the status (running ⚡ / done ✓ / error); show `summary` only on error
- [x] 3.3 Companion provider test: two events with one `id` (`started` then `ok`) yield a single chip ending in done; an `error` terminal yields a red chip with its summary; distinct ids yield distinct chips

## 4. Verification

- [x] 4.1 `task vet` + the affected Go test package green; `flutter test` (companion) green for the chat provider/widget tests
- [x] 4.2 `task swag` (regenerate docs for the updated `/chat` description); `openspec validate fix-chat-tool-status-chips --strict` passes
- [ ] 4.3 Manual smoke on a debug build: send "what should I eat today?" and confirm one chip per tool call, labeled by name, transitioning running→done with no stuck "running" chips
