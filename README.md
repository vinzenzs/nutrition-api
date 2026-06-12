# nutrition-api

A personal nutrition logging backend. Single user. Two clients: a mobile app
(barcode scans + manual entry) and an LLM coaching agent reached over MCP.

> **First time here?** See [**RUN_LOCAL.md**](RUN_LOCAL.md) for a 10-minute
> walkthrough from a fresh clone to a working API and registered MCP server.
> This README is the reference (config table, every endpoint, project layout).

Built with Go + Gin + Postgres. Product data comes from the public
[Open Food Facts](https://world.openfoodfacts.org) API; products are cached
locally on first lookup and the raw OFF payload is preserved per row for
future re-extraction.

## What it does

- Lookup-and-cache products by barcode from Open Food Facts
- Log meal entries by product + quantity in grams, or as freeform name +
  nutriment estimate (the path the LLM agent uses)
- Edit / delete meal entries
- Daily and ranged summaries computed in a user-supplied IANA timezone
- Search the product cache, ranked by recency of use
- Idempotent writes via `Idempotency-Key` so retries (especially from the
  agent) are safe

## Mobile companion

A Flutter (Android) companion app lives in [`apps/companion/`](apps/companion/).
It is a focused supplement to the agent — three screens (Today, Camera, Recent)
and three killer interactions: barcode→log, photo→log, and a one-tap hydration
home-screen widget that works offline. It talks to this same REST API using
`MOBILE_API_TOKEN`, paired once by scanning the QR printed by `task dev:pair`.
See [`apps/companion/README.md`](apps/companion/README.md) to build and run it.

## Garmin bridge

A small Python service in [`apps/garmin-bridge/`](apps/garmin-bridge/) fills the
recovery / fitness / hydration-balance / weight / workout tables from Garmin
Connect. It owns all of Garmin's unofficial-API surface (SSO, MFA, token
minting) so the Go backend stays Garmin-agnostic — a rare interactive MFA login
mints a long-lived token (stored in the backend), and a daily k8s CronJob drives
a headless `POST /sync` that maps a day's data onto this same REST API under
`GARMIN_API_TOKEN`. It is opt-in in the Helm chart (`garminBridge.enabled`). See
[`apps/garmin-bridge/README.md`](apps/garmin-bridge/README.md).

## Quickstart

```bash
# One command — brings up Postgres, generates dev env, runs the API
task dev

# Smoke test (from another terminal)
curl http://localhost:8080/healthz

# (Optional) Browse the OpenAPI docs
open http://localhost:8080/swagger/index.html
```

For the prerequisites, the dev-token defaults, and the full walkthrough,
see [`RUN_LOCAL.md`](RUN_LOCAL.md). For a more manual setup (your own
Postgres, your own tokens):

```bash
# 1. Start Postgres
docker run --rm -d --name nutrition-pg \
    -e POSTGRES_USER=nutrition -e POSTGRES_PASSWORD=nutrition \
    -e POSTGRES_DB=nutrition -p 5432:5432 postgres:17-alpine

# 2. Copy + edit env (generate real tokens with: openssl rand -hex 32)
cp .env.example .env

# 3. Run (migrations run on startup by default)
set -a; . ./.env; set +a
task run                # equivalent to: go run ./cmd/nutrition-api serve
```

The binary is a single Cobra-based command — invoke it as
`nutrition-api <subcommand>`:

| Subcommand                | Purpose                                                |
|---------------------------|--------------------------------------------------------|
| `nutrition-api serve`     | Run the HTTP REST API                                  |
| `nutrition-api mcp`       | Run the MCP server over stdio                          |
| `nutrition-api migrate`   | Apply pending database migrations and exit             |
| `nutrition-api version`   | Print the embedded version, commit, and build date     |

Run `nutrition-api <subcommand> --help` for the flags each accepts (the only
serve-specific flag today is `--addr`, which overrides `HTTP_ADDR`).

## Configuration (env)

| Variable                 | Default                                       | Purpose                                                              |
|--------------------------|-----------------------------------------------|----------------------------------------------------------------------|
| `DATABASE_URL`           | _required_                                    | Postgres connection string                                           |
| `HTTP_ADDR`              | `:8080`                                       | HTTP listen address                                                  |
| `MOBILE_API_TOKEN`       | _required_                                    | Bearer token for the mobile app                                      |
| `AGENT_API_TOKEN`        | _required_                                    | Bearer token for the LLM agent (must differ from `MOBILE_API_TOKEN`) |
| `GARMIN_API_TOKEN`       | _unset_                                       | Optional bearer token (`client_id=garmin`) for the garmin-bridge; when set must be ≥16 bytes and differ from the other two. Unset disables the `/garmin/token` endpoints (503 `garmin_disabled`) |
| `GARMIN_TOKEN_ENC_KEY`   | _unset_                                       | Base64-encoded 32-byte AES-256 key encrypting the stored Garmin token blob at rest; required only when `GARMIN_API_TOKEN` is set |
| `DEFAULT_USER_TZ`        | `UTC`                                         | IANA timezone used when summary endpoints omit `tz`                  |
| `OFF_TIMEOUT_SECONDS`    | `5`                                           | Open Food Facts request timeout                                      |
| `OFF_USER_AGENT_CONTACT` | `+https://github.com/vinzenzs/nutrition-api`  | Identification baked into the OFF `User-Agent`                       |
| `IDEMPOTENCY_TTL_HOURS`  | `24`                                          | How long idempotency records are retained before cleanup             |
| `MIGRATE_ON_START`       | `true`                                        | Run schema migrations at startup                                     |
| `SWAGGER_ENABLED`        | `false`                                       | Serve `/swagger/*` in release mode (always served in debug mode)     |
| `ANTHROPIC_API_KEY`      | _unset_                                       | Enables `POST /meals/from_photo`; when blank the endpoint returns 503 |
| `CLAUDE_VISION_MODEL`    | `claude-sonnet-4-6`                           | Model used by `/meals/from_photo`                                    |
| `VISION_TIMEOUT_SECONDS` | `15`                                          | Per-request timeout for the Anthropic call                           |
| `MEAL_FROM_PHOTO_MAX_BYTES` | `10485760`                                 | Max multipart body for `/meals/from_photo` (10 MB default)           |

## API at a glance

All endpoints require `Authorization: Bearer <token>`. Write endpoints accept
an optional `Idempotency-Key: <opaque-id>` header — retrying within the TTL
returns the original response; a different body with the same key returns
`409 idempotency_key_conflict`.

### Products

```bash
# Lookup by barcode (first call hits OFF, cached afterwards)
curl -X POST -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    http://localhost:8080/products/lookup/3017624010701

# Force a fresh OFF fetch and refresh the cache
curl -X POST -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/products/lookup/3017624010701?refresh=true"

# Create a manual product
curl -X POST -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"name":"Homemade granola","nutriments_per_100g":{"kcal":420,"protein_g":12}}' \
    http://localhost:8080/products

# Search the product cache (ranked by recency of use)
curl -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/products/search?q=granola"

# List the cache (paginated, most-recently-used first). Optional ?source=off|manual|recipe
curl -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/products?source=manual&limit=20"

# Fetch one. Response includes `last_logged_quantity_g` for previously logged
# products — the phone's scan→log flow uses it as the default quantity, so a
# repeat scan of a product the user keeps eating goes from 3 taps to 2.
curl -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    http://localhost:8080/products/<uuid>

# Delete a product. Returns 204 on success; 409 product_in_use_as_component
# with the using-recipes list if the product is referenced by any recipe.
curl -X DELETE -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    http://localhost:8080/products/<uuid>

# Fetch a recipe with its component breakdown
curl -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/products/<recipe-uuid>?expand=components"

# Create a composite recipe from existing component products
curl -X POST -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
         "name":"Morning skyr bowl",
         "serving_size_g": 250,
         "components": [
           {"product_id":"<skyr-uuid>","quantity_g":200},
           {"product_id":"<oats-uuid>","quantity_g":40},
           {"product_id":"<honey-uuid>","quantity_g":10}
         ]
       }' \
    http://localhost:8080/products/recipes

# Recompute a recipe's nutriments after a component changed (e.g. OFF refresh)
curl -X POST -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    http://localhost:8080/products/recipes/<recipe-uuid>/recompute
```

