# mcp-server — delta for add-recipe-ingredients

## ADDED Requirements

### Requirement: import_cookidoo_recipe tool wraps the server-side import endpoint

The MCP server SHALL expose an `import_cookidoo_recipe` tool taking `{url, serving_size_g?}` that issues exactly one HTTP call to `POST /products/import/cookidoo` and forwards the response body verbatim. As a write tool it SHALL auto-derive an idempotency key when the agent does not supply one. The tool description SHALL state that omitting `serving_size_g` creates the recipe without nutriments and that the agent should estimate serving mass from the returned ingredients and follow up via the product update tool.

#### Scenario: Tool forwards the import response verbatim

- **WHEN** the agent calls `import_cookidoo_recipe` with a valid Cookidoo URL and `serving_size_g`
- **THEN** the MCP server issues `POST /products/import/cookidoo` with that JSON body
- **AND** the tool result is the REST response body verbatim

#### Scenario: REST errors surface as isError tool results

- **WHEN** the REST endpoint returns `502 cookidoo_unavailable`
- **THEN** the tool result has `isError=true` and carries the REST error body
