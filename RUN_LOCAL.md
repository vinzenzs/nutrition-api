# Running nutrition-api locally

The fast path is two commands. The detailed path is below for when you want to
understand or override what's happening.

---

## The fast path

```bash
brew install go-task/tap/go-task    # macOS; or see https://taskfile.dev/installation/
task dev
```

That's it. `task dev` brings up a local Postgres container, writes a
`.env.local` with dev defaults if one doesn't already exist, and starts the
REST API on `http://localhost:8080`. Migrations run automatically as part of
startup.

When the banner prints, the API is ready. In another terminal:

```bash
curl http://localhost:8080/healthz
# → {"status":"ok"}
```

The Swagger UI is at <http://localhost:8080/swagger/index.html>.

To stop everything: Ctrl-C the `task dev` terminal. The Postgres container
keeps running (use it for the next iteration). To shut Postgres down:

```bash
task db:down       # stops + removes the container; KEEPS the data volume
```

The Postgres data lives in a named Docker volume (`nutrition-pg-data`), so
your products, meals, goals and weight entries survive `task db:down` and
re-appear on the next `task db:up`.

To start completely fresh (delete container, **delete the data volume**,
regenerate `.env.local`):

```bash
task dev:reset
```

To wipe just the data without restarting the dev loop:

```bash
task db:wipe       # destructive — drops the volume
```

---

## Prerequisites

| Tool                                  | Why                                                | Check                            |
|---------------------------------------|----------------------------------------------------|----------------------------------|
| Go 1.24+                              | Build the binaries                                 | `go version`                     |
| Docker (with compose v2) **or** Podman ≥ 4.x (with `podman compose`) | Run Postgres locally via `compose.yml`             | `docker compose version` or `podman compose version` |
| Task (taskfile.dev)                   | Drive the dev workflow                             | `task --version`                 |
| `curl`                                | Smoke-test the API                                 | `curl --version`                 |
| `qrencode` (optional)                 | Print the pairing QR for the Flutter companion app via `task dev:pair` | `qrencode --version` |

Task install:

- macOS: `brew install go-task/tap/go-task`
- Linux: `sh -c "$(curl -fsSL https://taskfile.dev/install.sh)" -- -d -b ~/.local/bin`
- Other: see <https://taskfile.dev/installation/>

`task db:up` prefers `docker compose` when available and falls back to
`podman compose`. Docker Desktop and modern `docker.io` ship the compose v2
plugin out of the box; Podman ≥ 4.x has native `podman compose`.

`qrencode` is only needed if you plan to pair the Flutter companion app
(`apps/companion/`) to the backend — install with `brew install qrencode`
on macOS or `apt-get install qrencode` on Linux. Without it the backend
runs fine; you just can't generate the pairing QR.

---

## What `task dev` does, exactly

If you want to know what's happening under the hood:

1. **`task db:up`** runs first as a dependency. It drives `docker compose up
   -d postgres` against [`compose.yml`](compose.yml), which declares:
   - image `postgres:17-alpine`, container name `nutrition-pg`, port `5432`
   - a named volume `nutrition-pg-data` mounted at `/var/lib/postgresql/data`
     (this is what makes your data survive container removal)
   - a healthcheck running `pg_isready -U nutrition -d nutrition` every 2 s.

   `task db:up` then loops on `pg_isready` inside the container until it
   succeeds (up to 12 s) so the API doesn't start before Postgres is reachable.
   The task is idempotent — re-running it when the container is already
   healthy is a no-op.

   First-time migration note: if you have an old `nutrition-pg` container
   from before this repo used compose (created via `docker run` directly),
   `task db:up` detects it has no compose labels and removes it once so
   compose can take over the name. The data volume is independent of the
   container, so no data is lost.

2. **`.env.local` is written if missing.** The file is git-ignored. Contents:

   ```bash
   DATABASE_URL=postgres://nutrition:nutrition@localhost:5432/nutrition?sslmode=disable
   MOBILE_API_TOKEN=dev-mobile-token-0000000000aaaa
   AGENT_API_TOKEN=dev-agent-token-0000000000bbbbb
   DEFAULT_USER_TZ=Europe/Berlin
   HTTP_ADDR=:8080
   MIGRATE_ON_START=true
   NUTRITION_API_URL=http://localhost:8080
   MCP_REQUEST_TIMEOUT_SECONDS=10
   SWAGGER_ENABLED=true
   # Uncomment to enable POST /meals/from_photo (Claude Vision plate analysis).
   # Without this, the endpoint returns 503 vision_unavailable.
   # ANTHROPIC_API_KEY=sk-ant-...
   ```

   These dev tokens are not secret — the auth middleware exists so the API
   doesn't accidentally trust unauthenticated clients, not to defend against
   anyone who can already read your filesystem. For production, generate real
   tokens with `openssl rand -hex 32`. `ANTHROPIC_API_KEY` is genuinely
   secret — keep it out of git.