### Meals

```bash
# Log a meal from a known product
curl -X POST -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    -H "Content-Type: application/json" \
    -H "Idempotency-Key: $(uuidgen)" \
    -d '{"product_id":"<uuid>","quantity_g":150,"logged_at":"2026-06-06T12:30:00Z","meal_type":"lunch"}' \
    http://localhost:8080/meals

# Freeform log (LLM-agent friendly)
curl -X POST -H "Authorization: Bearer $AGENT_API_TOKEN" \
    -H "Content-Type: application/json" \
    -H "Idempotency-Key: $(uuidgen)" \
    -d '{
         "name":"banana",
         "nutriments_per_100g":{"kcal":89,"protein_g":1.1,"carbs_g":22.8,"fat_g":0.3},
         "quantity_g":120,
         "logged_at":"2026-06-06T10:00:00Z",
         "save_as_product": true
       }' \
    http://localhost:8080/meals/freeform

# List meals in a window (half-open [from, to))
curl -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/meals?from=2026-06-01T00:00:00Z&to=2026-06-07T00:00:00Z"

# Patch
curl -X PATCH -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"quantity_g":200}' \
    http://localhost:8080/meals/<uuid>

# Delete
curl -X DELETE -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    http://localhost:8080/meals/<uuid>

# Optionally link a meal to a workout (per add-meal-workout-link). The link is
# metadata for grouping; the workout-fueling summary aggregates by logged_at
# time-window matching, not by this tag. On PATCH, "" clears the link.
curl -X POST -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"product_id":"<uuid>","quantity_g":80,"logged_at":"2026-06-07T07:30:00Z","workout_id":"<workout-uuid>"}' \
    http://localhost:8080/meals
curl -X PATCH -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"workout_id":""}' \
    http://localhost:8080/meals/<uuid>
```

#### Photo of meal

`POST /meals/from_photo` accepts a JPEG or PNG of a plate and asks Claude
Vision to estimate `name` + `nutriments_per_100g`, then logs the result the
same way `/meals/freeform` would. Useful for "what's in this restaurant
dish" capture from the mobile app. Requires `ANTHROPIC_API_KEY` to be set
in the server's env; without it the endpoint returns `503
vision_unavailable`. HEIC is rejected with `415 unsupported_media_type` —
convert to JPEG on the client (e.g. iOS Photos export). Max body 10 MB
(configurable via `MEAL_FROM_PHOTO_MAX_BYTES`).

```bash
curl -X POST -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    -H "Idempotency-Key: $(uuidgen)" \
    -F "image=@plate.jpg" \
    -F "quantity_g=250" \
    -F "meal_type=lunch" \
    -F "logged_at=2026-06-09T12:30:00Z" \
    http://localhost:8080/meals/from_photo
```

Response is `{"meal": {...}, "inference": {"model": "...",
"confidence": 0.85, "claude_input_tokens": ..., "claude_output_tokens":
..., "original_image_bytes": ..., "resized_to": {"width": ..., "height":
...}}}` — `meal` matches the same envelope the other meal endpoints
return. Low-confidence results (< 0.5) are still returned; the client
should prompt the user to verify or edit before saving.

### Hydration

Volume-only — never mixed with grams. Separate table, separate endpoints, separate
daily summary. The optional `note` carries beverage context (e.g. `water`,
`iced coffee`, `electrolytes`). For drinks with nutriments (Coke, juice), also
log the macros via `/meals/freeform`.

```bash
# Log a hydration entry
curl -X POST -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    -H "Content-Type: application/json" \
    -H "Idempotency-Key: $(uuidgen)" \
    -d '{"quantity_ml":500,"logged_at":"2026-06-07T08:00:00Z","note":"water"}' \
    http://localhost:8080/hydration

# List in a window (half-open [from, to), 92-day cap)
curl -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/hydration?from=2026-06-07T00:00:00Z&to=2026-06-08T00:00:00Z"

# Patch / delete (same shape as meals)
curl -X PATCH -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"quantity_ml":250}' \
    http://localhost:8080/hydration/<uuid>
curl -X DELETE -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    http://localhost:8080/hydration/<uuid>

# Daily total + entries (separate from /summary/daily, which is nutrients-only)
curl -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/summary/hydration/daily?date=2026-06-07&tz=Europe/Berlin"

# Optionally tag a sip with a workout (per add-meal-workout-link). PATCH "" clears.
curl -X POST -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"quantity_ml":250,"logged_at":"2026-06-07T08:45:00Z","workout_id":"<workout-uuid>"}' \
    http://localhost:8080/hydration
```

### Workout fuel

In-session fueling — gels, electrolyte drinks, salt tabs, caffeine pre-race. Sibling to
hydration, deliberately a separate table so the ml-only hydration totals never have to
carry mg/g fields (the unit-isolation rationale that shipped with `add-hydration-tracking`).
Routing rule: plain water / juice (volume only) → `/hydration`; anything with electrolytes,
carbs, or caffeine → `/workout-fuel`. `name` is required (rehearsal data depends on knowing
*what* you took); at least one of `quantity_ml`/`carbs_g`/`sodium_mg`/`potassium_mg`/`caffeine_mg`
must be supplied. `caffeine_mg: 0` means "measured, no caffeine"; omitting means "not measured".

```bash
# Log a gel
curl -X POST -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    -H "Content-Type: application/json" \
    -H "Idempotency-Key: $(uuidgen)" \
    -d '{"name":"Maurten Gel 100","logged_at":"2026-06-07T08:45:00Z","carbs_g":25,"sodium_mg":0,"caffeine_mg":100}' \
    http://localhost:8080/workout-fuel

# Log an electrolyte drink tagged to a workout
curl -X POST -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    -H "Content-Type: application/json" \
    -H "Idempotency-Key: $(uuidgen)" \
    -d '{"name":"Skratch","logged_at":"2026-06-07T08:30:00Z","quantity_ml":500,"carbs_g":20,"sodium_mg":380,"workout_id":"<workout-uuid>"}' \
    http://localhost:8080/workout-fuel

# List entries in a window (half-open [from, to), 92-day cap)
curl -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/workout-fuel?from=2026-06-07T00:00:00Z&to=2026-06-08T00:00:00Z"

# Patch: change sodium; explicit null clears a column; "" on workout_id clears the link.
curl -X PATCH -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"sodium_mg":420}' \
    http://localhost:8080/workout-fuel/<uuid>

