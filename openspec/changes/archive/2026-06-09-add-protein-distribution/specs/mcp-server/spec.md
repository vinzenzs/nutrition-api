## ADDED Requirements

### Requirement: protein_distribution tool wraps the protein-distribution endpoint

The MCP server SHALL expose one tool `protein_distribution` that invokes `GET /summary/protein-distribution` with `Authorization: Bearer <AGENT_API_TOKEN>`, forwards the response body via `toToolResult`, and does NOT send an `Idempotency-Key` header (the endpoint is read-only). Inputs mirror the REST query parameters; outputs are passed through verbatim.

#### Scenario: protein_distribution calls GET /summary/protein-distribution with the supplied parameters

- **WHEN** the agent calls `protein_distribution` with `{"date":"2026-06-09","tz":"Europe/Berlin","body_weight_kg":72.5}`
- **THEN** the wrapper issues `GET /summary/protein-distribution?date=2026-06-09&tz=Europe/Berlin&body_weight_kg=72.5`
- **AND** does NOT include an `Idempotency-Key` header
- **AND** returns the REST `200` response body as the tool result content

#### Scenario: tz and body_weight_kg are omitted when unset

- **WHEN** the agent calls `protein_distribution` with only `date` set
- **THEN** the request URL does NOT include `tz` or `body_weight_kg` query parameters
- **AND** the REST backend applies `DEFAULT_USER_TZ` and the stored body-weight resolution

#### Scenario: Tool description names the MPS rule and resolution order

- **WHEN** the agent reads the `protein_distribution` tool description
- **THEN** the description states the MPS-per-meal rule (~0.3 g protein per kg body weight per meal) and that the response's `mps_effective_meal_count / meal_count` is the headline metric
- **AND** notes the gap heuristic (3–5h sweet spot — meals too close together are not independent triggers; gaps over 5h close the MPS window)
- **AND** explains the body-weight resolution order (explicit `body_weight_kg` > rolling 7-day average of stored weights > most-recent stored weight before the date)
- **AND** notes that this is a read tool (no idempotency-key forwarded)

#### Scenario: REST 4xx errors are forwarded as isError

- **WHEN** the REST backend returns `400 weight_data_missing`
- **THEN** the tool result has `isError: true`
- **AND** the response body is the verbatim REST error payload
