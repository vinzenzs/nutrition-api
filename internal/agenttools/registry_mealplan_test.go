package agenttools

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The mealplan domain contributes five MCP tools: one read (list) and four
// writes (create/update/delete/mark-eaten). Names are shared with the chat
// surface, but MCPRegistry() resolves to these MCP-exposed entries.
func TestMealPlan_RegisteredOnMCPSurface(t *testing.T) {
	specs := ByName(MCPRegistry())
	cases := map[string]Tier{
		"create_planned_meal":     TierWriteAuto,
		"list_planned_meals":      TierRead,
		"update_planned_meal":     TierWriteAuto,
		"delete_planned_meal":     TierWriteAuto,
		"mark_planned_meal_eaten": TierWriteAuto,
	}
	for name, tier := range cases {
		s, ok := specs[name]
		require.Truef(t, ok, "tool %s not registered on the MCP surface", name)
		assert.Equalf(t, tier, s.Tier, "tool %s tier", name)
		assert.Truef(t, s.MCPExposed, "tool %s should be MCP-exposed", name)
		assert.NotNilf(t, s.SchemaType, "tool %s should carry a SchemaType", name)
	}
}

func TestMealPlan_CreateBuildShape(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["create_planned_meal"].Build(json.RawMessage(
		`{"plan_date":"2026-06-12","slot":"dinner","product_id":"prod-1","quantity_g":450,"idempotency_key":"k1"}`))
	require.NoError(t, err)
	assert.Equal(t, "POST", call.Method)
	assert.Equal(t, "/plan", call.Path)
	assert.Empty(t, call.Query)
	body := string(call.Body)
	assert.Contains(t, body, `"plan_date":"2026-06-12"`)
	assert.Contains(t, body, `"slot":"dinner"`)
	assert.Contains(t, body, `"product_id":"prod-1"`)
	assert.Contains(t, body, `"quantity_g":450`)
	// idempotency_key is never part of the REST body.
	assert.NotContains(t, body, "idempotency_key")
	// notes omitted → not serialized.
	assert.NotContains(t, body, "notes")
}

func TestMealPlan_ListBuildShape(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["list_planned_meals"].Build(json.RawMessage(`{"from":"2026-06-12","to":"2026-06-14"}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/plan", call.Path)
	assert.Equal(t, "2026-06-12", call.Query.Get("from"))
	assert.Equal(t, "2026-06-14", call.Query.Get("to"))
	assert.Empty(t, call.Body)
}

func TestMealPlan_UpdateBuildShape(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["update_planned_meal"].Build(json.RawMessage(`{"id":"p1","status":"skipped"}`))
	require.NoError(t, err)
	assert.Equal(t, "PATCH", call.Method)
	assert.Equal(t, "/plan/p1", call.Path)
	body := string(call.Body)
	assert.Contains(t, body, `"status":"skipped"`)
	// omitted fields must not be sent.
	assert.False(t, strings.Contains(body, "plan_date"), "omitted fields must not be serialized")
	assert.NotContains(t, body, "idempotency_key")
}

func TestMealPlan_UpdateRequiresID(t *testing.T) {
	specs := ByName(MCPRegistry())
	_, err := specs["update_planned_meal"].Build(json.RawMessage(`{"status":"skipped"}`))
	require.Error(t, err)
}

func TestMealPlan_DeleteBuildShape(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["delete_planned_meal"].Build(json.RawMessage(`{"id":"p1"}`))
	require.NoError(t, err)
	assert.Equal(t, "DELETE", call.Method)
	assert.Equal(t, "/plan/p1", call.Path)
	assert.Empty(t, call.Query)
}

func TestMealPlan_DeleteRequiresID(t *testing.T) {
	specs := ByName(MCPRegistry())
	_, err := specs["delete_planned_meal"].Build(json.RawMessage(`{}`))
	require.Error(t, err)
}

func TestMealPlan_MarkEatenBuildShape(t *testing.T) {
	specs := ByName(MCPRegistry())
	// minimal: id only → empty body object, eaten path.
	call, err := specs["mark_planned_meal_eaten"].Build(json.RawMessage(`{"id":"p1"}`))
	require.NoError(t, err)
	assert.Equal(t, "POST", call.Method)
	assert.Equal(t, "/plan/p1/eaten", call.Path)
	assert.Equal(t, "{}", string(call.Body))

	// with overrides → both fields serialized.
	call, err = specs["mark_planned_meal_eaten"].Build(json.RawMessage(
		`{"id":"p1","quantity_g":300,"logged_at":"2026-06-12T10:00:00Z"}`))
	require.NoError(t, err)
	body := string(call.Body)
	assert.Contains(t, body, `"quantity_g":300`)
	assert.Contains(t, body, `"logged_at":"2026-06-12T10:00:00Z"`)
}

func TestMealPlan_MarkEatenRequiresID(t *testing.T) {
	specs := ByName(MCPRegistry())
	_, err := specs["mark_planned_meal_eaten"].Build(json.RawMessage(`{"quantity_g":300}`))
	require.Error(t, err)
}
