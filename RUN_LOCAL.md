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
task db:down
```

To start completely fresh (delete container, regenerate `.env.local`):

```bash
task dev:reset
```

---

## Prerequisites

| Tool                | Why                                                | Check                  |
|---------------------|----------------------------------------------------|------------------------|
| Go 1.24+            | Build the binaries                                 | `go version`           |
| Docker or Podman    | Run Postgres locally                               | `docker info` or `podman info` |
| Task (taskfile.dev) | Drive the dev workflow                             | `task --version`       |
| `curl`              | Smoke-test the API                                 | `curl --version`       |

Task install:

- macOS: `brew install go-task/tap/go-task`
- Linux: `sh -c "$(curl -fsSL https://taskfile.dev/install.sh)" -- -d -b ~/.local/bin`
- Other: see <https://taskfile.dev/installation/>

If you use Podman with a Docker-compatible socket, `docker` and `podman` are
both auto-detected — `task db:up` prefers `docker` when both are present, falls
back to `podman` otherwise.

---

## What `task dev` does, exactly

If you want to know what's happening under the hood:

1. **`task db:up`** runs first as a dependency. It looks for a container named
   `nutrition-pg`:
   - already running → no-op
   - exists but stopped → starts it
   - doesn't exist → `docker run -d --name nutrition-pg -e POSTGRES_USER=nutrition -e POSTGRES_PASSWORD=nutrition -e POSTGRES_DB=nutrition -p 5432:5432 postgres:17-alpine`
   - waits until `pg_isready` succeeds (up to 6 seconds).

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
   ```

   These dev tokens are not secret — the auth middleware exists so the API
   doesn't accidentally trust unauthenticated clients, not to defend against
   anyone who can already read your filesystem. For production, generate real
   tokens with `openssl rand -hex 32`.

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

# Workout fueling round-trip (per add-meal-workout-link):
#   1. log a workout
#   2. log a meal during the intra window, with workout_id set
#   3. log a hydration sip during intra (no workout_id — time-window match still picks it up)
#   4. fetch /workouts/{id}/fueling — pre/intra/post buckets with nutrition + hydration sub-objects
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

# Fetch fueling. Both contributions land in intra (time-window match, not tag).
curl -s -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/workouts/$WID/fueling" | jq .
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

The `race-prep` capability is a stateless computation primitive: it returns a
carb-load schedule but writes nothing. Pair it with `PUT /goals/overrides/{date}`
to put the per-day targets into the goals layer so adherence reflects them.

```bash
# 1. Compute the schedule for a race four weeks out.
RACE_DATE=$(date -u -v+28d +%Y-%m-%d 2>/dev/null || date -u -d '+28 days' +%Y-%m-%d)
SCHEDULE=$(curl -s -H "Authorization: Bearer $MOBILE_API_TOKEN" \
    "http://localhost:8080/race-prep/carb-load?race_date=$RACE_DATE&body_weight_kg=70")
echo "$SCHEDULE" | jq .

# 2. Push each schedule entry into a daily goal override so adherence on
#    those days reflects the carb-load target. Each carb gram = 4 kcal, so we
#    set kcal/carbs together; protein and fat stay at whatever the agent
#    decided is appropriate for the rest of the day (kept simple here).
echo "$SCHEDULE" | jq -c '.schedule[]' | while read -r entry; do
    DATE=$(echo "$entry" | jq -r .date)
    CARBS=$(echo "$entry" | jq -r .target_carbs_g)
    curl -s -X PUT -H "Authorization: Bearer $MOBILE_API_TOKEN" \
        -H "Content-Type: application/json" \
        -d "{\"carbs_g\":{\"min\":$CARBS,\"max\":$CARBS}}" \
        "http://localhost:8080/goals/overrides/$DATE" > /dev/null
    echo "set override for $DATE → ${CARBS}g carbs"
done
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
