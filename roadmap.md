# Project Roadmap

_Generated from OpenSpec changes. Last refreshed: 2026-06-08 by the `roadmap` skill._

## Implemented

| Date | Change | Summary | Implementer(s) | Commit |
|---|---|---|---|---|
| 2026-06-08 | add-carb-load-auto-apply | `plan_carb_load` returns a deterministic per-day carb-load schedule but stops there — the agent then has to issue N separate `set_daily_goal_override` calls to actually put those numbers into the goal | Vinzenz Stadtmueller | _uncommitted_ |
| 2026-06-08 | add-date-varying-goals | A real endurance-training day looks nothing like a rest day in calories or carbs. The user's working numbers are roughly 2200 kcal training / 1900 kcal rest — same target shape, different values. | Vinzenz Stadtmueller | [`5d141a1`](https://github.com/vinzenzs/nutrition-api/commit/5d141a1) |
| 2026-06-08 | add-energy-availability | For an endurance athlete in a deficit, Energy Availability (EA) is the single most important number — it predicts both performance ceiling and longer-term hormonal/bone health. The Loucks bands are co | Vinzenz Stadtmueller | [`5d141a1`](https://github.com/vinzenzs/nutrition-api/commit/5d141a1) |
| 2026-06-08 | add-hydration-tracking | The system can answer "what did I eat?" but not "what did I drink?" For an endurance-training user, that's a real blind spot: a 2-hour ride can easily produce ≥2 L of sweat loss, and intake without th | Vinzenz Stadtmueller | [`5d141a1`](https://github.com/vinzenzs/nutrition-api/commit/5d141a1) |
| 2026-06-08 | add-last-logged-quantity | The Flutter companion app's killer interaction is **barcode-scan → log a meal in 2 taps**. The "log" tap is the moment that decides whether scanning is fast enough to use, and the *default quantity* | Vinzenz Stadtmueller | [`debcd6d`](https://github.com/vinzenzs/nutrition-api/commit/debcd6d) |
| 2026-06-08 | add-meal-workout-link | `add-workouts-capability` and `add-weight-log` landed the missing endurance-training primitives as standalone tables. They were deliberately scoped to "table + CRUD only" so each could ship in a small | Vinzenz Stadtmueller | [`5d141a1`](https://github.com/vinzenzs/nutrition-api/commit/5d141a1) |
| 2026-06-08 | add-weight-log | The Garmin coach reads body-weight data from Garmin Connect, but the API itself has nowhere to record a weight measurement taken any other way — a kitchen scale at the in-laws', a hotel gym, a smart s | Vinzenz Stadtmueller | [`5d141a1`](https://github.com/vinzenzs/nutrition-api/commit/5d141a1) |
| 2026-06-08 | add-workout-fuel | Sodium targets during endurance work are 300–800 mg/hr — entirely invisible to the system today. `log_hydration` records ml and a free-text `note`; it cannot tell you sodium intake during a 90-minute | Vinzenz Stadtmueller | [`5d141a1`](https://github.com/vinzenzs/nutrition-api/commit/5d141a1) |
| 2026-06-08 | add-workouts-capability | Today the API has no concept of a workout. That single gap blocks at least six tier-1/tier-2 needs that have surfaced from real triathlon-training use: attaching meals or fuel entries to a session, co | Vinzenz Stadtmueller | [`debcd6d`](https://github.com/vinzenzs/nutrition-api/commit/debcd6d) |
| 2026-06-07 | add-cookidoo-importer | Cookidoo (Vorwerk's Thermomix recipe platform) has no public API, but its recipe pages embed Schema.org `Recipe` JSON-LD for SEO — the same standard most major recipe sites use. The user cooks from Co | Vinzenz Stadtmueller | [`debcd6d`](https://github.com/vinzenzs/nutrition-api/commit/debcd6d) |
| 2026-06-07 | add-mcp-server | The REST API now stores meals and answers daily/range summary questions, but the LLM coaching agent — one of the two clients the platform was designed for — has no way to reach it yet. MCP is the righ | Vinzenz Stadtmueller | [`debcd6d`](https://github.com/vinzenzs/nutrition-api/commit/debcd6d) |
| 2026-06-07 | add-meal-logging-mvp | There is no backend yet for logging what the user eats. The user wants a personal nutrition log that they can write to from two clients — a mobile app (for barcode scans and quick entry) and, later, a | Vinzenz Stadtmueller | [`debcd6d`](https://github.com/vinzenzs/nutrition-api/commit/debcd6d) |
| 2026-06-07 | add-product-management-tools | The MCP test session that produced `harden-write-paths` and `unify-adherence-shape` left two product-hygiene findings unaddressed: | Vinzenz Stadtmueller | [`debcd6d`](https://github.com/vinzenzs/nutrition-api/commit/debcd6d) |
| 2026-06-07 | add-race-prep-primitives | A race week has a specific carb-loading shape — roughly 8–12 g of carbohydrate per kg of body weight per day for the 3 days before, then ~2 g/kg as a pre-race meal on race morning — and it's exactly t | Vinzenz Stadtmueller | [`debcd6d`](https://github.com/vinzenzs/nutrition-api/commit/debcd6d) |
| 2026-06-07 | add-swagger-cobra-viper | The API server's entrypoint mixes ad-hoc env parsing with bespoke helpers, has no CLI surface for ops tasks (e.g., running migrations alone), and exposes no machine-readable API contract for mobile/ag | Vinzenz Stadtmueller | [`debcd6d`](https://github.com/vinzenzs/nutrition-api/commit/debcd6d) |
| 2026-06-07 | daily-use-essentials | The MVP ships single-ingredient logging, macro-only summaries, and no concept of "did I hit my day?" — fine for a demo, painful by day three. Real meals are 3–5 ingredients (skyr + oats + berries + ho | Vinzenz Stadtmueller | [`debcd6d`](https://github.com/vinzenzs/nutrition-api/commit/debcd6d) |
| 2026-06-07 | harden-write-paths | A real MCP-driven test session surfaced two correctness bugs and one rough edge in the write paths: | Vinzenz Stadtmueller | [`debcd6d`](https://github.com/vinzenzs/nutrition-api/commit/debcd6d) |
| 2026-06-07 | streamline-local-dev | Local development today requires a handful of disconnected steps: start a Postgres container, edit `.env` with hand-generated tokens, source it, then `make run`. Each step has its own failure mode and | Vinzenz Stadtmueller | [`debcd6d`](https://github.com/vinzenzs/nutrition-api/commit/debcd6d) |
| 2026-06-07 | unify-adherence-shape | The same MCP test session that produced `harden-write-paths` also flagged three API-shape inconsistencies. None corrupts data; all annoy clients and make the API feel under-baked: | Vinzenz Stadtmueller | [`debcd6d`](https://github.com/vinzenzs/nutrition-api/commit/debcd6d) |

## Planned

| Change | Summary | Proposed by | Proposed |
|---|---|---|---|
| add-flutter-companion-app | The system was designed for two clients from day one — agent (via MCP) and mobile (`MOBILE_API_TOKEN`) — but only the agent client has materialized. The agent is great at conversation: parsing freefor | Vinzenz Stadtmueller | 2026-06-08 |
| add-meal-from-photo | The Flutter companion app's second killer interaction is **photo-of-meal**: the user is about to eat something that has no barcode and no obvious analog in the cache (a restaurant plate, a homemade di | Vinzenz Stadtmueller | 2026-06-08 |
| add-rolling-window-summaries | Almost every nutrition-science recommendation an endurance athlete cares about is a multi-day average, not a single-day total: | Vinzenz Stadtmueller | 2026-06-08 |

---
_To regenerate: ask Claude "update the roadmap"._
_For the operational queue (in-progress + up-next + backlog), see [`continuity.md`](continuity.md)._
