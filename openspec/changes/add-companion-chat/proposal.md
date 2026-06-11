# Proposal: add-companion-chat

## Why

The mobile-companion spec reserved a disabled fourth nav slot precisely so a v2 chat could land without restructuring — the backend chat loop (`add-chat-backend`), planned meals (`add-meal-plan`), and shopping list (`add-shopping-list`) now exist to fill it. This change closes the user-facing loop: ask "what should I eat?", pick an option in chat, see today's pick on the Today screen, tap "ate it" at dinner, and carry the consolidated shopping list to the store.

## What Changes

- The reserved fourth nav slot becomes **Chat**: a conversation screen consuming the `POST /chat` SSE stream (markdown rendering, tool-activity chips, tappable Cookidoo links). Transcript is held client-side (Drift), matching the stateless backend.
- Chat is the app's first online-only surface: offline it shows a disabled composer with an inline notice — an explicit, scoped exception to the no-offline-banner rule.
- The Today screen gains a **plan card**: today's planned meals with a one-tap "Ate it" action (`POST /plan/{id}/eaten` through the outbox) and a skip affordance.
- A **shopping list screen**, reachable from a cart icon in the Today header (nav stays at four slots): check-off, clear-checked, all writes through the outbox.
- **BREAKING** (spec-level): the v1 requirement "no chat surface SHALL exist" is replaced — chat is now in scope; the slot-reservation scenario is fulfilled and retired.

## Capabilities

### New Capabilities

_None — all changes extend the existing `mobile-companion` capability._

### Modified Capabilities

- `mobile-companion`: the three-screens requirement becomes four (chat slot activated); new requirements for the chat screen + its connectivity exception, the Today plan card with eaten/skip, and the shopping list screen.

## Impact

- **App code** (`apps/companion/`): new `ui/chat/` (screen, SSE client, transcript store), `ui/shopping/`, plan card in `ui/today/`; new providers (`chat_provider`, `plan_provider`, `shopping_provider`); Drift schema bump for transcript + cached plan/shopping rows; outbox replay coverage for the three new write endpoints; `home_shell.dart` slot activation.
- **Backend**: none (consumes existing surfaces).
- **Dependencies**: a Dart SSE client (likely hand-rolled on `http` streaming — decide in design) and a markdown renderer for chat bubbles.
- **Sequencing**: last of the five; requires `add-chat-backend`, `add-meal-plan`, `add-shopping-list` deployed.
