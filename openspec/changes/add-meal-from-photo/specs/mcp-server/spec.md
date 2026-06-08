## ADDED Requirements

### Requirement: log_meal_from_photo tool mirrors the REST photo endpoint

The MCP server SHALL expose a `log_meal_from_photo` tool that maps onto `POST /meals/from_photo`. The tool accepts a base64-encoded image in the input (since MCP transports JSON, not multipart) and a small set of metadata fields. The wrapper decodes the base64, builds the multipart body, and forwards. Response handling mirrors the existing tool conventions (REST 2xx → success result, REST 4xx/5xx → `isError=true` with the REST body verbatim).

#### Scenario: log_meal_from_photo tool calls the REST endpoint

- **WHEN** the agent calls `log_meal_from_photo` with `{"image_base64":"...","quantity_g":250,"logged_at":"2026-06-07T12:30:00Z","meal_type":"lunch"}`
- **THEN** the wrapper base64-decodes the image and issues `POST /meals/from_photo` as a multipart request with the image and metadata
- **AND** returns the REST 201 response body (the `{meal, inference}` envelope) as the tool result content

#### Scenario: log_meal_from_photo forwards idempotency_key as a header

- **WHEN** the agent supplies `idempotency_key` in the input
- **THEN** the wrapper sets `Idempotency-Key: <value>` on the REST request

#### Scenario: log_meal_from_photo derives an idempotency_key when omitted

- **WHEN** the agent does not supply `idempotency_key`
- **THEN** the wrapper computes a deterministic key from the tool name and canonicalized input (note: the base64 image is part of the canonical input, so two byte-identical calls collapse)
- **AND** sets `Idempotency-Key` on the REST request to the derived value

#### Scenario: vision_unavailable error is forwarded with isError=true

- **WHEN** the REST API responds `503 vision_unavailable`
- **THEN** the tool result has `isError=true`
- **AND** the content payload contains the JSON body unchanged so the agent can explain the configuration gap to the user

#### Scenario: Tool description names the use case and the missing-MCP-image caveat

- **WHEN** the agent enumerates tools via `tools/list`
- **THEN** the `log_meal_from_photo` description states: "Log a meal from a photo of the food. Provide the image as base64 in `image_base64`. Most current MCP clients do not pass photos through to tools — this tool exists for future MCP-aware UIs and agentic test harnesses that can pass image bytes. For natural-language meal logging, use log_meal_freeform."
