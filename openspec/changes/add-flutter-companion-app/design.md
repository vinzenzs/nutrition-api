## Context

The system has a Go REST API and an MCP wrapper that exposes it to Claude Desktop and Claude Code. There is no end-user mobile client yet, even though `MOBILE_API_TOKEN` has been a first-class auth path since the MVP. This change introduces one — a Flutter app, Android only in v1.

The design decisions below were sketched during an exploratory session with the user. They are restated here for the record (and so a future change that ports to iOS or Web can re-examine them with full reasoning, not just the conclusions).

The app's stance is **companion, not replacement**. The agent is the surface for conversation, planning, recipe building, "why is my iron low this week" reflection. The app is the surface for three things conversation is structurally bad at: glance-ability, camera-driven input, and one-tap friction.

## Goals / Non-Goals

**Goals:**

- A v1 that ships in 2-3 weeks of solo work.
- Three killer interactions that work materially better on the phone than via the agent: barcode→log, photo→log, hydration widget.
- Offline-first behavior — the supermarket basement case (no signal, scan a known barcode, log a meal).
- A capability spec written so iOS and Web ports later inherit the same invariants.
- A widget that genuinely works one-tap (no app open required).

**Non-Goals:**

- Replacement of the agent. The app does not need a chat surface, a recipe builder, or a generalized search; the agent does those.
- iOS or Web ports. Future changes; this one is Android only.
- Real OAuth, multi-user, or per-user data isolation. Single-user sideload.
- Push notifications, meal reminders, hydration nudges.
- A native Watch or Wear OS surface.
- Health Connect or HealthKit integration (v1.5).
- Image persistence on either client or server.

## Decisions

### 1. Flutter over native Compose

The earlier explore pass leaned toward native Compose for an Android-only build, but the user's stated trajectory is iOS and Web later. With cross-platform back on the table, Flutter is the right choice: native Compose loses its single-language win the moment a second platform shows up. Cost paid in v1 (Dart + Kotlin for the widget) is a deposit on cheaper iOS/Web later.

**Alternatives considered (and why each was wrong for this trajectory):**

- *Native Compose only.* Loses cross-platform. Forces a from-scratch iOS rewrite.
- *Kotlin Multiplatform Mobile.* Compose Multiplatform on iOS is still maturing; Health Connect coverage is patchy off Android.
- *React Native.* Bridge overhead similar to Flutter; ecosystem split between RN Core and Expo; Dart is at least one language to learn end-to-end where JS pulls in toolchain tax.
- *PWA / web-tech wrapper.* Killer interactions (widget, offline barcode) die on the web platform.

### 2. Android only in v1, capability spec platform-agnostic

The capability requirements describe *what* must hold (offline outbox, idempotent replay, A-lite widget contract, token in secure storage) — never *how* on a specific platform. Implementation specifics for Android live in design.md and tasks.md; iOS/Web implementations later satisfy the same spec.

### 3. Companion shape — three screens, no chat

```
   Today          Camera         Recent
   ─────          ──────         ──────
   adherence      viewfinder     today's meal
   rings (kcal,   + scan/photo   entries +
   protein,       mode toggle    hydration
   fiber, iron,                   entries
   B12)
   recent meals   on first cap:
   (last 3)       product card,
                  log button
   +💧 button
   ⚙ settings
```

A bottom-nav slot is reserved for a `+`/chat affordance reserved for v2. Wiring the slot now means v2 chat lands without a redesign.

### 4. Offline-first with explicit outbox

Local SQLite (via Drift) holds three tables:

```
products_cache         ← stale-while-revalidate snapshot of products
   id, name, brand, source,
   nutriments_per_100g (jsonb-ish),
   serving_size_g,
   last_logged_quantity_g,  ← drives default quantity in scan flow
   refreshed_at

recent_summary         ← last ~7 days of /summary/daily?date=... responses
   date, tz, totals (jsonb), entries (jsonb), refreshed_at

pending_writes         ← outbox queue
   id (local uuid),
   method, path, body (bytes),
   idem_key,
   created_at,
   status (pending | in_flight | done | failed_permanent),
   last_attempt_at, attempt_count, last_error

widget_failures        ← spillover from the widget's direct-HTTP path (A-lite)
   id, body (bytes), idem_key, created_at
```

