# Tasks: add-garmin-mcp-login

## 1. Backend proxy

- [x] 1.1 `internal/config`: add `GARMIN_BRIDGE_URL` (optional)
- [x] 1.2 `internal/garmincontrol`: handlers `POST /garmin/login` and `POST /garmin/login/mfa` that forward to `GARMIN_BRIDGE_URL` (http client, bounded timeout), returning the bridge's status + body verbatim; `503 garmin_disabled` when unset
- [x] 1.3 Ensure the proxy logger does not log credentials/token (forward-only; nothing sensitive originates here, but redact bodies to be safe)
- [x] 1.4 Wire in `internal/httpserver` behind auth (any identity); swag annotations
- [x] 1.5 Handler tests: forwards to a stub bridge (verifies path + verbatim passthrough of status/body); 503 when `GARMIN_BRIDGE_URL` unset; login carries no credentials

## 2. MCP tools

- [x] 2.1 `internal/mcpserver`: `garmin_login` (no args → `POST /garmin/login`) and `garmin_submit_mfa` (`code` → `POST /garmin/login/mfa`), forwarding bodies verbatim; descriptions per spec (relay needs_mfa → ask for code → submit)
- [x] 2.2 Bump the expected-tools list in `mcp_integration_test.go`; wrapper tests for path/body construction and passthrough

## 3. Docs + verification

- [x] 3.1 `task swag`; README MCP table gains `garmin_login` + `garmin_submit_mfa`; a note on the "re-link Garmin from chat" flow
- [x] 3.2 `task vet` + `task test` green; `openspec validate add-garmin-mcp-login --strict` passes
