package agenttools

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkoutFuel_BuildShapes(t *testing.T) {
	specs := ByName(MCPRegistry())
	for _, n := range []string{"log_workout_fuel", "list_workout_fuel", "patch_workout_fuel", "delete_workout_fuel"} {
		_, ok := specs[n]
		require.Truef(t, ok, "tool %q not on the MCP surface", n)
	}

	// log → POST /workout-fuel; body excludes idempotency_key; caffeine_mg:0 preserved.
	call, err := specs["log_workout_fuel"].Build(json.RawMessage(
		`{"name":"SIS gel","logged_at":"2026-06-07T08:00:00Z","carbs_g":22,"caffeine_mg":0,"idempotency_key":"k"}`))
	require.NoError(t, err)
	assert.Equal(t, "POST", call.Method)
	assert.Equal(t, "/workout-fuel", call.Path)
	var body map[string]any
	require.NoError(t, json.Unmarshal(call.Body, &body))
	assert.Equal(t, "SIS gel", body["name"])
	assert.EqualValues(t, 22, body["carbs_g"])
	assert.EqualValues(t, 0, body["caffeine_mg"], "explicit zero caffeine is preserved")
	_, hasKey := body["idempotency_key"]
	assert.False(t, hasKey, "idempotency_key is not in the body")
	_, hasSodium := body["sodium_mg"]
	assert.False(t, hasSodium, "unset optional fields are omitted")

	// list → GET /workout-fuel?from&to
	call, _ = specs["list_workout_fuel"].Build(json.RawMessage(`{"from":"2026-06-01T00:00:00Z","to":"2026-06-30T00:00:00Z"}`))
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/workout-fuel", call.Path)
	assert.Equal(t, "2026-06-01T00:00:00Z", call.Query.Get("from"))

	// patch → PATCH /workout-fuel/{id}; only supplied fields; workout_id clear sentinel.
	call, _ = specs["patch_workout_fuel"].Build(json.RawMessage(`{"id":"f1","quantity_ml":250}`))
	assert.Equal(t, "PATCH", call.Method)
	assert.Equal(t, "/workout-fuel/f1", call.Path)
	assert.JSONEq(t, `{"quantity_ml":250}`, string(call.Body))
	call, _ = specs["patch_workout_fuel"].Build(json.RawMessage(`{"id":"f1","workout_id":""}`))
	assert.JSONEq(t, `{"workout_id":""}`, string(call.Body), "empty-string clears the workout link")

	// delete → DELETE /workout-fuel/{id}
	call, _ = specs["delete_workout_fuel"].Build(json.RawMessage(`{"id":"f1"}`))
	assert.Equal(t, "DELETE", call.Method)
	assert.Equal(t, "/workout-fuel/f1", call.Path)
}
