## MODIFIED Requirements

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
