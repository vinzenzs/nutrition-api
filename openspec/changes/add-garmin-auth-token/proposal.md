# Proposal: add-garmin-auth-token

## Why

The backend has homes for Garmin-sourced data (recovery, fitness, hydration-
balance, weigh-ins, planned workouts) but nothing fills them. The chosen
integration is a separate Python `garmin-bridge` service that owns all Garmin
auth/fetch (`garth` + `garminconnect`) and POSTs the mapped data to this REST
API. Two backend prerequisites unblock it, and this change delivers the smaller,
purely-backend one: a place to keep the bridge's auth token and a dedicated
identity for it to call the API under.

The bridge's Garmin login is interactive (MFA) and rare; the daily sync is
headless and frequent. To keep the bridge **stateless across k8s reschedules**,
the long-lived garth token blob it mints at login must live somewhere durable
the bridge can read before each sync — Postgres, owned by the backend. The
backend stores it as an opaque, encrypted blob; it contains no Garmin logic.

## What Changes

- **New `garmin-auth` capability**: a single-row store for the bridge's auth
  token blob (the serialized garth OAuth1/OAuth2 token), encrypted at rest.
  - `PUT /garmin/token` — the bridge persists the blob after a successful login.
  - `GET /garmin/token` — the bridge reads it before each headless sync.
  - `DELETE /garmin/token` — clears it (forces re-login).
  - The blob is opaque to the backend (stored/returned verbatim after decrypt);
    the backend never parses, refreshes, or interprets it.
- **New dedicated `GARMIN_API_TOKEN` auth identity** (`client_id = "garmin"`),
  alongside `MOBILE_API_TOKEN` / `AGENT_API_TOKEN`, so the bridge authenticates
  under its own provenance and can be revoked independently. **Unlike the other
  two it is OPTIONAL** — Garmin integration is opt-in (mirrors `ANTHROPIC_API_KEY`);
  when unset, the `/garmin/token` endpoints reject with `503 garmin_disabled`.
- **Encryption at rest**: the blob is encrypted with a key from config
  (`GARMIN_TOKEN_ENC_KEY`); the plaintext is never logged. The `/garmin/token`
  endpoints require the `garmin` identity (the agent/mobile tokens cannot read
  or write the Garmin token).

## Capabilities

### New Capabilities

- `garmin-auth`: durable, encrypted, single-row store for the garmin-bridge's
  opaque auth token blob, with read/write/clear scoped to the dedicated garmin
  identity. Holds no Garmin protocol knowledge.

### Modified Capabilities

- `auth`: adds a third, **optional** static bearer identity
  (`GARMIN_API_TOKEN` → `client_id = "garmin"`) and extends startup validation
  (when set, it must be ≥16 bytes and differ from the other two).

## Impact

- **DB**: one migration creating `garmin_tokens` (single-row; `ciphertext`,
  `nonce`, `updated_at`).
- **Config**: `GARMIN_API_TOKEN` (optional) and `GARMIN_TOKEN_ENC_KEY` (required
  only when `GARMIN_API_TOKEN` is set) added to `internal/config`.
- **Code**: new `internal/garminauth` package (standard capability shape) with
  an `internal/garminauth/crypto.go` for AES-GCM seal/open; `internal/auth`
  middleware learns the optional third token + `client_id = "garmin"`;
  `internal/httpserver` wiring; route guard requiring the `garmin` identity.
- **Docs**: `task swag`; README config table + a one-line note on the endpoints.
- **Sequencing**: trunk for the Garmin integration — `add-garmin-bridge` depends
  on this (token store + identity) and on `add-workout-activity-dedup` (the
  other backend prerequisite). No client or Garmin-protocol work here.

### Out of scope (explicit non-goals)

- **Any Garmin protocol logic** (auth flow, fetch, mapping) — all of that lives
  in `add-garmin-bridge`.
- **Multi-account / token rotation policy** — single user, single token blob;
  re-login overwrites it.
- **The MCP login proxy** — that is `add-garmin-mcp-login`, layered on the bridge.
- **A general secrets vault** — this is one purpose-built opaque blob, not a KV
  store; resist growing it.
