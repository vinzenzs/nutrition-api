# Proposal: fix-chat-tool-status-chips

## Why

The companion Chat screen shows a permanent row of duplicated "running" and
"done" chips instead of a clean trail of which tools the agent ran. Two defects
combine:

1. **No coalescing.** The backend emits two `tool` SSE events per call —
   `("started","running")` before execution and `("ok"/"error", …)` after
   (`internal/chat/service.go`). The Flutter provider appends *every* event
   (`chat_provider.dart` `tools.add(ev)`), so each call leaves a frozen
   "running" chip **and** a "done" chip. Four grounding reads → four stuck
   "running" + four "done". The "running" chips aren't live; they're leftovers.
2. **Uninformative labels.** The chip renders the `summary` ("running"/"done")
   rather than the tool `name`, so the user can't tell *which* tool ran
   (`daily_context`, `list_races`, …). The name is sent but never shown, and the
   Anthropic `tool_use` id — the natural key to pair the started and completed
   events — is not included in the SSE event at all.

The current `mobile-companion` spec already says these should be **"transient
activity chips (name + outcome summary)"** — so this is a fix that re-aligns the
implementation with its own spec, plus a small SSE-shape addition (an `id`) so
the pairing is robust even when the same tool is called twice in a turn.

## What Changes

- **`tool` SSE event gains a stable `id`** (the Anthropic `tool_use` id):
  `{id, name, status, summary}`. The `started` event and the matching
  `ok`/`error` event for one call SHALL carry the **same** `id`.
- **The companion coalesces tool chips by `id`**: the `started` event
  creates/updates one chip (running, ⚡); the `ok`/`error` event updates that
  **same** chip in place (done ✓ / error, red). No duplicate chips; no frozen
  "running" leftovers.
- **The chip is labeled by the tool name** (humanized), with status conveyed by
  the avatar icon + color — not by the redundant "running"/"done" summary. An
  error summary is still surfaced on failure.
- **No new endpoints, tables, or migrations.** Purely a streamed-event field
  addition + client rendering fix.

## Capabilities

### New Capabilities

<!-- None. -->

### Modified Capabilities

- `nutrition-chat`: the `tool` SSE event shape gains an `id` shared by a call's
  started and completed events.
- `mobile-companion`: the Chat screen coalesces tool events by `id` into one
  status-transitioning chip labeled by tool name.

## Impact

- **Backend**: `internal/chat/service.go` (emit `id` on both `sse.tool` calls;
  the loop already has `call.ID`), the `sseWriter.tool` helper, and the
  `/chat` swag description.
- **Companion**: `apps/companion/lib/domain/chat.dart` (`ChatToolEvent` gains
  `id`), `chat_client.dart` (parse `id`), `chat_provider.dart` (upsert by `id`
  instead of append), `chat_page.dart` (`_ToolChip` labels by name).
- **Tests**: backend SSE assertion that a call's two events share an `id`;
  companion provider test that two events with one `id` yield a single chip that
  ends in `done`.
- **No breaking changes**: `id` is an added field; older clients ignoring it
  keep working (they'd still double-render, i.e. today's behavior).