# Delete
curl -X DELETE -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    http://localhost:8080/workout-fuel/<uuid>
```

Note: workout-fuel ml does NOT contribute to `/summary/hydration/daily` and workout-fuel
carbs do NOT contribute to `/summary/daily` macro adherence. The two capabilities own
their own totals; the workout-anchored `/workouts/{id}/fueling` is the only composer.

### Workouts

A training session: sport, time window, optional intensity (`tss`) and burn (`kcal_burned`).
Backend exposes the write endpoint; the source-specific *writer* lives outside — `garmin.py`
translates Garmin Connect activities into the API shape and POSTs them. No Garmin code lives
in the backend; the same `/workouts` endpoint accepts manual entries, Strava, Apple Health,
or anything else a writer wants to translate. `external_id` is the dedup key — re-POSTing
the same workout updates it in place (Garmin re-syncs land here automatically). For first-time
backfill or any batch import, `/workouts/bulk` accepts up to 100 items per request with
per-item results so partial failures are reportable.

Beyond the core fields, a row carries optional **ingestion metrics** a watch typically
measures: `distance_m` (metres), `avg_power_w` (watts), `temperature_c` (°C), and
`sweat_loss_ml` (estimated sweat loss, ml — personalises fluid targets). All are nullable
and source-agnostic. A `session_group` key links the legs of a **brick / multisport** session:
set the same key (e.g. the Garmin parent activity id) on every leg, then fetch them together
with `?session_group=`. Each leg stays its own real row with its own sport and window — no
merged pseudo-workout. The fueling endpoint echoes `sweat_loss_ml` + `temperature_c` (sweat/
heat context for evaluating intake); the performance fields are not echoed there.

```bash
# Manual workout (gym session — no Garmin tracking)
curl -X POST -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
         "source":"manual",
         "sport":"strength",
         "name":"Push day",
         "started_at":"2026-06-07T18:00:00Z",
         "ended_at":"2026-06-07T19:00:00Z"
       }' \
    http://localhost:8080/workouts

# Garmin-shaped workout (external_id makes the call idempotent)
curl -X POST -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
         "external_id":"garmin:1234567",
         "source":"garmin",
         "sport":"bike",
         "name":"Morning Z2",
         "started_at":"2026-06-07T08:00:00Z",
         "ended_at":"2026-06-07T09:30:00Z",
         "kcal_burned":850,
         "avg_hr":135,
         "tss":78
       }' \
    http://localhost:8080/workouts
# Re-POSTing with the same external_id returns 200 (UPDATE), not 201 (INSERT).

# Bulk upsert (e.g. first-time Garmin backfill; max 100/request)
curl -X POST -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"workouts": [
          {"external_id":"garmin:1","source":"garmin","sport":"bike","started_at":"...","ended_at":"...","kcal_burned":700},
          {"external_id":"garmin:2","source":"garmin","sport":"run","started_at":"...","ended_at":"...","kcal_burned":420}
        ]}' \
    http://localhost:8080/workouts/bulk
# → { "results": [{"index":0,"id":"...","created":true}, {"index":1,"id":"...","created":true}] }

# Brick / multisport: post each leg with a shared session_group, then fetch them together.
curl -X POST -H "Authorization: Bearer $MOBILE_API_TOKEN" -H "Content-Type: application/json" \
    -d '{"external_id":"garmin:9876543-1","source":"garmin","sport":"bike",
         "started_at":"2026-06-07T08:00:00Z","ended_at":"2026-06-07T09:00:00Z",
         "distance_m":30000,"avg_power_w":190,"temperature_c":24,"sweat_loss_ml":900,
         "session_group":"garmin:9876543"}' \
    http://localhost:8080/workouts
curl -X POST -H "Authorization: Bearer $MOBILE_API_TOKEN" -H "Content-Type: application/json" \
    -d '{"external_id":"garmin:9876543-2","source":"garmin","sport":"run",
         "started_at":"2026-06-07T09:05:00Z","ended_at":"2026-06-07T09:35:00Z",
         "distance_m":5000,"session_group":"garmin:9876543"}' \
    http://localhost:8080/workouts
# Fetch both legs of that brick (window still required):
curl -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/workouts?from=2026-06-07T00:00:00Z&to=2026-06-08T00:00:00Z&session_group=garmin:9876543"

# List window
curl -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/workouts?from=2026-06-01T00:00:00Z&to=2026-06-08T00:00:00Z"

# Planned (scheduled) workout — status defaults to "completed"; "planned" allows a
# future started_at (up to 1 year). Promote it to completed via PATCH when it happens.
curl -X POST -H "Authorization: Bearer $MOBILE_API_TOKEN" -H "Content-Type: application/json" \
    -d '{"source":"garmin","sport":"bike","status":"planned","name":"Sat long ride","started_at":"2026-07-04T07:00:00Z","ended_at":"2026-07-04T10:00:00Z"}' \
    http://localhost:8080/workouts
# Fetch just the plan: GET /workouts?...&status=planned

# Patch TSS / notes / status (immutable: source, external_id, sport, started_at, ended_at)
curl -X PATCH -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"tss":85,"notes":"FTP changed last month"}' \
    http://localhost:8080/workouts/<uuid>

# Rehearsal-outcome data: PATCH rpe (Borg CR-10, 1..10) and gi_distress_score
# (1=no distress, 5=severe) after a fueling-rehearsal session. The canonical
# post-ride flow — pair with the workout-fuel entries logged during the ride
# to evaluate "what did I take vs how did it feel."
curl -X PATCH -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"rpe":7,"gi_distress_score":2,"notes":"SIS gel at km 60 sat heavy"}' \
    http://localhost:8080/workouts/<uuid>

# Retract a logged rehearsal value via explicit JSON null (tri-state PATCH).
# `clear_rpe: true` on the MCP tool is the same effect from the agent side.
curl -X PATCH -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"rpe":null}' \
    http://localhost:8080/workouts/<uuid>

# Delete
curl -X DELETE -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    http://localhost:8080/workouts/<uuid>

# Workout fueling: pre / intra / post intake windows (per add-meal-workout-link;
# extended by add-workout-fuel with the workout_fuel sub-object).
# Defaults: pre_window_min=240 (4h), post_window_min=60. Both bounded [0, 720].
# Aggregation is by logged_at time-window — an UNTAGGED meal in the pre-window
# still contributes; a tagged meal logged 8h before does NOT.
curl -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/workouts/<uuid>/fueling?pre_window_min=180&post_window_min=90"
# → {
#     "workout_id": "...",
#     "started_at": "...", "ended_at": "...",
#     "pre_window":   { "minutes": 180,
#                       "nutrition":    {"totals": {...}, "entry_count": 1},
#                       "hydration":    {"total_ml": 0,   "entry_count": 0},
#                       "workout_fuel": {"totals": {},    "entry_count": 0} },
#     "intra_window": { "minutes":  90,
#                       "nutrition":    {"totals": {...}, "entry_count": 0},
#                       "hydration":    {"total_ml": 500, "entry_count": 1},
#                       "workout_fuel": {"totals": {"quantity_ml": 500, "carbs_g": 45,
#                                                    "sodium_mg": 380, "caffeine_mg": 100},
#                                        "entry_count": 2} },
#     "post_window":  { "minutes":  90,
#                       "nutrition":    {"totals": {...}, "entry_count": 1},
#                       "hydration":    {"total_ml": 0,   "entry_count": 0},
#                       "workout_fuel": {"totals": {},    "entry_count": 0} }
#   }
```

### Body weight

A measurement event — kg, optionally body-fat % and the smart-scale biometrics a full Garmin
weigh-in reports (`muscle_mass_kg`, `body_water_pct`, `bone_mass_kg`, `bmi`), with a `note` for
context that affects readings (post-workout, hotel scale, non-morning timing). Multiple
measurements per day are allowed; the trend endpoint smooths them with a rolling average. Each
trend point carries `sample_count` so callers can tell a real trend from a sparse-data mirage (a
`rolling_avg_kg` from `sample_count: 1` is just that one sample).

```bash
# Log a morning weighing — body-fat % and smart-scale biometrics are all optional
curl -X POST -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    -H "Content-Type: application/json" \
    -H "Idempotency-Key: $(uuidgen)" \
    -d '{"weight_kg":72.5,"body_fat_pct":14.2,"muscle_mass_kg":58.4,"body_water_pct":55.1,"bone_mass_kg":3.2,"bmi":22.4,"logged_at":"2026-06-07T07:00:00Z","note":"morning, fasted"}' \
    http://localhost:8080/weight

