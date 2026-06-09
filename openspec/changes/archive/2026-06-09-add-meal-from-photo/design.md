## Context

The Flutter app's second killer input mode is photo-of-meal. The earlier design pass with the user landed on backend-mediated (rather than phone-direct-to-Claude) for three reasons that are worth restating because they shape this whole change:

1. **One API key, one bill.** The server is the only thing holding `ANTHROPIC_API_KEY`. No Keystore distribution problem.
2. **Multiple clients reuse the same primitive.** The Flutter app, the (future) iOS app, the cookidoo importer when a user pastes a recipe with a photo, even agent-driven workflows — all call the same endpoint.
3. **Pattern match to existing code.** `internal/off/` is a small, focused package that integrates one external service with fixture-based tests. `internal/vision/` should look almost identical, just for a different upstream.

The latency budget is set by Claude Vision: ~700-1500ms for a single-image inference at modest sizes. Sync HTTP is comfortable inside that window with sensible timeouts. Async-with-polling is more code for negligible benefit.

## Goals / Non-Goals

**Goals:**

- A single new HTTP endpoint that takes an image and returns a logged meal entry.
- A small `internal/vision/` package that owns the Claude integration with the same test conventions as `internal/off/` (recorded response fixtures, no live calls in CI).
- Strict-JSON prompt contract so the Go side can unmarshal directly without LLM-glue parsing.
- The meal entry returned matches the existing `POST /meals/freeform` shape exactly, plus one extra `inference` block — so existing client code that handles freeform meals just works.
- Idempotency: byte-identical re-uploads with the same `Idempotency-Key` skip the Claude call entirely.

**Non-Goals:**

- Image persistence. The bytes are held in memory during the request and discarded after the response is sent. If a user wants photo history, that's a separate change.
- Multiple model providers. Single Claude path; abstracting over Gemini / GPT-4V belongs in a future change if it earns priority.
- Per-image cost / quota tracking. Single-user system; out of scope.
- Photo-of-barcode. On-device camera scanning handles that.
- Confidence-based retry loops on the backend. The endpoint reports a confidence score and lets the client decide.

## Decisions

### 1. Raw HTTP to Claude, no SDK

Mirror `internal/off/` — a `Client` struct over `*http.Client`, hand-written request/response types, fixture-based tests. Adding `anthropic-sdk-go` would pull a substantial dependency and SDK opinions into the codebase for one endpoint.

**Alternative considered:** use the official Anthropic Go SDK. Rejected — extra dependency, SDK API churn risk, harder to fixture in tests. The Messages API we need is straightforward HTTP.

### 2. Strict-JSON prompt with a single retry on parse failure

The prompt asks Claude to respond with EXACTLY the following JSON shape:

```json
{
  "name": "<short dish name>",
  "nutriments_per_100g": {
    "kcal": <number>,
    "protein_g": <number>,
    "carbs_g": <number>,
    "fat_g": <number>,
    "fiber_g": <number|null>,
    "sugar_g": <number|null>,
    "salt_g": <number|null>
  },
  "confidence": <0.0–1.0>,
  "notes": "<optional one-liner: 'unclear if includes sauce' etc.>"
}
```

The system prompt insists on no prose, no code fences, no commentary. If the first response fails to parse, the client retries ONCE with a follow-up message "Your last response was not valid JSON. Reply with ONLY the JSON object." Beyond one retry, the endpoint returns `502 vision_response_unparseable`.

**Alternative considered:** tool-use / JSON-mode if Claude exposes one. The Anthropic API supports tool-forced responses; using `tools` with `tool_choice: {type: "tool", name: "report_meal"}` is arguably cleaner than prose-prompt JSON. **Decision: use tool-forced output.** Cleaner contract, lower retry rate. The fixture format includes the tool-use response shape.

### 3. Image resizing on the server before sending to Claude

Reasons:

- Bandwidth cost (the server bills outgoing bytes to Claude).
- Claude Vision's documented preference for images with max edge ≤ 1568px.
- Latency: smaller images = faster vision inference.

We use Go's `image/jpeg` + `golang.org/x/image/draw` to downscale anything larger; JPEG-encode at quality 85. HEIC is converted via a small native dependency (`github.com/strukturag/libheif` or its Go wrapper). If HEIC support adds operational complexity (CGo etc.), drop HEIC for v1 and return `415 unsupported_media_type` — most phone photo apps can save as JPEG.

**Alternative considered:** send original bytes through. Rejected — pays a 5x bandwidth cost for no benefit.

### 4. The `inference` block in the response

The endpoint's response is:

```json
{
  "meal": { /* same shape as POST /meals/freeform's response */ },
  "inference": {
    "model": "claude-sonnet-4-6",
    "confidence": 0.78,
    "notes": "unclear if salad includes dressing",
    "original_image_bytes": 4128719,
    "resized_to": [1568, 1170],
    "claude_input_tokens": 1240,
    "claude_output_tokens": 89
  }
}
```

