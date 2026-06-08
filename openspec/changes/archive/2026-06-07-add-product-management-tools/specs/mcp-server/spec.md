## ADDED Requirements

### Requirement: Product management tools mirror the cleanup REST endpoints

The MCP server SHALL expose two new tools — `list_products` and `delete_product` — that wrap the new `GET /products` and `DELETE /products/{id}` REST endpoints. Each tool invokes its endpoint with `Authorization: Bearer <AGENT_API_TOKEN>` and forwards the REST response body as the tool's content, using the same success/error mapping as the existing tools.

#### Scenario: list_products tool calls the GET /products endpoint

- **WHEN** the agent calls `list_products` with `{}` (no filters)
- **THEN** the wrapper issues `GET /products`
- **AND** returns the REST 200 response body as the tool result content

#### Scenario: list_products forwards source filter and pagination

- **WHEN** the agent calls `list_products` with `{"source":"manual","limit":20,"offset":40}`
- **THEN** the wrapper issues `GET /products?source=manual&limit=20&offset=40`

#### Scenario: list_products is read-only and never sends Idempotency-Key

- **WHEN** the agent calls `list_products`
- **THEN** the REST request does NOT include an `Idempotency-Key` header
- **AND** the wrapper does not consult or store any idempotency record

#### Scenario: delete_product tool calls the DELETE /products/{id} endpoint

- **WHEN** the agent calls `delete_product` with `{"product_id":"<uuid>"}`
- **THEN** the wrapper issues `DELETE /products/<uuid>`
- **AND** returns the REST response body as the tool result content
- **AND** on a 204 response, the tool result content is an empty body and `isError = false`

#### Scenario: delete_product surfaces the 409 in-use body verbatim

- **WHEN** the REST API responds with `409 product_in_use_as_component` and the body lists the using recipes
- **THEN** the tool result has `isError = true`
- **AND** the content payload contains the JSON body unchanged, including the `recipes` array and the `hint` field

#### Scenario: delete_product auto-derives an idempotency key when omitted

- **WHEN** the agent calls `delete_product` twice in a row with the same `product_id` and no explicit `idempotency_key`
- **THEN** both REST requests carry the same auto-derived `Idempotency-Key` header
- **AND** the second call's result reflects the backend's response to a duplicate delete (either 204 from the cached response, or 404 if the cache TTL elapsed and the row is genuinely gone)

#### Scenario: delete_product description guides the agent on the 409 path

- **WHEN** the agent reads the `delete_product` tool description
- **THEN** the description notes that products used as recipe components produce a 409 with the using recipes listed
- **AND** instructs the agent to delete or replace the listed recipes before retrying

#### Scenario: list_products description points at the cleanup pattern

- **WHEN** the agent reads the `list_products` tool description
- **THEN** the description notes the recency ordering and the source filter
- **AND** mentions that combining list_products + delete_product is the standard way to clean up leftover products from prior sessions