# List entries in a window (half-open [from, to), 92-day cap)
curl -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/weight?from=2026-06-01T00:00:00Z&to=2026-06-08T00:00:00Z"

# Rolling-average trend (default window_days=7; 366-day range cap)
curl -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/weight/trend?from=2026-05-01&to=2026-06-07&window_days=7&tz=Europe/Berlin"
# → { "points": [
#       {"date":"2026-05-01","rolling_avg_kg":73.4,"sample_count":5},
#       {"date":"2026-05-02","rolling_avg_kg":null,"sample_count":0},   # gap day
#       ...
#     ] }

# Patch / delete (standard CRUD)
curl -X PATCH -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"body_fat_pct":13.8}' \
    http://localhost:8080/weight/<uuid>
curl -X DELETE -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    http://localhost:8080/weight/<uuid>
```

### Recovery metrics

One daily snapshot per calendar date of the recovery signals a wellness device reports — sleep
(`sleep_seconds`, `sleep_score`), `hrv_ms`, `resting_hr`, `stress_avg`, body battery
(`body_battery_charged`/`drained`), `training_readiness`. Identified by `date`; "POST every day
you see" and re-pushing the same date full-replaces it (no surrogate id, no duplicate-day rows).
Every metric is optional — NULL means the device didn't report it. This is the recovery context
for deciding whether today's deficit / training load is tolerable.

```bash
# Upsert a day's recovery snapshot (re-POST same date → 200 update, full-replace)
curl -X POST -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"date":"2026-06-09","sleep_seconds":27000,"sleep_score":82,"hrv_ms":61,"resting_hr":48,"stress_avg":28,"training_readiness":74}' \
    http://localhost:8080/recovery-metrics

# List a window (inclusive YYYY-MM-DD, 92-day cap), get / delete one day
curl -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/recovery-metrics?from=2026-06-01&to=2026-06-30"
curl -H "Authorization: Bearer $MOBILE_API_TOKEN" http://localhost:8080/recovery-metrics/2026-06-09
curl -X DELETE -H "Authorization: Bearer $MOBILE_API_TOKEN" http://localhost:8080/recovery-metrics/2026-06-09
```

### Fitness metrics

Sister capability to recovery — one daily snapshot of `vo2max_running`/`vo2max_cycling`, race
predictions (`race_predictor_{5k,10k,half,full}_seconds`, stored as **seconds** — format
`h:mm:ss` yourself), and `acute_load`/`chronic_load`. The acute:chronic ratio is
`acute_load / chronic_load`, derived at read time (not stored). Same date-keyed upsert + list /
get / delete shape as recovery-metrics.

```bash
curl -X POST -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"date":"2026-06-09","vo2max_running":54,"race_predictor_5k_seconds":1230,"acute_load":420.5,"chronic_load":380}' \
    http://localhost:8080/fitness-metrics
curl -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/fitness-metrics?from=2026-06-01&to=2026-06-30"
```

### Hydration balance

One daily snapshot of the body's water balance — `sweat_loss_ml` (estimated sweat out),
`activity_intake_ml` (fluid taken during activity; a real `0` is meaningful), and `goal_ml` (the
daily hydration goal). Same date-keyed upsert + list / get / delete shape as recovery/fitness.

This is **distinct from `/hydration`**: `/hydration` is the user's per-entry *logged intake* (and
its summary sums those entries); hydration-balance is a device's *daily estimate* (one row/day).
The daily balance reads sweat-out from here and total-in from the hydration summary — the API
stores both primitives; the agent computes the deficit/ratio.

```bash
curl -X POST -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"date":"2026-06-09","sweat_loss_ml":2400,"activity_intake_ml":1800,"goal_ml":3000}' \
    http://localhost:8080/hydration-balance
curl -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/hydration-balance?from=2026-06-01&to=2026-06-30"
```

### Energy availability

Per-day Energy Availability over a window, with Loucks band classification
(`< 30 kcal/kg FFM/day` = low, `30..45` = sub-optimal, `>= 45` = adequate).
Pure composition over `meals` (intake), `workouts.kcal_burned` (exercise burn),
and `body_weight_entries` (composition). No new schema; nothing persisted.

FFM resolution order (highest-trust first):
1. `lean_mass_kg` query param (explicit, wins over everything).
2. `body_fat_pct` query param + window-resolved body weight.
3. Most recent in-window `body_weight_entries.body_fat_pct` + window-resolved body weight.
4. `body_weight × 0.85` fallback (loudly flagged via `composition.composition_estimated: true`).

Days with workouts missing `kcal_burned` are listed in `missing_burn_workout_ids`
and **excluded** from `window.avg_ea` — silently zeroing low-data days would make
them look healthier than they are, the most dangerous failure mode for this metric.

```bash
# Most common shape: relies on stored body weight + body-fat % from the smart scale
curl -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/energy/availability?from=2026-06-01T00:00:00Z&to=2026-06-08T00:00:00Z&tz=Europe/Berlin"

# Explicit lean mass — wins over any stored composition data
curl -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/energy/availability?from=2026-06-01T00:00:00Z&to=2026-06-08T00:00:00Z&lean_mass_kg=62"

# → {
#     "from": "...", "to": "...", "tz": "Europe/Berlin",
#     "days": [
#       { "date": "2026-06-01",
#         "intake_kcal": 2400, "exercise_energy_kcal": 600,
#         "ea": 30.0, "band": "sub_optimal",
#         "missing_burn_workout_ids": [], "complete_data": true },
#       ...
#     ],
#     "window": { "avg_ea": 33.5, "band": "sub_optimal",
#                 "days_with_complete_data": 7, "total_days": 7 },
#     "composition": { "ffm_kg": 60.0, "source": "explicit_lean_mass",
#                      "body_weight_kg": 72.0, "body_weight_source": "rolling_7d_avg" }
#   }

# Documented failure: no body-weight data AND no lean_mass_kg override
# → 400 weight_data_missing
```

### Summaries

```bash
# Daily totals + entries (in a user-supplied tz), with goals-aware adherence
curl -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/summary/daily?date=2026-06-06&tz=Europe/Berlin"

# Daily scoped to a single meal type (omits adherence)
curl -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/summary/daily?date=2026-06-06&meal_type=breakfast"

# Per-day breakdown over an inclusive range (max 92 days)
curl -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/summary/range?from=2026-06-01&to=2026-06-30&tz=Europe/Berlin"

# Per-meal-type breakdown across a range ("what's my average breakfast this week?")
curl -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/summary/range?from=2026-06-01&to=2026-06-07&group_by=meal_type"

