package agenttools

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// set_daily_goal_override → PUT /goals/overrides/{date}. Full-replace: only
// supplied goal fields appear in the body, each as a unified {min?, max?} range.
// Writes get an auto-derived idempotency key from the dispatcher; PUT is handled
// centrally (the header is skipped on PUT), so Build must NOT set a query.
func TestBuild_SetDailyGoalOverride(t *testing.T) {
	specs := ByName(MCPRegistry())

	spec, ok := specs["set_daily_goal_override"]
	require.True(t, ok, "set_daily_goal_override must be registered on the MCP surface")
	assert.True(t, spec.Tier.IsWrite())
	assert.Equal(t, TierWriteAuto, spec.Tier)

	in := json.RawMessage(`{"date":"2026-06-15","kcal":{"min":2280,"max":2520}}`)
	call, err := spec.Build(in)
	require.NoError(t, err)
	assert.Equal(t, "PUT", call.Method)
	assert.Equal(t, "/goals/overrides/2026-06-15", call.Path)
	assert.Empty(t, call.Query)
	// date is a path segment, not part of the body.
	assert.JSONEq(t, `{"kcal":{"min":2280,"max":2520}}`, string(call.Body))
}

// Min-only / max-only fields serialize with just that bound; omitted goal fields
// are absent from the body (full-replace clear-on-omit semantics).
func TestBuild_SetDailyGoalOverride_PartialBounds(t *testing.T) {
	specs := ByName(MCPRegistry())
	in := json.RawMessage(`{"date":"2026-06-15","fiber_g":{"min":30},"sugar_g":{"max":50}}`)
	call, err := specs["set_daily_goal_override"].Build(in)
	require.NoError(t, err)
	assert.Equal(t, "PUT", call.Method)
	assert.Equal(t, "/goals/overrides/2026-06-15", call.Path)
	assert.JSONEq(t, `{"fiber_g":{"min":30},"sugar_g":{"max":50}}`, string(call.Body))
}

// An override with no nutrient fields marshals to an empty JSON object body.
func TestBuild_SetDailyGoalOverride_EmptyBody(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["set_daily_goal_override"].Build(json.RawMessage(`{"date":"2026-06-15"}`))
	require.NoError(t, err)
	assert.Equal(t, "PUT", call.Method)
	assert.Equal(t, "/goals/overrides/2026-06-15", call.Path)
	assert.JSONEq(t, `{}`, string(call.Body))
}

// get_daily_goal_override → GET /goals/overrides/{date}, no query/body.
func TestBuild_GetDailyGoalOverride(t *testing.T) {
	specs := ByName(MCPRegistry())

	spec, ok := specs["get_daily_goal_override"]
	require.True(t, ok, "get_daily_goal_override must be registered on the MCP surface")
	assert.Equal(t, TierRead, spec.Tier)

	call, err := spec.Build(json.RawMessage(`{"date":"2026-06-15"}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/goals/overrides/2026-06-15", call.Path)
	assert.Empty(t, call.Query)
	assert.Empty(t, call.Body)
}

// delete_daily_goal_override → DELETE /goals/overrides/{date}. Mutating write
// (gets an auto-derived key from the dispatcher); the idempotency_key arg field
// is schema-only and never appears in the path/query/body.
func TestBuild_DeleteDailyGoalOverride(t *testing.T) {
	specs := ByName(MCPRegistry())

	spec, ok := specs["delete_daily_goal_override"]
	require.True(t, ok, "delete_daily_goal_override must be registered on the MCP surface")
	assert.True(t, spec.Tier.IsWrite())
	assert.Equal(t, TierWriteAuto, spec.Tier)

	call, err := spec.Build(json.RawMessage(`{"date":"2026-06-15"}`))
	require.NoError(t, err)
	assert.Equal(t, "DELETE", call.Method)
	assert.Equal(t, "/goals/overrides/2026-06-15", call.Path)
	assert.Empty(t, call.Query)
	assert.Empty(t, call.Body)
}

// list_daily_goal_overrides → GET /goals/overrides?from=..&to=..
func TestBuild_ListDailyGoalOverrides(t *testing.T) {
	specs := ByName(MCPRegistry())

	spec, ok := specs["list_daily_goal_overrides"]
	require.True(t, ok, "list_daily_goal_overrides must be registered on the MCP surface")
	assert.Equal(t, TierRead, spec.Tier)

	call, err := spec.Build(json.RawMessage(`{"from":"2026-06-01","to":"2026-06-30"}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/goals/overrides", call.Path)
	assert.Equal(t, "2026-06-01", call.Query.Get("from"))
	assert.Equal(t, "2026-06-30", call.Query.Get("to"))
	assert.Empty(t, call.Body)
}
