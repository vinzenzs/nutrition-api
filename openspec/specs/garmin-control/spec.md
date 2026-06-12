# garmin-control Specification

## Purpose

Expose authenticated backend proxy endpoints that drive the garmin-bridge's
interactive multi-factor login from the nutrition API's own surface. The backend
forwards login and MFA requests to the bridge verbatim, carrying no credentials
of its own (the bridge holds them in its configuration) and surfacing nothing
sensitive — so the LLM coaching agent and mobile client can trigger a Garmin
re-authentication through the same authenticated API they use for everything
else, without ever touching the Garmin password or token blob.

## Requirements

### Requirement: Backend proxy endpoints drive the bridge's interactive login

The system SHALL expose `POST /garmin/login` and `POST /garmin/login/mfa` that
forward to the garmin-bridge at `GARMIN_BRIDGE_URL`, returning the bridge's
status code and body verbatim. The endpoints SHALL add no fields and parse
nothing. `POST /garmin/login` SHALL carry no credentials (the bridge reads them
from its own configuration); `POST /garmin/login/mfa` SHALL forward the supplied
6-digit code. When `GARMIN_BRIDGE_URL` is unset, the endpoints SHALL return
`503 garmin_disabled`. The endpoints SHALL require authentication.

#### Scenario: Start login forwards to the bridge

- **WHEN** an authenticated client `POST`s `/garmin/login`
- **THEN** the backend forwards the call to the bridge's `/login`
- **AND** returns the bridge's response verbatim (e.g. `{"needs_mfa": true}`)
- **AND** sends no credentials in the forwarded request

#### Scenario: Submit MFA forwards the code

- **WHEN** an authenticated client `POST`s `/garmin/login/mfa` with `{"code":"418923"}`
- **THEN** the backend forwards the code to the bridge's `/login/mfa`
- **AND** returns the bridge's success/error response verbatim

#### Scenario: Disabled when the bridge URL is unset

- **WHEN** `GARMIN_BRIDGE_URL` is unset and either endpoint is called
- **THEN** the response is `503 garmin_disabled`

#### Scenario: The password and token never appear on this path

- **WHEN** any login proxy request or response is logged
- **THEN** no Garmin password or token blob appears (only the bridge's
  non-sensitive status; the password lives solely in the bridge's secret)