3. **A banner prints** with the URL, both tokens, and an example curl.

4. **`go run ./cmd/nutrition-api serve`** starts with `.env.local` loaded.
   The binary applies pending migrations automatically (the
   `MIGRATE_ON_START=true` default), then begins listening.

---

## Other useful tasks

```bash
task --list           # show all available tasks
task build            # compile bin/nutrition-api
task install          # build + copy to ~/.local/bin/nutrition-api
task test             # full test suite (boots Postgres via testcontainers)
task vet              # go vet ./...
task swag             # regenerate docs/ from handler annotations
task migrate          # apply pending migrations (uses the built binary)
task migrate:up       # apply pending migrations via golang-migrate CLI
task migrate:down     # roll back one migration
task migrate:new NAME=add_widget   # scaffold a new migration pair
task mcp:install      # alias for `install` — same binary serves REST and MCP
task db:up            # start the Postgres compose service (idempotent)
task db:down          # stop + remove the container; keep the data volume
task db:wipe          # stop + remove the container AND delete the data volume
```

### Importing recipes from Cookidoo

A Chrome extension at [`extensions/cookidoo/`](extensions/cookidoo/) imports
Cookidoo recipes into the cache. With `task dev` running, load it once via
`chrome://extensions/` (Developer mode → Load unpacked → pick the
`extensions/cookidoo/` directory), set the options (API URL + your
`MOBILE_API_TOKEN`), then click the toolbar button on any Cookidoo recipe
page to preview and save. See
[`extensions/cookidoo/README.md`](extensions/cookidoo/README.md) for the full
walkthrough and limits (Chrome only, JSON-LD dependent, no auto-sync).

---

## Trying the API end-to-end

