# garmin-bridge

A small Python service that authenticates to **Garmin Connect**, fetches a day's
recovery / fitness / hydration / weight / activity data, and writes it to the
nutrition REST API. It is the project's first non-Go, non-Flutter component, and
it exists so that all of Garmin's churning, unofficial-API surface stays in
Python — when Garmin breaks something the fix is `pip upgrade garminconnect`,
not Go work. See [`openspec/changes/archive/*-add-garmin-bridge/`](../../openspec)
for the full design.

## How it works

Two planes share one process:

```
LOGIN (rare, interactive)                   SYNC (daily, headless)
 email+pw → SSO → MFA → token                stored token → fresh OAuth2 → GET → map → POST
        └── PUT /garmin/token ───────────────────▲
                                                  GET /garmin/token
```

- **Login** (`POST /login` → `POST /login/mfa`) performs Garmin SSO with
  credentials read from config (never the request body), handles MFA, and
  persists the minted `garth` token blob to the backend (`PUT /garmin/token`).
  The blob is **never** returned to the caller or logged.
- **Sync** (`POST /sync`) reads the stored token (`GET /garmin/token`), refreshes
  the short-lived OAuth2 access token (no MFA, no human), fetches the day(s), maps
  them, and POSTs to the existing REST endpoints under `GARMIN_API_TOKEN`.
  - With an explicit `{"date":"YYYY-MM-DD"}` body it syncs **exactly that day**.
  - With **no body** it syncs a **rolling window** — today plus the previous
    `SYNC_LOOKBACK_DAYS` days (default `2` → a 3-day window). A same-day-only sync
    misses a day's own completed activities (they happen after the 05:00 cron) and
    Garmin's later-computed training-load / VO2max / race-predictor metrics
    (recomputed once activities process); re-pulling recent days lets those late
    signals land. Date-keyed upserts + `external_id` workout dedup make the
    re-pull safe, and each day is independent — one failing day never sinks the
    window.

The token lives in the backend (Postgres, via `add-garmin-auth-token`), so the
bridge holds **no durable state** — a restart or k8s reschedule resumes by
reading the token. The only in-memory state is the transient SSO context between
the two login calls, which is why the bridge runs as a **single replica**.

### Mapping

| Garmin data                              | REST endpoint        | Idempotency            |
| ---------------------------------------- | -------------------- | ---------------------- |
| sleep / HRV / RHR / stress / readiness   | `POST /recovery-metrics` | upsert by date     |
| VO2max / training-load / race predictions| `POST /fitness-metrics`  | upsert by date     |
| sweat loss / hydration                   | `POST /hydration-balance`| upsert by date     |
| weigh-ins                                | `POST /weight`           | per entry          |
| activities                               | `POST /workouts/bulk`    | upsert by `external_id = "garmin:<id>"` |

Activity dedup needs no new field: the `workouts` capability already UPSERTs by
`external_id`, so the bridge just POSTs every activity it sees. Re-running a day
is safe for everything.

## Configuration (env)

| Variable            | Required | Meaning                                                |
| ------------------- | -------- | ------------------------------------------------------ |
| `GARMIN_EMAIL`      | yes      | Garmin Connect account email                           |
| `GARMIN_PASSWORD`   | yes      | Garmin Connect password (Secret only)                  |
| `GARMIN_API_TOKEN`  | yes      | Bearer token for the backend's `garmin` identity       |
| `NUTRITION_API_URL` | yes      | Backend base URL, e.g. `http://kazper`          |
| `SYNC_TZ`           | no       | IANA tz "today" resolves in (default `UTC`)            |
| `SYNC_LOOKBACK_DAYS`| no       | Dateless `/sync` rolling-window lookback (default `2` → today + 2 prior; `0` = today only) |
| `PORT`              | no       | Listen port (default `8080`)                           |
| `LOG_LEVEL`         | no       | `INFO` (default), `DEBUG`, …                           |

Missing required vars crash the process at startup (fail-fast). The password and
token are scrubbed from all logs by a redacting logging filter.

## Run locally

```bash
cd apps/garmin-bridge
python3 -m venv .venv && . .venv/bin/activate
pip install -e ".[dev]"

export GARMIN_EMAIL=you@example.com
export GARMIN_PASSWORD='…'
export GARMIN_API_TOKEN='…'                 # must match the backend's GARMIN_API_TOKEN
export NUTRITION_API_URL=http://localhost:8080

python -m garmin_bridge      # serves on :8080
```

