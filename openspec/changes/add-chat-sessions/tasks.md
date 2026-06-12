## 1. Schema

- [x] 1.1 Verified the migration head: `032_add_workout_garmin_schedule_ids` (the committed Garmin-scheduling migration) already holds `032`, so chat-sessions takes `033`.
- [x] 1.2 `task migrate:new NAME=add_chat_sessions`; in the `.up.sql` create `chat_sessions(id uuid pk, title text null, created_at timestamptz not null default now(), updated_at timestamptz not null default now(), last_message_at timestamptz not null default now())` and `chat_messages(id uuid pk, session_id uuid not null references chat_sessions(id) on delete cascade, seq bigint generated always as identity, role text not null, content jsonb not null, created_at timestamptz not null default now())` with index `chat_messages_session_seq_idx (session_id, seq)`.
- [x] 1.3 In the `.down.sql` drop `chat_messages` then `chat_sessions`.

## 2. `internal/chatsessions/` package (persistence + CRUD)

- [x] 2.1 `types.go`: `Session` (id, title `*string` with omitempty, created_at, updated_at, last_message_at) and `Message` (role, raw `content json.RawMessage`) shapes; request bodies for create/rename.
- [x] 2.2 `repo.go` against `store.Querier`: `CreateSession`, `ListSessions` (order by `last_message_at desc, created_at desc`, headers only), `GetSession` + `GetMessages` (ordered by `seq`), `RenameSession`, `DeleteSession`, `AppendMessage` (insert a turn + bump `last_message_at`), and a `SessionExists` helper for `/chat` validation. Usable with either `*pgxpool.Pool` or `pgx.Tx`.
- [x] 2.3 `service.go`: validation + sentinel errors mapping 1:1 to codes — `ErrSessionNotFound` → `session_not_found`; title trim/length bounds; empty-string title clears (untitled).
- [x] 2.4 `handlers.go` with swag annotations and `Register(rg *gin.RouterGroup)`: `POST /chat/sessions`, `GET /chat/sessions`, `GET /chat/sessions/{id}` (header + ordered turns at full fidelity), `PATCH /chat/sessions/{id}`, `DELETE /chat/sessions/{id}` (204).
- [x] 2.5 Per-handler integration tests (testcontainers Postgres): create (with/without title), list ordering by recency, get-with-transcript, rename + clear, delete returns 204 and cascades messages, all `{id}` routes 404 `session_not_found` on unknown id.

## 3. `/chat` rework — session-backed loop

- [x] 3.1 Change `ChatRequest` to `{session_id string, message string}`; update `validateTranscript` → validate non-empty `message` (`empty_message`) and drop the now-unreachable `system_role_not_allowed` / `invalid_role` transcript checks.
- [x] 3.2 Handler: 503 when `svc == nil`; resolve `session_id` via the sessions repo → `404 session_not_found` before streaming; pass `session_id` + `message` + bearer into `svc.stream`.
- [x] 3.3 Inject the `chatsessions` repo into `chat.Service` (constructor/setter). Replace `initialMessages(inbound)` with a load of the session's stored turns truncated to `CHAT_MAX_HISTORY_MESSAGES`; defensively drop a trailing unmatched `tool_use` turn.
- [x] 3.4 Persist the new user `message` turn (bump `last_message_at`) before the loop; if the session is untitled, set its title from a trimmed/truncated prefix of the first user message.
- [x] 3.5 In the loop, persist each completed round — the assistant turn (with `tool_use` blocks) and its `tool_result` user turn — together in one transaction so a stored session never ends on a dangling `tool_use`; persist the final assistant text turn on `done`.
- [x] 3.6 Confirm tool dispatch, idempotency-key derivation, system prompt, round/timeout caps, and SSE event shapes are unchanged.

## 4. Wiring

- [x] 4.1 In `internal/httpserver/server.go`, construct the `chatsessions` repo/service/handlers unconditionally (persistence works without an API key) and register the routes.
- [x] 4.2 Inject the `chatsessions` repo into `chat.Service` when `chatSvc != nil`; keep the loopback-handler wiring order intact.

## 5. Tests, docs, validation

- [x] 5.1 Update `internal/chat` loop/handler tests for the `{session_id, message}` shape: turns persisted in order, history loaded from the store on the next turn, 404 on unknown session, mid-stream failure leaves the user turn + completed rounds persisted.
- [x] 5.2 `task swag` to regenerate `docs/` for the changed `/chat` body and the five new `/chat/sessions` routes.
- [x] 5.3 `task test` (or `-p 1` on contention) green; `task vet` clean.
- [x] 5.4 `openspec validate add-chat-sessions --strict` passes; tick tasks as completed.
