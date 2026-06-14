package agenttools

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// daily_context → GET /context/daily with date set; tz omitted when unset.
func TestBuild_DailyContext_DateOnly(t *testing.T) {
	specs := ByName(MCPRegistry())

	spec, ok := specs["daily_context"]
	require.True(t, ok, "daily_context must be registered on the MCP surface")
	assert.Equal(t, TierRead, spec.Tier)

	call, err := spec.Build(json.RawMessage(`{"date":"2026-07-15"}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/context/daily", call.Path)
	assert.Equal(t, "2026-07-15", call.Query.Get("date"))
	assert.Empty(t, call.Query.Get("tz"), "unset tz must not be sent")
	assert.Empty(t, call.Body)
}

// Optional tz is forwarded as a query param when present.
func TestBuild_DailyContext_OptionalTZForwarded(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["daily_context"].Build(json.RawMessage(`{"date":"2026-07-15","tz":"Europe/Berlin"}`))
	require.NoError(t, err)
	assert.Equal(t, "Europe/Berlin", call.Query.Get("tz"))
	assert.Equal(t, "2026-07-15", call.Query.Get("date"))
}

// Empty input still produces a GET with an (empty) date param, matching the
// bespoke handler which unconditionally set date.
func TestBuild_DailyContext_EmptyInput(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["daily_context"].Build(json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/context/daily", call.Path)
	assert.Equal(t, "", call.Query.Get("date"))
	assert.Empty(t, call.Query.Get("tz"))
}
