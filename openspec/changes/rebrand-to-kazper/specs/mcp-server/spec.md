## ADDED Requirements

### Requirement: The server and API identify as Kazper

The MCP server SHALL announce its name as **kazper** (rather than "nutrition") so the agent runtime lists it under the product identity, and the REST API's OpenAPI title SHALL be **Kazper**. This is an identity/branding change only — no tool names, schemas, routes, idempotency, or error behavior change.

#### Scenario: The MCP server announces the Kazper identity

- **WHEN** the agent runtime connects and reads the server info
- **THEN** the announced server name is `kazper`
- **AND** the announced tool surface is otherwise unchanged

#### Scenario: The API documents itself as Kazper

- **WHEN** the generated OpenAPI document is served
- **THEN** its title is `Kazper`
