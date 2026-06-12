# garmin-auth Specification

## Purpose

Provide a durable, encrypted-at-rest, single-row store for the garmin-bridge's
opaque auth token blob, with read/write/clear scoped to a dedicated garmin
identity. Holds no Garmin protocol knowledge — the blob is stored and returned
verbatim.

## Requirements

### Requirement: An opaque garmin token blob is stored encrypted at rest

The system SHALL store a single garmin authentication token blob, supplied
verbatim by the garmin-bridge, encrypted at rest with a configured key
(`GARMIN_TOKEN_ENC_KEY`, AES-256-GCM). The blob is opaque: the system MUST NOT
parse, refresh, validate, or otherwise interpret its contents, and MUST return
it byte-identical on read. There SHALL be at most one stored blob (single user).
The plaintext blob MUST NOT appear in any log line.

#### Scenario: Store then read returns the blob verbatim

- **WHEN** the garmin client `PUT`s `/garmin/token` with a blob
- **THEN** the blob is encrypted and persisted (replacing any prior value)
- **AND** a subsequent `GET /garmin/token` returns the exact same bytes

#### Scenario: Reading when nothing is stored

- **WHEN** `GET /garmin/token` is called and no blob has been stored
- **THEN** the response is `404 garmin_token_not_found`

#### Scenario: Clearing forces re-login

- **WHEN** the garmin client `DELETE`s `/garmin/token`
- **THEN** the stored blob is removed
- **AND** a subsequent `GET` returns `404 garmin_token_not_found`

#### Scenario: At rest the blob is ciphertext, not plaintext

- **WHEN** the `garmin_tokens` row is inspected directly in the database
- **THEN** the stored bytes are the ciphertext, not the supplied blob
- **AND** decryption requires `GARMIN_TOKEN_ENC_KEY` (absent from the database)

### Requirement: The token endpoints are restricted to the garmin identity and gated on config

`POST/PUT`, `GET`, and `DELETE` on `/garmin/token` SHALL be authorized only for
the `garmin` client identity; the `mobile` and `agent` identities SHALL receive
`403 forbidden`. When `GARMIN_API_TOKEN` is not configured (the integration is
off), these endpoints SHALL return `503 garmin_disabled`.

#### Scenario: Garmin identity may manage the token

- **WHEN** a request to `/garmin/token` carries `Authorization: Bearer <GARMIN_API_TOKEN>`
- **THEN** the request is authorized and the operation proceeds

#### Scenario: Other identities are forbidden

- **WHEN** a request to `/garmin/token` carries the mobile or agent token
- **THEN** the response is `403 forbidden` and no read or write occurs

#### Scenario: Endpoints are off when the integration is unconfigured

- **WHEN** `GARMIN_API_TOKEN` is unset and any `/garmin/token` request arrives
- **THEN** the response is `503 garmin_disabled`
