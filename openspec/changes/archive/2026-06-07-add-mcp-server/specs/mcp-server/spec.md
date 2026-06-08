## ADDED Requirements

### Requirement: MCP server runs over stdio with a tool-only surface

The system SHALL provide an MCP server binary at `cmd/mcp/` that communicates with the agent runtime over stdio using JSON-RPC, exposing only tools (no resources or prompts in v1).

#### Scenario: Binary starts and registers tools on stdin connection

- **WHEN** the agent runtime spawns the MCP binary and opens stdin
- **THEN** the binary registers the eight tools defined in this spec and announces them on the MCP `initialize` exchange
- **AND** the binary remains running, processing JSON-RPC requests over stdin

#### Scenario: Binary exits on stdin EOF or SIGTERM

- **WHEN** the agent runtime closes stdin or sends SIGTERM
- **THEN** the binary stops accepting new requests
- **AND** exits with status 0 within 2 seconds

#### Scenario: Binary refuses to start without an agent token

- **WHEN** the process starts without `AGENT_API_TOKEN` set or set to an empty string
- **THEN** the binary writes an error to stderr identifying the missing variable
- **AND** exits with a non-zero status code before announcing any tools

#### Scenario: Binary smoke-checks the REST API before announcing tools

- **WHEN** the process starts
- **THEN** the binary issues `GET <NUTRITION_API_URL>/healthz` once
- **AND** if it fails, logs the failure to stderr but still announces tools (so the agent can surface the error on the first tool call rather than the process disappearing silently)

### Requirement: Eight MCP tools mirror the agent-relevant REST endpoints

The system SHALL expose the following tools, each invoking the corresponding REST endpoint with `Authorization: Bearer <AGENT_API_TOKEN>`.

#### Scenario: lookup_product_by_barcode tool calls the REST lookup endpoint

- **WHEN** the agent calls `lookup_product_by_barcode` with `{"barcode":"3017624010701"}`
- **THEN** the wrapper issues `POST /products/lookup/3017624010701` to the REST API
- **AND** returns the REST 200 response body as the tool result content

#### Scenario: lookup_product_by_barcode forwards the refresh flag

- **WHEN** the agent calls `lookup_product_by_barcode` with `{"barcode":"X","refresh":true}`
- **THEN** the wrapper issues `POST /products/lookup/X?refresh=true`

#### Scenario: search_products tool forwards the query string

- **WHEN** the agent calls `search_products` with `{"q":"yogurt"}`
- **THEN** the wrapper issues `GET /products/search?q=yogurt`

#### Scenario: log_meal tool calls the REST meals endpoint

- **WHEN** the agent calls `log_meal` with `{"product_id":"<uuid>","quantity_g":150,"logged_at":"2026-06-06T12:30:00Z"}`
- **THEN** the wrapper issues `POST /meals` with the same JSON body
- **AND** returns the REST 201 response body as the tool result content

#### Scenario: log_meal_freeform tool calls the REST freeform endpoint

- **WHEN** the agent calls `log_meal_freeform` with `{"name":"banana","nutriments_per_100g":{"kcal":89},"quantity_g":120,"logged_at":"2026-06-06T10:00:00Z"}`
- **THEN** the wrapper issues `POST /meals/freeform` with the same JSON body

#### Scenario: patch_meal tool calls the REST patch endpoint

- **WHEN** the agent calls `patch_meal` with `{"meal_id":"<uuid>","quantity_g":200}`
- **THEN** the wrapper issues `PATCH /meals/<uuid>` with body `{"quantity_g":200}`

#### Scenario: delete_meal tool calls the REST delete endpoint

- **WHEN** the agent calls `delete_meal` with `{"meal_id":"<uuid>"}`
- **THEN** the wrapper issues `DELETE /meals/<uuid>`
- **AND** returns an empty tool result content when the REST response is 204

#### Scenario: daily_summary tool calls the REST daily endpoint

- **WHEN** the agent calls `daily_summary` with `{"date":"2026-06-06","tz":"Europe/Berlin"}`
- **THEN** the wrapper issues `GET /summary/daily?date=2026-06-06&tz=Europe/Berlin`

#### Scenario: daily_summary omits tz when not supplied

- **WHEN** the agent calls `daily_summary` with `{"date":"2026-06-06"}`
- **THEN** the wrapper issues `GET /summary/daily?date=2026-06-06` (no `tz` param)
- **AND** the REST API's default-tz behaviour determines the timezone

