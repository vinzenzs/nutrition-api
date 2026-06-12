# Design: fix-chat-tool-status-chips

## Context

The server-side chat agent (`add-chat-backend`) streams a `tool` SSE event
around each tool call, and the companion (`add-companion-chat`) renders them as
chips. The agent loop in `internal/chat/service.go` does, per call:

```go
sse.tool(call.Name, "started", "running")          // before dispatch
res := s.dispatcher.execute(...)
name, status, summary := toolEventFields(call.Name, res)
sse.tool(name, status, summary)                     // after dispatch (ok|error)
```

and the provider appends each event to a flat list rendered as chips. With no
shared key the two events can't be merged, so both persist — and the chip shows
`summary` ("running"/"done") rather than the name. The fix is to give each call
a stable id and coalesce on it.

## Goals / Non-Goals

**Goals:**
- One chip per tool call that transitions running → done/error.
- Chips labeled by the tool name, status shown by icon/colour.
- Robust pairing even if the same tool name is called twice in a turn.

**Non-Goals:**
- No change to which tools run, dispatch, or persistence.
- No auto-dismiss/animation of chips (they stay for the turn, cleared on the
  next turn as today). Fully-transient chips are a possible later polish.
- No change to the `done`/`error`/`text` events.

## Decisions

### D1: The `tool` event carries the `tool_use` id

The loop already holds `call.ID` (the Anthropic `tool_use` id, unique within a
turn). Both `sse.tool` calls for a given call SHALL include it:

```
event: tool
data: {"id":"toolu_01ABC…","name":"daily_context","status":"started","summary":""}
…
data: {"id":"toolu_01ABC…","name":"daily_context","status":"ok","summary":""}
```

`sse.tool` gains an `id` parameter. The `summary` stays for the error case
(`"failed (status 502)"`); on success it MAY be empty (the client labels by
name). Using the real `tool_use` id (not a name) makes coalescing correct when a
turn calls the same tool twice.

### D2: The client upserts chips by id

`ChatToolEvent` gains an `id`. The provider replaces its append with an
upsert-by-id: a `started` event inserts a chip (status running); the matching
`ok`/`error` event finds the chip with the same id and updates its status (and
error summary). One chip per id, ending in its terminal status. Order is
preserved by first-seen id.

```dart
final i = tools.indexWhere((t) => t.id == ev.id);
if (i >= 0) { tools[i] = ev; } else { tools.add(ev); }
```

### D3: The chip labels by name; status is the icon

`_ToolChip` already maps status to an avatar (✓ for ok, ⚡ otherwise, error icon
when error). The label SHALL be the (humanized) tool **name**; the summary is
shown only when it adds information (errors). Result: `daily_context` shows as a
single chip "daily context" that flips ⚡→✓ when the call finishes.

### D4: Backward and persistence behaviour unchanged

`id` is additive; an older client that ignores it simply keeps today's
double-render. The persisted transcript and `done` event are untouched —
reopening a session from history still renders text bubbles without chips (chips
are live-turn only), exactly as the session-history requirement states.

## Risks / Trade-offs

- **Same `tool_use` id reused across turns?** Anthropic ids are unique per
  message/turn; the provider clears `tools` at each turn start, so cross-turn
  collision is impossible in practice.
- **Humanizing the name** (`daily_context` → "daily context") is cosmetic; if a
  raw name is clearer for debugging we can keep it raw. Minor.

## Migration Plan

None — no schema. Ship backend + client together; the field is additive.

## Open Questions

- Should successful chips auto-dismiss shortly after `done` (truly "transient")
  rather than persist for the turn? Deferred; persist-for-turn matches current
  behaviour and the screenshot intent (a trail of what was looked up).
