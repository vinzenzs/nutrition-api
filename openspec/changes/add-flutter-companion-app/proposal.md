## Why

The system was designed for two clients from day one — agent (via MCP) and mobile (`MOBILE_API_TOKEN`) — but only the agent client has materialized. The agent is great at conversation: parsing freeform meals, building recipes, reflecting on a week of data. It is structurally bad at three things the phone is structurally good at:

1. **Glance-ability.** "Am I on track right now?" — a number on a home screen, not a typed question.
2. **Camera-driven input.** Barcode in your hand, photo of a plate in front of you.
3. **In-the-moment friction.** Standing in a café queue, walking the dog, drinking a glass of water. Unlocking a laptop and opening Claude Code is the wrong shape.

The companion app's stance is **not a replacement for the agent — a focused supplement**. Three screens, three killer interactions. Anything that needs natural language stays in the agent. The two clients converge on the same REST API and the same database.

V1 is Android only, sideloaded APK, single user (you), Flutter. V2 will add a chat affordance inside the app (so non-MCP users can drive the system). V3+ will port to iOS and Web. Choosing Flutter now is what makes V3+ tractable.

## What Changes

### v1 screens

```
┌─────────────┐   ┌─────────────┐   ┌─────────────┐
│   TODAY     │   │   CAMERA    │   │   RECENT    │
│             │   │             │   │             │
│ adherence   │   │ [viewfinder]│   │ today's     │
│ rings + 💧  │   │             │   │ logs:       │
│ recent meals│   │ ▸ scan      │   │  banana     │
│ [+💧 button]│   │ ▸ photo     │   │  nutella    │
│             │   │             │   │  250ml 💧   │
│ settings ⚙  │   │             │   │             │
└─────────────┘   └─────────────┘   └─────────────┘
```

### v1 killer interactions

- **Barcode → log**: viewfinder → detection → product card with default quantity (from `products.last_logged_quantity_g`, falling back to `serving_size_g`, falling back to 100g) → 2 taps total.
- **Photo → log**: viewfinder switches to photo mode → snap → `POST /meals/from_photo` → returned meal entry confirmed with one tap.
- **Hydration widget**: Android home-screen widget. Tap = `POST /hydration` for one configured glass size. Works offline via the **A-lite pattern** (direct HTTP from Kotlin; on failure, write to a `widget_failures` SQLite table that the Flutter app drains on next foreground).

### v1 client architecture

- **Offline-first.** Outbox queue (`pending_writes`) for every mutation. Idempotency-Key generated client-side per operation. Replays on app foreground, network change, and a 15-min `WorkManager` backstop.
- **Stale-while-revalidate** reads. Today and Recent render from a local `recent_summary` cache, refresh in the background. No "offline" banner — the timestamp on the data is the implicit freshness signal.
- **Static-token auth** stored in Android Keystore via `flutter_secure_storage`. One-time pairing: scan a QR code printed by the backend on `task dev` boot.
- **Goals are read-only** in v1. Edit goals via the agent. The app shows the rings; setting them is rare and the agent does it better.
- **Health Connect** is deferred to v1.5 (separate change).

### Backend dependencies

The Flutter app blocks on two backend changes already drafted but unapplied:

- **`add-last-logged-quantity`**: adds the column the phone needs to default barcode quantities cleanly. Without it, the scan→log flow drops from 2 taps to 3.
- **`add-meal-from-photo`**: the new `POST /meals/from_photo` endpoint + `internal/vision/` integration. Without it, the photo-of-meal killer interaction does not exist.

Both should ship before this change starts. They are scoped small (half a day, two days respectively).

## Capabilities

### New Capabilities

- `mobile-companion`: Platform-agnostic specification of the companion app's behavior — offline-first outbox, idempotent replay, the A-lite widget contract, secure token storage, the camera-first input model, and the read-only-goals stance. V1 implementation is Flutter on Android; the capability is written so future iOS and Web ports are bound by the same invariants without re-spec.

### Modified Capabilities

None. The Flutter app is a new consumer of existing capabilities (`auth`, `meals`, `products`, `hydration`, `nutrition-goals`, summaries) and does not change the REST contract. The two backend changes it depends on (`add-last-logged-quantity`, `add-meal-from-photo`) carry their own capability deltas separately.

## Impact

- **New codebase location.** Flutter source lives at `apps/companion/` in this monorepo (sibling to `cmd/`, `internal/`). Rationale captured in design.md.
- **New dependencies.** Flutter SDK toolchain (Dart), `mobile_scanner`, `image_picker`, `flutter_secure_storage`, `drift`, `riverpod` (or equivalent), `dio` (or `http`), `home_widget` (thin Kotlin bridge), `connectivity_plus`, `workmanager`. List finalized in design.md.
- **Build pipeline.** New Android-specific tasks in `Taskfile.yml`: `task app:run`, `task app:build`, `task app:test`. The Go backend's test pipeline is untouched.
- **Distribution.** Sideloaded APK for v1. No Play Store listing.
- **Backend hits no new endpoints from the app alone** — it uses the existing surface plus the two endpoints introduced by the dependency changes. No additional API contract work belongs to this change.
- **Documentation.** New `apps/companion/README.md` for the app build. README at repo root gets a "Mobile companion" subsection pointing into it. RUN_LOCAL.md gets a section on pairing (the QR-code flow).
- **CI.** v1 does not gate the Go CI on Flutter builds — the app can compile-break without breaking backend CI. Optional follow-up: a parallel CI job that runs `flutter test` on the app on PRs that touch `apps/companion/`.

### Out of scope (explicit non-goals)

- iOS and Web ports. The capability spec is written platform-agnostically so they are *cheaper* later, but no iOS/Web code lands in this change.
- Apple Watch / Wear OS complications.
- In-app chat with Claude (v2 ambition; the architecture leaves a slot for it but no chat surface ships in v1).
- Apple HealthKit / Google Health Connect integration (v1.5).
- Real OAuth / multi-user. Static token is fine for single-user sideload; the auth model survives v1 only.
- Push notifications, meal reminders, scheduled hydration nudges. All v2+.
- Photo storage / meal-photo history. The backend currently does not store the image; the app does not cache it either.
- Editing recipes from the app. Recipes are agent-driven in v1; the app can log them and view their components via `?expand=components`.
- Light/dark theme branching. Material You dynamic colors come for free on Android 12+; that is the look.
