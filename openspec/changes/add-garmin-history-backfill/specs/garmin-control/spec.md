# garmin-control Specification (delta)

## ADDED Requirements

### Requirement: Backend proxy triggers a history backfill on the bridge

The system SHALL expose `POST /garmin/backfill` that forwards a `{from, to}` body to the garmin-bridge's `POST /sync/backfill` at `GARMIN_BRIDGE_URL`, returning the bridge's status code and body verbatim. The endpoint SHALL add no fields and parse nothing beyond passing the body through. When `GARMIN_BRIDGE_URL` is unset, the endpoint SHALL return `503 garmin_disabled`. The endpoint SHALL require authentication. Because a paced backfill can run longer than an interactive call, the proxy SHALL apply a timeout sufficient to cover a capped backfill rather than the short interactive-login timeout.

#### Scenario: Backfill trigger forwards to the bridge

- **WHEN** an authenticated client `POST`s `/garmin/backfill` with `{"from":"2026-03-01","to":"2026-04-30"}`
- **THEN** the backend forwards the body to the bridge's `POST /sync/backfill`
- **AND** returns the bridge's response verbatim (including the per-day summary and `days_total`/`days_ok`/`days_failed` roll-up)

#### Scenario: Partial-failure status is passed through

- **WHEN** the bridge completes the range with one or more failed days and returns `207`
- **THEN** the backend returns `207` with the bridge's body unchanged
- **AND** adds no interpretation of its own

#### Scenario: Disabled when the bridge URL is unset

- **WHEN** `GARMIN_BRIDGE_URL` is unset and `POST /garmin/backfill` is called
- **THEN** the response is `503 garmin_disabled`

#### Scenario: Unauthenticated callers are rejected

- **WHEN** an unauthenticated client calls `POST /garmin/backfill`
- **THEN** the request is rejected by the auth middleware before any forward to the bridge
