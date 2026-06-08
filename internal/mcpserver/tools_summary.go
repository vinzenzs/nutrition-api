package mcpserver

import (
	"context"
	"net/url"
	"strconv"

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

// RollingSummaryArgs is the input shape for `rolling_summary`. The window is
// [anchor_date - (window_days - 1) days, anchor_date], both inclusive, in the
// requested `tz`.
type RollingSummaryArgs struct {
	AnchorDate string `json:"anchor_date" jsonschema:"calendar date in YYYY-MM-DD (the trailing window ends here, inclusive)"`
	WindowDays int    `json:"window_days" jsonschema:"window size in calendar days; range [2, 30]. Typical values: 3 (acute), 7 (weekly trend), 14 (training-block trend), 30 (block-length trend)."`
	TZ         string `json:"tz,omitempty" jsonschema:"IANA timezone (e.g. Europe/Berlin). If omitted, the REST server uses DEFAULT_USER_TZ."`
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

func handleRollingSummary(ctx context.Context, c *apiClient, args RollingSummaryArgs) *mcp.CallToolResult {
	q := url.Values{}
	q.Set("anchor_date", args.AnchorDate)
	q.Set("window_days", strconv.Itoa(args.WindowDays))
	if args.TZ != "" {
		q.Set("tz", args.TZ)
	}
	status, body, err := c.Get(ctx, "/summary/rolling", q)
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

	mcp.AddTool(server, &mcp.Tool{
		Name: "rolling_summary",
		Description: "Get the trailing-window average of nutrition totals as of `anchor_date`. " +
			"The window is `[anchor_date - (window_days - 1) days, anchor_date]`, BOTH INCLUSIVE, " +
			"in the requested `tz` (omit to use DEFAULT_USER_TZ). IMPORTANT: averages are computed " +
			"across DAYS WITH LOGGED MEALS (`days_with_data`), NOT across `total_days` — a 7-day " +
			"window with 5 days logged returns the 5-day mean. The `days_with_data` and `total_days` " +
			"fields expose the divisor so you can spot sparse windows; surface that to the user when " +
			"they differ. Per-day rows carry `has_data: bool` distinguishing 'no meal logged' from " +
			"'logged a zero-kcal meal.' Typical windows: 3 (acute), 7 (weekly trend), 14 (training-" +
			"block trend), 30 (block-length trend). Adherence is computed against the goal that " +
			"applies AT `anchor_date` (honoring per-date overrides).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args RollingSummaryArgs) (*mcp.CallToolResult, any, error) {
		return handleRollingSummary(ctx, c, args), nil, nil
	})
}