```bash
# Source the dev env into your shell
set -a; . ./.env.local; set +a

# Lookup a barcode (Nutella). First call hits Open Food Facts and caches.
curl -s -X POST \
    -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    http://localhost:8080/products/lookup/3017624010701 | jq .

# Log a meal. Copy the id from the response above.
PRODUCT_ID="<paste id here>"
curl -s -X POST \
    -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    -H "Content-Type: application/json" \
    -H "Idempotency-Key: $(uuidgen)" \
    -d "{\"product_id\":\"$PRODUCT_ID\",\"quantity_g\":15,\"logged_at\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\",\"meal_type\":\"breakfast\"}" \
    http://localhost:8080/meals | jq .

# Daily summary for today
curl -s -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/summary/daily?date=$(date +%Y-%m-%d)&tz=Europe/Berlin" | jq .

# Trailing 7-day rolling average. Averages divide by days_with_data (logged days),
# not total_days — both are in the response so sparse weeks are loud.
curl -s -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/summary/rolling?anchor_date=$(date +%Y-%m-%d)&window_days=7&tz=Europe/Berlin" \
    | jq '{anchor_date, window_days, days_with_data, total_days, averages_kcal: .averages.kcal, goal_source}'

# Per-meal protein distribution with MPS-threshold flags (0.3 g/kg per meal).
# Body weight is resolved from stored entries (rolling 7d avg) — pass
# body_weight_kg=… to override.
curl -s -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/summary/protein-distribution?date=$(date +%Y-%m-%d)&tz=Europe/Berlin" \
    | jq '{mps_threshold_g, total_protein_g,
           score: "\(.mps_effective_meal_count)/\(.meal_count) meals over threshold",
           body_weight_source}'

# Per-session fueling recommendation for a planned 90-min Z3 bike. Body
# weight passed explicitly so no stored weight is needed first.
curl -s -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/race-prep/recommend-workout-fuel?sport=bike&duration_min=90&intensity_zone=3&body_weight_kg=72" \
    | jq '{pre: .pre_workout.carbs_g, intra_g_per_hour: .intra_workout.carbs_g_per_hour,
           post_carbs: .post_workout.carbs_g, post_protein: .post_workout.protein_g}'

# Log a glass of water and check today's hydration total.
curl -s -X POST -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    -H "Content-Type: application/json" \
    -H "Idempotency-Key: $(uuidgen)" \
    -d "{\"quantity_ml\":500,\"logged_at\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\",\"note\":\"water\"}" \
    http://localhost:8080/hydration | jq .

curl -s -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/summary/hydration/daily?date=$(date +%Y-%m-%d)&tz=Europe/Berlin" | jq .

# Log a body weight and inspect the 7-day rolling trend. Each trend point carries
# sample_count — a value from sample_count: 1 is just that one sample, not a trend.
curl -s -X POST -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    -H "Content-Type: application/json" \
    -H "Idempotency-Key: $(uuidgen)" \
    -d "{\"weight_kg\":72.5,\"body_fat_pct\":14.2,\"logged_at\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\",\"note\":\"morning, fasted\"}" \
    http://localhost:8080/weight | jq .

FROM=$(date -u -v-30d +%Y-%m-%d 2>/dev/null || date -u -d '-30 days' +%Y-%m-%d)
curl -s -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/weight/trend?from=$FROM&to=$(date +%Y-%m-%d)&window_days=7&tz=Europe/Berlin" \
    | jq '.points[-7:]'   # last week of the trend

# Workout fueling round-trip (per add-meal-workout-link, extended by add-workout-fuel):
#   1. log a workout
#   2. log a meal during the intra window, with workout_id set
#   3. log a hydration sip during intra (no workout_id — time-window match still picks it up)
#   4. log a workout-fuel entry (gel) during intra, with workout_id set
#   5. fetch /workouts/{id}/fueling — three sub-objects per window
#   6. fetch /summary/hydration/daily — confirm the gel's ml does NOT bleed in
WID=$(curl -s -X POST -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    -H "Content-Type: application/json" \
    -d "{\"source\":\"manual\",\"sport\":\"bike\",\"started_at\":\"$(date -u -v-3H +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -d '-3 hours' +%Y-%m-%dT%H:%M:%SZ)\",\"ended_at\":\"$(date -u -v-90M +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -d '-90 minutes' +%Y-%m-%dT%H:%M:%SZ)\"}" \
    http://localhost:8080/workouts | jq -r .id)

# Use the banana product from earlier as the intra-workout snack.
curl -s -X POST -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    -H "Content-Type: application/json" \
    -H "Idempotency-Key: $(uuidgen)" \
    -d "{\"product_id\":\"$PRODUCT_ID\",\"quantity_g\":100,\"logged_at\":\"$(date -u -v-2H +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -d '-2 hours' +%Y-%m-%dT%H:%M:%SZ)\",\"workout_id\":\"$WID\"}" \
    http://localhost:8080/meals | jq .

# An UNTAGGED sip in the time-window — should still appear in intra_window.hydration.
curl -s -X POST -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    -H "Content-Type: application/json" \
    -H "Idempotency-Key: $(uuidgen)" \
    -d "{\"quantity_ml\":250,\"logged_at\":\"$(date -u -v-2H +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -d '-2 hours' +%Y-%m-%dT%H:%M:%SZ)\"}" \
    http://localhost:8080/hydration | jq .

# Workout-fuel entry: a Maurten gel with sodium + carbs + caffeine, tagged.
curl -s -X POST -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    -H "Content-Type: application/json" \
    -H "Idempotency-Key: $(uuidgen)" \
    -d "{\"name\":\"Maurten Gel 100\",\"logged_at\":\"$(date -u -v-2H +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -d '-2 hours' +%Y-%m-%dT%H:%M:%SZ)\",\"quantity_ml\":40,\"carbs_g\":25,\"sodium_mg\":120,\"caffeine_mg\":100,\"workout_id\":\"$WID\"}" \
    http://localhost:8080/workout-fuel | jq .

# Fetch fueling. All three sub-objects populate; intra_window.workout_fuel.entry_count = 1.
curl -s -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/workouts/$WID/fueling" | jq .

# Daily hydration: only the 250ml plain-water sip — the gel's 40ml does NOT count here
# (the unit-isolation rule: workout_fuel owns its own totals, hydration owns its own).
curl -s -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/summary/hydration/daily?date=$(date +%Y-%m-%d)&tz=Europe/Berlin" | jq .

# Energy Availability over a week — closes T1 #4. Pure composition over
# meals + workouts + body weight; no new schema.
#   1. Window from a week ago through today.
#   2. Default path: relies on the stored body-weight entries (with body-fat % if logged).
#   3. Override path: pass lean_mass_kg explicitly to force the highest-trust composition source.
EA_FROM=$(date -u -v-7d +%Y-%m-%dT00:00:00Z 2>/dev/null || date -u -d '-7 days' +%Y-%m-%dT00:00:00Z)
EA_TO=$(date -u +%Y-%m-%dT00:00:00Z)

# Stored-composition path (composition.source should be "stored_body_fat" or "estimated_85pct"
# depending on whether body-fat % is in the weight entries).
curl -s -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/energy/availability?from=$EA_FROM&to=$EA_TO&tz=Europe/Berlin" | jq .

# Explicit-lean-mass override (composition.source = "explicit_lean_mass"; body weight echoed for context).
curl -s -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/energy/availability?from=$EA_FROM&to=$EA_TO&tz=Europe/Berlin&lean_mass_kg=62" | jq .
```

