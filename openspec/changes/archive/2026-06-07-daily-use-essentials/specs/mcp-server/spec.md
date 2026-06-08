## ADDED Requirements

### Requirement: Recipe and goals tools mirror new REST endpoints

The MCP server SHALL expose four new tools that map onto the REST endpoints added by the daily-use-essentials change. Each tool invokes its REST endpoint with `Authorization: Bearer <AGENT_API_TOKEN>` and forwards the REST response body as the tool's content, using the same success/error mapping as the existing eight tools.

#### Scenario: create_recipe tool calls the recipe creation endpoint

- **WHEN** the agent calls `create_recipe` with `{"name":"Morning skyr bowl","components":[{"product_id":"<id>","quantity_g":200}],"serving_size_g":250}`
- **THEN** the wrapper issues `POST /products/recipes` with the body
- **AND** returns the REST 201 response body as the tool result content

#### Scenario: create_recipe forwards an explicit idempotency key

- **WHEN** the agent supplies `idempotency_key` in the input
- **THEN** the wrapper sets `Idempotency-Key: <value>` on the REST request

#### Scenario: create_recipe derives an idempotency key when omitted

- **WHEN** the agent does not supply `idempotency_key`
- **THEN** the wrapper computes a deterministic key from the tool name and canonicalized input
- **AND** sets `Idempotency-Key` on the REST request to the derived value

#### Scenario: recompute_recipe tool calls the recompute endpoint

- **WHEN** the agent calls `recompute_recipe` with `{"product_id":"<id>"}`
- **THEN** the wrapper issues `POST /products/recipes/<id>/recompute`
- **AND** returns the REST response body as the tool result content

#### Scenario: get_goals tool calls the goals endpoint

- **WHEN** the agent calls `get_goals` with `{}`
- **THEN** the wrapper issues `GET /goals`
- **AND** returns the REST response body verbatim (including `{"goals": null}` when unset)

#### Scenario: set_goals tool calls the PUT goals endpoint

- **WHEN** the agent calls `set_goals` with a body containing any goals fields
- **THEN** the wrapper issues `PUT /goals` with that body
- **AND** returns the REST response body as the tool result content

#### Scenario: set_goals supports idempotency-key forwarding and derivation

- **WHEN** `set_goals` is called with or without `idempotency_key`
- **THEN** the wrapper applies the same idempotency rules as other write tools (explicit key wins; derived key otherwise)

### Requirement: Existing summary and freeform tools accept new optional parameters

The pre-existing `daily_summary`, `range_summary`, and `log_meal_freeform` tools SHALL forward new optional parameters added by the REST changes, without breaking input compatibility for agents that ignore them.

#### Scenario: daily_summary forwards meal_type when present

- **WHEN** the agent calls `daily_summary` with `{"date":"2026-06-06","meal_type":"breakfast"}`
- **THEN** the wrapper issues `GET /summary/daily?date=2026-06-06&meal_type=breakfast`

#### Scenario: daily_summary omits meal_type when not supplied

- **WHEN** the agent calls `daily_summary` without `meal_type`
- **THEN** the wrapper does not include `meal_type` in the query string

#### Scenario: range_summary forwards group_by when present

- **WHEN** the agent calls `range_summary` with `{"from":"2026-06-01","to":"2026-06-07","group_by":"meal_type"}`
- **THEN** the wrapper issues `GET /summary/range?from=2026-06-01&to=2026-06-07&group_by=meal_type`

#### Scenario: log_meal_freeform accepts micros in nutriments_per_100g

- **WHEN** the agent calls `log_meal_freeform` with `nutriments_per_100g` that includes any of `iron_mg`, `calcium_mg`, `vitamin_d_mcg`, `vitamin_b12_mcg`, `vitamin_c_mg`, `magnesium_mg`, `potassium_mg`, or `zinc_mg`
- **THEN** the wrapper forwards those fields verbatim in the REST request body
- **AND** the tool's input schema documents the micros as optional alongside macros

### Requirement: New tool descriptions guide the agent toward composite logging

The tool descriptions on `log_meal` and `log_meal_freeform` SHALL be extended to mention recipes as the preferred path for multi-ingredient meals, and `search_products` description SHALL note that recipe products appear in results just like OFF and manual products.

#### Scenario: log_meal_freeform description mentions create_recipe

- **WHEN** the agent enumerates tools via `tools/list`
- **THEN** the `log_meal_freeform` description includes a sentence pointing to `create_recipe` for "meals you eat repeatedly that have 2+ ingredients"

#### Scenario: search_products description mentions recipes

- **WHEN** the agent enumerates tools
- **THEN** the `search_products` description notes that results include products with `source` of `off`, `manual`, or `recipe`
