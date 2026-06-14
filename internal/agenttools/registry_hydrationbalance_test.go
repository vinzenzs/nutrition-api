package agenttools

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The four hydration-balance tools are registered on the MCP-only surface with
// the expected tiers.
func TestHydrationBalance_RegisteredWithTiers(t *testing.T) {
	specs := ByName(MCPRegistry())
	wantTier := map[string]Tier{
		"log_hydration_balance":    TierWriteAuto,
		"list_hydration_balance":   TierRead,
		"get_hydration_balance":    TierRead,
		"delete_hydration_balance": TierWriteAuto,
	}
	for name, wt := range wantTier {
		s, ok := specs[name]
		require.Truef(t, ok, "tool %s not registered on MCP surface", name)
		assert.Equalf(t, wt, s.Tier, "tier wrong for %s", name)
		assert.NotNilf(t, s.SchemaType, "tool %s should carry a SchemaType", name)
	}
}

// log_hydration_balance → POST /hydration-balance with the snapshot fields,
// dropping idempotency_key. Optional millilitre fields are omitted when absent.
func TestHydrationBalance_Log(t *testing.T) {
	specs := ByName(MCPRegistry())

	// All fields supplied.
	call, err := specs["log_hydration_balance"].Build(json.RawMessage(
		`{"date":"2026-06-14","sweat_loss_ml":1500,"activity_intake_ml":900,"goal_ml":3000,"idempotency_key":"k1"}`))
	require.NoError(t, err)
	assert.Equal(t, "POST", call.Method)
	assert.Equal(t, "/hydration-balance", call.Path)
	assert.Nil(t, call.Query)

	var body map[string]any
	require.NoError(t, json.Unmarshal(call.Body, &body))
	assert.Equal(t, "2026-06-14", body["date"])
	assert.EqualValues(t, 1500, body["sweat_loss_ml"])
	assert.EqualValues(t, 900, body["activity_intake_ml"])
	assert.EqualValues(t, 3000, body["goal_ml"])
	// idempotency_key is never forwarded in the body.
	_, hasKey := body["idempotency_key"]
	assert.False(t, hasKey)

	// A real 0 activity intake must be preserved (pointer, not omitted by value).
	call0, err := specs["log_hydration_balance"].Build(json.RawMessage(
		`{"date":"2026-06-14","sweat_loss_ml":1200,"activity_intake_ml":0}`))
	require.NoError(t, err)
	var body0 map[string]any
	require.NoError(t, json.Unmarshal(call0.Body, &body0))
	assert.EqualValues(t, 0, body0["activity_intake_ml"])

	// Omitted optional fields are absent from the body (omitempty on pointers).
	callMin, err := specs["log_hydration_balance"].Build(json.RawMessage(`{"date":"2026-06-14"}`))
	require.NoError(t, err)
	var bodyMin map[string]any
	require.NoError(t, json.Unmarshal(callMin.Body, &bodyMin))
	assert.Equal(t, "2026-06-14", bodyMin["date"])
	_, hasSweat := bodyMin["sweat_loss_ml"]
	assert.False(t, hasSweat)
	_, hasIntake := bodyMin["activity_intake_ml"]
	assert.False(t, hasIntake)
	_, hasGoal := bodyMin["goal_ml"]
	assert.False(t, hasGoal)
}

// list_hydration_balance → GET /hydration-balance?from&to.
func TestHydrationBalance_List(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["list_hydration_balance"].Build(json.RawMessage(
		`{"from":"2026-06-01","to":"2026-06-14"}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/hydration-balance", call.Path)
	assert.Equal(t, "2026-06-01", call.Query.Get("from"))
	assert.Equal(t, "2026-06-14", call.Query.Get("to"))
	assert.Nil(t, call.Body)
}

// get_hydration_balance → GET /hydration-balance/{date}, date path-escaped.
func TestHydrationBalance_Get(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["get_hydration_balance"].Build(json.RawMessage(`{"date":"2026-06-14"}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/hydration-balance/2026-06-14", call.Path)
	assert.Nil(t, call.Query)
	assert.Nil(t, call.Body)
}

// delete_hydration_balance → DELETE /hydration-balance/{date}, date path-escaped.
func TestHydrationBalance_Delete(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["delete_hydration_balance"].Build(json.RawMessage(
		`{"date":"2026-06-14","idempotency_key":"k1"}`))
	require.NoError(t, err)
	assert.Equal(t, "DELETE", call.Method)
	assert.Equal(t, "/hydration-balance/2026-06-14", call.Path)
	assert.Nil(t, call.Query)
	// The handler attaches the idempotency key out-of-band (Tier.IsWrite); it is
	// never part of the path or body.
	assert.Nil(t, call.Body)
}