The `meal` block is the canonical meal entry (so existing client code that handles freeform meals just calls `response.meal`). The `inference` block is informative — clients can decide to surface confidence to the user, log token usage, etc.

**Alternative considered:** flatten everything into one object. Rejected — pollutes the canonical meal shape with vision-specific metadata.

### 5. Error mapping

Mirror `off-integration`'s error taxonomy:

| Condition | HTTP status | Body |
|---|---|---|
| `ANTHROPIC_API_KEY` not set | `503` | `{"error":"vision_unavailable","reason":"ANTHROPIC_API_KEY not configured"}` |
| Image > `MEAL_FROM_PHOTO_MAX_BYTES` | `413` | `{"error":"image_too_large","max_bytes":<n>}` |
| Image unsupported / unparseable | `415` | `{"error":"unsupported_media_type"}` |
| Claude timeout | `504` | `{"error":"vision_timeout","retry_after_seconds":30}` |
| Claude 5xx | `504` | `{"error":"vision_upstream_error","retry_after_seconds":30}` |
| Claude 4xx (other than rate-limit) | `502` | `{"error":"vision_unexpected_response","status":<n>}` |
| Claude rate-limit | `429` | `{"error":"vision_rate_limited","retry_after_seconds":<from header>}` |
| Response unparseable after one retry | `502` | `{"error":"vision_response_unparseable"}` |

### 6. Idempotency

The standard middleware applies: body hash includes the raw image bytes plus the form-data metadata fields. Byte-identical re-uploads with the same key return the cached response — no second Claude call. Different image, same key → `409 idempotency_key_conflict` per the existing contract.

A real-world note: phone OS image-pickers can re-encode photos on second selection (HEIC→JPEG transcoding can yield different bytes). The retry path that matters most is the client-driven "I lost network, replay the queue" — which uses *exactly* the bytes that were originally sent. Byte-strict matching is correct here. A user manually re-taking a photo gets a new meal entry (different bytes, new Claude call) — also correct.

### 7. MCP tool

The MCP wrapper exposes `log_meal_from_photo` with input shape:

```json
{
  "image_base64": "...",
  "quantity_g": 250,
  "logged_at": "2026-06-07T12:30:00Z",
  "meal_type": "lunch",
  "note": "...",
  "idempotency_key": "..."
}
```

The wrapper base64-decodes, builds a multipart form, posts. Most current agent runtimes don't have direct image access, so this tool's day-one usage is "agentic test harness + future MCP UIs that pass images through." Adding the tool now keeps the surface consistent.

## Risks / Trade-offs

- **Cost of a single endpoint = some users will use it constantly.** Single-user system; the user is the budget owner. If multi-user comes later, per-user quota is the right answer at that time. Not designing for it now.
- **Claude misidentifies food.** Confidence score is the safety valve. Below ~0.6 the client should prompt the user to refine via the freeform path. Spec'ing the threshold in the client, not the backend (different surfaces have different UX).
- **PII / privacy.** Photos are not stored. Logged at info level we record: request id, image size, model, latency, token counts. No image bytes hit logs.
- **HEIC support adds CGo.** If integrating libheif blocks the build pipeline, drop HEIC for v1 — return `415` and ask the client to send JPEG. The Flutter `image_picker` plugin can request JPEG output.
- **Rate limits cascade weirdly.** If the user hammers the endpoint, Claude rate-limits → server returns 429 → client backs off. The retry hint from Claude's `retry-after` header is forwarded.
- **Body-hash idempotency over a 5MB body.** SHA-256 over 5MB is ~30ms on a typical server. Acceptable.

## Migration Plan

- No schema migration. No new tables, no new columns.
- `.env.example` gains `ANTHROPIC_API_KEY` (commented out by default with a note that the photo endpoint returns 503 without it) and `MEAL_FROM_PHOTO_MAX_BYTES`.
- Existing deployments without the key continue to work; only the new endpoint refuses.
- Rollback: removing the route and the env vars is sufficient. No data state to unwind.

## Open Questions

- **HEIC: ship in v1 or defer?** Strong lean toward defer (cleaner build) and require JPEG from the client. Flutter's `image_picker` can do this with one flag.
- **Should the resized image be cached for the duration of the idempotency record?** Today the idempotency record stores the response body. The original image isn't stored. Replays return the cached *result*, never re-run vision. Good.
- **Tool-forced JSON or prose-prompt JSON?** Recommendation in this doc is tool-forced. Slight code complexity bump on the backend (handle the `tool_use` content block instead of a text block) but lower parse-failure rate. Decision: tool-forced.
- **Multipart vs JSON-with-base64 for the request body?** Multipart is the right shape for binary uploads — smaller, no base64 overhead. JSON-with-base64 is simpler for the agent path. **Decision: multipart for the HTTP endpoint, agent path goes via the MCP wrapper which handles the base64 → multipart shim.**
