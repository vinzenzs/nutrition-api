# Design: add-companion-chat

## Context

The app is offline-first with a strict pattern: mutating calls enqueue in the Drift outbox with client-minted idempotency keys; reads render stale-while-revalidate with no offline banner. Chat breaks the mold — it is a live SSE stream that cannot be outboxed — while the two satellite surfaces (plan card, shopping list) fit the existing pattern exactly. The design keeps that line sharp: chat is the lone online-only island; everything chat *produces* (plans, shopping items) syncs and mutates offline like any other data.

## Goals / Non-Goals

**Goals:**
- Chat that feels native to the app's speed culture: streamed deltas, tool chips, no spinners-of-mystery.
- Selection consequences visible outside chat immediately (Today plan card, shopping list).
- Offline honesty without infecting the rest of the app with banners.
- Nav stays four slots; shopping rides the Today header.

**Non-Goals:**
- No structured option-card protocol in v1 — options are assistant prose; the user replies in text. (A `done`-event extension for tappable options is a v2 candidate once real usage shows the friction.)
- No widget changes (Kotlin home-screen widget untouched).
- No multi-conversation management UI: one active conversation + "new chat" reset.
- No iOS-specific work beyond the existing platform-variance rule.

## Decisions

### D1: Transcript client-side in Drift, one active conversation

New `chat_messages` table (role, content, created_at, conversation_id) with a single active conversation pointer in prefs. Each send posts the truncated history per the backend contract. "New chat" starts a fresh conversation_id; old ones retained locally for scrollback (no sync — the backend is stateless by design and the consequences are already synced data).

### D2: SSE consumed via `http` streamed response, hand-rolled parser

The SSE format here is four known event types on a single POST response — a ~50-line parser on `http.Client.send()` beats adding a package dependency for a protocol subset. Reconnection semantics are deliberately simple: a dropped stream marks the turn failed with a retry affordance (the backend's idempotency-key derivation makes retry safe); no auto-resume mid-turn.

### D3: Tool events render as activity chips, recipes as link chips

`tool` events become transient chips under the streaming bubble ("Searching Cookidoo…", "Added 14 items to shopping list ✓"). When the final message references an imported recipe, its `external_url` renders as a tappable chip opening Cookidoo (Thermomix instructions live there; the app never embeds them). Failures (`status: error`) render the chip red with the summary — the assistant's follow-up text explains, the app doesn't re-interpret.

### D4: Chat's offline state is a composer-level notice, not an app banner

When connectivity is absent (existing reachability signal from the outbox worker), the chat composer disables with inline text "Chat needs a connection". Transcript stays readable. No global banner anywhere — the no-offline-banner rule survives intact for every other screen; the spec delta scopes the exception to the chat composer alone.

### D5: Plan card and shopping list are ordinary outbox citizens

"Ate it" → `POST /plan/{id}/eaten` enqueued with client-minted key, optimistic flip to eaten, Today's adherence refreshes when the summary revalidates. Skip → `PATCH /plan/{id}`. Shopping check-off → `PATCH /shopping/items/{id}` enqueued, optimistic check. Cached `planned_meals` and `shopping_items` tables render stale-while-revalidate exactly like recents. One deliberate guard: "Ate it" is disabled (greyed, "syncing…") while a *prior* eaten/skip for the same entry is still pending in the outbox, preventing offline double-taps from queueing a guaranteed 409.

### D6: Shopping entry point is the Today header cart icon with badge

Settings already lives top-right on Today; the cart joins it, badged with the open-item count. Alternatives rejected: fifth nav slot (spec reserved exactly four), burying it in chat (the list is most needed in a store, two taps max from cold start).

## Risks / Trade-offs

- [SSE through mobile networks/proxies can buffer or drop] → simple turn-retry posture (D2) rather than resume protocols; worst case the user re-sends and idempotent writes replay.
- [Optimistic eaten flip can be wrong if the server 409s (e.g. eaten on desktop concurrently)] → outbox failure surfaces revert the flip and toast; the next plan revalidation reconciles truth.
- [Prose options (no structured cards) make selection slightly clumsy] → accepted for v1; measured friction beats speculative protocol.
- [Drift transcript grows unbounded] → cap retained conversations (e.g. last 20) at insert time.

## Migration Plan

Drift schema migration (new tables) ships with the app update; no backend deploy coupling beyond requiring the three backend changes live first. Feature-gate: if `POST /chat` returns 503 (key unset), the Chat tab shows a "not configured on this server" state rather than hiding.

## Open Questions

- Markdown renderer choice (`flutter_markdown` vs minimal custom) — decide at implementation by binary-size impact.
- Whether the Kotlin widget should later show tonight's plan — explicitly deferred.