# Trailing-window average — closes the "how am I doing this week / this block?"
# question. Window is [anchor_date − (window_days − 1), anchor_date], both
# inclusive, calendar-day buckets in the requested tz.
# IMPORTANT: averages divide by days_with_data (logged days), not total_days.
# Both divisors are exposed so a sparse window is loud.
# Per-day rows carry has_data=true|false distinguishing "no meals logged" from
# "logged a zero-kcal meal." window_days is bounded [2, 30].
curl -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/summary/rolling?anchor_date=2026-06-08&window_days=7&tz=Europe/Berlin"
# → {
#     "anchor_date": "2026-06-08", "window_days": 7, "tz": "Europe/Berlin",
#     "averages":        { "kcal": 2280.5, "protein_g": 128.0, ... },
#     "days_with_data": 6, "total_days": 7,        # sparse — one day was empty
#     "days": [
#       { "date": "2026-06-02", "totals": {...}, "has_data": true  },
#       { "date": "2026-06-03", "totals": {...}, "has_data": true  },
#       { "date": "2026-06-04", "totals": {...}, "has_data": false },
#       ...
#     ],
#     "adherence":   { "protein_g": { "actual": 128.0, "target": {...}, "status": "on" }, ... },
#     "goal_source": "default"
#   }
```

#### Daily context bundle

One read returns everything the agent needs to start a session: adherence,
nutrition totals, hydration ml, today's workouts, fuel entries, the latest
body-weight reading (with `is_carryover` when the entry is from a previous
day), the training phase covering the date, and whether a goal override is
in force. Composition-only over existing primitives — no schema, no writes.

```bash
curl -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/context/daily?date=2026-07-15&tz=Europe/Berlin"
# → {
#     "date": "2026-07-15", "tz": "Europe/Berlin",
#     "adherence":    { "goal_source": "phase_template", "phase_name": "build-block-2",
#                       "adherence": { "kcal": {...}, "protein_g": {...}, ... } },
#     "nutrition":    { "totals": {...}, "entries_count": 3 },
#     "hydration":    { "total_ml": 2100, "entries_count": 5 },
#     "workouts":     [ { "id":"...", "sport":"bike", "duration_min": 75, "kcal_burned": 720 } ],
#     "workout_fuel": [ { "id":"...", "name":"gel", "carbs_g": 25, "workout_id": "..." } ],
#     "weight":       { "weight_kg": 70.5, "body_fat_pct": 14.2, "is_carryover": false },
#     "phase":        { "name":"build-block-2", "type":"build",
#                       "start_date":"...","end_date":"...","default_template_name":"build-default" },
#     "goal_override":{ "present": false, "goals": null }
#   }
```

For deep dives into one slice (per-entry breakdowns, full meal lists, range
queries), use the dedicated endpoints — `/summary/daily`, `/workouts`, etc.
The aggregator deliberately omits per-entry detail to keep the bundle agent-readable.

### Protein distribution (MPS per meal)

One row per logged meal, annotated with `mps_effective: bool` against the
0.3 g/kg body-weight muscle-protein-synthesis threshold. The headline metric
is `mps_effective_meal_count / meal_count` — a daily total of 180 g doesn't
matter if it landed as 20/20/140, since MPS triggers per-meal, not per-day.

Body weight resolution order: explicit `body_weight_kg` query param > rolling
7-day mean of stored entries ending at `date` (inclusive) > most-recent
stored entry strictly before `date`. With no stored data and no override:
`400 weight_data_missing`.

```bash
curl -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/summary/protein-distribution?date=2026-06-09&tz=Europe/Berlin"

# Override the resolved body weight:
curl -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/summary/protein-distribution?date=2026-06-09&tz=Europe/Berlin&body_weight_kg=72.5"
# → {
#     "date": "2026-06-09", "tz": "Europe/Berlin",
#     "body_weight_kg": 72.5, "body_weight_source": "explicit",
#     "mps_threshold_g": 21.8,
#     "total_protein_g": 123.0,
#     "meal_count": 4, "mps_effective_meal_count": 3,
#     "meals": [
#       { "logged_at": "2026-06-09T07:30:00Z", "logged_at_hour": 9,
#         "meal_type": "breakfast", "protein_g": 28.0,
#         "mps_effective": true,  "gap_minutes_since_previous": null },
#       { "logged_at": "2026-06-09T11:00:00Z", "logged_at_hour": 13,
#         "meal_type": "lunch",     "protein_g": 18.0,
#         "mps_effective": false, "gap_minutes_since_previous": 210 },
#       ...
#     ]
#   }
```

`gap_minutes_since_previous` is `null` on the first meal. The MPS-trigger
sweet spot is 3–5 hours between protein doses; gaps under 3h aren't
independent triggers, gaps over 5h close the MPS window. The endpoint
returns the raw number — "this gap is short / long / fine" framing is
agent-side.

### Goals

```bash
# Get current goals (returns {"goals": null} if unset)
curl -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    http://localhost:8080/goals

# Set / replace goals (PUT semantics — absent fields are cleared)
curl -X PUT -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
         "kcal": {"min": 2090, "max": 2310},
         "protein_g": {"min": 150, "max": 190},
         "fiber_g": {"min": 30},
         "sugar_g": {"max": 50},
         "iron_mg": {"min": 14},
         "vitamin_b12_mcg": {"min": 2.4}
       }' \
    http://localhost:8080/goals
```

#### Daily goal overrides

Per-date overrides sit on top of the default singleton. Use them when a single
date (training day, rest day, race day) needs different targets. PUT is
full-replace; absent fields stored as null. The summary endpoints carry a
`goal_source: "override" | "phase_template" | "default" | "none"` field so
callers can see which set produced the day's adherence rows.

```bash
# Training-day override
curl -X PUT -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
         "kcal": {"min": 2280, "max": 2520},
         "protein_g": {"min": 160, "max": 200},
         "carbs_g": {"min": 350, "max": 450}
       }' \
    http://localhost:8080/goals/overrides/2026-06-15

# Read one (404 override_not_found if no row)
curl -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    http://localhost:8080/goals/overrides/2026-06-15

# List a window (max 366 days; dates without an override are omitted)
curl -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/goals/overrides?from=2026-06-01&to=2026-06-30"

# Delete — date falls back to the default goals
curl -X DELETE -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    http://localhost:8080/goals/overrides/2026-06-15
```

#### Training phases and goal templates

A training phase is a named date range tagged with a training `type` (`base`,
`build`, `peak`, `recovery`, `race_week`, `off_season`, `other`) and an
optional `default_template_id` pointing at a reusable goal template. The
effective-goals chain becomes **per-date override > phase template > default
singleton > none** — phases describe intent over a period, per-date overrides
describe deliberate exceptions, the singleton is the baseline. Templates are
named, reusable goal-sets sharing the same `{min?, max?}` Range shape as the
default goals. Editing a template's bounds propagates to every phase pointing
at it on next adherence read (no apply step; intentionally cheap to evolve).

```bash
# Create a reusable template (PUT — name in URL is canonical, no Idempotency-Key)
curl -X PUT -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
         "kcal":     {"min": 2280, "max": 2520},
         "protein_g":{"min": 160, "max": 200},
         "carbs_g":  {"min": 350, "max": 450},
         "notes":    "build-block default daily targets"
       }' \
    http://localhost:8080/goal-templates/build-default

# Create a phase pointing at that template
TPL_ID=$(curl -s -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    http://localhost:8080/goal-templates/build-default | jq -r .template.id)
curl -X POST -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    -H "Content-Type: application/json" \
    -d "{
         \"name\":\"build-block-2\",
         \"type\":\"build\",
         \"start_date\":\"2026-07-01\",
         \"end_date\":\"2026-07-28\",
         \"default_template_id\":\"$TPL_ID\",
         \"notes\":\"weeks 5-8 of 16-week plan\"
       }" \
    http://localhost:8080/phases

# List phases intersecting a window (max 730 days)
curl -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/phases?from=2026-06-01&to=2026-09-30"

# Now /summary/daily on dates inside the phase shows goal_source=phase_template
curl -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/summary/daily?date=2026-07-15" | \
    jq '{goal_source, phase_name, kcal_target: .adherence.kcal.target}'
# → {"goal_source":"phase_template","phase_name":"build-block-2","kcal_target":{"min":2280,"max":2520}}

# Per-date overrides still win — set one for a single workout day
curl -X PUT -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"carbs_g":{"min":700}}' \
    http://localhost:8080/goals/overrides/2026-07-15

