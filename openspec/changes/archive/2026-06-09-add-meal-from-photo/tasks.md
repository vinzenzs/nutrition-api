## 1. Vision package (internal/vision/)

- [ ] 1.1 New package directory `internal/vision/` mirroring `internal/off/` layout: `client.go`, `types.go`, `parse.go`, `errors.go`, `client_test.go`.
- [ ] 1.2 `types.go`: `ParseRequest{image []byte; metadata}` and `ParseResult{Name, NutrimentsPer100g, Confidence, Notes, InputTokens, OutputTokens, ResizedTo [2]int}`.
- [ ] 1.3 `client.go`: `Client` struct over `*http.Client` with `baseURL`, `apiKey`, `model`, `timeout`. `New(cfg Config)` constructor returns `ErrAPIKeyMissing` when key is empty.
- [ ] 1.4 `client.go`: `Parse(ctx, req ParseRequest) (*ParseResult, error)`. Builds the Anthropic Messages request with `tools: [report_meal]` and `tool_choice: {type:"tool", name:"report_meal"}`. Image goes in a `source: {type:"base64", media_type:"image/jpeg", data:"..."}` content block.
- [ ] 1.5 `parse.go`: decode the Anthropic response, find the `tool_use` content block, validate input against the schema. One-retry path on missing/invalid tool_use, then `ErrVisionResponseUnparseable`.
- [ ] 1.6 `errors.go`: `ErrAPIKeyMissing`, `ErrVisionTimeout`, `ErrVisionUpstreamError`, `ErrVisionRateLimited{RetryAfterSeconds int}`, `ErrVisionUnexpectedResponse{StatusCode int}`, `ErrVisionResponseUnparseable`. Helper `IsVisionError(err)`.
- [ ] 1.7 Image resize helper: takes raw image bytes, decodes (JPEG/PNG only in v1), downscales to max-edge 1568px with `golang.org/x/image/draw`, re-encodes JPEG q=85. Returns resized bytes + final dimensions.
- [ ] 1.8 Reject HEIC for v1 with `ErrUnsupportedMediaType` — document in design.md and the handler maps to `415`.

## 2. Vision tests with fixtures

- [ ] 2.1 `testdata/vision/well_formed.json` — recorded Anthropic response with a valid `tool_use` for `report_meal` (name, full nutriments, confidence 0.85).
- [ ] 2.2 `testdata/vision/low_confidence.json` — confidence 0.4, sparse nutriments.
- [ ] 2.3 `testdata/vision/missing_tool_use.json` — model returned a text block instead of a tool_use (parse-failure path).
- [ ] 2.4 `testdata/vision/rate_limit.json` — 429 envelope with `retry-after`.
- [ ] 2.5 `testdata/vision/server_error.json` — 503 envelope.
- [ ] 2.6 `client_test.go`: stub `*http.Client` routing by URL, assert request shape (model, tools, tool_choice, image media_type), assert each error path.
- [ ] 2.7 Resize-helper unit test: a 3000x2000 input gets resized to 1568x1045; an 800x600 input is unchanged byte-for-byte.

## 3. Config

- [ ] 3.1 Extend `internal/config/` with `AnthropicAPIKey string`, `ClaudeVisionModel string` (default `claude-sonnet-4-6`), `VisionTimeout time.Duration` (default 15s), `MealFromPhotoMaxBytes int64` (default 10485760).
- [ ] 3.2 Extend `.env.example` with each new var (commented with a note that `from_photo` returns 503 without `ANTHROPIC_API_KEY`).
- [ ] 3.3 In `internal/httpserver/server.go`, build a `*vision.Client` (or nil if the key is missing) and pass to the meals handlers.

## 4. Handler: POST /meals/from_photo

