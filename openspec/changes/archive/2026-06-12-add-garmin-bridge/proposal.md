# Proposal: add-garmin-bridge

## Why

The backend stores recovery, fitness, hydration-balance, weigh-in, and workout
data but nothing fills those tables — the highest-leverage gap before the
2026-07-24 race. Garmin Connect has no official API; the maintained way to reach
it is the Python `garth` + `garminconnect` libraries. Rather than reimplement
Garmin's churning private API and MFA login in Go, this change adds a small
Python **garmin-bridge** service that owns all Garmin auth and fetch, maps the
results to the existing REST surface, and runs on a schedule — so when Garmin
breaks its API the fix is `pip upgrade garminconnect`, not Go work.

The design was settled in an explore session. The bridge has two planes: an
**interactive MFA login** (rare, ~yearly) that mints a long-lived garth token,
and a **headless daily sync** (frequent) that reuses it. Keeping the token in
Postgres (via `add-garmin-auth-token`) makes the bridge stateless across k8s
reschedules.

## What Changes

- **New `apps/garmin-bridge/`** — a Python service (the repo's first non-Go,
  non-Flutter component, sibling to `apps/companion/`) exposing:
  - `POST /login` → starts Garmin SSO (credentials from a Secret), returns
    `{needs_mfa: true}` when MFA is required.
  - `POST /login/mfa` → completes login with the 6-digit code, persists the
    minted token blob to the backend (`PUT /garmin/token`).
  - `POST /sync` → reads the token (`GET /garmin/token`), refreshes the OAuth2
    access token, fetches the requested day, **maps and POSTs to the existing
    REST API** under `GARMIN_API_TOKEN`.
- **Mapping** (bridge → existing endpoints, all idempotent by date or batch):
  - sleep / HRV / RHR / stress → `POST /recovery-metrics` (date upsert)
  - VO2max / training-load → `POST /fitness-metrics` (date upsert)
  - sweat loss → `POST /hydration-balance` (date upsert)
  - weigh-ins / biometrics → `POST /weight`
  - activities → `POST /workouts/bulk` with `source = "garmin"` and
    `external_id = "garmin:<activity_id>"` — the **existing** UPSERT-by-`external_id`
    on `/workouts` dedups re-syncs, so no new dedup work is needed
- **Deployment** (extends the existing Helm chart): a `garmin-bridge`
  Deployment + ClusterIP Service (**1 replica** — it holds in-memory SSO state
  between the two login calls), and a **k8s CronJob** that triggers `POST /sync`
  daily. Credentials and `GARMIN_API_TOKEN` come from Secrets.
- **No backend Go changes** beyond what the two prerequisite changes already
  add — the bridge consumes the REST API as an ordinary authenticated client.

## Capabilities

### New Capabilities

- `garmin-bridge`: a sidecar service that authenticates to Garmin Connect
  (interactive MFA login + headless token-driven sync), fetches a day's
  recovery/fitness/hydration/weight/activity data, and writes it to the
  nutrition REST API. Defines the control-plane/data-plane split, the
  statelessness contract (token in the backend), and the mapping invariants.

### Modified Capabilities

_None — the bridge consumes existing REST capabilities. It depends only on
`add-garmin-auth-token` (token store + identity). Activity dedup needs no new
work: the `workouts` capability already UPSERTs by `external_id`, exactly the
"POST every activity you see" contract the bridge wants._

## Impact

- **New toolchain in the monorepo**: Python (deps via `requirements.txt` /
  `pyproject`, its own `Dockerfile`). Rationale captured in design.md so the
  Python-in-a-Go-repo choice is legible.
- **Helm**: new Deployment, Service, CronJob, and Secret keys (Garmin
  credentials, `GARMIN_API_TOKEN`, the backend base URL). The bridge is opt-in —
  absent config, it simply isn't deployed.
- **Backend**: none directly; relies on `GARMIN_API_TOKEN` (from
  `add-garmin-auth-token`). Activity dedup is already provided by the existing
  `workouts` `external_id` UPSERT — no backend change needed.
- **Docs**: a `apps/garmin-bridge/README.md` (login bootstrap, the MFA flow,
  running the sync locally) + a repo-root pointer.

### Out of scope (explicit non-goals)

- **The MCP login proxy** (`garmin_login` / `garmin_submit_mfa` tools) — layered
  on this bridge as `add-garmin-mcp-login`. Until it lands, login is a manual
  `kubectl exec` / direct `POST /login` call.
- **Rewriting any Garmin logic in Go** — the whole point is to keep it in Python.
- **Historical backfill UX** — `POST /sync {date}` can be looped for backfill,
  but no dedicated multi-day importer is built here.
- **Multi-user / multiple Garmin accounts** — single user, single token.
