package mcpserver

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type LogHydrationArgs struct {
	QuantityMl     float64 `json:"quantity_ml" jsonschema:"volume drunk in millilitres; must be greater than zero"`
	LoggedAt       string  `json:"logged_at" jsonschema:"when the drink was consumed, RFC 3339 timestamp"`
	Note           string  `json:"note,omitempty" jsonschema:"optional free-text beverage context (e.g. 'water', 'iced coffee', 'electrolytes')"`
	WorkoutID      string  `json:"workout_id,omitempty" jsonschema:"optional UUID of an existing workout to link this hydration entry to. The link is metadata; workout_fueling_summary aggregates by logged_at time-window matching, not by this tag."`
	IdempotencyKey string  `json:"idempotency_key,omitempty" jsonschema:"optional retry key; if omitted, a stable key is derived from the other args. To log the same drink twice, pass a distinct key."`
}

type ListHydrationArgs struct {
	From string `json:"from" jsonschema:"inclusive RFC 3339 lower bound on logged_at"`
	To   string `json:"to" jsonschema:"exclusive RFC 3339 upper bound on logged_at; max 92 days from 'from'"`
}

type PatchHydrationArgs struct {
	ID             string   `json:"id" jsonschema:"the id of the hydration entry to update"`
	QuantityMl     *float64 `json:"quantity_ml,omitempty" jsonschema:"new volume in millilitres; must be greater than zero if supplied"`
	LoggedAt       *string  `json:"logged_at,omitempty" jsonschema:"new RFC 3339 timestamp"`
	Note           *string  `json:"note,omitempty" jsonschema:"new beverage note"`
	// WorkoutID supports the empty-string sentinel: \"<uuid>\" sets, \"\" clears, missing leaves unchanged.
	WorkoutID      *string  `json:"workout_id,omitempty" jsonschema:"new workout link: \"<uuid>\" sets, \"\" clears, omit to leave unchanged"`
	IdempotencyKey string   `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

type DeleteHydrationArgs struct {
	ID             string `json:"id" jsonschema:"the id of the hydration entry to delete"`
	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

type DailyHydrationSummaryArgs struct {
	Date string `json:"date" jsonschema:"calendar date in YYYY-MM-DD"`
	TZ   string `json:"tz,omitempty" jsonschema:"IANA timezone (e.g. Europe/Berlin). If omitted, the REST server uses DEFAULT_USER_TZ."`
}

func handleLogHydration(ctx context.Context, c *apiClient, args LogHydrationArgs) *mcp.CallToolResult {
	body, err := json.Marshal(struct {
		QuantityMl float64 `json:"quantity_ml"`
		LoggedAt   string  `json:"logged_at"`
		Note       string  `json:"note,omitempty"`
		WorkoutID  string  `json:"workout_id,omitempty"`
	}{
		QuantityMl: args.QuantityMl,
		LoggedAt:   args.LoggedAt,
		Note:       args.Note,
		WorkoutID:  args.WorkoutID,
	})
	if err != nil {
		return toToolResult(0, nil, &transportError{inner: err})
	}
	key := effectiveIdempotencyKey(args.IdempotencyKey, "log_hydration", args)
	status, respBody, err := c.Post(ctx, "/hydration", nil, body, key)
	return toToolResult(status, respBody, err)
}

func handleListHydration(ctx context.Context, c *apiClient, args ListHydrationArgs) *mcp.CallToolResult {
	q := url.Values{}
	q.Set("from", args.From)
	q.Set("to", args.To)
	status, body, err := c.Get(ctx, "/hydration", q)
	return toToolResult(status, body, err)
}

func handlePatchHydration(ctx context.Context, c *apiClient, args PatchHydrationArgs) *mcp.CallToolResult {
	payload := map[string]any{}
	if args.QuantityMl != nil {
		payload["quantity_ml"] = *args.QuantityMl
	}
	if args.LoggedAt != nil {
		payload["logged_at"] = *args.LoggedAt
	}
	if args.Note != nil {
		payload["note"] = *args.Note
	}
	if args.WorkoutID != nil {
		payload["workout_id"] = *args.WorkoutID
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return toToolResult(0, nil, &transportError{inner: err})
	}
	key := effectiveIdempotencyKey(args.IdempotencyKey, "patch_hydration", args)
	status, respBody, err := c.Patch(ctx, "/hydration/"+url.PathEscape(args.ID), body, key)
	return toToolResult(status, respBody, err)
}

func handleDeleteHydration(ctx context.Context, c *apiClient, args DeleteHydrationArgs) *mcp.CallToolResult {
	key := effectiveIdempotencyKey(args.IdempotencyKey, "delete_hydration", args)
	status, respBody, err := c.Delete(ctx, "/hydration/"+url.PathEscape(args.ID), key)
	if err == nil && status == 204 {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: ""}}}
	}
	return toToolResult(status, respBody, err)
}

func handleDailyHydrationSummary(ctx context.Context, c *apiClient, args DailyHydrationSummaryArgs) *mcp.CallToolResult {
	q := url.Values{}
	q.Set("date", args.Date)
	if args.TZ != "" {
		q.Set("tz", args.TZ)
	}
	status, body, err := c.Get(ctx, "/summary/hydration/daily", q)
	return toToolResult(status, body, err)
}

func registerHydrationTools(server *mcp.Server, c *apiClient) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "log_hydration",
		Description: "Record a volume of fluid the user drank at a specific time. The optional `note` " +
			"carries beverage context (e.g. 'water', 'iced coffee', 'electrolytes'). Use this for ANY " +
			"drink — water, coffee, sports drinks. For beverages with nutriments (Coke, juice), " +
			"additionally log via log_meal_freeform with the macros.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args LogHydrationArgs) (*mcp.CallToolResult, any, error) {
		return handleLogHydration(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "list_hydration",
		Description: "List hydration entries whose logged_at falls within the half-open [from, to) " +
			"RFC 3339 window. Window is capped at 92 days. Use daily_hydration_summary instead when " +
			"you want a one-day total without paging through individual entries.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args ListHydrationArgs) (*mcp.CallToolResult, any, error) {
		return handleListHydration(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "patch_hydration",
		Description: "Partially update an existing hydration entry. Only supplied fields are changed.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args PatchHydrationArgs) (*mcp.CallToolResult, any, error) {
		return handlePatchHydration(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_hydration",
		Description: "Delete a hydration entry. Returns an empty result on success.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args DeleteHydrationArgs) (*mcp.CallToolResult, any, error) {
		return handleDeleteHydration(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "daily_hydration_summary",
		Description: "Return the total ml and per-entry list for one calendar day. This is the " +
			"volume-only summary — separate from daily_summary, which is the nutrient-only summary. " +
			"Combine both when the user asks 'how did I do today?'",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args DailyHydrationSummaryArgs) (*mcp.CallToolResult, any, error) {
		return handleDailyHydrationSummary(ctx, c, args), nil, nil
	})
}