#### Scenario: range_summary tool forwards from/to/tz

- **WHEN** the agent calls `range_summary` with `{"from":"2026-06-01","to":"2026-06-07","tz":"UTC"}`
- **THEN** the wrapper issues `GET /summary/range?from=2026-06-01&to=2026-06-07&tz=UTC`

### Requirement: Write tools auto-derive idempotency keys when none is supplied

The system SHALL set the `Idempotency-Key` HTTP header on every write tool call. When the tool input contains an explicit `idempotency_key`, that value SHALL be used; otherwise the wrapper SHALL compute a stable key as `sha256_hex(<tool_name> + "|" + canonical_json(<tool_args_without_idempotency_key>))`.

#### Scenario: Explicit idempotency_key is forwarded verbatim

- **WHEN** the agent calls `log_meal_freeform` with `{"name":"X","quantity_g":100,"logged_at":"…","idempotency_key":"abc-123"}`
- **THEN** the REST request carries `Idempotency-Key: abc-123`

#### Scenario: Missing idempotency_key is derived from arguments

- **WHEN** the agent calls `log_meal_freeform` twice in a row with byte-identical other arguments and no `idempotency_key`
- **THEN** both REST requests carry the same auto-derived `Idempotency-Key` header
- **AND** the second call returns the same meal id as the first

#### Scenario: Different arguments produce different auto-derived keys

- **WHEN** the agent calls `log_meal_freeform` twice with the same name but different `quantity_g`
- **THEN** the two REST requests carry different `Idempotency-Key` headers
- **AND** both meals are persisted independently

#### Scenario: Derivation excludes the idempotency_key field from the canonical form

- **WHEN** the agent calls `log_meal_freeform` once with no `idempotency_key` and once with the same args plus an explicit `idempotency_key`
- **THEN** the two REST requests carry different `Idempotency-Key` values (the explicit one wins; the auto-derived one is not used)

### Requirement: REST errors are returned as MCP tool results with isError=true

The system SHALL surface REST 4xx and 5xx responses as MCP tool results carrying the REST response body verbatim, with `isError=true` so the agent runtime classifies the call as failed.

#### Scenario: product_not_found is forwarded with the next hint

- **WHEN** the REST API responds to `lookup_product_by_barcode` with `404` and body `{"error":"product_not_found","barcode":"X","next":"POST /meals/freeform"}`
- **THEN** the tool result has `isError=true`
- **AND** the content payload contains the JSON body unchanged, including the `next` field

#### Scenario: idempotency_key_conflict is forwarded

- **WHEN** the REST API responds with `409` and body `{"error":"idempotency_key_conflict"}`
- **THEN** the tool result has `isError=true`
- **AND** the content payload contains the JSON body unchanged

#### Scenario: upstream_timeout is forwarded

- **WHEN** the REST API responds with `504` and body `{"error":"upstream_timeout","retry_after_seconds":30}`
- **THEN** the tool result has `isError=true`
- **AND** the content payload contains the JSON body unchanged

#### Scenario: Transport-level failures produce a synthetic error envelope

- **WHEN** the wrapper cannot reach the REST API (DNS failure, connection refused, timeout)
- **THEN** the tool result has `isError=true`
- **AND** the content payload is `{"error":"transport","detail":"<description>"}`

### Requirement: Configuration is read from environment variables

The system SHALL read its configuration from environment variables at process start and refuse to start when required variables are missing.

#### Scenario: Missing AGENT_API_TOKEN halts startup

- **WHEN** the process starts with `AGENT_API_TOKEN` unset or empty
- **THEN** the binary writes an error to stderr identifying the variable
- **AND** exits with a non-zero status code

#### Scenario: NUTRITION_API_URL defaults to localhost

- **WHEN** the process starts without `NUTRITION_API_URL` set
- **THEN** tool calls target `http://localhost:8080`

#### Scenario: Per-request timeout is configurable

- **WHEN** the process starts with `MCP_REQUEST_TIMEOUT_SECONDS=20`
- **THEN** the wrapper applies a 20-second timeout on each REST call

#### Scenario: Default per-request timeout is 10 seconds

- **WHEN** the process starts without `MCP_REQUEST_TIMEOUT_SECONDS` set
- **THEN** the wrapper applies a 10-second timeout on each REST call