# Phase-with-template clearing: PATCH with empty-string sentinel
curl -X PATCH -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"default_template_id":""}' \
    http://localhost:8080/phases/<phase-uuid>

# Delete a template — refused with 409 template_in_use if any phase references it
curl -X DELETE -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    http://localhost:8080/goal-templates/build-default
```

### Race prep

Two endpoints: a stateless carb-load schedule (`GET`) and an apply step
(`POST`) that writes the schedule into per-date goal overrides in one round
trip. The compute path is pure — same inputs always produce the same output
— useful for "what-if" exploration. The apply path is the recommended way
to lock a plan in.

```bash
# Compute only: stateless carb-load schedule. Default protocol: 3 load days
# at 10 g carbs/kg/day + race day at 2 g/kg.
curl -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/race-prep/carb-load?race_date=2026-07-24&body_weight_kg=70"

# Custom protocol: 2-day mini-load for a sprint tri, lighter race-morning meal.
curl -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/race-prep/carb-load?race_date=2026-07-24&body_weight_kg=70&days_before=2&carbs_per_kg_per_day=8&race_day_carbs_per_kg=2.5"

# Compute AND apply: writes the carb_g goal min-bound for each schedule day
# into the per-date goal overrides. Preserves any existing kcal/protein/
# other macros on those dates (merge-only). Atomic: if any per-date write
# fails, the whole apply rolls back.
curl -X POST -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"race_date":"2026-07-24","body_weight_kg":70}' \
    "http://localhost:8080/race-prep/carb-load/apply"
```

Compute response shape (`GET /race-prep/carb-load`):

```json
{
  "race_date": "2026-07-24",
  "body_weight_kg": 70,
  "params": {"days_before": 3, "carbs_per_kg_per_day": 10, "race_day_carbs_per_kg": 2},
  "schedule": [
    {"date": "2026-07-21", "days_before": 3, "target_carbs_g": 700, "rationale": "carb-load day 1 of 3"},
    {"date": "2026-07-22", "days_before": 2, "target_carbs_g": 700, "rationale": "carb-load day 2 of 3"},
    {"date": "2026-07-23", "days_before": 1, "target_carbs_g": 700, "rationale": "carb-load day 3 of 3"},
    {"date": "2026-07-24", "days_before": 0, "target_carbs_g": 140, "rationale": "race morning, pre-race meal ~3-4h before start"}
  ]
}
```

Apply response shape (`POST /race-prep/carb-load/apply`): same as compute,
plus an `applied` array reporting per-date outcome. `created: false` means
the apply merged into a pre-existing override (e.g. a training-day kcal
template stayed intact while only the carbs bound was updated).

```json
{
  "race_date": "2026-07-24",
  "body_weight_kg": 70,
  "params": {"days_before": 3, "carbs_per_kg_per_day": 10, "race_day_carbs_per_kg": 2},
  "schedule": [ ... same as above ... ],
  "applied": [
    {"date": "2026-07-21", "carbs_g_min": 700, "created": true},
    {"date": "2026-07-22", "carbs_g_min": 700, "created": false},
    {"date": "2026-07-23", "carbs_g_min": 700, "created": true},
    {"date": "2026-07-24", "carbs_g_min": 140, "created": true}
  ]
}
```

#### Per-session fueling recommendation

`plan_carb_load` covers the 1–4 days before the race. `daily_summary` +
phases cover today's macro targets. Neither answers the *forward-looking*
per-session question: "what should I eat before, during, and after
tomorrow's 90-min Z2 ride?" That's what
`GET /race-prep/recommend-workout-fuel` is for. Stateless; literature-grounded
(Jeukendrup / Burke / ISSN consensus); reuses the same 0.3 g/kg MPS threshold
the `protein_distribution` endpoint uses, so a user who hits the post-workout
protein number automatically hits the per-meal MPS bar.

Two input modes, exactly one required:

1. **Workout-mode** — `workout_id=<uuid>` pulls sport, duration (from
   `ended_at - started_at`), and intensity zone (from `tss` via the Coggan IF
   mapping; defaults to Z2 with a disclosure note when TSS is absent).
2. **Explicit-mode** — `sport=bike&duration_min=90&intensity_zone=3` for
   planned-tomorrow sessions that don't have a workout row yet.

Body weight resolution: explicit `body_weight_kg` query param wins; otherwise
the rolling-7d-mean of stored entries; otherwise the most-recent stored
entry before today; otherwise `400 weight_data_missing`.

```bash
# Workout-mode (pulls sport/duration/intensity from the workout row).
curl -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/race-prep/recommend-workout-fuel?workout_id=<uuid>"

# Explicit-mode for a planned-tomorrow session.
curl -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/race-prep/recommend-workout-fuel?sport=bike&duration_min=90&intensity_zone=3&body_weight_kg=72"
```

Response shape:

```json
{
  "inputs": {
    "sport": "bike", "duration_min": 90, "intensity_zone": 3,
    "body_weight_kg": 72.0, "body_weight_source": "rolling_7d_avg"
  },
  "pre_workout":  { "window_minutes_before": [60, 120], "carbs_g": 108, "carbs_g_per_kg": 1.5, "rationale": "..." },
  "intra_workout": { "applicable": true, "carbs_g_per_hour": 60, "carbs_g_total": 90,
                     "fluid_ml_per_hour": 700, "sodium_mg_per_hour": 600, "rationale": "..." },
  "post_workout": { "window_minutes_after": [0, 60], "carbs_g": 72, "protein_g": 21.6,
                    "rationale": "Recovery window: 1 g/kg CHO inside 60 min … 0.3 g/kg protein hits the per-meal MPS threshold." },
  "notes": [
    "Intra-session sodium target is a midpoint; the validated range is 300-800 mg/hr ...",
    "CHO/hr buckets: < 45 min none required; 45-90 min 30 g/hr; ...",
    "For races > 90 min, also run `plan_carb_load` for the 24-72h pre-loading schedule."
  ]
}
```

Notable behaviours: `intra_workout.applicable: false` for sessions under
45 min, all strength sessions, and swims ≤ 120 min (no realistic in-session
intake during a swim set). `sport=run` caps `carbs_g_per_hour` at 60 even on
long runs where bike/row would suggest 90 — running's impact loading limits
GI tolerance. Run cap behaviour surfaces in both the rationale string and an
extra entry in `notes[]` so agents that summarise notes separately see it too.

### Garmin token store

`PUT` / `GET` / `DELETE /garmin/token` hold a single opaque auth-token blob for
the (separate) garmin-bridge service — stored encrypted at rest (AES-256-GCM)
and returned byte-identical. These endpoints are **garmin-identity-only**: only
a request bearing `GARMIN_API_TOKEN` may use them (the mobile and agent tokens
get `403 forbidden`). When `GARMIN_API_TOKEN` is unset the integration is off
and the endpoints return `503 garmin_disabled`. The backend never parses or
interprets the blob.

## API documentation (Swagger / OpenAPI)

Annotated handlers generate an OpenAPI 2.0 spec, committed to `docs/`. When
the server runs in debug mode (the default for `go run`), the interactive
Swagger UI is mounted at `http://localhost:8080/swagger/index.html`. In
release mode it is gated behind `SWAGGER_ENABLED=true`.

```bash
task swag    # regenerate docs/docs.go, docs/swagger.json, docs/swagger.yaml
```

The generated `docs/` directory is committed so plain `go build` does not
require `swag` to be installed.

## MCP server (LLM agent integration)