### The MFA login flow (one-time, ~yearly)

```bash
# 1. Start SSO. If Garmin requires a code:
curl -X POST localhost:8080/login
# → {"needs_mfa": true}

# 2. Complete it with the 6-digit code from your authenticator/email:
curl -X POST localhost:8080/login/mfa -H 'Content-Type: application/json' -d '{"code":"123456"}'
# → {"logged_in": true}      (token now persisted in the backend; never returned)
```

If Garmin does not require MFA, `POST /login` persists the token directly and
returns `{"logged_in": true}`.

### Manual sync

```bash
curl -X POST localhost:8080/sync                                  # rolling window: today + SYNC_LOOKBACK_DAYS prior
curl -X POST localhost:8080/sync -H 'Content-Type: application/json' -d '{"date":"2026-06-10"}'  # exactly that day
```

A dateless `POST /sync` returns a per-day window result (`days`, `days_total`,
`days_ok`, `days_failed`); an explicit `date` returns that day's per-capability
summary. Status is `200` when every synced day/capability landed and `207` on any
partial failure. With no stored token it returns `409 login_required` and writes
nothing.

### Scheduling (writing the plan to the watch)

The bridge also writes structured workouts **to** Garmin (per
`add-garmin-scheduling`). The backend sends only our step model; the bridge owns
the `garminconnect` payload translation (`workout_builder.py`). All four read the
stored token (no MFA) and return `409 login_required` if it's absent.

```bash
# Compile our step model into a Garmin structured workout and create it
curl -X POST localhost:8080/workouts -H 'Content-Type: application/json' \
    -d '{"sport":"run","name":"VO2","steps":[{"type":"step","intent":"warmup","duration":{"kind":"time","seconds":600},"target":{"kind":"hr_zone","low":1,"high":2}}]}'
# → {"garmin_workout_id":"<id>"}

# Schedule it on a date → returns the calendar entry id
curl -X POST localhost:8080/schedule -H 'Content-Type: application/json' \
    -d '{"garmin_workout_id":"<id>","date":"2026-06-12"}'
# → {"garmin_schedule_id":"<id>"}

# Unschedule (idempotent — an already-gone id is a no-op success)
curl -X DELETE "localhost:8080/schedule?schedule_id=<id>"      # → {"unscheduled":true}

# Read the calendar for a range (for reconciliation)
curl "localhost:8080/calendar?from=2026-06-01&to=2026-06-30"   # → {"from","to","items":[…]}
```

These are driven by the backend's `garmin-control` endpoints
(`/garmin/schedule/workout`, `/garmin/schedule/plan`, `/garmin/calendar`) and the
matching MCP tools — see the repo-root README. The `garminconnect` payload shape
never leaves the bridge.

## Tests

```bash
. .venv/bin/activate
pytest        # mapping, login (incl. MFA + redaction), sync (incl. idempotency)
```

The mapping is exercised against a recorded Garmin fixture
(`tests/fixtures/garmin_day.json`); login and sync run against a stub backend.

## Deploy (Helm, opt-in)

The bridge is rendered by the existing chart only when `garminBridge.enabled` is
true. Bootstrap order:

1. Deploy with the bridge enabled and its Secret values set:

   ```yaml
   garminBridge:
     enabled: true
     cron:
       enabled: false        # keep the daily sync off until login succeeds
     secrets:
       garminEmail: you@example.com
       garminPassword: "…"
       garminApiToken: "…"   # == the backend's GARMIN_API_TOKEN
   ```

2. Log in once (the pod holds SSO state in memory, so target the Service):

   ```bash
   kubectl port-forward svc/<release>-kazper-garmin-bridge 8080:80
   curl -X POST localhost:8080/login
   curl -X POST localhost:8080/login/mfa -d '{"code":"123456"}' -H 'Content-Type: application/json'
   ```

3. Enable the CronJob (`garminBridge.cron.enabled: true`) and `helm upgrade`. To
   import history on first run, set `garminBridge.cron.backfillDays` to the
   number of days to walk back, then reset it to `0`.

Rollback is `garminBridge.enabled: false` (or scale to zero) — the backend is
unaffected; the token simply sits unused in Postgres.

## Docker

```bash
docker build -t garmin-bridge apps/garmin-bridge
```

Slim, non-root `python:3.11-slim` image; `CMD python -m garmin_bridge`.
