package mcpserver

import (
	"context"
	"net/url"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// DailyContextArgs is the input shape for the daily_context tool. The
// wrapper is read-only — no idempotency_key field.
type DailyContextArgs struct {
	Date string `json:"date" jsonschema:"calendar date in YYYY-MM-DD"`
	TZ   string `json:"tz,omitempty" jsonschema:"IANA timezone (defaults to DEFAULT_USER_TZ)"`
}

func handleDailyContext(ctx context.Context, c *apiClient, args DailyContextArgs) *mcp.CallToolResult {
	q := url.Values{}
	q.Set("date", args.Date)
	if args.TZ != "" {
		q.Set("tz", args.TZ)
	}
	status, body, err := c.Get(ctx, "/context/daily", q)
	return toToolResult(status, body, err)
}

func registerDailyContextTools(server *mcp.Server, c *apiClient) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "daily_context",
		Description: "Get the day's full context bundle in one call: adherence + nutrition totals + hydration ml + " +
			"today's workouts + workout-fuel entries + body-weight state (with carryover from the most recent prior " +
			"entry when no fresh log) + training-phase context + goal-override presence. Recommended as the FIRST " +
			"call of a session — collapses what would otherwise be 5-7 separate tool calls (daily_summary, " +
			"daily_hydration_summary, list_workouts, list_workout_fuel, list_weights, get_daily_goal_override, " +
			"list_phases). For deep dives into one slice — per-entry breakdowns, full meal lists, range queries — " +
			"use the dedicated tools; they include the per-entry detail this aggregator deliberately omits. " +
			"Read-only.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args DailyContextArgs) (*mcp.CallToolResult, any, error) {
		return handleDailyContext(ctx, c, args), nil, nil
	})
}
