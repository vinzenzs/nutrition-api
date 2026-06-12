# Tasks: add-garmin-bridge

## 1. Python service scaffold

- [x] 1.1 `apps/garmin-bridge/` — Python project (`pyproject.toml`/`requirements.txt`): `garminconnect`, `garth`, a web framework (FastAPI or Flask — decide by image size), `httpx`; `.gitignore`, `README.md` stub
- [x] 1.2 Config via env: Garmin email/password, `GARMIN_API_TOKEN`, `NUTRITION_API_URL`, sync timezone; fail fast if required vars are unset
- [x] 1.3 `/healthz` endpoint; structured logging that redacts password + token blob

## 2. Auth (control plane)

- [x] 2.1 `POST /login` — start Garmin SSO via garth/garminconnect using config credentials; detect MFA-required and return `{needs_mfa: true}`, retaining the in-progress SSO state
- [x] 2.2 `POST /login/mfa` — resume with the supplied code, complete login, serialize the garth token blob, `PUT /garmin/token` to the backend; never return the blob
- [x] 2.3 Login error handling: bad credentials, wrong/expired MFA code, Garmin lockout → typed error responses; nothing sensitive in logs
- [x] 2.4 Tests: MFA-required path returns needs_mfa; mfa step persists the blob (stub backend); password/blob absent from logs

## 3. Sync (data plane)

- [x] 3.1 `POST /sync` — `GET /garmin/token` from backend (404 → "login required"), load it into garth, refresh OAuth2 (no MFA), fetch the day's data
- [x] 3.2 Mapping module: Garmin payload → REST bodies — sleep/HRV/RHR/stress → `/recovery-metrics`; VO2max/load → `/fitness-metrics`; sweat loss → `/hydration-balance`; weigh-ins → `/weight`; activities → `/workouts/bulk` (`source=garmin`, `external_id="garmin:<activity_id>"` — existing UPSERT dedups)
- [x] 3.3 POST each mapped payload under `GARMIN_API_TOKEN`; surface partial failures (per-capability) without aborting the whole sync; report a summary
- [x] 3.4 Tests: a recorded Garmin fixture → expected REST calls against a stub nutrition API; re-run idempotency (date upserts + activity dedup); missing-token path

## 4. Containerization + Helm

- [x] 4.1 Slim `apps/garmin-bridge/Dockerfile` (python:slim, non-root)
- [x] 4.2 Helm: opt-in `garmin-bridge` Deployment (1 replica) + ClusterIP Service + `/healthz` probes; Secret keys for Garmin creds + `GARMIN_API_TOKEN`; values gate so it's absent when unconfigured
- [x] 4.3 Helm: a CronJob (daily) that `curl`s `POST /sync` on the bridge Service; optional first-run N-day backfill arg
- [x] 4.4 Document the bootstrap: deploy, `POST /login` once, complete MFA, then enable the CronJob

## 5. Docs + verification

- [x] 5.1 `apps/garmin-bridge/README.md`: local run, the MFA login flow, manual `/sync`, the token-in-backend model; repo-root README pointer
- [x] 5.2 Python tests green (`pytest`); a manual end-to-end against a real Garmin account + a local backend (login → sync → rows appear in recovery/fitness/hydration-balance/weight/workouts)
- [x] 5.3 `openspec validate add-garmin-bridge --strict` passes
