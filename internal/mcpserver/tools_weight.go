package mcpserver

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type LogWeightArgs struct {
	WeightKg       float64  `json:"weight_kg" jsonschema:"body weight in kilograms; must be greater than zero"`
	LoggedAt       string   `json:"logged_at" jsonschema:"when the measurement was taken, RFC 3339 timestamp"`
	BodyFatPct     *float64 `json:"body_fat_pct,omitempty" jsonschema:"optional body-fat percentage, 0..100"`
	Note           string   `json:"note,omitempty" jsonschema:"optional free-text context (e.g. 'morning, fasted', 'hotel scale', 'post-workout')"`
	IdempotencyKey string   `json:"idempotency_key,omitempty" jsonschema:"optional retry key; if omitted, a stable key is derived from the other args"`
}

type ListWeightsArgs struct {
	From string `json:"from" jsonschema:"inclusive RFC 3339 lower bound on logged_at"`
	To   string `json:"to" jsonschema:"exclusive RFC 3339 upper bound on logged_at; max 92 days from 'from'"`
}

type PatchWeightArgs struct {
	ID             string   `json:"id" jsonschema:"the id of the body-weight entry to update"`
	WeightKg       *float64 `json:"weight_kg,omitempty" jsonschema:"new weight in kg; must be greater than zero if supplied"`
	BodyFatPct     *float64 `json:"body_fat_pct,omitempty" jsonschema:"new body-fat % (0..100) if supplied"`
	LoggedAt       *string  `json:"logged_at,omitempty" jsonschema:"new RFC 3339 timestamp"`
	Note           *string  `json:"note,omitempty" jsonschema:"new note"`
	IdempotencyKey string   `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

type DeleteWeightArgs struct {
	ID             string `json:"id" jsonschema:"the id of the body-weight entry to delete"`
	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

type WeightTrendArgs struct {
	From       string `json:"from" jsonschema:"inclusive start date YYYY-MM-DD"`
	To         string `json:"to" jsonschema:"inclusive end date YYYY-MM-DD; max 366 days from 'from'"`
	WindowDays *int   `json:"window_days,omitempty" jsonschema:"trailing window length in days, 1..30 (default 7)"`
	TZ         string `json:"tz,omitempty" jsonschema:"IANA timezone (e.g. Europe/Berlin); if omitted, the REST server uses DEFAULT_USER_TZ"`
}

func handleLogWeight(ctx context.Context, c *apiClient, args LogWeightArgs) *mcp.CallToolResult {
	payload := struct {
		WeightKg   float64  `json:"weight_kg"`
		LoggedAt   string   `json:"logged_at"`
		BodyFatPct *float64 `json:"body_fat_pct,omitempty"`
		Note       string   `json:"note,omitempty"`
	}{
		WeightKg:   args.WeightKg,
		LoggedAt:   args.LoggedAt,
		BodyFatPct: args.BodyFatPct,
		Note:       args.Note,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return toToolResult(0, nil, &transportError{inner: err})
	}
	key := effectiveIdempotencyKey(args.IdempotencyKey, "log_weight", args)
	status, respBody, err := c.Post(ctx, "/weight", nil, body, key)
	return toToolResult(status, respBody, err)
}

func handleListWeights(ctx context.Context, c *apiClient, args ListWeightsArgs) *mcp.CallToolResult {
	q := url.Values{}
	q.Set("from", args.From)
	q.Set("to", args.To)
	status, body, err := c.Get(ctx, "/weight", q)
	return toToolResult(status, body, err)
}

func handlePatchWeight(ctx context.Context, c *apiClient, args PatchWeightArgs) *mcp.CallToolResult {
	payload := map[string]any{}
	if args.WeightKg != nil {
		payload["weight_kg"] = *args.WeightKg
	}
	if args.BodyFatPct != nil {
		payload["body_fat_pct"] = *args.BodyFatPct
	}
	if args.LoggedAt != nil {
		payload["logged_at"] = *args.LoggedAt
	}
	if args.Note != nil {
		payload["note"] = *args.Note
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return toToolResult(0, nil, &transportError{inner: err})
	}
	key := effectiveIdempotencyKey(args.IdempotencyKey, "patch_weight", args)
	status, respBody, err := c.Patch(ctx, "/weight/"+url.PathEscape(args.ID), body, key)
	return toToolResult(status, respBody, err)
}

func handleDeleteWeight(ctx context.Context, c *apiClient, args DeleteWeightArgs) *mcp.CallToolResult {
	key := effectiveIdempotencyKey(args.IdempotencyKey, "delete_weight", args)
	status, respBody, err := c.Delete(ctx, "/weight/"+url.PathEscape(args.ID), key)
	if err == nil && status == 204 {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: ""}}}
	}
	return toToolResult(status, respBody, err)
}

func handleWeightTrend(ctx context.Context, c *apiClient, args WeightTrendArgs) *mcp.CallToolResult {
	q := url.Values{}
	q.Set("from", args.From)
	q.Set("to", args.To)
	if args.WindowDays != nil {
		q.Set("window_days", strconv.Itoa(*args.WindowDays))
	}
	if args.TZ != "" {
		q.Set("tz", args.TZ)
	}
	status, body, err := c.Get(ctx, "/weight/trend", q)
	return toToolResult(status, body, err)
}

func registerWeightTools(server *mcp.Server, c *apiClient) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "log_weight",
		Description: "Record a body-weight measurement, optionally with body-fat %. Multiple " +
			"measurements per day are fine — the trend tool smooths them. Use `note` for context " +
			"that affects readings (post-workout, post-meal, hotel scale, non-morning timing). " +
			"Body-weight feeds the EA computation, race-day fuelling math, and the trend signal.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args LogWeightArgs) (*mcp.CallToolResult, any, error) {
		return handleLogWeight(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "list_weights",
		Description: "List body-weight entries whose logged_at falls within the half-open [from, to) " +
			"RFC 3339 window. Window is capped at 92 days. Use weight_trend instead when you want a " +
			"smoothed trajectory rather than raw entries.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args ListWeightsArgs) (*mcp.CallToolResult, any, error) {
		return handleListWeights(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "patch_weight",
		Description: "Partially update an existing body-weight entry. Only supplied fields are changed.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args PatchWeightArgs) (*mcp.CallToolResult, any, error) {
		return handlePatchWeight(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_weight",
		Description: "Delete a body-weight entry. Returns an empty result on success.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args DeleteWeightArgs) (*mcp.CallToolResult, any, error) {
		return handleDeleteWeight(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "weight_trend",
		Description: "Return a rolling-average body-weight curve for a date range. `window_days` " +
			"defaults to 7 (suppresses normal daily noise from hydration, glycogen, food in gut). " +
			"Each point carries `sample_count` — a `rolling_avg_kg` from `sample_count: 1` is NOT a " +
			"trend, it's just that one sample. Check `sample_count` before basing decisions on a value.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args WeightTrendArgs) (*mcp.CallToolResult, any, error) {
		return handleWeightTrend(ctx, c, args), nil, nil
	})
}