`nutrition-api mcp` is a stdio MCP server that fronts the REST API for an LLM
agent. It is a thin wrapper: each MCP tool issues exactly one HTTP call to the
REST API, using `AGENT_API_TOKEN` for auth. The REST API must be running before
the agent calls any tool.

```bash
# Build and install the single binary; the MCP server is `nutrition-api mcp`.
task install             # builds bin/nutrition-api and copies to ~/.local/bin/nutrition-api
```

### Register with Claude Desktop

In `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "nutrition": {
      "command": "/Users/<you>/.local/bin/nutrition-api",
      "args": ["mcp"],
      "env": {
        "NUTRITION_API_URL": "http://localhost:8080",
        "AGENT_API_TOKEN": "<your AGENT_API_TOKEN>"
      }
    }
  }
}
```

### Register with Claude Code

In `~/.claude/mcp.json` (or via `claude mcp add`):

```json
{
  "mcpServers": {
    "nutrition": {
      "command": "/Users/<you>/.local/bin/nutrition-api",
      "args": ["mcp"],
      "env": {
        "NUTRITION_API_URL": "http://localhost:8080",
        "AGENT_API_TOKEN": "<your AGENT_API_TOKEN>"
      }
    }
  }
}
```

### Tools exposed

| Tool                          | REST endpoint                          | Purpose                                                       |
|-------------------------------|----------------------------------------|---------------------------------------------------------------|
| `lookup_product_by_barcode`   | `POST /products/lookup/{barcode}`      | OFF lookup with local cache; `refresh=true` re-fetches.       |
| `search_products`             | `GET /products/search?q=…`             | Recall by name/brand (results include `off`, `manual`, `recipe`). |
| `list_products`               | `GET /products?source=…&limit=…&offset=…` | Enumerate the cache, recency-first. Pair with `delete_product` to clean up. |
| `delete_product`              | `DELETE /products/{id}`                | Permanently delete a product. Historical meals keep their snapshots; in-use-as-component returns 409. |
| `create_recipe`               | `POST /products/recipes`               | Define a composite multi-ingredient meal as a reusable recipe. |
| `recompute_recipe`            | `POST /products/recipes/{id}/recompute`| Refresh recipe nutriments after a component changed.          |
| `log_meal`                    | `POST /meals`                          | Log from a known `product_id` + grams (recipes work here).    |
| `log_meal_freeform`           | `POST /meals/freeform`                 | Log from agent-supplied name + nutriment estimate (macros + micros). |
| `patch_meal`                  | `PATCH /meals/{id}`                    | Edit portion / time / meal_type / note.                       |
| `delete_meal`                 | `DELETE /meals/{id}`                   | Remove a meal entry.                                          |
| `daily_summary`               | `GET /summary/daily?date=…&tz=…&meal_type=…` | Per-day totals + entries; goal-based adherence when set.  |
| `range_summary`               | `GET /summary/range?from=…&to=…&group_by=…`  | Per-day breakdown across an inclusive range (max 92 days). |
| `rolling_summary`             | `GET /summary/rolling?anchor_date=…&window_days=…&tz=…` | Trailing-window nutrition average + per-day rows. Averages divide by `days_with_data`, not `total_days` (both exposed). Window bounded `[2, 30]`. |
| `protein_distribution`        | `GET /summary/protein-distribution?date=…&tz=…&body_weight_kg=…` | Per-meal protein with `mps_effective` against the 0.3 g/kg MPS threshold. Headline metric: `mps_effective_meal_count / meal_count`. Body weight resolved from stored entries unless overridden. |
| `get_goals`                   | `GET /goals`                           | Read current goals.                                           |
| `set_goals`                   | `PUT /goals`                           | Set/replace daily macro and micro targets.                    |
| `set_daily_goal_override`     | `PUT /goals/overrides/{date}`          | Override default goals for one date (training / rest / race day). |
| `get_daily_goal_override`     | `GET /goals/overrides/{date}`          | Read the override for one date (404 if none).                 |
| `delete_daily_goal_override`  | `DELETE /goals/overrides/{date}`       | Remove an override; date falls back to default goals.         |
| `list_daily_goal_overrides`   | `GET /goals/overrides?from=…&to=…`     | List overrides in a window (max 366 days).                    |
| `log_hydration`               | `POST /hydration`                      | Record a volume of fluid drunk at a moment in time.           |
| `list_hydration`              | `GET /hydration?from=…&to=…`           | List hydration entries in a half-open window (92-day cap).    |
| `patch_hydration`             | `PATCH /hydration/{id}`                | Edit volume / time / note.                                    |
| `delete_hydration`            | `DELETE /hydration/{id}`               | Remove a hydration entry.                                     |
| `daily_hydration_summary`     | `GET /summary/hydration/daily?date=…&tz=…` | Volume-only daily summary, separate from `daily_summary`. |
| `plan_carb_load`              | `GET /race-prep/carb-load?…` or `POST /race-prep/carb-load/apply` | Stateless carb-load schedule for a race. Pass `apply: true` to ALSO write the carb_g goal min-bound for each schedule day (merging into existing overrides — non-carb fields preserved). |
| `recommend_workout_fuel`      | `GET /race-prep/recommend-workout-fuel?workout_id=…` OR `?sport=…&duration_min=…&intensity_zone=…` | Pre/intra/post fueling recommendation for ONE session. Workout-mode pulls from a workouts row; explicit-mode for planned-tomorrow sessions. Reuses the 0.3 g/kg MPS threshold from `protein_distribution` for the post-protein number. |
| `create_race`                 | `POST /races`                          | Create a persistent race with ordered legs (`{ordinal, discipline, expected_duration_min?, …}`). The durable structure the per-leg fueling plan is computed over. |
| `list_races`                  | `GET /races`                           | List stored races with their legs, ordered by date. |
| `get_race`                    | `GET /races/{id}`                      | Fetch one race with its legs. |
| `update_race`                 | `PATCH /races/{id}`                     | Update race fields; a `legs` array replaces all legs wholesale (omit to leave unchanged). |
| `delete_race`                 | `DELETE /races/{id}`                    | Delete a race; its legs cascade. |
| `plan_race_fueling`           | `GET /races/{id}/fueling-plan?body_weight_kg=…&sweat_rate_ml_per_hr=…` | Deterministic per-leg in-event fueling baseline: carbs/sodium/fluid per hour + total per leg. Carbs band by total duration × discipline; fluid/sodium from sweat rate (flagged default otherwise). A baseline to adjust for weather/gut/course. |
| `log_workout`                 | `POST /workouts`                       | Record a workout (manual entries, sweat-rate windows). Garmin sessions come via `garmin.py`, not this tool. |
| `list_workouts`               | `GET /workouts?from=…&to=…`            | List workouts in a 92-day window.                             |
| `get_workout`                 | `GET /workouts/{id}`                   | Fetch a workout by id.                                        |
| `patch_workout`               | `PATCH /workouts/{id}`                 | Edit `name`/`notes`/`kcal_burned`/`avg_hr`/`tss`. Sport, window, source, external_id are immutable. |
| `delete_workout`              | `DELETE /workouts/{id}`                | Remove a workout.                                             |
| `log_weight`                  | `POST /weight`                         | Record a body-weight measurement, optionally with body-fat %. |
| `list_weights`                | `GET /weight?from=…&to=…`              | List body-weight entries in a 92-day window.                  |
| `patch_weight`                | `PATCH /weight/{id}`                   | Edit weight / body-fat % / logged_at / note.                  |
| `delete_weight`               | `DELETE /weight/{id}`                  | Remove a body-weight entry.                                   |
| `weight_trend`                | `GET /weight/trend?from=…&to=…&window_days=…` | Rolling-average weight trend; each point carries `sample_count`. |
| `workout_fueling_summary`     | `GET /workouts/{id}/fueling?pre_window_min=…&post_window_min=…` | Pre/intra/post intake totals for a workout. Three sub-objects per window: `nutrition` (meals), `hydration` (hydration entries), `workout_fuel` (workout-fuel entries). |
| `log_workout_fuel`            | `POST /workout-fuel`                   | Record an in-session fueling event — gel, electrolyte drink, salt tab, caffeine. Plain water belongs in `log_hydration`. |
| `list_workout_fuel`           | `GET /workout-fuel?from=…&to=…`        | List workout-fuel entries in a half-open window (92-day cap).  |
| `patch_workout_fuel`          | `PATCH /workout-fuel/{id}`             | Edit name / quantitative fields / note / workout link.         |
| `delete_workout_fuel`         | `DELETE /workout-fuel/{id}`            | Remove a workout-fuel entry.                                   |
| `weekly_energy_summary`       | `GET /energy/availability?from=…&to=…&tz=…&lean_mass_kg=…&body_fat_pct=…` | Per-day Energy Availability + window aggregate with Loucks bands. Days missing `kcal_burned` are flagged and excluded from `window.avg_ea`. |
| `create_phase`                | `POST /phases`                         | Create a training phase (named date range tagged `base`/`build`/`peak`/`recovery`/`race_week`/`off_season`/`other`). Optional `default_template_id` makes the phase drive adherence. |
| `list_phases`                 | `GET /phases?from=…&to=…`              | List phases intersecting a window (max 730 days). |
| `get_phase`                   | `GET /phases/{id}`                     | Fetch one phase, including resolved `default_template_name`. |
| `update_phase`                | `PATCH /phases/{id}`                   | Partial update. Tri-state on `default_template_id`: empty string clears, UUID sets, missing leaves unchanged. |
| `delete_phase`                | `DELETE /phases/{id}`                  | Delete a phase. Dates that were inside fall through to override → singleton default. |
| `set_goal_template`           | `PUT /goal-templates/{name}`           | Create or replace a named goal template. Full-replace; editing propagates to every phase pointing at it. |
| `list_goal_templates`         | `GET /goal-templates`                  | List every template ordered by name. |
| `get_goal_template`           | `GET /goal-templates/{name}`           | Fetch one template by name. |
| `delete_goal_template`        | `DELETE /goal-templates/{name}`        | Refused with 409 `template_in_use` (referencing_phases echoed) if any phase points at it. |
| `daily_context`               | `GET /context/daily?date=…&tz=…`       | One call returns adherence + totals + hydration + today's workouts + fuel + weight + phase + goal-override presence. Recommended first call of a session. |

