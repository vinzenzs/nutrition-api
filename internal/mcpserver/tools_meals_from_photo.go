package mcpserver

import (
	"bytes"
	"context"
	"encoding/base64"
	"mime/multipart"
	"strconv"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// LogMealFromPhotoArgs is the MCP input for the photo-of-meal tool. The
// agent supplies a base64-encoded image (most current MCP runtimes don't
// pass binary content directly); the wrapper decodes and re-packages as a
// multipart upload before posting to /meals/from_photo. The optional metadata
// fields mirror the freeform path so the agent can set quantity / meal_type /
// note in the same call.
type LogMealFromPhotoArgs struct {
	ImageBase64    string   `json:"image_base64" jsonschema:"image bytes, base64-encoded (JPEG or PNG; HEIC is rejected with 415 in v1)"`
	QuantityG      *float64 `json:"quantity_g,omitempty" jsonschema:"meal quantity in grams, default 100"`
	LoggedAt       string   `json:"logged_at,omitempty" jsonschema:"RFC 3339 timestamp; default now()"`
	MealType       string   `json:"meal_type,omitempty" jsonschema:"breakfast | lunch | dinner | snack"`
	Note           string   `json:"note,omitempty" jsonschema:"optional free-text note"`
	IdempotencyKey string   `json:"idempotency_key,omitempty" jsonschema:"optional retry key. If omitted, a stable key is derived from the (decoded image bytes + metadata) — byte-identical replays return the original meal without a second Claude call."`
}

func handleLogMealFromPhoto(ctx context.Context, c *apiClient, args LogMealFromPhotoArgs) *mcp.CallToolResult {
	imageBytes, err := base64.StdEncoding.DecodeString(args.ImageBase64)
	if err != nil {
		return toToolResult(0, nil, &transportError{inner: err})
	}

	// Build the multipart form.
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	fw, err := mw.CreateFormFile("image", "meal.jpg")
	if err != nil {
		return toToolResult(0, nil, &transportError{inner: err})
	}
	if _, err := fw.Write(imageBytes); err != nil {
		return toToolResult(0, nil, &transportError{inner: err})
	}
	if args.QuantityG != nil {
		_ = mw.WriteField("quantity_g", strconv.FormatFloat(*args.QuantityG, 'f', -1, 64))
	}
	if args.LoggedAt != "" {
		_ = mw.WriteField("logged_at", args.LoggedAt)
	}
	if args.MealType != "" {
		_ = mw.WriteField("meal_type", args.MealType)
	}
	if args.Note != "" {
		_ = mw.WriteField("note", args.Note)
	}
	if err := mw.Close(); err != nil {
		return toToolResult(0, nil, &transportError{inner: err})
	}

	// Derive idempotency key from the (image + metadata) shape — same
	// convention the other write tools use. The image bytes ensure two
	// distinct uploads with no explicit key still get distinct keys; an
	// explicit key from the agent overrides the derivation.
	key := effectiveIdempotencyKey(args.IdempotencyKey, "log_meal_from_photo", args)

	status, respBody, err := c.PostMultipart(ctx, "/meals/from_photo", body.Bytes(), mw.FormDataContentType(), key)
	return toToolResult(status, respBody, err)
}

// registerMealPhotoTool registers the one tool that cannot flow through the
// generic registry dispatcher: log_meal_from_photo posts multipart/form-data
// (the documented DD5 exception), which agenttools.HTTPCall does not model. The
// rest of the meals domain is ported onto the shared registry
// (unify-mcp-tool-registry); this bespoke registration survives for the photo
// upload only. Its registration block is unchanged from the prior
// registerMealsTools.
func registerMealPhotoTool(server *mcp.Server, c *apiClient) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "log_meal_from_photo",
		Description: "Log a meal from a photo via Claude Vision. The image is supplied as " +
			"base64-encoded bytes (JPEG or PNG; HEIC is rejected with 415 in v1). Backend resizes " +
			"to max 1568px edge, calls Claude Vision with a tool-forced output, then creates a " +
			"freeform meal entry. Returns the canonical meal block plus an `inference` block with " +
			"model, confidence (0–1), token usage, and image dimensions. Use the freeform path " +
			"directly when the user can describe the meal in text — this tool exists for the " +
			"future MCP-aware UI that passes images through, and for headless test harnesses. " +
			"503 vision_unavailable when ANTHROPIC_API_KEY is not configured on the REST server.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args LogMealFromPhotoArgs) (*mcp.CallToolResult, any, error) {
		return handleLogMealFromPhoto(ctx, c, args), nil, nil
	})
}