### Recipe + goals walkthrough

```bash
# 1. Create two manual products. Capture their ids.
SKYR=$(curl -s -X POST -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"name":"Skyr","nutriments_per_100g":{"kcal":60,"protein_g":11,"calcium_mg":120}}' \
    http://localhost:8080/products | jq -r .id)

OATS=$(curl -s -X POST -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"name":"Oats","nutriments_per_100g":{"kcal":380,"protein_g":13,"fiber_g":10,"iron_mg":4.7}}' \
    http://localhost:8080/products | jq -r .id)

# 2. Compose them into a recipe.
RECIPE=$(curl -s -X POST -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    -H "Content-Type: application/json" \
    -d "{\"name\":\"Morning skyr bowl\",\"serving_size_g\":240,\"components\":[
         {\"product_id\":\"$SKYR\",\"quantity_g\":200},
         {\"product_id\":\"$OATS\",\"quantity_g\":40}
       ]}" \
    http://localhost:8080/products/recipes | jq -r .id)

# 3. Set daily goals.
curl -s -X PUT -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
         "kcal": {"min": 2090, "max": 2310},
         "protein_g": {"min": 150, "max": 190},
         "fiber_g": {"min": 30},
         "iron_mg": {"min": 14}
       }' \
    http://localhost:8080/goals | jq .

# 4. Log the recipe as a single meal (one log call covers the whole bowl).
curl -s -X POST -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    -H "Content-Type: application/json" \
    -d "{\"product_id\":\"$RECIPE\",\"quantity_g\":240,\"logged_at\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\",\"meal_type\":\"breakfast\"}" \
    http://localhost:8080/meals | jq .

# 5. Check today's summary — totals include the recipe's contribution, and
#    the response now includes an `adherence` block with on/under/over per goal.
curl -s -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/summary/daily?date=$(date +%Y-%m-%d)&tz=Europe/Berlin" | jq .

# 6. Inspect the meal entry with the component breakdown:
MEAL_ID=$(curl -s -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/meals?from=$(date -u +%Y-%m-%dT00:00:00Z)&to=$(date -u -v+1d +%Y-%m-%dT00:00:00Z 2>/dev/null || date -u -d 'tomorrow' +%Y-%m-%dT00:00:00Z)" \
    | jq -r '.meals[-1].id')
curl -s -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/meals/$MEAL_ID?expand=components" | jq .

# 7. Training-day override: today's targets are higher than the default.
TODAY=$(date +%Y-%m-%d)
curl -s -X PUT -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"kcal":{"min":2280,"max":2520},"protein_g":{"min":160,"max":200}}' \
    "http://localhost:8080/goals/overrides/$TODAY" | jq .

# 8. Same daily summary call — note `goal_source: "override"` and the new
#    adherence bounds (override values, not defaults).
curl -s -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/summary/daily?date=$TODAY&tz=Europe/Berlin" | jq '{goal_source, kcal_target: .adherence.kcal.target}'

# 9. Delete the override; the next summary call swings back to "default".
curl -s -X DELETE -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/goals/overrides/$TODAY"
curl -s -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/summary/daily?date=$TODAY&tz=Europe/Berlin" | jq '{goal_source, kcal_target: .adherence.kcal.target}'
```

