package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type GetAthleteConfigArgs struct{}

func handleGetAthleteConfig(ctx context.Context, c *apiClient, _ GetAthleteConfigArgs) *mcp.CallToolResult {
	status, body, err := c.Get(ctx, "/athlete-config", nil)
	return toToolResult(status, body, err)
}

func registerAthleteConfigTools(server *mcp.Server, c *apiClient) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "athlete_config_get",
		Description: "Fetch the athlete's physiology configuration (singleton): FTP, threshold HR and " +
			"run/swim paces, max HR, lactate-threshold HR, and HR-zone (and optional power-zone) " +
			"boundaries. Returns null before any config has been set. Use to interpret workout " +
			"detail — e.g. to know what heart rate a zone-4 second actually corresponds to.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args GetAthleteConfigArgs) (*mcp.CallToolResult, any, error) {
		return handleGetAthleteConfig(ctx, c, args), nil, nil
	})
}
