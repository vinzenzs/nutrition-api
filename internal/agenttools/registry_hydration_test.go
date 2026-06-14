package agenttools

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The hydration domain contributes five MCP-only tools: log/list/patch/delete
// plus the volume-only daily summary. Reads are TierRead; mutations are
// TierWriteAuto (the generic dispatcher attaches a derived idempotency key).
func TestHydration_RegisteredWithExpectedTiers(t *testing.T) {
	specs := ByName(MCPRegistry())
	tiers := map[string]Tier{
		"log_hydration":           TierWriteAuto,
		"list_hydration":          TierRead,
		"patch_hydration":         TierWriteAuto,
		"delete_hydration":        TierWriteAuto,
		"daily_hydration_summary": TierRead,
	}
	for name, want := range tiers {
		s, ok := specs[name]
		require.Truef(t, ok, "tool %s not registered on the MCP surface", name)
		assert.Equalf(t, want, s.Tier, "tool %s tier", name)
		assert.Truef(t, s.MCPExposed, "tool %s should be MCP-exposed", name)
		assert.NotNilf(t, s.SchemaType, "tool %s should carry a SchemaType", name)
	}
}

func TestHydration_LogHydration_BuildShape(t *testing.T) {
	specs := ByName(MCPRegistry())

	// Full body: quantity_ml + logged_at + note (+ workout_id omitted).
	call, err := specs["log_hydration"].Build(json.RawMessage(
		`{"quantity_ml":500,"logged_at":"2026-06-07T08:00:00Z","note":"water"}`))
	require.NoError(t, err)
	assert.Equal(t, "POST", call.Method)
	assert.Equal(t, "/hydration", call.Path)
	assert.Empty(t, call.Query)
	assert.JSONEq(t,
		`{"quantity_ml":500,"logged_at":"2026-06-07T08:00:00Z","note":"water"}`,
		string(call.Body))

	// workout_id present is forwarded; idempotency_key is dropped from the body.
	call, err = specs["log_hydration"].Build(json.RawMessage(
		`{"quantity_ml":250,"logged_at":"2026-06-07T08:00:00Z","workout_id":"w1","idempotency_key":"k"}`))
	require.NoError(t, err)
	assert.JSONEq(t,
		`{"quantity_ml":250,"logged_at":"2026-06-07T08:00:00Z","workout_id":"w1"}`,
		string(call.Body))

	// Minimal body: empty note/workout_id are omitted via omitempty.
	call, err = specs["log_hydration"].Build(json.RawMessage(
		`{"quantity_ml":500,"logged_at":"2026-06-07T08:00:00Z"}`))
	require.NoError(t, err)
	assert.JSONEq(t,
		`{"quantity_ml":500,"logged_at":"2026-06-07T08:00:00Z"}`,
		string(call.Body))
}

func TestHydration_ListHydration_BuildShape(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["list_hydration"].Build(json.RawMessage(
		`{"from":"2026-06-01T00:00:00Z","to":"2026-06-08T00:00:00Z"}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/hydration", call.Path)
	assert.Equal(t, "2026-06-01T00:00:00Z", call.Query.Get("from"))
	assert.Equal(t, "2026-06-08T00:00:00Z", call.Query.Get("to"))
	assert.Empty(t, call.Body)
}

func TestHydration_PatchHydration_OnlySuppliedFields(t *testing.T) {
	specs := ByName(MCPRegistry())

	call, err := specs["patch_hydration"].Build(json.RawMessage(
		`{"id":"abc","quantity_ml":250}`))
	require.NoError(t, err)
	assert.Equal(t, "PATCH", call.Method)
	assert.Equal(t, "/hydration/abc", call.Path)
	assert.JSONEq(t, `{"quantity_ml":250}`, string(call.Body))

	// Empty-string workout_id sentinel is forwarded as "" (clears the link).
	call, err = specs["patch_hydration"].Build(json.RawMessage(
		`{"id":"abc","workout_id":""}`))
	require.NoError(t, err)
	assert.JSONEq(t, `{"workout_id":""}`, string(call.Body))

	// No editable fields → empty object body.
	call, err = specs["patch_hydration"].Build(json.RawMessage(`{"id":"abc"}`))
	require.NoError(t, err)
	assert.JSONEq(t, `{}`, string(call.Body))
}

func TestHydration_DeleteHydration_BuildShape(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["delete_hydration"].Build(json.RawMessage(`{"id":"abc"}`))
	require.NoError(t, err)
	assert.Equal(t, "DELETE", call.Method)
	assert.Equal(t, "/hydration/abc", call.Path)
	assert.Empty(t, call.Body)
}

func TestHydration_DailyHydrationSummary_BuildShape(t *testing.T) {
	specs := ByName(MCPRegistry())

	call, err := specs["daily_hydration_summary"].Build(json.RawMessage(
		`{"date":"2026-06-07","tz":"Europe/Berlin"}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/summary/hydration/daily", call.Path)
	assert.Equal(t, "2026-06-07", call.Query.Get("date"))
	assert.Equal(t, "Europe/Berlin", call.Query.Get("tz"))

	// tz omitted when unset.
	call, err = specs["daily_hydration_summary"].Build(json.RawMessage(
		`{"date":"2026-06-07"}`))
	require.NoError(t, err)
	assert.Equal(t, "2026-06-07", call.Query.Get("date"))
	assert.False(t, call.Query.Has("tz"))
}
