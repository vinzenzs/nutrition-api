# Nutrition Companion (Flutter, Android)

The mobile companion to the kazper backend. It is a **focused supplement
to the agent, not a replacement** — three screens and three killer interactions
the phone does better than a chat surface:

- **Barcode → log** in two taps (scan mode).
- **Photo → log** via the backend vision endpoint (photo mode).
- **One-tap hydration** from an Android home-screen widget — works offline.

Everything that needs natural language (recipe building, "why is my iron low",
goal setting) stays in the agent. Goals here are **read-only**.

The capability spec lives at
[`openspec/specs/mobile-companion/spec.md`](../../openspec/specs/mobile-companion/spec.md);
the architecture rationale is in
[`docs/architecture.md`](docs/architecture.md). v1 is Android only.

## Prerequisites

- [Flutter SDK](https://docs.flutter.dev/get-started/install) (stable, Dart ≥ 3.11).
- Android SDK (min SDK 26 / Android 8.0).
- A running backend (`task dev` in the repo root) with `MOBILE_API_TOKEN` set.
- `qrencode` on the backend host for pairing (`brew install qrencode`).

All tasks are driven from the **repo root** via [Task](https://taskfile.dev/):

```bash
task app:run     # flutter run with dart-defines
task app:build   # flutter build apk --release
task app:test    # flutter test
task app:pair    # alias for `task dev:pair` — print the pairing QR
```

## Run it

1. Boot the backend: `task dev`.
2. Connect an Android device or start an emulator.
3. `task app:run`.
4. On first launch the app shows the **pairing** screen. Run `task dev:pair`
   on the backend, then point the phone at the QR code printed in the terminal.
   The payload is `{"base_url": "...", "token": "..."}`; the token lands in the
   Android Keystore (`flutter_secure_storage`) and is mirrored to
   EncryptedSharedPreferences for the widget.

## Build an APK

```bash
task app:build
# → build/app/outputs/flutter-apk/app-release.apk  (sideload it)
```

## Architecture at a glance

```
lib/
  data/
    auth/          token store (Keystore) + Kotlin bridge
    db/            Drift: products_cache, recent_summary, pending_writes, widget_failures
    net/           Dio client + idempotency-key minting
    sync/          outbox worker + replay triggers (foreground / connectivity / WorkManager)
    repository.dart  read (stale-while-revalidate) + write (enqueue) surface
  domain/          hand-rolled JSON models mirroring the REST responses
  state/           Riverpod providers (today, recent, scan, goals, pairing)
  ui/              today / camera / recent / pair / settings + the home shell
android/app/src/main/kotlin/.../widget/
                   the A-lite hydration widget (Kotlin + Room + WorkManager + OkHttp)
```

- **Offline-first.** Every mutation is enqueued in `pending_writes` with a
  client-minted `Idempotency-Key` before any network call, then flushed by the
  outbox worker. Reads render from a local cache and revalidate in the
  background — no offline banner.
- **A-lite widget.** The Kotlin widget POSTs hydration directly on tap (no
  Flutter wakeup); on failure it writes a row to Drift's `widget_failures`
  table, which the app drains into the outbox on next foreground.

## Tests

```bash
task app:test            # Dart: providers, outbox worker, screens, integration
# Kotlin widget test (Robolectric) runs via Gradle:
cd android && ./gradlew testDebugUnitTest
```

The Flutter app is **not** a CI gate on the Go backend in v1 — run `task app:test`
manually before pushing app changes.
