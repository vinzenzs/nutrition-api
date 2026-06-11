# Companion app architecture

This is a condensed capture of the design that ships with the
`add-flutter-companion-app` OpenSpec change. The authoritative, fully-reasoned
version (with alternatives considered) is the change's
[`design.md`](../../../openspec/changes/archive/) — look under the archived
change once it lands, or
`openspec/changes/add-flutter-companion-app/design.md` while it is in flight.
The platform-agnostic invariants are in
[`openspec/specs/mobile-companion/spec.md`](../../../openspec/specs/mobile-companion/spec.md).

## Stance

Companion, not replacement. The agent owns conversation, planning, recipes and
reflection; the app owns glance-ability, camera input and one-tap friction.
Both converge on the same REST API and database.

## Offline-first outbox

```
mutation ─▶ pending_writes (Drift)  ─▶ outbox worker ─▶ REST API
            id, method, path,           drains in           (idempotent
            body, idem_key (UUIDv4),     arrival order        replay safe)
            status, attempt_count

replay triggers: app foreground · connectivity change · 15-min WorkManager
read path:  GET ─▶ render from recent_summary / products_cache, revalidate in
            background. No offline banner; freshness is implicit.
```

A 2xx marks a row `done`; a 4xx (incl. `409 idempotency_key_conflict`) marks it
`failed_permanent`; a 5xx / network error keeps it pending and bumps
`attempt_count`.

## The A-lite hydration widget

```
                         user taps widget
                                │
                                ▼
                  ┌──────────────────────────┐
                  │  Kotlin AppWidgetProvider │
                  └──────────┬───────────────┘
                             │ enqueue WorkManager one-shot
                             ▼
                  ┌──────────────────────────┐
                  │  HydrationTapWorker       │
                  │   reads token (TokenBridge / EncryptedSharedPreferences)
                  │   POST /hydration (OkHttp, fresh Idempotency-Key)
                  └──────────┬───────────────┘
              ┌──────────────┴──────────────┐
              ▼                              ▼
        ─── 2xx ───                   ─── failure ───
        update Room snapshot          write widget_failures row (Drift file)
        refresh widget UI                     │
                                              ▼
                                   Flutter on next foreground drains
                                   widget_failures → pending_writes outbox
```

The widget never reads anything Flutter doesn't refresh: Flutter pushes fresh
totals into the Room snapshot (via the `widget_bridge` method channel) after
every hydration log, and the snapshot is the only thing the widget renders.

## Token storage

The bearer token lives in the Android Keystore via `flutter_secure_storage`,
mirrored to EncryptedSharedPreferences (same Keystore primitive) so the Kotlin
widget worker can read it without Flutter. It is never written to plain
SharedPreferences, the Drift DB, or outbox row bodies.

## State management

Riverpod. Providers wrap the local Drift query *and* the network refresh, which
is a clean fit for the stale-while-revalidate read model. Material 3 with
dynamic color for the look.
