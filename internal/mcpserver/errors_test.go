package mcpserver

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestToToolResult_2xxIsSuccess(t *testing.T) {
	r := toToolResult(200, []byte(`{"ok":true}`), nil)
	assert.False(t, r.IsError)
	require.Len(t, r.Content, 1)
	tc, ok := r.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	assert.Equal(t, `{"ok":true}`, tc.Text)
}

func TestToToolResult_4xxIsError(t *testing.T) {
	body := []byte(`{"error":"product_not_found","barcode":"X","next":"POST /meals/freeform"}`)
	r := toToolResult(404, body, nil)
	assert.True(t, r.IsError)
	tc, ok := r.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	assert.JSONEq(t, string(body), tc.Text, "REST error body must be forwarded verbatim")
}

func TestToToolResult_5xxIsError(t *testing.T) {
	body := []byte(`{"error":"upstream_timeout","retry_after_seconds":30}`)
	r := toToolResult(504, body, nil)
	assert.True(t, r.IsError)
	tc, ok := r.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	assert.JSONEq(t, string(body), tc.Text)
}

func TestToToolResult_TransportErrorIsSynthesized(t *testing.T) {
	r := toToolResult(0, nil, &transportError{inner: errors.New("dial tcp: connection refused")})
	assert.True(t, r.IsError)
	tc, ok := r.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	var payload map[string]string
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &payload))
	assert.Equal(t, "transport", payload["error"])
	assert.Contains(t, payload["detail"], "connection refused")
}
