package agenttools

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// get_goals → GET /goals, no query/body. Pure read.
func TestBuild_GetGoals(t *testing.T) {
	specs := ByName(MCPRegistry())

	spec, ok := specs["get_goals"]
	require.True(t, ok, "get_goals must be registered on the MCP surface")
	assert.Equal(t, TierRead, spec.Tier)

	call, err := spec.Build(json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/goals", call.Path)
	assert.Empty(t, call.Query)
	assert.Empty(t, call.Body)
}

// get_goals tolerates empty input (no required fields).
func TestBuild_GetGoals_EmptyInput(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["get_goals"].Build(nil)
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/goals", call.Path)
}

// set_goals → PUT /goals. Full-replace: only supplied goal fields appear in the
// body, each as a unified {min?, max?} range. Mirrors the bespoke handler's
// body shape exactly.
func TestBuild_SetGoals(t *testing.T) {
	specs := ByName(MCPRegistry())

	spec, ok := specs["set_goals"]
	require.True(t, ok, "set_goals must be registered on the MCP surface")
	// Writes get an auto-derived idempotency key from the dispatcher; PUT is
	// handled centrally (the header is skipped on PUT).
	assert.True(t, spec.Tier.IsWrite())
	assert.Equal(t, TierWriteAuto, spec.Tier)

	in := json.RawMessage(`{"kcal":{"min":2090,"max":2310},"protein_g":{"min":150,"max":190}}`)
	call, err := spec.Build(in)
	require.NoError(t, err)
	assert.Equal(t, "PUT", call.Method)
	assert.Equal(t, "/goals", call.Path)
	assert.Empty(t, call.Query)
	assert.JSONEq(t,
		`{"kcal":{"min":2090,"max":2310},"protein_g":{"min":150,"max":190}}`,
		string(call.Body))
}

// A min-only / max-only field serializes with just that bound, and omitted goal
// fields are absent from the body (clear-on-omit full-replace semantics).
func TestBuild_SetGoals_PartialBounds(t *testing.T) {
	specs := ByName(MCPRegistry())
	in := json.RawMessage(`{"fiber_g":{"min":30},"sugar_g":{"max":50},"iron_mg":{"min":14}}`)
	call, err := specs["set_goals"].Build(in)
	require.NoError(t, err)
	assert.Equal(t, "PUT", call.Method)
	assert.Equal(t, "/goals", call.Path)
	assert.JSONEq(t,
		`{"fiber_g":{"min":30},"sugar_g":{"max":50},"iron_mg":{"min":14}}`,
		string(call.Body))
}

// An empty set_goals call clears every goal: an empty JSON object body.
func TestBuild_SetGoals_EmptyClearsAll(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["set_goals"].Build(json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.Equal(t, "PUT", call.Method)
	assert.Equal(t, "/goals", call.Path)
	assert.JSONEq(t, `{}`, string(call.Body))
}
