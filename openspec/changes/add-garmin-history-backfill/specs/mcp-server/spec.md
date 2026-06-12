# mcp-server Specification (delta)

## ADDED Requirements

### Requirement: garmin_backfill tool triggers a bounded history backfill

The MCP server SHALL expose one tool `garmin_backfill` that wraps `POST /garmin/backfill`. The tool takes `from` and `to` (`YYYY-MM-DD`, both required) plus an optional `idempotency_key`. It SHALL issue exactly one HTTP call to the backend with `Authorization: Bearer <AGENT_API_TOKEN>` and forward the backend's response body verbatim via `toToolResult`. As a POST-style write tool it SHALL set an `Idempotency-Key` header: the explicit `idempotency_key` when supplied, otherwise an auto-derived stable key `sha256_hex("garmin_backfill|" + canonical_json(<args_without_idempotency_key>))`, per the existing write-tool convention. A `503 garmin_disabled` response from the backend (bridge unconfigured) SHALL be forwarded verbatim with `isError = true`.

#### Scenario: garmin_backfill calls POST /garmin/backfill

- **WHEN** the agent calls `garmin_backfill` with `{"from":"2026-03-01","to":"2026-04-30"}`
- **THEN** the wrapper issues `POST /garmin/backfill` with body `{"from":"2026-03-01","to":"2026-04-30"}`
- **AND** sets `Idempotency-Key` to the agent's explicit key (if any) or to the derived stable key
- **AND** returns the backend response body (per-day summary plus roll-up) as the tool result content

#### Scenario: Explicit idempotency_key is forwarded verbatim

- **WHEN** the agent calls `garmin_backfill` with `{"from":"2026-03-01","to":"2026-04-30","idempotency_key":"backfill-spring-block"}`
- **THEN** the REST request carries `Idempotency-Key: backfill-spring-block`
- **AND** the derived-key formula is NOT used (the explicit key wins)

#### Scenario: Partial-failure body passes through as a non-error result envelope

- **WHEN** the backend returns `207` with a body whose roll-up reports `days_failed` ≥ 1
- **THEN** the wrapper forwards the body verbatim
- **AND** the agent can read `days_failed` and the per-day entries to decide which dates to re-issue

#### Scenario: garmin_disabled is surfaced as an error

- **WHEN** the backend returns `503 {"error":"garmin_disabled"}` because `GARMIN_BRIDGE_URL` is unset
- **THEN** the tool result has `isError = true`
- **AND** the content payload contains the JSON body unchanged

#### Scenario: Tool description explains the bounded, paced, resumable semantics

- **WHEN** the agent reads the `garmin_backfill` tool description
- **THEN** the description states that the call re-syncs an inclusive historical date range to backfill detail that the rolling daily sync no longer reaches
- **AND** notes that the range is capped, paced between days, and may take a while
- **AND** notes that re-runs are safe (idempotent) so failed dates can simply be re-issued
