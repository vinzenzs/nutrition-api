package agenttools

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// daily_summary_get → GET /daily-summary/{date}, date path-escaped, no query/body.
func TestBuild_DailySummaryGet(t *testing.T) {
	specs := ByName(MCPRegistry())

	spec, ok := specs["daily_summary_get"]
	require.True(t, ok, "daily_summary_get must be registered on the MCP surface")
	assert.Equal(t, TierRead, spec.Tier)

	call, err := spec.Build(json.RawMessage(`{"date":"2026-06-14"}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/daily-summary/2026-06-14", call.Path)
	assert.Empty(t, call.Query)
	assert.Empty(t, call.Body)
}

// A date with characters needing escaping is path-escaped exactly as the
// bespoke handler did via url.PathEscape.
func TestBuild_DailySummaryGet_PathEscape(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["daily_summary_get"].Build(json.RawMessage(`{"date":"a b"}`))
	require.NoError(t, err)
	assert.Equal(t, "/daily-summary/a%20b", call.Path)
}