### Plan a race week

The `race-prep` capability has two endpoints: a stateless `GET` returning the
carb-load schedule, and a `POST /race-prep/carb-load/apply` that ALSO writes
the per-day carb min-bound into the per-date goal overrides — in one round
trip, inside a single transaction. The apply step merges into pre-existing
overrides (e.g. training-day kcal/protein templates), touching only `carbs_g`.

```bash
# 1. (Optional) Preview the schedule for a race three days out — pure compute,
#    no persistence. Useful for "what-if" exploration.
RACE_DATE=$(date -u -v+3d +%Y-%m-%d 2>/dev/null || date -u -d '+3 days' +%Y-%m-%d)
curl -s -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/race-prep/carb-load?race_date=$RACE_DATE&body_weight_kg=70" | jq .

# 2. Compute AND apply: writes the carb_g goal min-bound for each schedule day
#    into the per-date goal overrides. The response includes an `applied` array
#    reporting per-date outcome (`created: true` = new row, `false` = merged).
curl -s -X POST -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    -H "Content-Type: application/json" \
    -d "{\"race_date\":\"$RACE_DATE\",\"body_weight_kg\":70}" \
    "http://localhost:8080/race-prep/carb-load/apply" | jq .

# 3. Verify the override is now driving adherence on the first load day
#    (today + 1 day in this example).
LOAD_DAY=$(date -u -v+1d +%Y-%m-%d 2>/dev/null || date -u -d '+1 day' +%Y-%m-%d)
curl -s -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/summary/daily?date=$LOAD_DAY" | \
    jq '{goal_source, carbs_target: .adherence.carbs_g.target}'
# Expected: { "goal_source": "override", "carbs_target": { "min": 700 } }
```

### Plan per-leg race-day fueling

`plan_carb_load` covers the days *before* a race. For the in-event plan, store
the race with its legs once, then compute the per-leg carbs/sodium/fluid:

```bash
# 1. Create the race with ordered legs (swim → bike → run).
RACE=$(curl -s -X POST -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
          "name": "Allgäu Sprint",
          "race_date": "2026-07-24",
          "race_type": "sprint",
          "legs": [
            {"ordinal": 1, "discipline": "swim",  "expected_duration_min": 15},
            {"ordinal": 2, "discipline": "bike",  "expected_duration_min": 90},
            {"ordinal": 3, "discipline": "run",   "expected_duration_min": 50}
          ]
        }' \
    http://localhost:8080/races)
RACE_ID=$(echo "$RACE" | jq -r .id)

# 2. Compute the per-leg fueling plan. Supply a measured sweat rate to
#    personalise fluid + sodium (omit it and you get a flagged 600 ml/hr default).
curl -s -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/races/$RACE_ID/fueling-plan?body_weight_kg=70&sweat_rate_ml_per_hr=900" | \
    jq '.legs[] | {discipline, carbs_g_per_hr, sodium_mg_per_hr, fluid_ml_per_hr}'
# Total 155 min → 90 g/hr carb baseline: swim 0, bike 90, run 63 (0.7×).
# sweat 900 ml/hr → ~720 mg/hr sodium, 900 ml/hr fluid on bike+run; swim is dry.
```

The numbers are a deterministic baseline — the agent layers weather, gut
tolerance and course profile on top.

### Set up a training phase with a goal template

The phase + template pattern is the "I'm in build block 2, weeks 5-8 of my
plan" story. A template is a reusable goal-set; a phase is a named date range
that points at one. Editing the template propagates to every date inside the
phase — no apply step. Per-date overrides still win.