Write tools accept an optional `idempotency_key`. When omitted, the wrapper
derives a stable key from the tool arguments so the agent's automatic retries
do not double-log. To intentionally log the same item twice, pass a distinct
`idempotency_key`.

### MCP environment variables

| Variable                       | Default                  | Purpose                                                            |
|--------------------------------|--------------------------|--------------------------------------------------------------------|
| `NUTRITION_API_URL`            | `http://localhost:8080`  | Where the REST API lives, as seen by the MCP process.              |
| `AGENT_API_TOKEN`              | _required_               | Bearer token for the REST API.                                     |
| `MCP_REQUEST_TIMEOUT_SECONDS`  | `10`                     | Per-tool HTTP timeout.                                             |

### Local sanity check

```bash
# In one terminal: the REST API
task run    # or: task dev (sets up env automatically)

# In another: launch the MCP server (waits for JSON-RPC on stdin)
go run ./cmd/nutrition-api mcp
```

The MCP integration test (`go test -tags=integration ./internal/mcpserver/`)
builds the binary, spawns `nutrition-api mcp`, exchanges `initialize` +
`tools/list` over stdio, and asserts the expected tools are announced.

## Deploying

The repo ships a `Dockerfile`, a Helm chart at
[`deploy/helm/nutrition-api/`](deploy/helm/nutrition-api/README.md),
and three GitHub Actions workflows under
[`.github/workflows/`](.github/workflows/) — see the
[chart README](deploy/helm/nutrition-api/README.md) for the install /
upgrade walkthrough.

Container images publish to:

- `ghcr.io/vinzenzs/nutrition-api:main` — rolling tip-of-main
- `ghcr.io/vinzenzs/nutrition-api:sha-<short>` — per-commit pin
- `ghcr.io/vinzenzs/nutrition-api:vX.Y.Z` + `:latest` — tagged releases

Packaged Helm charts publish on `v*` tags to
`oci://ghcr.io/vinzenzs/charts/nutrition-api`. The chart assumes an
externally provisioned Postgres reachable via `DATABASE_URL` — there is
no in-chart database.

## Development

```bash
task dev                      # one-command local: Postgres up + env + serve
task run                      # start the API (uses current env)
task test                     # full test suite (boots Postgres via testcontainers)
task vet                      # go vet
task build                    # compile bin/nutrition-api
task install                  # build + copy to ~/.local/bin/nutrition-api
task db:up                    # start the local Postgres container (idempotent)
task db:down                  # stop and remove the local Postgres container
task migrate                  # apply migrations using the binary
task migrate:up               # apply migrations via golang-migrate CLI
task migrate:new NAME=add_widget   # scaffold a new migration pair
task swag                     # regenerate docs/ from handler annotations
task --list                   # show every available target
```

Integration tests boot a Postgres container via
[testcontainers-go](https://golang.testcontainers.org/). Docker or Podman must
be running. On Podman, the test helper disables the Ryuk reaper automatically
(`t.Cleanup` handles teardown).

## Project layout

```
cmd/nutrition-api/        single binary: serve | mcp | migrate | version
docs/                     generated OpenAPI (committed; regenerate with `task swag`)
internal/auth/            two-token bearer middleware
internal/config/          Viper-backed env + flag loader shared by all subcommands
internal/httpserver/      Gin router, request logging, Swagger gate, lifecycle
internal/mcpserver/       MCP server fronting the REST API for the LLM agent
internal/idempotency/     middleware + repo + cleanup ticker
internal/off/             Open Food Facts client + parser
internal/products/        repo, service, handlers
internal/meals/           repo, service, handlers
internal/hydration/       volume-only intake log (CRUD + daily summary)
internal/workouts/        training sessions (CRUD + bulk upsert) — writers translate sources here
internal/bodyweight/      body-weight log + rolling-average trend (kg, optional body-fat %)
internal/workoutfueling/  pre/intra/post fueling aggregation per workout (meals + hydration + workout-fuel)
internal/workoutfuel/     in-session fueling log (gels, electrolyte drinks, caffeine) — sibling to hydration
internal/energy/          Energy Availability over a window (kcal / kg FFM / day, Loucks bands)
internal/summary/         daily + range computation
internal/store/           shared pgx pool + embedded migrations
internal/e2e/             single happy-path end-to-end test
testdata/off/             recorded OFF JSON fixtures
openspec/                 OpenSpec proposals, designs, and specs
```

## Design notes

See [`openspec/changes/add-meal-logging-mvp/`](openspec/changes/add-meal-logging-mvp/)
for the proposal, design document, and per-capability specifications. The data
model keeps `products` (reusable definitions) and `meal_entries` (events in
time) distinct, with freeform meal entries also storing a nutriment snapshot
inline so historical summaries do not silently change when a linked product is
later edited.

## Non-goals for v1

- HTTP / SSE transport for the MCP server (stdio only — covers Claude Desktop and Claude Code)
- MCP resources and prompts (tools only)
- Trends / coaching endpoints
- OFF text search (the freeform endpoint covers the "no barcode" case)
- Multi-user, OAuth, web UI
