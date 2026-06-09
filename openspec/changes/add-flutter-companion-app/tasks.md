## 1. Repo scaffolding

- [x] 1.1 Create `apps/companion/` and initialize a Flutter project via `flutter create --org com.corelyr --project-name nutrition_companion --platforms=android .` (run from inside the dir).
- [x] 1.2 Set min SDK to 26 (Android 8) in `android/app/build.gradle`. Set target / compile SDK to current latest.
- [x] 1.3 Add `apps/companion/.gitignore` entries for `build/`, `.dart_tool/`, `.flutter-plugins-dependencies`, platform-specific cruft.
- [x] 1.4 Update repo root `.gitignore` if there are conflicts with the Flutter ones.
- [x] 1.5 Extend `Taskfile.yml` with: `task app:run` (flutter run), `task app:build` (flutter build apk --release), `task app:test` (flutter test), `task app:pair` (alias for `task dev:pair`).

## 2. Backend dev:pair helper

- [x] 2.1 Extend `Taskfile.yml` with a `dev:pair` task that prints the pairing QR code: `qrencode -t ANSIUTF8 "$(printf '{"base_url":"http://%s:%s","token":"%s"}' "$(hostname -I | awk '{print $1}')" "${HTTP_ADDR#:}" "$MOBILE_API_TOKEN")"`.
- [x] 2.2 Document the prerequisite (`brew install qrencode` on macOS) in RUN_LOCAL.md.
- [x] 2.3 Test that the QR scans correctly via a phone QR reader; the payload should parse as JSON.

## 3. Dart dependencies

- [x] 3.1 Add to `pubspec.yaml`: `flutter_riverpod`, `drift`, `drift_flutter`, `sqlite3_flutter_libs`, `dio`, `mobile_scanner`, `image_picker`, `flutter_secure_storage`, `connectivity_plus`, `workmanager`, `home_widget`, `qr_code_scanner` (for pairing scan), `intl`, `path_provider`. _Implementation note: `qr_code_scanner` is discontinued upstream; `mobile_scanner` is used for both barcodes and the pairing-QR scan to keep the dep surface small._
- [x] 3.2 Dev deps: `drift_dev`, `build_runner`, `flutter_lints`, `mocktail`.
- [x] 3.3 Run `flutter pub get` and resolve any peer-conflict warnings.
- [x] 3.4 Add a `dart-defines.json` file checked into the repo (NOT a secret file) with build-time toggles (debug logging level, feature flags). Reference it via `--dart-define-from-file=dart-defines.json` in `task app:run`.

## 4. Local persistence (Drift)

- [x] 4.1 Define a `AppDatabase` in `lib/data/db/app_database.dart` with the four tables specified in design.md: `products_cache`, `recent_summary`, `pending_writes`, `widget_failures`.
- [x] 4.2 Generate Drift code with `dart run build_runner build --delete-conflicting-outputs`.
- [x] 4.3 Add migration v1 (initial) and a placeholder v2 hook for the future.
- [x] 4.4 Write a `dao/products_cache_dao.dart` with: `upsertFromApi(product)`, `getById(id)`, `recentlyScanned(limit)`.
- [x] 4.5 Write a `dao/pending_writes_dao.dart` with: `enqueue(method, path, body, idemKey)`, `pendingInArrivalOrder()`, `markDone(id)`, `markFailedPermanent(id, error)`, `markInFlight(id)`.
- [x] 4.6 Write a `dao/widget_failures_dao.dart` with: `drainInto(pendingWritesDao)`.
- [x] 4.7 Write a `dao/recent_summary_dao.dart` with: `upsertForDate(date, tz, summary)`, `getForDate(date, tz)`.

## 5. Secure storage and token bridge

- [x] 5.1 `lib/data/auth/token_store.dart`: `TokenStore` interface with `getToken()`, `getBaseUrl()`, `pair(baseUrl, token)`, `clear()`. Implementation uses `flutter_secure_storage`.
- [x] 5.2 Native Android: `android/app/src/main/kotlin/.../TokenBridge.kt` — small helper class exposing `getToken()`/`getBaseUrl()` via EncryptedSharedPreferences. Both the Flutter app (via method channel) and the widget worker read through this.
- [x] 5.3 Mirror the secure storage writes into EncryptedSharedPreferences on pair: Flutter writes once, calls a method channel to ask the bridge to mirror.
- [x] 5.4 Unit test the `TokenStore` (round trip pair → get).

