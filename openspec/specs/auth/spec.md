# auth Specification

## Purpose

Define authentication and idempotency requirements for the nutrition API.

## Requirements

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

### Requirement: Idempotency-Key header on write endpoints

The system SHALL accept an optional `Idempotency-Key` header on every `POST`, `PATCH`, and `DELETE` endpoint, store the request fingerprint and response, and replay the stored response on subsequent requests with the same key within a configurable TTL. The system SHALL NOT accept the header on `PUT` requests; a PUT carrying the header is rejected with `400 Bad Request` so the caller does not silently rely on a replay semantic that does not apply to state-replacing writes.

#### Scenario: First request with a key runs and stores the response

- **WHEN** a write request arrives with `Idempotency-Key: abc123` and no prior record exists
- **THEN** the system runs the handler
- **AND** stores `(client_id, method, path, key) → (status, response_body, request_body_hash, created_at)`
- **AND** returns the handler's response to the client

#### Scenario: Replay with the same key returns the stored response

- **WHEN** a second request arrives within `IDEMPOTENCY_TTL_HOURS` (default 24) with the same `(client_id, method, path, Idempotency-Key)` and a request body whose hash matches the stored fingerprint
- **THEN** the system returns the stored status and response body
- **AND** does NOT run the handler again

#### Scenario: Same key with different request body returns 409

- **WHEN** a request arrives with an `Idempotency-Key` that matches a stored record but whose request body hash differs from the stored fingerprint
- **THEN** the system returns `409 Conflict` with `{"error":"idempotency_key_conflict"}`
- **AND** does not run the handler
- **AND** does not modify the stored record

#### Scenario: Different clients sharing a key are isolated

- **WHEN** two requests arrive with the same `Idempotency-Key` value but from different `client_id`s
- **THEN** each is treated as an independent record
- **AND** neither replays the other's response

#### Scenario: Records older than TTL are purged

- **WHEN** an `idempotency_records` row's `created_at` is older than `IDEMPOTENCY_TTL_HOURS`
- **THEN** the cleanup routine removes the row
- **AND** a subsequent request with the same key is treated as first-arrival

#### Scenario: Idempotency-Key on GET requests is ignored

- **WHEN** a `GET` request includes an `Idempotency-Key` header
- **THEN** the system processes the request normally
- **AND** does not store or consult the idempotency table

#### Scenario: Idempotency-Key on PUT requests is rejected

- **WHEN** a `PUT` request includes a non-empty `Idempotency-Key` header
- **THEN** the system returns `400 Bad Request`
- **AND** the response body is `{"error":"idempotency_unsupported_for_put","hint":"use If-Match with ETag for retry-safety"}`
- **AND** the handler is NOT run
- **AND** the idempotency table is not consulted or modified

#### Scenario: PUT without the header proceeds normally

- **WHEN** a `PUT` request arrives without an `Idempotency-Key` header (or with an empty one)
- **THEN** the system processes the request normally
- **AND** does not consult or modify the idempotency table

### Requirement: Auth precedes idempotency

The system SHALL run authentication before idempotency replay so that a leaked idempotency key cannot be replayed without valid credentials.

#### Scenario: Replay without valid auth is rejected at 401

- **WHEN** a request carries a valid `Idempotency-Key` for a previously-stored record but no valid `Authorization` header
- **THEN** the system returns `401 Unauthorized`
- **AND** does not return the stored response body
