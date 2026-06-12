package mcpserver

import (
	"context"
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// GarminLoginArgs is intentionally empty: the bridge holds the credentials, so
// starting the login takes no arguments (design D3 of add-garmin-mcp-login).
type GarminLoginArgs struct{}

// GarminSubmitMFAArgs carries only the ephemeral 6-digit code — the single
// secret that ever transits the agent on this path (never the password/token).
type GarminSubmitMFAArgs struct {
	Code string `json:"code" jsonschema:"the 6-digit MFA code from the user's authenticator app or email"`
}

func handleGarminLogin(ctx context.Context, c *apiClient, _ GarminLoginArgs) *mcp.CallToolResult {
	// One HTTP call, no body, no idempotency key: starting an interactive login
	// is not a replayable write.
	status, body, err := c.Post(ctx, "/garmin/login", nil, nil, "")
	return toToolResult(status, body, err)
}

func handleGarminSubmitMFA(ctx context.Context, c *apiClient, args GarminSubmitMFAArgs) *mcp.CallToolResult {
	body, err := json.Marshal(struct {
		Code string `json:"code"`
	}{Code: args.Code})
	if err != nil {
		return toToolResult(0, nil, &transportError{inner: err})
	}
	status, respBody, err := c.Post(ctx, "/garmin/login/mfa", nil, body, "")
	return toToolResult(status, respBody, err)
}

func registerGarminTools(server *mcp.Server, c *apiClient) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "garmin_login",
		Description: "Start re-linking the user's Garmin account (renews the ~yearly-expiring Garmin token). " +
			"Takes no arguments — the bridge holds the credentials. If the result is `{\"needs_mfa\": true}`, " +
			"ask the user for the 6-digit code from their authenticator app, then call `garmin_submit_mfa` with it. " +
			"A `{\"logged_in\": true}` result means no code was needed and re-linking is already complete. " +
			"A `503 garmin_disabled` result means the Garmin integration is not configured on this server.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args GarminLoginArgs) (*mcp.CallToolResult, any, error) {
		return handleGarminLogin(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "garmin_submit_mfa",
		Description: "Complete a Garmin re-link by submitting the 6-digit MFA code the user read from their " +
			"authenticator. Call this only after `garmin_login` returned `{\"needs_mfa\": true}`. A " +
			"`{\"logged_in\": true}` result means the token was renewed; an error (e.g. `mfa_invalid`) means the " +
			"code was wrong or expired — call `garmin_login` again to restart.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args GarminSubmitMFAArgs) (*mcp.CallToolResult, any, error) {
		return handleGarminSubmitMFA(ctx, c, args), nil, nil
	})
}
