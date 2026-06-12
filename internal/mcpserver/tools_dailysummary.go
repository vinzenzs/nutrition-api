package mcpserver

import (
	"context"
	"net/url"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type GetDailySummaryArgs struct {
	Date string `json:"date" jsonschema:"the snapshot date YYYY-MM-DD"`
}

func handleGetDailySummary(ctx context.Context, c *apiClient, args GetDailySummaryArgs) *mcp.CallToolResult {
	status, body, err := c.Get(ctx, "/daily-summary/"+url.PathEscape(args.Date), nil)
	return toToolResult(status, body, err)
}

func registerDailySummaryTools(server *mcp.Server, c *apiClient) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "daily_summary_get",
		Description: "Fetch Garmin's whole-day energy/activity snapshot for a single date (YYYY-MM-DD): " +
			"active vs resting vs total kcal, steps, floors, intensity minutes, distance. This is the " +
			"total-daily-expenditure context — including non-workout movement (NEAT) — that the " +
			"energy-availability number deliberately excludes. Read it alongside EA, never as a substitute " +
			"for EA's exercise-burn denominator.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args GetDailySummaryArgs) (*mcp.CallToolResult, any, error) {
		return handleGetDailySummary(ctx, c, args), nil, nil
	})
}
