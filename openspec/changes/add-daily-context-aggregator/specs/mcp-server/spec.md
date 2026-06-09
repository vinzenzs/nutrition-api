## ADDED Requirements

### Requirement: Daily context tool wraps the aggregator endpoint

The MCP server SHALL expose one tool, `daily_context`, wrapping `GET /context/daily`. The tool invokes the endpoint with `Authorization: Bearer <AGENT_API_TOKEN>` and forwards the REST response body verbatim. The tool is read-only: the wrapper does NOT send an `Idempotency-Key` header, and the input schema does NOT expose an `idempotency_key` property. Required input: `date`. Optional input: `tz`.

#### Scenario: daily_context calls GET /context/daily with the supplied params

- **WHEN** the agent calls `daily_context` with `{"date":"2026-07-15"}`
- **THEN** the wrapper issues `GET /context/daily?date=2026-07-15`
- **AND** does NOT include an `Idempotency-Key` header
- **AND** forwards the response body verbatim to the tool result

#### Scenario: Optional tz is passed when supplied

- **WHEN** the agent calls `daily_context` with `{"date":"2026-07-15","tz":"Europe/Berlin"}`
- **THEN** the wrapper issues `GET /context/daily?date=2026-07-15&tz=Europe/Berlin`

#### Scenario: Optional tz is omitted when not supplied

- **WHEN** the agent calls `daily_context` with `{"date":"2026-07-15"}` (no `tz`)
- **THEN** the wrapper issues `GET /context/daily?date=2026-07-15` (no `tz` query param)
- **AND** the REST endpoint's `DEFAULT_USER_TZ` fallback determines the timezone

#### Scenario: Validation errors from the endpoint are forwarded verbatim

- **WHEN** the REST endpoint returns `400 {"error":"date_invalid"}`
- **THEN** the wrapper forwards the response body verbatim
- **AND** the tool result has `isError = true`

#### Scenario: daily_context input schema reflects the parameter contract

- **WHEN** the agent inspects the `daily_context` tool input schema
- **THEN** `date` is required
- **AND** `tz` is optional
- **AND** there is no `idempotency_key` property

#### Scenario: daily_context description names the pattern

- **WHEN** the agent reads the `daily_context` tool description
- **THEN** the description recommends `daily_context` as the first tool of a session — one call returns adherence, totals, hydration, today's workouts, fuel entries, weight state, training phase, and goal-override presence — collapsing what would otherwise be 5-7 separate tool calls
- **AND** notes that for deep dives into one slice (per-entry breakdowns, full meal lists, range queries) the dedicated tools (`daily_summary`, `list_workouts`, `list_workout_fuel`, `list_hydration`, etc.) are the right tool — they include per-entry detail the aggregator deliberately omits

#### Scenario: Tool count integration test is updated

- **WHEN** the MCP integration test (`mcp_integration_test.go`) enumerates exposed tools
- **THEN** the expected-tools assertion includes the new tool name `daily_context`