```bash
# 1. Create a reusable template (PUT, name in URL is canonical).
curl -s -X PUT -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
         "kcal":     {"min": 2280, "max": 2520},
         "protein_g":{"min": 160, "max": 200},
         "carbs_g":  {"min": 350, "max": 450},
         "notes":    "build block default targets"
       }' \
    http://localhost:8080/goal-templates/build-default | jq '.template | {name, id}'

# 2. Create a build phase pointing at it. (For demo, start today and run 14 days.)
TPL_ID=$(curl -s -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    http://localhost:8080/goal-templates/build-default | jq -r .template.id)
START=$(date -u +%Y-%m-%d)
END=$(date -u -v+14d +%Y-%m-%d 2>/dev/null || date -u -d '+14 days' +%Y-%m-%d)
curl -s -X POST -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    -H "Content-Type: application/json" \
    -d "{
         \"name\":\"build-block-demo\",
         \"type\":\"build\",
         \"start_date\":\"$START\",
         \"end_date\":\"$END\",
         \"default_template_id\":\"$TPL_ID\"
       }" \
    http://localhost:8080/phases | jq '.phase | {name, type, start_date, end_date, default_template_name}'

# 3. Override one specific workout day inside the phase (per-date override wins).
MID=$(date -u -v+3d +%Y-%m-%d 2>/dev/null || date -u -d '+3 days' +%Y-%m-%d)
curl -s -X PUT -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"carbs_g":{"min":700}}' \
    http://localhost:8080/goals/overrides/$MID > /dev/null

# 4. /summary/range shows the per-day mix: phase_template on most days,
#    override on the workout day.
curl -s -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/summary/range?from=$START&to=$END" | \
    jq '.days[] | {date, goal_source, phase_name}'
# Expected mostly { "goal_source": "phase_template", "phase_name": "build-block-demo" }
# with one { "goal_source": "override" } row on the workout day.
```

### One read: today's full context

`GET /context/daily` (and the matching MCP tool `daily_context`) is the
"one tool, many sources" frame. The agent calls it once at the start of a
session and gets adherence + totals + hydration + today's workouts + fuel
entries + body-weight state + training phase + goal-override presence —
the same data that would otherwise require 5-7 separate tool calls.

```bash
TODAY=$(date -u +%Y-%m-%d)
curl -s -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/context/daily?date=$TODAY" | jq '
      {
        adherence:    .adherence.goal_source,
        phase:        .phase.name,
        kcal_total:   .nutrition.totals.kcal,
        hydration_ml: .hydration.total_ml,
        workouts:     (.workouts | length),
        weight:       (.weight | {weight_kg, is_carryover}),
        override:     .goal_override.present
      }'
# Expected (depends on what you have seeded):
# {
#   "adherence": "phase_template",
#   "phase":     "build-block-demo",
#   "kcal_total": 1820,
#   "hydration_ml": 1500,
#   "workouts": 1,
#   "weight":   { "weight_kg": 72.5, "is_carryover": false },
#   "override": false
# }
```

For per-entry detail (full meal list, individual hydration entries, etc.),
use the dedicated endpoints — the aggregator deliberately omits them.

### A daily Garmin push (recovery + fitness + planned workout)

The recovery/fitness snapshots are date-keyed: a writer (e.g. a `garmin.py`
sync) POSTs one row per day and re-pushes the same date to update in place.
`daily_context` then surfaces them as same-day-or-null `recovery` / `fitness`
blocks. Here's the shape an importer's daily push takes:

```bash
DAY=$(date -u +%Y-%m-%d)

# Recovery snapshot (sleep / HRV / RHR / stress / body battery / readiness)
curl -s -X POST -H "Authorization: Bearer $MOBILE_API_TOKEN" -H "Content-Type: application/json" \
    -d "{\"date\":\"$DAY\",\"sleep_seconds\":27000,\"sleep_score\":82,\"hrv_ms\":61,\"resting_hr\":48,\"stress_avg\":28,\"training_readiness\":74}" \
    http://localhost:8080/recovery-metrics > /dev/null

# Fitness snapshot (VO2max / race predictions in seconds / acute+chronic load)
curl -s -X POST -H "Authorization: Bearer $MOBILE_API_TOKEN" -H "Content-Type: application/json" \
    -d "{\"date\":\"$DAY\",\"vo2max_running\":54,\"race_predictor_5k_seconds\":1230,\"acute_load\":420.5,\"chronic_load\":380}" \
    http://localhost:8080/fitness-metrics > /dev/null

# Hydration balance (Garmin's daily sweat-out / activity-intake / goal — distinct from /hydration)
curl -s -X POST -H "Authorization: Bearer $MOBILE_API_TOKEN" -H "Content-Type: application/json" \
    -d "{\"date\":\"$DAY\",\"sweat_loss_ml\":2400,\"activity_intake_ml\":1800,\"goal_ml\":3000}" \
    http://localhost:8080/hydration-balance > /dev/null

# Tomorrow's planned ride from the training calendar (status=planned allows the future date)
curl -s -X POST -H "Authorization: Bearer $MOBILE_API_TOKEN" -H "Content-Type: application/json" \
    -d "{\"source\":\"garmin\",\"sport\":\"bike\",\"status\":\"planned\",\"name\":\"Z2 endurance\",\"started_at\":\"$(date -u -v+1d +%Y-%m-%dT07:00:00Z 2>/dev/null || date -u -d '+1 day' +%Y-%m-%dT07:00:00Z)\",\"ended_at\":\"$(date -u -v+1d +%Y-%m-%dT09:00:00Z 2>/dev/null || date -u -d '+1 day' +%Y-%m-%dT09:00:00Z)\"}" \
    http://localhost:8080/workouts > /dev/null

# daily_context now carries recovery + fitness + hydration_balance for today
curl -s -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/context/daily?date=$DAY" | jq '{recovery: .recovery.resting_hr, fitness: .fitness.vo2max_running, sweat_loss: .hydration_balance.sweat_loss_ml}'
# → { "recovery": 48, "fitness": 54, "sweat_loss": 2400 }
```

