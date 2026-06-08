package mcpserver

import (
	"context"
	"net/url"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type DailySummaryArgs struct {
	Date     string `json:"date" jsonschema:"calendar date in YYYY-MM-DD"`
	TZ       string `json:"tz,omitempty" jsonschema:"IANA timezone (e.g. Europe/Berlin). If omitted, the REST server uses DEFAULT_USER_TZ."`
	MealType string `json:"meal_type,omitempty" jsonschema:"optional filter: breakfast | lunch | dinner | snack. When set, totals and entries are scoped to that meal type and adherence is omitted."`
}

type RangeSummaryArgs struct {
	From    string `json:"from" jsonschema:"inclusive start date YYYY-MM-DD"`
	To      string `json:"to" jsonschema:"inclusive end date YYYY-MM-DD; max 92 days from 'from'"`
	TZ      string `json:"tz,omitempty" jsonschema:"IANA timezone (e.g. Europe/Berlin). If omitted, the REST server uses DEFAULT_USER_TZ."`
	GroupBy string `json:"group_by,omitempty" jsonschema:"optional: meal_type. When set, each day returns by_meal_type totals instead of a single totals object; adherence is omitted."`
}

func handleDailySummary(ctx context.Context, c *apiClient, args DailySummaryArgs) *mcp.CallToolResult {
	q := url.Values{}
	q.Set("date", args.Date)
	if args.TZ != "" {
		q.Set("tz", args.TZ)
	}
	if args.MealType != "" {
		q.Set("meal_type", args.MealType)
	}
	status, body, err := c.Get(ctx, "/summary/daily", q)
	return toToolResult(status, body, err)
}

func handleRangeSummary(ctx context.Context, c *apiClient, args RangeSummaryArgs) *mcp.CallToolResult {
	q := url.Values{}
	q.Set("from", args.From)
	q.Set("to", args.To)
	if args.TZ != "" {
		q.Set("tz", args.TZ)
	}
	if args.GroupBy != "" {
		q.Set("group_by", args.GroupBy)
	}
	status, body, err := c.Get(ctx, "/summary/range", q)
	return toToolResult(status, body, err)
}

func registerSummaryTools(server *mcp.Server, c *apiClient) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "daily_summary",
		Description: "Get the user's nutriment totals and meal entries for a calendar date in the " +
			"supplied timezone. Returns kcal, protein/carbs/fat/fiber/sugar/salt grams plus each " +
			"meal's effective name and quantity. Omit tz to use the REST server's default timezone.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args DailySummaryArgs) (*mcp.CallToolResult, any, error) {
		return handleDailySummary(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "range_summary",
		Description: "Get per-day nutriment totals across an inclusive date range (max 92 days). " +
			"Useful for 'how did I do this week?' style questions. Omit tz to use the REST server's " +
			"default timezone.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args RangeSummaryArgs) (*mcp.CallToolResult, any, error) {
		return handleRangeSummary(ctx, c, args), nil, nil
	})
}
