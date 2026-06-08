package mcpserver

import (
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// toToolResult turns an REST API response into an MCP tool result.
//   - Transport errors (network failure) → isError=true, body
//     {"error":"transport","detail":"…"}.
//   - HTTP 2xx → success result with content = response body.
//   - HTTP non-2xx → isError=true result with content = response body verbatim.
//
// The body is always JSON; we return it as text content because the
// agent reads tool output as text and our REST API only emits JSON.
func toToolResult(status int, body []byte, err error) *mcp.CallToolResult {
	if err != nil {
		envelope, _ := json.Marshal(map[string]string{
			"error":  "transport",
			"detail": err.Error(),
		})
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(envelope)}},
			IsError: true,
		}
	}
	result := &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(body)}},
	}
	if status < 200 || status >= 300 {
		result.IsError = true
	}
	return result
}