## 6. Networking and outbox

- [x] 6.1 `lib/data/net/api_client.dart`: Dio client configured with base URL + bearer interceptor (reads from `TokenStore`). Adds `Idempotency-Key` and `User-Agent` headers per request.
- [x] 6.2 `lib/data/net/idempotency.dart`: helper to mint a UUID-v4 per outbox row.
- [x] 6.3 `lib/data/sync/outbox_worker.dart`: a stream-based worker that drains `pending_writes` in arrival order, classifies responses (2xx → done, 4xx-not-409 → failed_permanent, 5xx / network → keep pending with backoff).
- [x] 6.4 `lib/data/sync/replay_triggers.dart`: wire `connectivity_plus` listener, `WidgetsBindingObserver` for app foreground, and a `Workmanager` periodic 15-min task to invoke the worker.
- [x] 6.5 On every foreground, drain `widget_failures` into `pending_writes` first (so widget retries flow through the same queue).
- [x] 6.6 Unit tests for the worker: cached replay returns same id; 4xx-non-409 marks failed_permanent; 5xx leaves row pending; backoff increments `attempt_count`.

## 7. Domain models

- [x] 7.1 `lib/domain/models.dart`: Dart classes for Product, MealEntry, DailySummary, HydrationEntry, Goals, Adherence. JSON serialization via `json_serializable` or hand-rolled fromJson (small surface; hand-roll is fine).
- [x] 7.2 Match nullability to the backend JSON (`omitempty` becomes `null`).

## 8. Riverpod providers

- [ ] 8.1 `lib/state/today_provider.dart`: `dailySummaryProvider` — reads `recent_summary` immediately, fires `GET /summary/daily` in background, emits both states.
- [ ] 8.2 `lib/state/recent_provider.dart`: today's meal entries + today's hydration entries (combines two API calls).
- [ ] 8.3 `lib/state/scan_provider.dart`: holds the current scanned barcode state, the cached product lookup result, and the log-in-progress state.
- [ ] 8.4 `lib/state/goals_provider.dart`: read-only goals state.
- [ ] 8.5 Provider unit tests where the API is stubbed; behaviour-only tests, no Drift on-disk.

## 9. Screen: Today

- [ ] 9.1 `lib/ui/today/today_page.dart`: scaffold with adherence rings (kcal, protein, fiber, iron, B12) plus a hydration progress card.
- [ ] 9.2 "Recent meals" list (last 3) below the rings.
- [ ] 9.3 Floating action: log a configured glass of water (`POST /hydration`) without leaving the screen.
- [ ] 9.4 Top-right gear → Settings sheet.
- [ ] 9.5 No "edit goals" affordance.
- [ ] 9.6 Empty-state when goals are unset: shows raw totals, "Set goals via the assistant" hint.

## 10. Screen: Camera

- [ ] 10.1 `lib/ui/camera/camera_page.dart`: viewfinder via `mobile_scanner`. Mode toggle (scan / photo) as a segmented control at top.
- [ ] 10.2 Scan mode: on detection, fade to product card. Quantity default per spec: `last_logged_quantity_g` → `serving_size_g` → 100. Meal_type from time of day.
- [ ] 10.3 Product card "Log" button → enqueue `POST /meals` in outbox. Confirmation toast.
- [ ] 10.4 Photo mode: capture button → multipart `POST /meals/from_photo`. Confirm-or-undo card based on confidence (≥0.75 auto-commit + undo, 0.6–0.75 amber confirm, <0.6 editable sheet).
- [ ] 10.5 OFF 404 sheet: "Describe it" (freeform input) or "Take a photo" (switches mode).
- [ ] 10.6 Vision 503 sheet: "Describe instead."
- [ ] 10.7 Camera permission denial: clear empty state with "Open Settings" button.

## 11. Screen: Recent

- [ ] 11.1 `lib/ui/recent/recent_page.dart`: scrollable list combining today's meals and hydration entries, ordered by logged_at desc.
- [ ] 11.2 Tap a meal → detail sheet (edit quantity, change meal_type, delete).
- [ ] 11.3 Tap a hydration entry → delete (only).
- [ ] 11.4 Pull-to-refresh re-fetches summary.

## 12. Pairing flow