### Cleaning up leftover products

Test sessions leave products in the cache. List and delete them:

```bash
# Enumerate manual products (most recently used first)
curl -s -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/products?source=manual&limit=50" | jq .

# Permanently delete one. Historical meals keep their snapshots.
curl -s -X DELETE -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    http://localhost:8080/products/<uuid>
# → 204 No Content on success
# → 409 product_in_use_as_component if a recipe references it (with the
#   using-recipes list and a hint to delete the recipes first)
```

The same flow is available via the LLM agent: `list_products` + `delete_product`.

### Logging a workout manually

`/workouts` accepts any writer that targets its shape. The expected primary writer for
Garmin-sourced sessions is `garmin.py` (external — lives outside this repo); use the manual
path below when you don't have a watch on (gym sessions, sweat-rate windows, anything the
importer doesn't see).

```bash
# Log a strength session by hand
WORKOUT_ID=$(curl -s -X POST -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    -H "Content-Type: application/json" \
    -d "{
         \"source\":\"manual\",
         \"sport\":\"strength\",
         \"name\":\"Push day\",
         \"started_at\":\"$(date -u +%Y-%m-%dT18:00:00Z)\",
         \"ended_at\":\"$(date -u +%Y-%m-%dT19:00:00Z)\"
       }" \
    http://localhost:8080/workouts | jq -r .id)

# Round-trip: list today's workouts
curl -s -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/workouts?from=$(date -u +%Y-%m-%dT00:00:00Z)&to=$(date -u -v+1d +%Y-%m-%dT00:00:00Z 2>/dev/null || date -u -d 'tomorrow' +%Y-%m-%dT00:00:00Z)" \
    | jq .
```

For a Garmin-shaped re-sync example (POST same `external_id` twice → second call returns 200, not 201)
see the Workouts section of the main README.

### Fueling-rehearsal round-trip (RPE + GI distress)

After a long ride you'd typically have logged several `workout_fuel` entries
during the session. To capture *how it felt*, PATCH the workout itself with
`rpe` (Borg CR-10, 1..10) and `gi_distress_score` (1=no distress, 5=severe).
Then `GET /workouts/{id}/fueling` reads "perceived effort + GI + carbs/sodium/
caffeine totals" in one call — the natural shape for evaluating whether the
fueling strategy worked.

```bash
# 1. Log a long Z2 ride (manually, or land it via the Garmin importer).
RIDE_ID=$(curl -s -X POST -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    -H "Content-Type: application/json" \
    -d "{
         \"source\":\"manual\",\"sport\":\"bike\",
         \"name\":\"Z2 long ride — fueling rehearsal\",
         \"started_at\":\"$(date -u +%Y-%m-%dT07:00:00Z)\",
         \"ended_at\":\"$(date -u +%Y-%m-%dT10:00:00Z)\",
         \"kcal_burned\":1800,\"tss\":110,
         \"distance_m\":85000,\"avg_power_w\":175,
         \"temperature_c\":28,\"sweat_loss_ml\":2600
       }" \
    http://localhost:8080/workouts | jq -r .id)
# distance/power/temperature/sweat_loss are optional ingestion metrics (a watch
# measures them; omit what you don't have). sweat_loss_ml + temperature_c get
# echoed back on /fueling so you can weigh estimated loss against fluid taken.

# 2. Log a few workout_fuel entries during the ride (skipped here — see the
#    Workout fuel section). Each carries carbs/sodium/caffeine/notes.

# 3. After the ride: PATCH rpe + gi_distress_score onto the workout.
curl -s -X PATCH -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"rpe":7,"gi_distress_score":2,"notes":"third gel sat heavy at km 60"}' \
    http://localhost:8080/workouts/$RIDE_ID | jq '{rpe, gi_distress_score, notes}'

# 4. /workouts/{id}/fueling now surfaces both fields at the top level alongside
#    the pre/intra/post window totals — one call for the rehearsal evaluation.
curl -s -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/workouts/$RIDE_ID/fueling" | \
    jq '{rpe, gi_distress_score, sweat_loss_ml, temperature_c, intra_carbs: .intra_window.workout_fuel.totals.carbs_g}'
# Expected: { "rpe": 7, "gi_distress_score": 2, "sweat_loss_ml": 2600, "temperature_c": 28, "intra_carbs": 75 }
```

