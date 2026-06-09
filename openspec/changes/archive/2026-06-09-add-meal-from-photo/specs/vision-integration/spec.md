## ADDED Requirements

### Requirement: Vision client uses the Anthropic Messages API with a tool-forced JSON contract

The system SHALL fetch meal parses from Anthropic's Messages API at `https://api.anthropic.com/v1/messages` using `tool_choice` to force a `report_meal` tool response, so the server never has to parse free-form prose. The model is configurable via the `CLAUDE_VISION_MODEL` env var (default `claude-sonnet-4-6`). Authentication uses `x-api-key: $ANTHROPIC_API_KEY` and `anthropic-version: 2023-06-01`. A `User-Agent` of the form `nutrition-vision/<version>` is sent on every request.

#### Scenario: Request is sent with tool_choice forcing report_meal

- **WHEN** the vision client invokes `Parse(ctx, image, metadata)`
- **THEN** the HTTP body to Anthropic contains `tools: [{name: "report_meal", input_schema: {name, nutriments_per_100g, confidence, notes}}]`
- **AND** `tool_choice: {type: "tool", name: "report_meal"}`
- **AND** the image content block uses `source.type = "base64"` with the resized JPEG bytes

#### Scenario: Missing API key surfaces as a typed error before any HTTP call

- **WHEN** the vision client is invoked with `ANTHROPIC_API_KEY` empty or unset at process start
- **THEN** the constructor returns a typed `ErrAPIKeyMissing` error
- **AND** the handler maps this to `503 vision_unavailable` without attempting any outbound call

### Requirement: Images larger than the configured ceiling are rejected before being sent

The system SHALL refuse to forward images whose total byte length exceeds `MEAL_FROM_PHOTO_MAX_BYTES` (default 10 MiB). The check happens after multipart decode but before resize, so abusive uploads cost only the multipart parse.

#### Scenario: Oversized image is rejected with 413

- **WHEN** the client posts a multipart body whose `image` part exceeds `MEAL_FROM_PHOTO_MAX_BYTES`
- **THEN** the handler returns `413 Payload Too Large` with body `{"error":"image_too_large","max_bytes":<configured-value>}`
- **AND** no outbound call to Anthropic is made

### Requirement: Images are resized to a max edge of 1568 pixels before being sent upstream

Images larger than 1568 pixels on the longest edge SHALL be downscaled to 1568px on that edge, preserving aspect ratio, re-encoded as JPEG quality 85. Smaller images SHALL be forwarded unchanged.

#### Scenario: Large image is resized

- **WHEN** the input image is 3000x2000 pixels and 2 MiB
- **THEN** the resized image is 1568x1045 pixels (long edge 1568, aspect preserved)
- **AND** the resized payload is JPEG-encoded
- **AND** the response `inference.resized_to` is `[1568, 1045]`

#### Scenario: Small image is forwarded unchanged

- **WHEN** the input image is 800x600 pixels
- **THEN** the bytes sent to Anthropic match the input bytes byte-for-byte
- **AND** the response `inference.resized_to` reflects the original dimensions

### Requirement: Upstream errors are mapped to typed handler responses

The vision client SHALL map Anthropic errors to handler-visible error types so the meal handler can return consistent JSON error bodies.

#### Scenario: Anthropic timeout maps to 504 vision_timeout

- **WHEN** the call exceeds the configured `VISION_TIMEOUT_SECONDS` (default 15)
- **THEN** the client returns `ErrVisionTimeout`
- **AND** the handler returns `504 Gateway Timeout` with body `{"error":"vision_timeout","retry_after_seconds":30}`

#### Scenario: Anthropic 5xx maps to 504 vision_upstream_error

- **WHEN** Anthropic responds with a 500-range status
- **THEN** the handler returns `504` with body `{"error":"vision_upstream_error","retry_after_seconds":30}`

#### Scenario: Anthropic rate-limit response is forwarded

- **WHEN** Anthropic returns `429` with a `retry-after` header
- **THEN** the handler returns `429` with body `{"error":"vision_rate_limited","retry_after_seconds":<header value>}`

#### Scenario: Anthropic 4xx other than rate-limit maps to 502

- **WHEN** Anthropic returns `400`, `401`, `403`, or any unexpected 4xx
- **THEN** the handler returns `502 Bad Gateway` with body `{"error":"vision_unexpected_response","status":<n>}`

#### Scenario: Tool response missing or malformed maps to 502 after one retry

- **WHEN** Anthropic's response lacks a `tool_use` block for `report_meal`, or the tool input fails to validate against the schema
- **THEN** the client retries the same request ONCE with an additional system message "Your previous response did not invoke the report_meal tool. Use the tool."
- **AND** if the retry also lacks a valid `tool_use`, the handler returns `502 Bad Gateway` with body `{"error":"vision_response_unparseable"}`

### Requirement: Images are never persisted by the server

The system SHALL NOT write image bytes to disk or to the database at any point. Image bytes live in memory for the request lifetime and are released when the response is returned. The idempotency cache stores the response body (the parsed meal + inference block) but not the image.

#### Scenario: No bytes hit the filesystem

- **WHEN** the handler processes a 5 MiB photo
- **THEN** no file is created under any temp dir or product cache location
- **AND** logs at info level record only `{request_id, image_bytes, model, latency_ms, input_tokens, output_tokens}` — never the image content

#### Scenario: Replay returns cached response without a new Claude call

- **WHEN** the client re-uploads byte-identical bytes with the same `Idempotency-Key` within TTL
- **THEN** the idempotency middleware replays the original `201` response
- **AND** the vision client is not invoked
- **AND** no token cost is incurred upstream

### Requirement: Vision client tests run against recorded fixtures

The vision client SHALL be testable without making live HTTP calls to Anthropic.

#### Scenario: Tests use recorded fixtures from testdata/vision/

- **WHEN** the vision client test suite runs
- **THEN** the client is wired to read fixtures from `testdata/vision/<case>.json` containing recorded Anthropic responses
- **AND** the fixtures cover at minimum: a well-formed `report_meal` tool_use response, a `report_meal` response with low confidence, a response missing the tool_use block (parse-failure path), a rate-limit response with `retry-after`, a 5xx response, and a timeout-simulating error