- [ ] 12.1 `lib/ui/pair/pair_page.dart`: shown when `TokenStore.getToken()` is null at app start.
- [ ] 12.2 QR scanner (`qr_code_scanner`) reads the payload, validates JSON shape, stores via `TokenStore.pair`.
- [ ] 12.3 Failure UX: inline error, retry.
- [ ] 12.4 Once paired, navigate to Today and clear the pair page from the stack.

## 13. Settings sheet

- [ ] 13.1 Displays current `base_url` (read-only), server health (calls `GET /healthz`).
- [ ] 13.2 "Unpair" button → clears storage and returns to pair screen.
- [ ] 13.3 Configurable glass size (default 250ml). Stored in regular preferences (not secure).
- [ ] 13.4 Build version / commit ref display.

## 14. Android widget

- [ ] 14.1 Native Kotlin module `android/app/src/main/kotlin/.../widget/HydrationWidget.kt` implementing `AppWidgetProvider`. Layout in `android/app/src/main/res/layout/hydration_widget.xml`.
- [ ] 14.2 Room snapshot DB at `android/app/src/main/kotlin/.../widget/SnapshotDb.kt` — single table `hydration_snapshot(date PRIMARY KEY, total_ml, daily_goal_ml, updated_at)`.
- [ ] 14.3 `HydrationTapWorker.kt` (`androidx.work.CoroutineWorker`): reads token via `TokenBridge`, POSTs `/hydration`, updates Room snapshot, schedules widget refresh.
- [ ] 14.4 On non-2xx network failure: write row to `widget_failures` SQLite (path-aligned with Drift's).
- [ ] 14.5 `home_widget` plugin wiring on the Flutter side: after every hydration log, push fresh totals into the Room snapshot via the bridge.
- [ ] 14.6 Manifest entries: widget receiver, default config, periodic update (every 30 min via `android:updatePeriodMillis` as a backstop on top of explicit refresh requests).
- [ ] 14.7 Widget Kotlin unit test (against Robolectric or instrumented test) that the tap path makes the right HTTP call and writes to widget_failures on offline.

## 15. Goals rendering (read-only)

- [ ] 15.1 Adherence rings on Today read from the `adherence` block in the daily summary response.
- [ ] 15.2 Color bands: green for `on`, amber for `under`, red for `over`. Micros with non-null adherence appear as small dots below the macro rings.
- [ ] 15.3 If `adherence` is absent (no goals set), no rings; raw totals shown with the "Set goals via the assistant" hint.

## 16. Photo confidence handling

- [ ] 16.1 Implement the three confidence bands per spec (≥0.75 auto + undo, 0.6–0.75 amber confirm, <0.6 editable sheet).
- [ ] 16.2 Editable sheet: fields for name, quantity, kcal, protein, carbs, fat. Pre-populated from the inference. Submit calls `POST /meals/freeform` with the edited values.
- [ ] 16.3 "Undo" on auto-commit calls `DELETE /meals/{id}` (no confirmation prompt).

## 17. Tests

- [ ] 17.1 Widget tests for each screen (rings render, list renders, sheet opens) using golden tests for visual stability.
- [ ] 17.2 Integration test: pair → log meal via barcode (mocked API) → see it in Recent.
- [ ] 17.3 Integration test: offline log → reconnect → queue drains → meal appears in API.
- [ ] 17.4 Kotlin widget instrumented test for the tap path.

## 18. Docs

- [ ] 18.1 `apps/companion/README.md`: how to install Flutter, run the app, build the APK, pair to a local backend.
- [ ] 18.2 Repo-root README: add a "Mobile companion" subsection pointing into `apps/companion/`.
- [ ] 18.3 RUN_LOCAL.md: section on `task dev:pair` and what to expect (QR in the terminal).
- [ ] 18.4 Capture the architectural diagram from design.md into `apps/companion/docs/architecture.md` (or just link back to the OpenSpec design).

## 19. Pre-merge checks

- [ ] 19.1 `task vet` (Go side) clean — no changes here, but verify the backend still builds.
- [ ] 19.2 `task test` (Go side) green.
- [ ] 19.3 `task app:test` (Flutter side) green.
- [ ] 19.4 Manual: walk the killer interactions with a real backend and real device — scan a barcode, take a photo, tap the widget.
- [ ] 19.5 `openspec validate add-flutter-companion-app --strict` passes.
