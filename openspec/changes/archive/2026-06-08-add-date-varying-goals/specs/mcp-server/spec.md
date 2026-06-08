## ADDED Requirements

### Requirement: Daily goal override tools mirror the override REST endpoints

The MCP server SHALL expose four tools wrapping the new daily-goal-override REST surface: `set_daily_goal_override`, `get_daily_goal_override`, `delete_daily_goal_override`, and `list_daily_goal_overrides`. Each tool invokes its REST endpoint with `Authorization: Bearer <AGENT_API_TOKEN>` and forwards the REST response body as the tool's content via the existing `toToolResult` mapping. `set_daily_goal_override` follows the same posture as `set_goals`: the input schema does NOT expose `idempotency_key`, the wrapper never sends one, and the REST backend would reject it on PUT regardless.

#### Scenario: set_daily_goal_override calls PUT /goals/overrides/{date}

- **WHEN** the agent calls `set_daily_goal_override` with `{"date":"2026-06-15","goals":{"kcal":{"min":2280,"max":2520},"protein_g":{"min":160,"max":200}}}`
- **THEN** the wrapper issues `PUT /goals/overrides/2026-06-15` with the goals body
- **AND** does NOT include an `Idempotency-Key` header

#### Scenario: set_daily_goal_override input schema uses the unified Range shape

- **WHEN** the agent inspects the `set_daily_goal_override` tool input schema
- **THEN** every goal field uses the unified `{min?, max?}` Range shape (matching `set_goals`)
- **AND** there is no `kcal_target` property
- **AND** there is no `idempotency_key` property

#### Scenario: set_daily_goal_override description distinguishes from set_goals

- **WHEN** the agent reads the `set_daily_goal_override` tool description
- **THEN** the description states that the override is full-replace for that single date, replacing (not merging with) the default goals
- **AND** suggests typical use cases: training days, rest days, race weeks
- **AND** notes that retries are NOT safe (same constraint as set_goals)

#### Scenario: get_daily_goal_override calls GET /goals/overrides/{date}

- **WHEN** the agent calls `get_daily_goal_override` with `{"date":"2026-06-15"}`
- **THEN** the wrapper issues `GET /goals/overrides/2026-06-15`
- **AND** forwards a 404 with `{"error":"override_not_found"}` verbatim when no override exists

#### Scenario: delete_daily_goal_override calls DELETE /goals/overrides/{date}

- **WHEN** the agent calls `delete_daily_goal_override` with `{"date":"2026-06-15"}`
- **THEN** the wrapper issues `DELETE /goals/overrides/2026-06-15`
- **AND** on a 204 response, the tool result content is empty and `isError = false`
- **AND** uses the standard POST-style auto-derive idempotency rule on the DELETE

#### Scenario: list_daily_goal_overrides calls GET /goals/overrides with the range

- **WHEN** the agent calls `list_daily_goal_overrides` with `{"from":"2026-06-01","to":"2026-06-30"}`
- **THEN** the wrapper issues `GET /goals/overrides?from=2026-06-01&to=2026-06-30`
- **AND** does NOT include an `Idempotency-Key` header

#### Scenario: list_daily_goal_overrides description points at the audit use case

- **WHEN** the agent reads the `list_daily_goal_overrides` tool description
- **THEN** the description notes the typical use ("what's set for this week before I add more") and that dates without an override are omitted (the agent can infer they use the default)
