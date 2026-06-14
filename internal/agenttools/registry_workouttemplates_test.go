package agenttools

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The workout-template domain contributes exactly these five MCP tools, with the
// tiers the bespoke registrations implied (reads → TierRead; mutations → write).
func TestWorkoutTemplates_SurfaceAndTiers(t *testing.T) {
	specs := ByName(MCPRegistry())
	wantTier := map[string]Tier{
		"create_workout_template": TierWriteAuto,
		"list_workout_templates":  TierRead,
		"get_workout_template":    TierRead,
		"patch_workout_template":  TierWriteAuto,
		"delete_workout_template": TierWriteAuto,
	}
	for name, tier := range wantTier {
		s, ok := specs[name]
		require.Truef(t, ok, "missing MCP tool %s", name)
		assert.Equalf(t, tier, s.Tier, "tier wrong for %s", name)
		assert.NotNilf(t, s.SchemaType, "tool %s must carry a SchemaType for MCP schema reflection", name)
		assert.Truef(t, s.MCPExposed, "tool %s should be MCP-exposed", name)
	}
}

// create_workout_template → POST /workout-templates with the full body, the
// idempotency_key dropped.
func TestWorkoutTemplates_Create(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["create_workout_template"].Build(json.RawMessage(
		`{"sport":"run","name":"Tempo","description":"easy","estimated_duration_sec":3600,` +
			`"steps":[{"type":"step","intent":"warmup","duration":{"kind":"time","seconds":600}}],` +
			`"idempotency_key":"k1"}`))
	require.NoError(t, err)
	assert.Equal(t, "POST", call.Method)
	assert.Equal(t, "/workout-templates", call.Path)

	var body map[string]any
	require.NoError(t, json.Unmarshal(call.Body, &body))
	assert.Equal(t, "run", body["sport"])
	assert.Equal(t, "Tempo", body["name"])
	assert.Equal(t, "easy", body["description"])
	assert.Equal(t, float64(3600), body["estimated_duration_sec"])
	assert.Contains(t, body, "steps")
	// idempotency_key must not be forwarded in the REST body.
	assert.NotContains(t, body, "idempotency_key")
}

// create omits description/estimated_duration_sec when not supplied (omitempty),
// but always emits steps (no omitempty) — matching the bespoke marshal struct.
func TestWorkoutTemplates_CreateOmitsOptionals(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["create_workout_template"].Build(json.RawMessage(`{"sport":"bike","name":"Z2"}`))
	require.NoError(t, err)
	var body map[string]any
	require.NoError(t, json.Unmarshal(call.Body, &body))
	assert.NotContains(t, body, "description")
	assert.NotContains(t, body, "estimated_duration_sec")
	assert.Contains(t, body, "steps")
}

// list_workout_templates → GET /workout-templates, optional sport filter.
func TestWorkoutTemplates_List(t *testing.T) {
	specs := ByName(MCPRegistry())

	c1, err := specs["list_workout_templates"].Build(json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", c1.Method)
	assert.Equal(t, "/workout-templates", c1.Path)
	assert.Empty(t, c1.Query.Get("sport"))

	c2, err := specs["list_workout_templates"].Build(json.RawMessage(`{"sport":"swim"}`))
	require.NoError(t, err)
	assert.Equal(t, "swim", c2.Query.Get("sport"))
}

// get_workout_template → GET /workout-templates/{id}; missing id errors.
func TestWorkoutTemplates_Get(t *testing.T) {
	specs := ByName(MCPRegistry())

	call, err := specs["get_workout_template"].Build(json.RawMessage(`{"id":"abc"}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/workout-templates/abc", call.Path)

	_, err = specs["get_workout_template"].Build(json.RawMessage(`{}`))
	assert.Error(t, err)
}

// patch_workout_template → PATCH /workout-templates/{id}; only supplied fields in
// the body, id stripped, missing id errors.
func TestWorkoutTemplates_Patch(t *testing.T) {
	specs := ByName(MCPRegistry())

	call, err := specs["patch_workout_template"].Build(json.RawMessage(
		`{"id":"t1","name":"New name","steps":[{"type":"step","intent":"active"}]}`))
	require.NoError(t, err)
	assert.Equal(t, "PATCH", call.Method)
	assert.Equal(t, "/workout-templates/t1", call.Path)

	var body map[string]any
	require.NoError(t, json.Unmarshal(call.Body, &body))
	assert.Equal(t, "New name", body["name"])
	assert.Contains(t, body, "steps")
	assert.NotContains(t, body, "sport")
	assert.NotContains(t, body, "description")
	assert.NotContains(t, body, "id")

	_, err = specs["patch_workout_template"].Build(json.RawMessage(`{"name":"x"}`))
	assert.Error(t, err)
}

// patch with no editable fields produces an empty JSON object body, mirroring the
// bespoke handler's unconditional map marshal.
func TestWorkoutTemplates_PatchEmptyBody(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["patch_workout_template"].Build(json.RawMessage(`{"id":"t1"}`))
	require.NoError(t, err)
	assert.Equal(t, "{}", string(call.Body))
}

// delete_workout_template → DELETE /workout-templates/{id}; missing id errors.
func TestWorkoutTemplates_Delete(t *testing.T) {
	specs := ByName(MCPRegistry())

	call, err := specs["delete_workout_template"].Build(json.RawMessage(`{"id":"t9"}`))
	require.NoError(t, err)
	assert.Equal(t, "DELETE", call.Method)
	assert.Equal(t, "/workout-templates/t9", call.Path)

	_, err = specs["delete_workout_template"].Build(json.RawMessage(`{}`))
	assert.Error(t, err)
}
