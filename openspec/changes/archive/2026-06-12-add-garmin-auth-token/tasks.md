# Tasks: add-garmin-auth-token

## 1. Config + auth identity

- [x] 1.1 `internal/config`: add `GARMIN_API_TOKEN` (optional) and `GARMIN_TOKEN_ENC_KEY` (base64 32-byte key; required only when the garmin token is set) with validation that the key decodes to 32 bytes
- [x] 1.2 `internal/auth`: extend `Config` + `Middleware` with the optional third token → `client_id = "garmin"` (constant-time compare); recognize it only when set
- [x] 1.3 `internal/auth`: startup validation — when `GARMIN_API_TOKEN` set, require ≥16 bytes and distinct from mobile/agent; unset imposes nothing. Unit tests for accept/reject/startup cases

## 2. Migration + crypto

- [x] 2.1 Check migration head, then `task migrate:new NAME=add_garmin_tokens` — `garmin_tokens(id SMALLINT PK DEFAULT 1 CHECK (id=1), ciphertext BYTEA, nonce BYTEA, updated_at TIMESTAMPTZ)`
- [x] 2.2 `internal/garminauth/crypto.go`: AES-256-GCM `seal(plaintext) -> (ciphertext, nonce)` / `open(ciphertext, nonce) -> plaintext` over the config key; unit tests (round-trip, tamper-detect)

## 3. Capability package

- [x] 3.1 `internal/garminauth/{types,repo}.go`: single-row upsert / get / delete against `store.Querier`; `ErrNotFound`
- [x] 3.2 `service.go`: seal on store, open on read; sentinel errors; refuse operation when the enc key is unconfigured
- [x] 3.3 `handlers.go`: `PUT /garmin/token`, `GET /garmin/token`, `DELETE /garmin/token` (blob in body as opaque bytes), swag annotations, `Register(rg)`
- [x] 3.4 Identity guard: the three routes require `client_id = "garmin"` (403 otherwise); when `GARMIN_API_TOKEN` unset the handler short-circuits `503 garmin_disabled`
- [x] 3.5 Ensure the request/response logger does NOT log the `/garmin/token` body (redact or opt-out), so the plaintext blob never reaches logs

## 4. Wiring + tests

- [x] 4.1 Wire in `internal/httpserver` behind auth + idempotency (note: `GET` ignores Idempotency-Key already; `PUT` rejects it per existing PUT rule — confirm `PUT` semantics here)
- [x] 4.2 Handler integration tests: store→get verbatim round-trip; get-when-empty 404; delete clears; mobile/agent identities get 403; unset `GARMIN_API_TOKEN` → 503; DB row holds ciphertext not plaintext

## 5. Docs + verification

- [x] 5.1 `task swag`; README config table gains `GARMIN_API_TOKEN` + `GARMIN_TOKEN_ENC_KEY`; one-line note on the endpoints' garmin-only access
- [x] 5.2 `task vet` + `task test` green; `openspec validate add-garmin-auth-token --strict` passes
