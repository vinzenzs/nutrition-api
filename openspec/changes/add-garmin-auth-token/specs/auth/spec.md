# auth — delta for add-garmin-auth-token

## MODIFIED Requirements

### Requirement: Bearer token authentication with two static tokens

The system SHALL require every request to carry an `Authorization: Bearer <token>` header where `<token>` matches one of the env-configured static tokens, and SHALL reject all other requests with `401 Unauthorized`. There are two required identities — `MOBILE_API_TOKEN` (`client_id = "mobile"`) and `AGENT_API_TOKEN` (`client_id = "agent"`) — and one OPTIONAL identity, `GARMIN_API_TOKEN` (`client_id = "garmin"`), which is recognized only when configured.

#### Scenario: Mobile token is accepted

- **WHEN** a request includes `Authorization: Bearer <value-of-MOBILE_API_TOKEN>`
- **THEN** the request proceeds to the handler
- **AND** request context contains `client_id = "mobile"`

#### Scenario: Agent token is accepted

- **WHEN** a request includes `Authorization: Bearer <value-of-AGENT_API_TOKEN>`
- **THEN** the request proceeds to the handler
- **AND** request context contains `client_id = "agent"`

#### Scenario: Garmin token is accepted when configured

- **WHEN** `GARMIN_API_TOKEN` is set and a request includes `Authorization: Bearer <value-of-GARMIN_API_TOKEN>`
- **THEN** the request proceeds to the handler
- **AND** request context contains `client_id = "garmin"`

#### Scenario: Garmin token is not recognized when unset

- **WHEN** `GARMIN_API_TOKEN` is unset and a request presents some bearer value as the garmin token
- **THEN** the system returns `401 Unauthorized` with `{"error":"auth_invalid"}` (no garmin identity exists)

#### Scenario: Missing Authorization header is rejected

- **WHEN** a request has no `Authorization` header
- **THEN** the system returns `401 Unauthorized` with `{"error":"auth_required"}`

#### Scenario: Wrong scheme is rejected

- **WHEN** a request has `Authorization: Basic ...` or any non-Bearer scheme
- **THEN** the system returns `401 Unauthorized` with `{"error":"auth_required"}`

#### Scenario: Unknown bearer token is rejected

- **WHEN** a request has `Authorization: Bearer <something>` where `<something>` matches no configured token
- **THEN** the system returns `401 Unauthorized` with `{"error":"auth_invalid"}`

#### Scenario: Tokens are not logged in plaintext

- **WHEN** any request is logged (success or failure)
- **THEN** the log line does not contain the raw token value
- **AND** the log line contains the resolved `client_id` for successful auths

### Requirement: Token startup validation

The system SHALL refuse to start if either `MOBILE_API_TOKEN` or `AGENT_API_TOKEN` is unset, empty, or shorter than 16 bytes. When `GARMIN_API_TOKEN` is set, it MUST also be at least 16 bytes and MUST differ from both other tokens; when unset, it imposes no startup constraint.

#### Scenario: Missing env vars halt startup

- **WHEN** the process starts with `MOBILE_API_TOKEN` unset
- **THEN** startup fails with an error mentioning the missing variable
- **AND** the HTTP server does not begin listening

#### Scenario: Identical tokens halt startup

- **WHEN** `MOBILE_API_TOKEN` and `AGENT_API_TOKEN` are set to the same value
- **THEN** startup fails with an error stating the two tokens must differ

#### Scenario: Garmin token sharing a value halts startup

- **WHEN** `GARMIN_API_TOKEN` is set equal to the mobile or agent token
- **THEN** startup fails with an error stating the tokens must differ

#### Scenario: Unset garmin token does not affect startup

- **WHEN** `GARMIN_API_TOKEN` is unset but the two required tokens are valid and distinct
- **THEN** startup succeeds and the garmin identity is simply unavailable
