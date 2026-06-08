## ADDED Requirements

### Requirement: rolling_summary tool wraps the rolling-summary endpoint

The MCP server SHALL expose one tool `rolling_summary` that invokes `GET /summary/rolling` with `Authorization: Bearer <AGENT_API_TOKEN>`, forwards the response body via `toToolResult`, and does NOT send an `Idempotency-Key` header (the endpoint is read-only). Inputs mirror the REST query parameters; outputs are passed through verbatim.

#### Scenario: rolling_summary calls GET /summary/rolling with the supplied window

- **WHEN** the agent calls `rolling_summary` with `{"anchor_date":"2026-06-08","window_days":7,"tz":"Europe/Berlin"}`
- **THEN** the wrapper issues `GET /summary/rolling?anchor_date=2026-06-08&window_days=7&tz=Europe/Berlin`
- **AND** does NOT include an `Idempotency-Key` header
- **AND** returns the REST `200` response body as the tool result content

#### Scenario: tz is omitted from the query when unset

- **WHEN** the agent calls `rolling_summary` without `tz`
- **THEN** the request URL does NOT include a `tz` query parameter
- **AND** the REST backend applies `DEFAULT_USER_TZ`

#### Scenario: Tool description names the divisor rule and typical windows

- **WHEN** the agent reads the `rolling_summary` tool description
- **THEN** the description states that the window is `[anchor − (window_days − 1) days, anchor]` (both inclusive) in the requested `tz`
- **AND** explicitly states that averages are computed across days with logged meals (`days_with_data`), NOT across `total_days`
- **AND** names typical windows: 3 (acute), 7 (weekly trend), 14 (training-block trend), 30 (block-length trend)
- **AND** notes that this is a read tool (no idempotency-key forwarded)

#### Scenario: REST 4xx errors are forwarded as isError

- **WHEN** the REST backend returns `400 window_days_invalid` (e.g. `window_days = 1`)
- **THEN** the tool result has `isError: true`
- **AND** the response body is the verbatim REST error payload

#### Scenario: Sparse-window response passes through verbatim

- **WHEN** the REST backend returns a `200 OK` response with `days_with_data: 2` and `total_days: 7`
- **THEN** the tool result content is the same JSON body byte-for-byte
- **AND** the wrapper does NOT add any "sparse window" warning or annotation — the agent surfaces sparsity to the user from the response fields
