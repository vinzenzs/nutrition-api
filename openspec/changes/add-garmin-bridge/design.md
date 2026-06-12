# Design: add-garmin-bridge

## Context

Decided in an explore session. Garmin Connect has no official API; `garth` +
`garminconnect` (Python) are the maintained reverse-engineered clients, and
they carry the fragile, frequently-broken parts: the SSO login, MFA, and OAuth1
token minting. Porting that to Go means owning a breakage treadmill in an
unmaintained reimplementation. So all Garmin logic stays in Python, isolated in
a small service; the Go backend stays Garmin-agnostic and consumes its own REST
API. This is the project's first non-Go, non-Flutter component.

## Goals / Non-Goals

**Goals:**
- Fill the empty recovery/fitness/hydration-balance/weight/workout capabilities
  from Garmin, on a daily schedule, idempotently.
- Isolate Garmin's churn so a break is a `pip upgrade`, not a Go change.
- Survive k8s reschedules without losing the auth token (statelessness).
- Make the rare, interactive MFA login an explicit, separate operation.

**Non-Goals:**
- No Garmin code in Go. No structured re-modeling of Garmin payloads in the DB.
- No MCP login proxy (separate change). No multi-account support.

## Decisions

### D1: Two planes — interactive login, headless sync — sharing a token

```
LOGIN (rare, interactive)                  SYNC (daily, headless)
email+pw → SSO → MFA → OAuth1 token         stored OAuth1 → OAuth2 → GET → map → POST
   └── PUT /garmin/token (persist) ───────────────▲
                                                   GET /garmin/token (reuse)
```

MFA is a *login-time-only* event; the daily sync never prompts for it. The token
lives in the backend (`add-garmin-auth-token`), so the bridge holds no durable
state. **Alternative rejected:** a PVC on the bridge — ties it to a node/volume
and complicates reschedules; Postgres-via-backend keeps one source of state.

### D2: The bridge POSTs to the existing REST API, not the DB

The bridge maps a day's Garmin data to existing endpoints under
`GARMIN_API_TOKEN`. recovery/fitness/hydration-balance already upsert by date and
`/workouts/bulk` accepts a batch, so re-running a day is idempotent for free —
no new write code, no DB coupling in Python. **Alternative rejected:** the
bridge writing Postgres directly — duplicates the services' validation and
couples Python to the schema.

### D3: One replica, in-memory SSO state during login only

`garth`'s login is two calls (`/login` then `/login/mfa`) with intermediate SSO
state held in memory between them. Running >1 replica would route the second
call to a pod that never saw the first. Single-user ⇒ 1 replica is fine; the
login window is seconds. The daily sync holds no such state (it reads the token
fresh), so this constraint is login-only. **Alternative considered:** persisting
the intermediate SSO state too — unnecessary complexity for a single user.

### D4: OAuth2 refresh on every sync, OAuth1 bootstrap rarely

The stored OAuth1 token lasts ~a year and mints short-lived OAuth2 access tokens
with a simple exchange. Each `/sync` refreshes OAuth2 from the stored OAuth1
(garth handles this), so the only thing that ever needs a human is the OAuth1
bootstrap (login + MFA) when it first runs or finally expires.

### D5: Activity dedup uses the existing `external_id` UPSERT — no new field

Daily activity re-sync must not duplicate workouts, and the `workouts` capability
*already* solves this: `POST /workouts` UPSERTs on `external_id`, with the spec
explicitly framing it as "lets an external writer POST every activity it sees
without tracking what is already synced" (its example even uses
`external_id: "garmin:1234567", source: "garmin"`). The bridge sends
`external_id = "garmin:<activity_id>"`; re-syncs update in place. Date-keyed
metrics need nothing (the date is the key). **This removed a phantom
prerequisite** — an earlier sketch assumed a new `garmin_activity_id` column was
needed; grounding in the spec showed `external_id` already is it.

### D6: Python service shape

FastAPI (or Flask) + `garminconnect` + an `httpx` client to the nutrition API.
Three endpoints (`/login`, `/login/mfa`, `/sync`), a tiny mapping module
(Garmin payload → REST body per capability), and a `/healthz`. Config via env:
Garmin credentials, `GARMIN_API_TOKEN`, `NUTRITION_API_URL`. Containerized with
its own slim Python `Dockerfile`; deployed by the existing Helm chart as an
opt-in Deployment+Service+CronJob.

## Risks / Trade-offs

- [Garmin breaks the private API] → the whole reason for B; fix is `pip upgrade
  garminconnect` + redeploy the bridge. The Go side never moves.
- [Python in a Go repo adds a toolchain] → accepted and isolated under
  `apps/garmin-bridge/`; the Cookidoo extension already set the multi-language
  precedent. CI for it is optional (not a Go-build gate), mirroring the Flutter
  decision.
- [MFA login at an inconvenient time] → the manual `POST /login`/`/login/mfa`
  path works now; `add-garmin-mcp-login` later makes it a chat action from the
  phone.
- [Mapping drift if a capability's request shape changes] → the mapping module is
  small and the only coupling point; covered by the bridge's own tests against a
  recorded Garmin fixture + a stub nutrition API.
- [Credentials handling] → the Garmin password lives only in a k8s Secret read by
  the bridge; it is never sent to the nutrition API or (later) the chat — only
  the ephemeral MFA code transits the login call.

## Migration Plan

No DB migration in this change. The only backend prerequisite is
`add-garmin-auth-token` (token store + identity); activity dedup already exists
(`workouts` `external_id` UPSERT). Deploy order: `add-garmin-auth-token` first
(backend), then this bridge (Python + Helm). Rollback: scale the bridge to zero /
remove its Helm values; the backend is unaffected. Bootstrap: run `POST /login`
once and complete MFA before enabling the CronJob.

## Open Questions

- **FastAPI vs Flask** — decide at implementation by image size / familiarity;
  the three-endpoint surface is trivial either way.
- **Sync window** — daily is the baseline; whether to also sync intra-day
  (e.g. post-workout) is deferred until real use shows the lag matters.
- **Backfill** — looping `/sync {date}` over a range covers it; a first-run
  N-day backfill could be a CronJob arg, decided at implementation.