- [ ] 4.1 In `internal/meals/handlers.go` (or a new sibling file `handlers_from_photo.go`), add the new route registration: `rg.POST("/meals/from_photo", h.createFromPhoto)`.
- [ ] 4.2 Handler decodes `multipart/form-data`. Reads `image` part with `MaxMultipartMemory` cap; rejects >`MealFromPhotoMaxBytes` with `413 image_too_large`.
- [ ] 4.3 Parses metadata form fields (`quantity_g`, `logged_at`, `meal_type`, `note`), applies defaults (quantity 100, logged_at now), validates per existing meals validators (`quantity_g_invalid`, `meal_type_invalid`, `logged_at_too_far_future`). All validation happens BEFORE the vision call.
- [ ] 4.4 If `visionClient` is nil (no API key configured): return `503 vision_unavailable`.
- [ ] 4.5 Resize image, call `visionClient.Parse(ctx, ParseRequest{...})`.
- [ ] 4.6 Map vision errors to HTTP per design.md table.
- [ ] 4.7 On success: build a freeform meal input from the vision result and call the existing `meals.Service.CreateFreeform` (factor out a shared internal entry point if needed to avoid re-validating).
- [ ] 4.8 Build and return the response envelope `{meal, inference}`.
- [ ] 4.9 Godoc / swag annotations covering each error code.

## 5. Handler tests

- [ ] 5.1 Stub the vision client in handler tests (`stubVision` returning canned `ParseResult` or errors).
- [ ] 5.2 Happy path: multipart with valid JPEG → 201 + envelope with `meal` and `inference`.
- [ ] 5.3 Missing image part → 400 `image_required`.
- [ ] 5.4 Oversized image → 413 `image_too_large`.
- [ ] 5.5 Invalid `meal_type` → 400 BEFORE the stub vision client is called (assert call count is 0).
- [ ] 5.6 Stub returns `ErrVisionRateLimited{RetryAfterSeconds:42}` → 429 `vision_rate_limited` with `retry_after_seconds: 42`.
- [ ] 5.7 Stub returns `ErrVisionTimeout` → 504 `vision_timeout`.
- [ ] 5.8 Stub returns `ErrVisionResponseUnparseable` → 502 `vision_response_unparseable`.
- [ ] 5.9 No API key (vision client nil) → 503 `vision_unavailable`.
- [ ] 5.10 Idempotency replay: same image + same key → 201 with same meal id, stub vision client called exactly once across both requests.
- [ ] 5.11 Different image + same key → 409 `idempotency_key_conflict`.

## 6. MCP wrapper

- [ ] 6.1 Add `LogMealFromPhotoArgs` struct in `internal/mcpserver/tools_meals.go` (or a new sibling file) with `ImageBase64`, `QuantityG`, `LoggedAt`, `MealType`, `Note`, `IdempotencyKey`.
- [ ] 6.2 `handleLogMealFromPhoto`: decode base64, build multipart body, POST to `/meals/from_photo`. Forward / derive idempotency key per existing convention. Note: image bytes are part of the canonical input for derivation, which gives natural replay collapse.
- [ ] 6.3 Register the tool in `registerMealsTools` with the description from `specs/mcp-server/spec.md`.
- [ ] 6.4 MCP tool tests: stubbed apiclient, assert multipart body shape; assert idempotency-key forwarding and derivation; assert error passthrough for `vision_unavailable` and `vision_rate_limited`.

## 7. e2e

- [x] 7.1 In `internal/e2e/e2e_test.go`, add `TestE2E_PhotoToMealLandsInDailySummary` that boots the server with a stub vision client (injected via a new option on `bootServer`), POSTs a fake image, asserts 201, then GETs `summary/daily` for the day and confirms the meal entry is included with the inferred name.

## 8. Docs

- [x] 8.1 Update `README.md` with a "Photo of meal" subsection in the Meals examples, including a `curl -F image=@plate.jpg ...` command.
- [x] 8.2 Update `RUN_LOCAL.md` to mention the `ANTHROPIC_API_KEY` env var and that the endpoint returns 503 without it.
- [x] 8.3 `task swag` to regenerate `docs/`.

## 9. Pre-merge

- [x] 9.1 `task vet` clean.
- [x] 9.2 `task test` green.
- [ ] 9.3 Manual: with `ANTHROPIC_API_KEY` set in `.env.local`, post a real food photo, verify the parsed meal lands; without the key set, verify 503.
- [x] 9.4 `openspec validate add-meal-from-photo --strict` passes.
