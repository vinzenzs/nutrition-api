## ADDED Requirements

### Requirement: Race-prep tool wraps the carb-load endpoint

The MCP server SHALL expose one tool, `plan_carb_load`, wrapping the new `GET /race-prep/carb-load` REST endpoint. The tool invokes the endpoint with `Authorization: Bearer <AGENT_API_TOKEN>` and forwards the REST response body as the tool's content via the existing `toToolResult` mapping. The tool is read-only: the wrapper does not send an `Idempotency-Key` header, and the input schema does not expose an `idempotency_key` property.

#### Scenario: plan_carb_load calls GET /race-prep/carb-load with the supplied params

- **WHEN** the agent calls `plan_carb_load` with `{"race_date":"2026-07-24","body_weight_kg":70}`
- **THEN** the wrapper issues `GET /race-prep/carb-load?race_date=2026-07-24&body_weight_kg=70`
- **AND** does NOT include an `Idempotency-Key` header
- **AND** forwards the response body verbatim to the tool result

#### Scenario: Optional parameters are passed when supplied

- **WHEN** the agent calls `plan_carb_load` with `{"race_date":"2026-07-24","body_weight_kg":70,"days_before":2,"carbs_per_kg_per_day":8,"race_day_carbs_per_kg":2.5}`
- **THEN** the wrapper appends `days_before=2&carbs_per_kg_per_day=8&race_day_carbs_per_kg=2.5` to the query string
- **AND** does NOT include optional params that were not supplied

#### Scenario: Validation errors from the endpoint are forwarded verbatim

- **WHEN** the REST endpoint returns `400 {"error":"body_weight_kg_invalid","range":{"min":30,"max":200}}`
- **THEN** the wrapper forwards the response body verbatim
- **AND** the tool result has `isError = true`

#### Scenario: plan_carb_load input schema reflects the parameter contract

- **WHEN** the agent inspects the `plan_carb_load` tool input schema
- **THEN** `race_date` and `body_weight_kg` are required
- **AND** `days_before`, `carbs_per_kg_per_day`, and `race_day_carbs_per_kg` are optional
- **AND** there is no `idempotency_key` property

#### Scenario: plan_carb_load description points the agent at the override workflow

- **WHEN** the agent reads the `plan_carb_load` tool description
- **THEN** the description names the natural follow-up: translating each schedule entry into a goal override via `set_daily_goal_override`
- **AND** notes typical `days_before` values per race distance (sprint: 1-2, 70.3: 3, Ironman: 3-4)
- **AND** notes that `carbs_per_kg_per_day` defaults sit in the documented 8-12 g/kg range, lower for athletes with GI sensitivity
