# Proposal: add-garmin-mcp-login

## Why

The Garmin token expires roughly yearly (and on password change or a forced
re-auth), and renewing it requires an interactive MFA login. Without this change,
that means a `kubectl exec` into the bridge at whatever inconvenient moment it
expires. Since the companion app now has a chat surface, the natural fix is to
drive the re-login **from the phone, through the agent**: "re-link Garmin" →
"here's the code from my authenticator" → done for another year.

The bridge already exposes `POST /login` and `POST /login/mfa`, but it lives on an
internal ClusterIP that the MCP server (a REST-API client, often off-cluster)
can't reach. So the backend adds two thin proxy endpoints that forward to the
bridge, and the MCP server wraps them as two tools — preserving the project's
"one MCP tool = one HTTP call to the REST API" invariant.

## What Changes

- **New `garmin-control` capability**: two backend proxy endpoints that forward
  to the bridge (`GARMIN_BRIDGE_URL`):
  - `POST /garmin/login` → bridge `POST /login`; returns the bridge's response
    (e.g. `{needs_mfa: true}`).
  - `POST /garmin/login/mfa` → bridge `POST /login/mfa` with the supplied code.
  - Thin reverse-proxy: forward status + body verbatim; no Garmin logic. Returns
    `503 garmin_disabled` when `GARMIN_BRIDGE_URL` is unset.
- **Two new MCP tools** (modify `mcp-server`): `garmin_login` and
  `garmin_submit_mfa` (takes the 6-digit `code`), each issuing exactly one HTTP
  call to the corresponding proxy endpoint and forwarding the body verbatim.
- **Config**: `GARMIN_BRIDGE_URL` (optional; absent ⇒ the proxy + tools report
  the feature off).

## Capabilities

### New Capabilities

- `garmin-control`: backend proxy endpoints that let an authenticated caller
  drive the bridge's interactive login (start + submit-MFA) without reaching the
  bridge's internal address directly. Pure forwarding — no Garmin or token logic.

### Modified Capabilities

- `mcp-server`: adds `garmin_login` and `garmin_submit_mfa` tools and bumps the
  expected-tools list.

## Impact

- **Code**: a small `internal/garmincontrol` package (handlers that forward to
  `GARMIN_BRIDGE_URL` via the http client) + `internal/httpserver` wiring;
  `internal/mcpserver` gains the two tools + expected-tools-list bump; `config`
  gains `GARMIN_BRIDGE_URL`.
- **Security**: the Garmin password never transits this path — it stays in the
  bridge's Secret. Only the ephemeral 6-digit MFA code passes through the agent.
  The minted token is persisted by the bridge to the backend (per
  `add-garmin-auth-token`); it never returns to the MCP caller.
- **Docs**: `task swag`; README MCP table gains the two tools; a note on the
  "re-link Garmin" chat flow.
- **Sequencing**: layered on `add-garmin-bridge` (the `/login` endpoints it
  proxies) and `add-garmin-auth-token` (the bridge persists the token there).
  The companion app's Settings could later hit the same two endpoints for a
  "Re-link Garmin" button — out of scope here.

### Out of scope (explicit non-goals)

- **Any Garmin protocol or token handling in the backend** — the proxy forwards
  bytes; the bridge owns the flow.
- **A companion-app re-link UI** — same endpoints, different front-end, later.
- **Storing or echoing credentials** — the password lives only in the bridge's
  Secret; this path never sees it.
