## MODIFIED Requirements

### Requirement: Recipe and goals tools mirror new REST endpoints

The MCP server SHALL expose four new tools that map onto the REST endpoints added by the daily-use-essentials change. Each tool invokes its REST endpoint with `Authorization: Bearer <AGENT_API_TOKEN>` and forwards the REST response body as the tool's content, using the same success/error mapping as the existing eight tools. The `set_goals` tool's input schema uses the unified `{min?, max?}` Range shape for every goal field, including `kcal` (the legacy `kcal_target` scalar field is not exposed).

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

- **WHEN** the agent calls `set_goals` with a body like `{"kcal":{"min":2090,"max":2310},"protein_g":{"min":150,"max":190},"fiber_g":{"min":30},"sugar_g":{"max":50}}`
- **THEN** the wrapper issues `PUT /goals` with that body
- **AND** returns the REST response body as the tool result content

#### Scenario: set_goals input schema uses the unified Range shape

- **WHEN** the agent inspects the `set_goals` tool input schema
- **THEN** every goal field has the shape `{min?: number, max?: number}` (both bounds optional, at least one MUST be present per documented field)
- **AND** `kcal` appears under that shape
- **AND** there is no `kcal_target` property

#### Scenario: set_goals tool description guides the agent to construct ranges

- **WHEN** the agent reads the `set_goals` tool description
- **THEN** the description includes one sentence on how to convert a user-stated "I want N kcal a day" into an explicit `kcal: {min: N×0.95, max: N×1.05}` (or whatever tolerance the user implies)
- **AND** notes that single-bound goals (e.g. `fiber_g: {min: 30}`) are valid

#### Scenario: set_goals supports idempotency-key forwarding and derivation

- **WHEN** `set_goals` is called with or without `idempotency_key`
- **THEN** the wrapper applies the same idempotency rules as other write tools (explicit key wins; derived key otherwise)
