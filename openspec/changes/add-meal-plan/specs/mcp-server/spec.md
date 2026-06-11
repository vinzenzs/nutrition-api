# mcp-server — delta for add-meal-plan

## ADDED Requirements

### Requirement: Meal-plan tools mirror the plan REST endpoints

The MCP server SHALL expose `create_planned_meal`, `list_planned_meals`, `update_planned_meal`, `delete_planned_meal`, and `mark_planned_meal_eaten`, each issuing exactly one HTTP call to the corresponding `/plan` endpoint and forwarding the response body verbatim. Write tools SHALL auto-derive idempotency keys when the agent supplies none. `list_planned_meals` takes `{from, to}`. The `mark_planned_meal_eaten` description SHALL state that it logs a real meal entry now and is the only correct way to record a planned meal as eaten.

#### Scenario: Plan creation flows through one HTTP call

- **WHEN** the agent calls `create_planned_meal` with `{plan_date, slot, product_id, quantity_g}`
- **THEN** the MCP server issues `POST /plan` with that body and an auto-derived idempotency key
- **AND** the tool result is the REST response body verbatim

#### Scenario: Eaten conflict surfaces as isError

- **WHEN** the agent calls `mark_planned_meal_eaten` on an already-eaten entry
- **THEN** the tool result has `isError=true` carrying the `409 plan_entry_already_eaten` body
