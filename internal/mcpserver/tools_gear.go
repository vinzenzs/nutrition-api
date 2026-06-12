package mcpserver

import (
	"context"
	"net/url"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ListGearArgs struct {
	Retired *bool `json:"retired,omitempty" jsonschema:"optional filter by retirement state (true returns only retired gear, false only active)"`
}

func handleListGear(ctx context.Context, c *apiClient, args ListGearArgs) *mcp.CallToolResult {
	q := url.Values{}
	if args.Retired != nil {
		if *args.Retired {
			q.Set("retired", "true")
		} else {
			q.Set("retired", "false")
		}
	}
	status, body, err := c.Get(ctx, "/gear", q)
	return toToolResult(status, body, err)
}

func registerGearTools(server *mcp.Server, c *apiClient) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "gear_list",
		Description: "List the athlete's Garmin gear inventory (shoes, bikes, other equipment) with " +
			"accumulated distance, activity count, and retirement state. Use for gear-rotation context — " +
			"e.g. flagging shoes that are near or past their mileage budget. Optional `retired` filter.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args ListGearArgs) (*mcp.CallToolResult, any, error) {
		return handleListGear(ctx, c, args), nil, nil
	})
}