---

## Pairing the companion app

The Flutter companion app (`apps/companion/`) authenticates with the same
`MOBILE_API_TOKEN` the curl examples use. Pairing is a one-time QR scan:

```bash
# With `task dev` running and qrencode installed (see Prerequisites):
task dev:pair        # or `task app:pair`
```

This prints a QR code **in the terminal** encoding a JSON payload:

```json
{"base_url": "http://<your-LAN-ip>:8080", "token": "<MOBILE_API_TOKEN>"}
```

Open the app on first launch — it shows a pairing screen — and point the camera
at the QR. The app stores the token in the Android Keystore and the base URL in
preferences, then drops you on the Today screen. The phone must be on the same
network as the backend (the URL uses your LAN IP, not `localhost`). To re-pair
against a different backend, use **Unpair** in the app's Settings sheet and scan
again.

---

## Wire up the MCP server (Claude Code / Claude Desktop)

The MCP server is a subcommand of the same binary (`nutrition-api mcp`). It
talks to the REST API over HTTP using `AGENT_API_TOKEN`, so the REST API
(`task dev`) must be running.

```bash
task install          # builds and installs to ~/.local/bin/nutrition-api
```

Then in `~/.claude/mcp.json` (Claude Code) or
`~/Library/Application Support/Claude/claude_desktop_config.json`
(Claude Desktop):

```json
{
  "mcpServers": {
    "nutrition": {
      "command": "/Users/<you>/.local/bin/nutrition-api",
      "args": ["mcp"],
      "env": {
        "NUTRITION_API_URL": "http://localhost:8080",
        "AGENT_API_TOKEN": "<paste AGENT_API_TOKEN from .env.local>"
      }
    }
  }
}
```

Restart Claude Code (or quit Claude Desktop fully and reopen), then ask the
agent "What did I eat today?" — it should call the `daily_summary` tool and
return the meal you logged above.

---

## Troubleshooting

**`neither docker nor podman found in PATH`**
Install Docker Desktop, OrbStack, or Podman. Re-run `task dev`.

**`port is already allocated` from Postgres startup**
You already have something listening on `:5432`. Either stop it, or change
`POSTGRES_PORT` at the top of `Taskfile.yml`.

**`postgres did not become ready in time`**
The container started but `pg_isready` did not succeed within 6 seconds. Check
`docker logs nutrition-pg` (or `podman logs`). Often a stale data volume from
an interrupted run — `task dev:reset` clears it.

**`401 auth_required` from every curl**
You're not sending `Authorization: Bearer …`, or the token doesn't match the
one in `.env.local`. Re-source the env in your shell:
`set -a; . ./.env.local; set +a`.

**`product_not_found` from `POST /products/lookup/...`**
Open Food Facts doesn't recognise that barcode. The response body's `next`
field tells you to use `POST /meals/freeform` instead (supply name and
nutriments yourself).

**Tests fail with `bridge: network not found`**
You're on Podman and the testcontainers Ryuk reaper failed to launch. The
test helper should auto-disable Ryuk; if it doesn't, run
`TESTCONTAINERS_RYUK_DISABLED=true task test`.

---

## Next steps

- Reference docs (full env table, every endpoint, project layout) live in
  [`README.md`](README.md).
- Per-capability specs live in [`openspec/specs/`](openspec/specs/).
- Past changes (proposals + designs) are in
  [`openspec/changes/archive/`](openspec/changes/archive/).
