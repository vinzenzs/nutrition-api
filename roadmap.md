# Project Roadmap

_Generated from OpenSpec changes. Last refreshed: 2026-06-12 by the `roadmap` skill._

## Implemented

| Date | Change | Summary | Implementer(s) | Commit |
|---|---|---|---|---|
| 2026-06-11 | add-chat-backend | Phone-reachable nutrition chat — a server-side Anthropic agent loop over the REST API streamed as SSE, since the coaching agent otherwise only runs desktop stdio MCP. | Vinzenz Stadtmueller | [`9938907`](https://github.com/vinzenzs/nutrition-api/commit/9938907) |
| 2026-06-11 | add-companion-chat | Fills the reserved fourth nav slot: an in-app chat screen plus the Today plan card and a shopping-list screen. | Vinzenz Stadtmueller | [`c57c58e`](https://github.com/vinzenzs/nutrition-api/commit/c57c58e) |
| 2026-06-11 | add-companion-food-picker | Camera-screen food picker (recent / search / quick-create) so logging a food isn't limited to the three original fixed paths. | Vinzenz Stadtmueller | [`127a935`](https://github.com/vinzenzs/nutrition-api/commit/127a935) |
| 2026-06-11 | add-flutter-companion-app | The Flutter Android companion app — the long-promised mobile client (barcode scan, photo, home-screen hydration widget). | Vinzenz Stadtmueller | [`61568d9`](https://github.com/vinzenzs/nutrition-api/commit/61568d9) |
| 2026-06-11 | add-meal-plan | A planned-meal primitive: persist a meal *selection* that isn't yet an event; the eaten transition atomically logs a real meal entry. | Vinzenz Stadtmueller | [`4686f10`](https://github.com/vinzenzs/nutrition-api/commit/4686f10) |
| 2026-06-11 | add-race-fueling-plan | A durable race entity plus a deterministic per-leg race-day fuelling plan (carbs / sodium / fluid). | Vinzenz Stadtmueller | [`34e2f36`](https://github.com/vinzenzs/nutrition-api/commit/34e2f36) |
| 2026-06-11 | add-recipe-ingredients | Capture Cookidoo recipes' verbatim ingredient strings from the page JSON-LD, plus a server-side import endpoint. | Vinzenz Stadtmueller | [`78f547b`](https://github.com/vinzenzs/nutrition-api/commit/78f547b) |
| 2026-06-11 | add-shopping-list | A persistent shopping-list checklist (bulk write + check-off); the agent merges recipe ingredients into it. | Vinzenz Stadtmueller | [`1de88cd`](https://github.com/vinzenzs/nutrition-api/commit/1de88cd) |
| 2026-06-10 | add-deployment-pipeline | Ship the API off localhost with a real deployment pipeline. | Vinzenz Stadtmueller | [`8bad270`](https://github.com/vinzenzs/nutrition-api/commit/8bad270) |
| 2026-06-10 | add-garmin-daily-metrics | Store Garmin's daily recovery/fitness stream the importer previously computed on every run and discarded. | Vinzenz Stadtmueller | [`60a13e8`](https://github.com/vinzenzs/nutrition-api/commit/60a13e8) |
| 2026-06-10 | add-hydration-balance-metrics | Persist Garmin's daily sweat-loss and activity-intake hydration-balance numbers. | Vinzenz Stadtmueller | [`46978a2`](https://github.com/vinzenzs/nutrition-api/commit/46978a2) |
| 2026-06-10 | widen-workout-ingestion | Ingest the per-activity metrics Garmin already measures (distance, power, temperature, sweat loss) instead of guessing them. | Vinzenz Stadtmueller | [`6952e61`](https://github.com/vinzenzs/nutrition-api/commit/6952e61) |
| 2026-06-09 | add-daily-context-aggregator | One `/context/daily` call replacing the 5–7 separate queries the agent made to start every session. | Vinzenz Stadtmueller | [`8e51019`](https://github.com/vinzenzs/nutrition-api/commit/8e51019) |
| 2026-06-09 | add-meal-from-photo | Photo-of-meal logging via a backend Claude vision endpoint. | Vinzenz Stadtmueller | [`68d0f0c`](https://github.com/vinzenzs/nutrition-api/commit/68d0f0c) |
| 2026-06-09 | add-protein-distribution | Per-meal protein-distribution analysis — MPS is triggered per-meal, not per-day. | Vinzenz Stadtmueller | [`8e51019`](https://github.com/vinzenzs/nutrition-api/commit/8e51019) |
| 2026-06-09 | add-recommend-workout-fuel | Recommend in-workout fuel for a session, bridging pre-race carb-load and daily training targets. | Vinzenz Stadtmueller | [`66a2085`](https://github.com/vinzenzs/nutrition-api/commit/66a2085) |
| 2026-06-09 | add-rolling-window-summaries | Multi-day rolling averages — most nutrition-science targets are averages, not single-day totals. | Vinzenz Stadtmueller | [`e8c33b6`](https://github.com/vinzenzs/nutrition-api/commit/e8c33b6) |
| 2026-06-09 | add-training-phases-and-templates | Training phases plus goal templates, so each day knows its build/peak/recovery block. | Vinzenz Stadtmueller | [`8e51019`](https://github.com/vinzenzs/nutrition-api/commit/8e51019) |
| 2026-06-09 | add-workout-rpe-and-gi | Record RPE and GI-distress on workouts to support the build-phase fuelling-rehearsal mandate. | Vinzenz Stadtmueller | [`303cd60`](https://github.com/vinzenzs/nutrition-api/commit/303cd60) |
| 2026-06-08 | add-carb-load-auto-apply | One call to apply a carb-load schedule into goal overrides instead of N manual override calls. | Vinzenz Stadtmueller | [`4026c8e`](https://github.com/vinzenzs/nutrition-api/commit/4026c8e) |
| 2026-06-08 | add-date-varying-goals | Per-date goal overrides — distinct training-day vs rest-day targets. | Vinzenz Stadtmueller | [`5d141a1`](https://github.com/vinzenzs/nutrition-api/commit/5d141a1) |
| 2026-06-08 | add-energy-availability | Energy Availability (EA) computation with the Loucks risk bands. | Vinzenz Stadtmueller | [`5d141a1`](https://github.com/vinzenzs/nutrition-api/commit/5d141a1) |
| 2026-06-08 | add-hydration-tracking | Hydration logging in ml — answer "what did I drink?" alongside "what did I eat?". | Vinzenz Stadtmueller | [`5d141a1`](https://github.com/vinzenzs/nutrition-api/commit/5d141a1) |
| 2026-06-08 | add-last-logged-quantity | Track the last-logged quantity per product so the scan→log flow defaults sensibly (2 taps). | Vinzenz Stadtmueller | [`debcd6d`](https://github.com/vinzenzs/nutrition-api/commit/debcd6d) |
| 2026-06-08 | add-meal-workout-link | An optional `workout_id` link on meals, hydration, and fuel entries. | Vinzenz Stadtmueller | [`5d141a1`](https://github.com/vinzenzs/nutrition-api/commit/5d141a1) |
| 2026-06-08 | add-weight-log | Body-weight logging from any source, not just Garmin Connect. | Vinzenz Stadtmueller | [`5d141a1`](https://github.com/vinzenzs/nutrition-api/commit/5d141a1) |
| 2026-06-08 | add-workout-fuel | In-workout fuel entries (sodium / carbs / caffeine) beyond plain hydration ml. | Vinzenz Stadtmueller | [`5d141a1`](https://github.com/vinzenzs/nutrition-api/commit/5d141a1) |
| 2026-06-08 | add-workouts-capability | The workout primitive — the session entity the rest of the fuelling model hangs off. | Vinzenz Stadtmueller | [`debcd6d`](https://github.com/vinzenzs/nutrition-api/commit/debcd6d) |
| 2026-06-07 | add-cookidoo-importer | A Chrome extension importing Cookidoo recipes via their embedded Schema.org `Recipe` JSON-LD. | Vinzenz Stadtmueller | [`debcd6d`](https://github.com/vinzenzs/nutrition-api/commit/debcd6d) |
| 2026-06-07 | add-mcp-server | The MCP server — the LLM coaching agent's surface over the REST API. | Vinzenz Stadtmueller | [`debcd6d`](https://github.com/vinzenzs/nutrition-api/commit/debcd6d) |
| 2026-06-07 | add-meal-logging-mvp | The first backend: log what the user eats, writable from a mobile app and (later) the agent. | Vinzenz Stadtmueller | [`debcd6d`](https://github.com/vinzenzs/nutrition-api/commit/debcd6d) |
| 2026-06-07 | add-product-management-tools | Product-hygiene tools (list / delete) surfaced from an MCP test session. | Vinzenz Stadtmueller | [`debcd6d`](https://github.com/vinzenzs/nutrition-api/commit/debcd6d) |
| 2026-06-07 | add-race-prep-primitives | Stateless carb-loading math for race week (g/kg/day bands + race-morning meal). | Vinzenz Stadtmueller | [`debcd6d`](https://github.com/vinzenzs/nutrition-api/commit/debcd6d) |
| 2026-06-07 | add-swagger-cobra-viper | A Cobra/Viper CLI plus a Swagger/OpenAPI contract and config cleanup for the server entrypoint. | Vinzenz Stadtmueller | [`debcd6d`](https://github.com/vinzenzs/nutrition-api/commit/debcd6d) |
| 2026-06-07 | daily-use-essentials | Past the MVP: composite recipes, micronutrients, and "did I hit my day?" adherence. | Vinzenz Stadtmueller | [`debcd6d`](https://github.com/vinzenzs/nutrition-api/commit/debcd6d) |
| 2026-06-07 | harden-write-paths | Correctness fixes on the write paths surfaced by a real MCP-driven test session. | Vinzenz Stadtmueller | [`debcd6d`](https://github.com/vinzenzs/nutrition-api/commit/debcd6d) |
| 2026-06-07 | streamline-local-dev | One-command local development (Postgres up + `.env.local` + serve). | Vinzenz Stadtmueller | [`debcd6d`](https://github.com/vinzenzs/nutrition-api/commit/debcd6d) |
| 2026-06-07 | unify-adherence-shape | API-shape consistency fixes (adherence block and others) flagged in an MCP test session. | Vinzenz Stadtmueller | [`debcd6d`](https://github.com/vinzenzs/nutrition-api/commit/debcd6d) |

## Planned

_None — every proposed OpenSpec change has been implemented and archived. New work starts with `/opsx:propose <slug>` (triage framing lives in `openspec/priorities.md`)._

---
_To regenerate: ask Claude "update the roadmap"._
