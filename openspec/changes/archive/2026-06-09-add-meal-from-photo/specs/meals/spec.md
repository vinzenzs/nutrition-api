## ADDED Requirements

### Requirement: Log a meal entry from a photo

The system SHALL expose `POST /meals/from_photo` that accepts an image as `multipart/form-data` and creates a meal entry whose name and per-100g nutriments are inferred from the image by the vision-integration capability. The resulting meal entry is a normal freeform meal (`product_id = null`, snapshot columns populated) — interchangeable with entries created via `POST /meals/freeform`.

#### Scenario: Successful photo log

- **WHEN** the client posts a multipart body with an `image` part containing a JPEG of a recognisable dish, and form fields `{"quantity_g":250,"logged_at":"2026-06-07T12:30:00Z","meal_type":"lunch"}`
- **THEN** the system resizes the image if needed, calls the vision-integration capability to parse the dish, and creates a freeform meal entry with the parsed name and nutriments
- **AND** returns `201 Created` with body `{"meal": <MealEntry>, "inference": {"model":"...","confidence":<0..1>,"notes":"...","original_image_bytes":<n>,"resized_to":[w,h],"claude_input_tokens":<n>,"claude_output_tokens":<n>}}`
- **AND** the embedded `meal` block has the same shape as a `POST /meals/freeform` response (`product_id = null`, snapshot fields populated)

#### Scenario: Missing image part is rejected

- **WHEN** the client posts a multipart body with no `image` part
- **THEN** the system returns `400 Bad Request` with `{"error":"image_required"}`
- **AND** no vision call is made

#### Scenario: Default metadata is applied

- **WHEN** the client posts only the `image` part with no metadata
- **THEN** the system defaults `quantity_g` to 100, `logged_at` to the server's current UTC time, and stores no `meal_type` or `note`
- **AND** the resulting meal entry reflects those defaults

#### Scenario: Invalid meal_type is rejected before any vision call

- **WHEN** the client posts a `meal_type` form field that is not one of `breakfast`, `lunch`, `dinner`, `snack`
- **THEN** the system returns `400 Bad Request` with `{"error":"meal_type_invalid"}`
- **AND** the image is not sent upstream

#### Scenario: Non-positive quantity is rejected before any vision call

- **WHEN** the client posts `quantity_g` form field that is zero, negative, or non-numeric
- **THEN** the system returns `400 Bad Request` with `{"error":"quantity_g_invalid"}`
- **AND** the image is not sent upstream

#### Scenario: logged_at far in the future is rejected before any vision call

- **WHEN** the client posts a `logged_at` more than 24 hours in the future relative to server time
- **THEN** the system returns `400 Bad Request` with `{"error":"logged_at_too_far_future"}`
- **AND** the image is not sent upstream

#### Scenario: Vision unavailable surfaces the configuration gap to the caller

- **WHEN** the server is started without `ANTHROPIC_API_KEY`
- **AND** the client posts to `/meals/from_photo`
- **THEN** the system returns `503 Service Unavailable` with `{"error":"vision_unavailable","reason":"ANTHROPIC_API_KEY not configured"}`
- **AND** the rest of the API continues to operate normally

#### Scenario: Idempotent replay returns the cached meal without re-running vision

- **WHEN** the client posts the same image bytes and metadata with the same `Idempotency-Key` header within TTL
- **THEN** the system returns the cached `201` response from the first call, including the same meal id
- **AND** no second vision call is made (this is the canonical "lost network ack" recovery)

#### Scenario: Different image with same key returns 409

- **WHEN** the client posts a different image but the same `Idempotency-Key` as a previous successful call
- **THEN** the system returns `409 Conflict` with `{"error":"idempotency_key_conflict"}` (the existing body-hash mismatch behaviour applies; image bytes are part of the hashed body)