Write path: every mutation lands in `pending_writes` first, then the queue worker flushes. Each pending row carries a UUID-v4 `idem_key` generated at enqueue time. The harden-write-paths idempotency contract on the backend means replays are byte-exact safe.

Queue replay triggers:

- App foreground (`AppLifecycleState.resumed`).
- Network state change via `connectivity_plus`.
- `WorkManager` periodic job, 15-minute interval, as the backstop.

Read path: GET endpoints write through to `products_cache` / `recent_summary`. Stale data renders immediately; a background fetch revalidates. No banner.

### 5. The A-lite widget pattern

The widget is a regular Android `AppWidgetProvider` written in Kotlin. It:

1. Reads the current day's hydration progress from a Room snapshot (a tiny read-only mirror Flutter writes to on every hydration log).
2. On tap, fires a `WorkManager` one-shot that:
   - Reads bearer token from EncryptedSharedPreferences (same Keystore-backed storage the Flutter app uses via `flutter_secure_storage`).
   - `POST /hydration` with a freshly minted Idempotency-Key.
   - On 2xx, updates the Room snapshot and schedules a widget refresh.
   - On network failure, writes a row to `widget_failures` SQLite (read by the Flutter app on next foreground; the Flutter app drains it into `pending_writes`).

```
                         user taps widget
                                │
                                ▼
                  ┌──────────────────────────┐
                  │  Kotlin AppWidgetProvider │
                  └──────────┬───────────────┘
                             │ enqueue WorkManager
                             ▼
                  ┌──────────────────────────┐
                  │  one-shot worker          │
                  │   reads token             │
                  │   POST /hydration         │
                  └──────────┬───────────────┘
                  ┌──────────┴───────────────┐
                  │                          │
                  ▼                          ▼
            ─── 2xx ───              ─── failure ───
            update Room snap         write widget_failures
            refresh widget UI        row
                                            │
                                            ▼
                                  Flutter app on foreground
                                  drains widget_failures into
                                  pending_writes outbox
```

This pattern keeps the widget's tap-to-action loop fast on the happy path (no Flutter wakeup needed) while still benefiting from the app's queue on offline.

**Alternatives considered:**

- *Widget reads/writes the same Drift DB the Flutter app uses.* Schema duplication across Kotlin/Dart; Drift docs cover concurrent access but it's a known gotcha.
- *Widget always queues; Flutter does the actual POST.* Defeats the "tap = network call on happy path" goal; reintroduces app-wakeup latency.

### 6. Token storage and pairing

V1 is single-user, sideload. The bearer token is the existing `MOBILE_API_TOKEN`. Pairing happens once:

1. User runs `task dev:pair` (new task) on the backend. The task prints a QR code containing `{"base_url":"http://192.168.x.y:8080","token":"<MOBILE_API_TOKEN>"}`.
2. User opens the Flutter app on first launch. Setup screen: "Scan pairing QR."
3. App stores token in `flutter_secure_storage` (Android Keystore-backed). Base URL goes in a regular preference.
4. Widget reads the same EncryptedSharedPreferences via a shared Kotlin helper.

Web later breaks this model. A v2 change introduces session-token swap or real auth.

### 7. State management: Riverpod

Riverpod has the cleanest cross-platform story, works well with offline-first patterns (providers wrap the local Drift query *and* the network refresh), and lets us avoid the Bloc boilerplate. The user is single, the app is small — there is no team-style benefit from heavier patterns.

**Alternatives considered:** Bloc (more ceremony for the scale), Provider (lighter but loses async-data ergonomics), MobX (less idiomatic in Flutter ecosystem now).

### 8. UI library: Material 3 with dynamic color

Material 3 is Flutter's default and gets Material You dynamic colors on Android 12+ for free. Looks native. iOS port later either keeps Material everywhere (the Spotify route) or forks into Cupertino-styled-on-iOS — that decision is deferred and does not affect v1 code structure if we avoid hardcoded styling.

### 9. Camera and barcode

