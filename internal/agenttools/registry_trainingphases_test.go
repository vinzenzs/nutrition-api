package agenttools

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The training-phases domain contributes exactly these nine MCP tools (phase
// CRUD + goal-template CRUD), with the tiers the bespoke registrations implied
// (reads → TierRead; mutations → write).
func TestTrainingPhases_SurfaceAndTiers(t *testing.T) {
	specs := ByName(MCPRegistry())
	wantTier := map[string]Tier{
		"create_phase":         TierWriteAuto,
		"list_phases":          TierRead,
		"get_phase":            TierRead,
		"update_phase":         TierWriteAuto,
		"delete_phase":         TierWriteAuto,
		"set_goal_template":    TierWriteAuto,
		"list_goal_templates":  TierRead,
		"get_goal_template":    TierRead,
		"delete_goal_template": TierWriteAuto,
	}
	for name, tier := range wantTier {
		s, ok := specs[name]
		require.Truef(t, ok, "missing MCP tool %s", name)
		assert.Equalf(t, tier, s.Tier, "tier wrong for %s", name)
		assert.NotNilf(t, s.SchemaType, "tool %s must carry a SchemaType for MCP schema reflection", name)
		assert.Truef(t, s.MCPExposed, "tool %s should be MCP-exposed", name)
	}
}

// create_phase → POST /phases with the body, idempotency_key dropped.
func TestTrainingPhases_CreatePhase(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["create_phase"].Build(json.RawMessage(
		`{"name":"build-1","type":"build","start_date":"2026-07-01","end_date":"2026-07-28","idempotency_key":"my-key"}`))
	require.NoError(t, err)
	assert.Equal(t, "POST", call.Method)
	assert.Equal(t, "/phases", call.Path)
	assert.Empty(t, call.Query)

	var body map[string]any
	require.NoError(t, json.Unmarshal(call.Body, &body))
	assert.Equal(t, "build-1", body["name"])
	assert.Equal(t, "build", body["type"])
	assert.Equal(t, "2026-07-01", body["start_date"])
	assert.Equal(t, "2026-07-28", body["end_date"])
	// idempotency_key is a header, not a body field.
	assert.NotContains(t, body, "idempotency_key")
	// optionals omitted when not supplied (omitempty).
	assert.NotContains(t, body, "default_template_id")
	assert.NotContains(t, body, "notes")
}

// list_phases → GET /phases?from&to, no idempotency.
func TestTrainingPhases_ListPhases(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["list_phases"].Build(json.RawMessage(
		`{"from":"2026-07-01","to":"2026-07-31"}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/phases", call.Path)
	assert.Equal(t, "2026-07-01", call.Query.Get("from"))
	assert.Equal(t, "2026-07-31", call.Query.Get("to"))
	assert.Nil(t, call.Body)
}

// get_phase → GET /phases/{id} with the id path-escaped.
func TestTrainingPhases_GetPhase(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["get_phase"].Build(json.RawMessage(`{"phase_id":"abc-123"}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/phases/abc-123", call.Path)
}

// update_phase → PATCH /phases/{id} with the body minus phase_id.
func TestTrainingPhases_UpdatePhase(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["update_phase"].Build(json.RawMessage(
		`{"phase_id":"abc-123","name":"build-1-revised"}`))
	require.NoError(t, err)
	assert.Equal(t, "PATCH", call.Method)
	assert.Equal(t, "/phases/abc-123", call.Path)

	var body map[string]any
	require.NoError(t, json.Unmarshal(call.Body, &body))
	assert.NotContains(t, body, "phase_id", "phase_id consumed for the URL path; not in body")
	assert.Equal(t, "build-1-revised", body["name"])
	// untouched fields omitted (omitempty pointers).
	assert.NotContains(t, body, "default_template_id")
}

// update_phase tri-state: empty string clears the template link (sentinel
// forwarded to the backend, NOT dropped by omitempty since the pointer is set).
func TestTrainingPhases_UpdatePhaseClearsTemplate(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["update_phase"].Build(json.RawMessage(
		`{"phase_id":"abc","default_template_id":""}`))
	require.NoError(t, err)
	var body map[string]any
	require.NoError(t, json.Unmarshal(call.Body, &body))
	v, ok := body["default_template_id"]
	require.True(t, ok, "empty-string sentinel must be forwarded")
	assert.Equal(t, "", v)
}

// delete_phase → DELETE /phases/{id}.
func TestTrainingPhases_DeletePhase(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["delete_phase"].Build(json.RawMessage(`{"phase_id":"abc"}`))
	require.NoError(t, err)
	assert.Equal(t, "DELETE", call.Method)
	assert.Equal(t, "/phases/abc", call.Path)
}

// set_goal_template → PUT /goal-templates/{name}; name is the path, not body.
func TestTrainingPhases_SetGoalTemplate(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["set_goal_template"].Build(json.RawMessage(
		`{"name":"weekday-easy","kcal":{"min":2090,"max":2310}}`))
	require.NoError(t, err)
	assert.Equal(t, "PUT", call.Method)
	assert.Equal(t, "/goal-templates/weekday-easy", call.Path)

	var body map[string]any
	require.NoError(t, json.Unmarshal(call.Body, &body))
	assert.NotContains(t, body, "name", "name is the URL path, not a body field")
	kcal, ok := body["kcal"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(2090), kcal["min"])
	assert.Equal(t, float64(2310), kcal["max"])
	// absent nutrient bounds omitted (omitempty) → stored NULL by the backend.
	assert.NotContains(t, body, "protein_g")
}

// list_goal_templates → GET /goal-templates.
func TestTrainingPhases_ListGoalTemplates(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["list_goal_templates"].Build(json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/goal-templates", call.Path)
}

// get_goal_template → GET /goal-templates/{name}.
func TestTrainingPhases_GetGoalTemplate(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["get_goal_template"].Build(json.RawMessage(`{"name":"weekday-easy"}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/goal-templates/weekday-easy", call.Path)
}

// delete_goal_template → DELETE /goal-templates/{name}.
func TestTrainingPhases_DeleteGoalTemplate(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["delete_goal_template"].Build(json.RawMessage(`{"name":"weekday-easy"}`))
	require.NoError(t, err)
	assert.Equal(t, "DELETE", call.Method)
	assert.Equal(t, "/goal-templates/weekday-easy", call.Path)
}
