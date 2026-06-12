package mcpserver

import (
	"context"
	"net/url"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ListPersonalRecordsArgs struct {
	PRType string `json:"pr_type,omitempty" jsonschema:"optional filter to a single PR type (e.g. 5k, 10k, longest-ride)"`
}

func handleListPersonalRecords(ctx context.Context, c *apiClient, args ListPersonalRecordsArgs) *mcp.CallToolResult {
	q := url.Values{}
	if args.PRType != "" {
		q.Set("pr_type", args.PRType)
	}
	status, body, err := c.Get(ctx, "/personal-records", q)
	return toToolResult(status, body, err)
}

func registerPersonalRecordsTools(server *mcp.Server, c *apiClient) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "personal_records_list",
		Description: "List the athlete's Garmin personal records (fastest 5k/10k, longest ride, …) with " +
			"value, unit, and when each was achieved, most recent first. Use for PR-freshness coaching " +
			"context — e.g. framing race-prep advice around how sharp the athlete's top-end is. Optional " +
			"`pr_type` filter.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args ListPersonalRecordsArgs) (*mcp.CallToolResult, any, error) {
		return handleListPersonalRecords(ctx, c, args), nil, nil
	})
}
