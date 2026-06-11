# Tasks: add-companion-chat

## 1. Data layer

- [x] 1.1 Drift schema bump: `chat_messages` (conversation_id, role, content, created_at) with 20-conversation retention on insert; cached `planned_meals` and `shopping_items` tables + DAOs
- [x] 1.2 Repository methods: chat transcript CRUD, plan range read (`GET /plan?from&to`) into cache, shopping list read into cache
- [x] 1.3 Outbox coverage for the three new writes (`POST /plan/{id}/eaten`, `PATCH /plan/{id}`, `PATCH /shopping/items/{id}`, `DELETE /shopping/items?checked=true`) incl. replay-trigger registration and per-entry pending lookups

## 2. Chat screen

- [x] 2.1 SSE client: streamed POST via `http.Client.send()`, parser for `text`/`tool`/`done`/`error`, turn-retry on drop (identical resubmit)
- [x] 2.2 `chat_provider`: transcript state, active conversation pointer, send/stream/finalize lifecycle, history truncation matching backend contract, system-role exclusion
- [x] 2.3 `ui/chat/`: bubble list with markdown rendering (choose renderer by binary-size impact), streaming bubble, activity chips (transient, red on error), Cookidoo link chips opening externally, "new chat" reset
- [x] 2.4 Composer connectivity gating (reuse outbox reachability signal) + `503 chat_unavailable` → "not configured" state; activate the fourth nav slot in `home_shell.dart`

## 3. Plan card on Today

- [x] 3.1 `plan_provider`: today's entries stale-while-revalidate, optimistic eaten/skip flips, pending-in-outbox detection, conflict revert + toast
- [x] 3.2 Plan card UI in `ui/today/`: per-entry name/slot/quantity, "Ate it" + skip, disabled-while-pending hint, card absent when no entries; summary revalidation hook after eaten replay

## 4. Shopping list screen

- [x] 4.1 `shopping_provider`: cached list, optimistic check/uncheck, badge count stream, clear-bought with confirmation
- [x] 4.2 `ui/shopping/`: list (unchecked first), check-off rows, minimal add-item affordance, clear-bought action; cart icon + badge in Today header

## 5. Verification

- [x] 5.1 Widget/unit tests: SSE parser (all four events, mid-stream drop), provider optimistic/revert logic, pending-block behavior
- [ ] 5.2 Manual e2e against a local backend: plan 3 dinners in chat → entries on Today → ate-it offline → replay → adherence updates; shopping check-off offline in airplane mode
- [x] 5.3 Verify no offline indicator appears outside the chat composer (spec exception holds)