- `mobile_scanner` for barcodes — wraps ML Kit on Android, AVFoundation on iOS later.
- `image_picker` (or its CameraX-backed equivalent) for still capture.
- Request JPEG output (the `imageQuality` and source parameters on image_picker) so HEIC doesn't reach the backend in v1.

### 10. Codebase location: monorepo at apps/companion/

The repo is currently Go-only. Flutter source lives at `apps/companion/` rather than a separate repo because:

- The app's spec already lives in this repo's `openspec/specs/`. Splitting code from spec by repo boundary adds friction without benefit.
- The backend and the app coevolve. Cross-repo PRs would be a constant tax.
- The Cookidoo importer (separate package, Chrome MV3) has its capability spec here; same model.
- Single `task` driver can lift backend + app workflows.

Cost: contributors clone Go and Flutter toolchains both. Acceptable for a solo project.

### 11. CI strategy

Flutter is **NOT a CI gate** for the Go build in v1. Running `flutter test` per PR is too slow and noisy for a single-developer setup. Manual `task app:test` discipline before pushing app changes is enough.

Optional follow-up: a separate `flutter-ci.yml` workflow triggered on `apps/companion/**` changes only. Tag on the change spec, not in v1 scope.

## Risks / Trade-offs

- **Widget complexity vs Flutter ergonomics.** The widget is roughly half a week of Kotlin work — the most native-leaning part of the build. Real cost; pays off in the killer hydration UX.
- **Schema drift between widget Room and Flutter Drift.** Mitigated by writing the Room schema as a read-only view that Flutter updates via `home_widget` package after every hydration log. The widget never reads anything Flutter doesn't periodically refresh.
- **Token shared between Kotlin and Dart.** Both touch EncryptedSharedPreferences via Android Keystore. The bridge is a small Kotlin helper exposed through a Flutter method channel. Vetted pattern.
- **Offline pending queue is a moving part.** Failure modes worth thinking about explicitly: corrupted body bytes in SQLite, idempotency_key_conflict on replay if the user edited a request before retry, permanent 4xx errors that should not retry. Handled in spec scenarios.
- **No CI gate on the app.** A bad refactor to the Flutter codebase can ship without anyone noticing until next manual run. Mitigation: keep `task app:test` in the manual pre-merge ritual, and consider a GitHub Action later.
- **Flutter Web port inherits an awkward widget gap.** Web has no home-screen widgets; the hydration killer interaction does not translate. The Web port has to find a different surface (lock-screen quick action? notification action?). Out of scope here, but worth not pretending it does not exist.

## Migration Plan

This is a new client, not a refactor. There is nothing to migrate from. Two sequencing notes:

1. **Backend prereqs first.** `add-last-logged-quantity` and `add-meal-from-photo` should be applied and archived before this change starts. The app's killer interactions assume those endpoints exist.
2. **Walk before chat.** V1 ships without chat. Wiring the bottom nav with a placeholder `+` slot is sufficient — v2 lands chat in that slot without restructuring.

Rollback: delete `apps/companion/`. Nothing in the backend depends on it.

## Open Questions

- **HEIC.** Flutter `image_picker` can be told to return JPEG. v1 forces this and the backend rejects HEIC. If a user finds a flow where iOS later wants HEIC, address in the iOS-port change.
- **WorkManager periodicity.** 15 minutes is the Android-imposed minimum for periodic work. Anything shorter requires foreground services or exact alarms. 15 min is fine for the backstop role; foreground triggers cover the urgent case.
- **Photo confidence threshold.** The `confidence` field returned by `POST /meals/from_photo` is informational. Where does the app draw the "are you sure?" line? Current default: `< 0.6` shows a confirm sheet with editable name and quantity. Adjustable later.
- **Should the widget show kcal adherence too?** v1 says hydration only because hydration is the only thing that needs a one-tap log surface. Kcal "am I on track" reads as a quick app-foreground glance, not a tap-from-widget interaction. Revisit if a second widget is asked for.
- **PWA on Web later — is the killer-interaction story strong enough?** The two cross-platform killers (camera, offline) survive. The third (widget) does not. Acceptable; the Web port lives or dies on the first two.
