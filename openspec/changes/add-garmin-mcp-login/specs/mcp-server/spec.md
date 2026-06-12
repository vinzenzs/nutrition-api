# mcp-server — delta for add-garmin-mcp-login

## ADDED Requirements

### Requirement: Garmin login tools drive re-authentication from the agent

The MCP server SHALL expose `garmin_login` and `garmin_submit_mfa`, each issuing
exactly one HTTP call to the corresponding backend proxy endpoint and forwarding
the response body verbatim. `garmin_login` takes no arguments (the bridge holds
the credentials) and issues `POST /garmin/login`; `garmin_submit_mfa` takes a
single `code` and issues `POST /garmin/login/mfa`. The `garmin_login` tool
description SHALL instruct the agent to relay the `needs_mfa` result by asking the
user for the code from their authenticator, then call `garmin_submit_mfa`. The
expected-tools integration list SHALL include both.

#### Scenario: garmin_login starts the flow in one call

- **WHEN** the agent calls `garmin_login`
- **THEN** the MCP server issues `POST /garmin/login`
- **AND** the tool result is the proxy response verbatim (e.g. `{"needs_mfa":true}`)
- **AND** no credentials are sent in the tool call

#### Scenario: garmin_submit_mfa forwards the code

- **WHEN** the agent calls `garmin_submit_mfa` with `{"code":"418923"}`
- **THEN** the MCP server issues `POST /garmin/login/mfa` with that code
- **AND** the tool result is the proxy response verbatim

#### Scenario: Disabled bridge surfaces as a tool error

- **WHEN** `GARMIN_BRIDGE_URL` is unset and the agent calls either tool
- **THEN** the tool result carries the `503 garmin_disabled` body with `isError=true`

#### Scenario: Expected-tools list includes the login tools

- **WHEN** the MCP integration test enumerates registered tools
- **THEN** `garmin_login` and `garmin_submit_mfa` are both present
