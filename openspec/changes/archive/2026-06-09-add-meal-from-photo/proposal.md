## Why

The Flutter companion app's second killer interaction is **photo-of-meal**: the user is about to eat something that has no barcode and no obvious analog in the cache (a restaurant plate, a homemade dish, a cafeteria tray), they snap a photo, the app logs it. Conversation via the MCP agent can already do the "describe and estimate" half — but the agent path requires the user to type or speak the description, which is slow at a restaurant table and fragile in noisy environments.

The decided shape is backend-mediated (the design pass with the user picked this over phone-direct-to-Claude). Mirrors the existing `off-integration` pattern: backend owns the upstream integration, clients stay simple. Three reasons it's the right call:

1. **No API key on every device.** The Claude API key lives in the server's `.env`, not the phone's Keystore. One key to rotate, one bill, no fan-out.
2. **The cookidoo importer, the agent, and the Flutter app all want this same primitive.** A backend endpoint exposes it once; each client uses the same contract.
3. **The system already has a "backend mediates external upstreams" convention** (off-integration). Adding a Claude vision integration alongside it is consistent.

Sync semantics: the endpoint returns the parsed meal in the same response (~1.5s for Claude Vision on a 1MB image, well under typical HTTP timeout budgets). Async with polling was considered and rejected as unnecessary complexity for this latency.

## What Changes

- New endpoint `POST /meals/from_photo` accepting `multipart/form-data` with an `image` part (JPEG/PNG/HEIC) and optional metadata fields: `quantity_g` (default 100), `logged_at` (default `now()`), `meal_type`, `note`.
- The backend resizes oversized images to a max edge of 1568px (Claude Vision's recommended ceiling), sends them to Claude Vision with a strict-JSON prompt that asks for `{name, nutriments_per_100g, confidence}`, parses the response, and internally calls the existing freeform-meal-create path with the parsed data. The meal entry is returned in the same shape as `POST /meals/freeform` plus an extra top-level `inference` block carrying `{model, confidence, original_image_bytes, resized_to}` so the caller can decide whether to trust the estimate or prompt the user to refine.
- New env var `ANTHROPIC_API_KEY` (required only for `/meals/from_photo`; the endpoint refuses the request with `503 vision_unavailable` if unset, so the rest of the API runs unchanged without a key).
- New env var `CLAUDE_VISION_MODEL` (default `claude-sonnet-4-6`) so the model can be swapped without a redeploy.
- New env var `MEAL_FROM_PHOTO_MAX_BYTES` (default `10485760` = 10MB) to bound abuse.
- Standard `Idempotency-Key` header support: the body hash includes the raw image bytes, so two identical uploads with the same key return the original meal entry without a second Claude call.
- No image storage. The image is held in memory during the request, sent to Claude, then discarded. Future change can add optional `image_storage = true` if a user wants meal-photo history; out of scope here.
- MCP tool: a new `log_meal_from_photo` tool. The MCP wrapper does NOT auto-pass an image (agents don't have one); the tool exists so the agent can suggest "scan a photo if you can't describe it" with a tappable affordance in a future MCP-aware UI. Until then it's a documented tool with input `{image_base64, ...}` that an out-of-band caller could use.

## Capabilities

### New Capabilities

- `vision-integration`: How the system integrates with Claude Vision for image-to-meal parsing — request shape, prompt contract, error mapping, test fixtures, and the no-storage privacy guarantee. Sister capability to `off-integration`.

### Modified Capabilities

- `meals`: Add `### Requirement: Log a meal entry from a photo` parallel to the existing "Log a meal entry from freeform" requirement. Same response shape on success; same idempotency contract.
- `mcp-server`: Add the new `log_meal_from_photo` tool to the surface, taking the tool count from 13 → 14.

## Impact

- **Schema**: none. No tables, no columns. The meal entry created by the photo path uses the existing freeform-snapshot path under the hood.
- **New package**: `internal/vision/` holding the Claude client (parallel to `internal/off/`). Includes the prompt, the strict-JSON parser, error mapping (timeout → 504, 5xx → 504, 4xx → 502, unparseable response → 502), and fixture-based tests against recorded Claude responses.
- **New handler**: `POST /meals/from_photo` in `internal/meals/handlers.go` (or a sibling file if it gets large).
- **Service layer**: thin orchestration — resize → vision → existing CreateFreeform — most logic lives in the new vision package.
- **New dependency**: official Anthropic Go SDK (`github.com/anthropics/anthropic-sdk-go`) OR raw HTTP. Default decision in design.md is raw HTTP, mirroring how `internal/off/` is structured (no external SDK, just the standard HTTP client with a typed parser).
- **Tests**: fixture-based unit tests for the vision parser using recorded Claude responses; handler tests using a stubbed vision client; one e2e test that asserts a stubbed `from_photo` call lands a meal entry retrievable via daily summary.
- **Docs**: README gets a `### Photo-of-meal` curl example. RUN_LOCAL.md gets a setup note about `ANTHROPIC_API_KEY`. Swagger annotations document the new endpoint and error codes.
- **Cost / abuse**: per-token rate-limit is OUT of scope for v1 (single-user system); but the env-driven max image size is in.

## Out of scope

- Image storage / meal-photo history. Future change can add `?store=true` query param and a `meal_photos` table.
- Async / job-queue version. Sync is fine at Claude Vision latencies (<2s).
- Multi-shot ("did you mean…?") refinement loops. The endpoint returns a `confidence` score; the client UI decides what to do at low confidence. The agent UI can prompt a follow-up via the existing freeform path.
- Falling back to a different vision provider (Gemini, GPT-4V) on Claude unavailability. Multi-provider abstraction is its own change if/when it earns priority.
- Photo-of-barcode (taking a photo of packaging and recognising the barcode through OCR). The dedicated barcode scanner on-device is the right path for that.
