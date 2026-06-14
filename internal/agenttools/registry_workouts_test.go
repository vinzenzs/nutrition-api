package agenttools

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The workouts domain contributes eight MCP-only tools: full CRUD plus
// fulfill/unfulfill and the per-workout fueling summary.
func TestWorkouts_RegisteredWithExpectedTiers(t *testing.T) {
	specs := ByName(MCPRegistry())
	wantTier := map[string]Tier{
		"log_workout":             TierWriteAuto,
		"list_workouts":           TierRead,
		"get_workout":             TierRead,
		"patch_workout":           TierWriteAuto,
		"delete_workout":          TierWriteAuto,
		"fulfill_workout":         TierWriteAuto,
		"unfulfill_workout":       TierWriteAuto,
		"workout_fueling_summary": TierRead,
	}
	for name, tier := range wantTier {
		s, ok := specs[name]
		require.Truef(t, ok, "tool %s not registered on the MCP surface", name)
		assert.Equalf(t, tier, s.Tier, "tool %s tier", name)
		assert.Truef(t, s.MCPExposed, "tool %s should be MCP-exposed", name)
		assert.NotNilf(t, s.SchemaType, "tool %s should carry a SchemaType", name)
	}
}

func TestLogWorkout_PostsFullBody(t *testing.T) {
	specs := ByName(MCPRegistry())
	in := json.RawMessage(`{
		"external_id":"garmin:1","source":"garmin","sport":"bike","name":"Morning Z2",
		"started_at":"2026-06-07T08:00:00Z","ended_at":"2026-06-07T09:30:00Z",
		"kcal_burned":850,"avg_hr":135,"tss":78,"idempotency_key":"explicit-key"
	}`)
	call, err := specs["log_workout"].Build(in)
	require.NoError(t, err)
	assert.Equal(t, "POST", call.Method)
	assert.Equal(t, "/workouts", call.Path)

	var body map[string]any
	require.NoError(t, json.Unmarshal(call.Body, &body))
	assert.Equal(t, "garmin:1", body["external_id"])
	assert.Equal(t, "garmin", body["source"])
	assert.Equal(t, "bike", body["sport"])
	assert.Equal(t, "Morning Z2", body["name"])
	assert.EqualValues(t, 850, body["kcal_burned"])
	assert.EqualValues(t, 135, body["avg_hr"])
	assert.EqualValues(t, 78, body["tss"])
	// idempotency_key is NOT part of the REST body.
	_, hasKey := body["idempotency_key"]
	assert.False(t, hasKey)
}

func TestLogWorkout_RPEAndGIForwardedInBody(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["log_workout"].Build(json.RawMessage(`{
		"source":"manual","sport":"bike",
		"started_at":"2026-07-15T08:00:00Z","ended_at":"2026-07-15T09:30:00Z",
		"rpe":7,"gi_distress_score":2
	}`))
	require.NoError(t, err)
	var body map[string]any
	require.NoError(t, json.Unmarshal(call.Body, &body))
	assert.InDelta(t, 7.0, body["rpe"], 0.001)
	assert.InDelta(t, 2.0, body["gi_distress_score"], 0.001)
}

func TestLogWorkout_OmitsRPEAndGIWhenNil(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["log_workout"].Build(json.RawMessage(`{
		"source":"manual","sport":"bike",
		"started_at":"2026-07-15T08:00:00Z","ended_at":"2026-07-15T09:30:00Z"
	}`))
	require.NoError(t, err)
	body := string(call.Body)
	assert.NotContains(t, body, `"rpe"`)
	assert.NotContains(t, body, `"gi_distress_score"`)
}

func TestLogWorkout_IngestionMetricsForwardedInBody(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["log_workout"].Build(json.RawMessage(`{
		"source":"garmin","sport":"bike",
		"started_at":"2026-06-13T08:00:00Z","ended_at":"2026-06-13T11:00:00Z",
		"distance_m":80500,"avg_power_w":182,"temperature_c":27.5,
		"sweat_loss_ml":2400,"session_group":"garmin:554"
	}`))
	require.NoError(t, err)
	var body map[string]any
	require.NoError(t, json.Unmarshal(call.Body, &body))
	assert.EqualValues(t, 80500, body["distance_m"])
	assert.EqualValues(t, 182, body["avg_power_w"])
	assert.InDelta(t, 27.5, body["temperature_c"], 0.001)
	assert.EqualValues(t, 2400, body["sweat_loss_ml"])
	assert.Equal(t, "garmin:554", body["session_group"])
}

func TestLogWorkout_OmitsIngestionMetricsWhenNil(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["log_workout"].Build(json.RawMessage(`{
		"source":"manual","sport":"strength",
		"started_at":"2026-06-07T18:00:00Z","ended_at":"2026-06-07T19:00:00Z"
	}`))
	require.NoError(t, err)
	body := string(call.Body)
	for _, k := range []string{`"distance_m"`, `"avg_power_w"`, `"temperature_c"`, `"sweat_loss_ml"`, `"session_group"`} {
		assert.NotContains(t, body, k)
	}
}

func TestListWorkouts_BuildsQueryString(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["list_workouts"].Build(json.RawMessage(`{
		"from":"2026-06-01T00:00:00Z","to":"2026-06-08T00:00:00Z"
	}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/workouts", call.Path)
	assert.Equal(t, "2026-06-01T00:00:00Z", call.Query.Get("from"))
	assert.Equal(t, "2026-06-08T00:00:00Z", call.Query.Get("to"))
	assert.False(t, call.Query.Has("session_group"))
	assert.False(t, call.Query.Has("status"))
	assert.Empty(t, call.Body)
}

func TestListWorkouts_ForwardsSessionGroupAndStatus(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["list_workouts"].Build(json.RawMessage(`{
		"from":"2026-06-13T00:00:00Z","to":"2026-06-14T00:00:00Z",
		"session_group":"garmin:9876543","status":"planned"
	}`))
	require.NoError(t, err)
	assert.Equal(t, "garmin:9876543", call.Query.Get("session_group"))
	assert.Equal(t, "planned", call.Query.Get("status"))
}

func TestGetWorkout_CallsPathEscapedURL(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["get_workout"].Build(json.RawMessage(`{"id":"abc/123"}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/workouts/abc%2F123", call.Path)
}

func TestPatchWorkout_OmitsUnsetFields(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["patch_workout"].Build(json.RawMessage(`{"id":"w1","tss":85}`))
	require.NoError(t, err)
	assert.Equal(t, "PATCH", call.Method)
	assert.Equal(t, "/workouts/w1", call.Path)
	var body map[string]any
	require.NoError(t, json.Unmarshal(call.Body, &body))
	assert.EqualValues(t, 85, body["tss"])
	_, hasName := body["name"]
	assert.False(t, hasName, "unset fields must NOT appear in the PATCH body")
}

func TestPatchWorkout_SetRPEAndGI(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["patch_workout"].Build(json.RawMessage(`{"id":"w1","rpe":7,"gi_distress_score":2}`))
	require.NoError(t, err)
	var body map[string]any
	require.NoError(t, json.Unmarshal(call.Body, &body))
	assert.InDelta(t, 7.0, body["rpe"], 0.001)
	assert.InDelta(t, 2.0, body["gi_distress_score"], 0.001)
}

func TestPatchWorkout_ClearRPEEncodesJSONNull(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["patch_workout"].Build(json.RawMessage(`{"id":"w1","clear_rpe":true}`))
	require.NoError(t, err)
	assert.Contains(t, string(call.Body), `"rpe":null`)
}

func TestPatchWorkout_ClearGIEncodesJSONNull(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["patch_workout"].Build(json.RawMessage(`{"id":"w1","clear_gi_distress_score":true}`))
	require.NoError(t, err)
	assert.Contains(t, string(call.Body), `"gi_distress_score":null`)
}

func TestPatchWorkout_ClearSessionGroupEncodesJSONNull(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["patch_workout"].Build(json.RawMessage(`{"id":"w1","clear_session_group":true}`))
	require.NoError(t, err)
	assert.Contains(t, string(call.Body), `"session_group":null`)
}

func TestPatchWorkout_SetIngestionMetrics(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["patch_workout"].Build(json.RawMessage(`{"id":"w1","sweat_loss_ml":1850,"temperature_c":31}`))
	require.NoError(t, err)
	var body map[string]any
	require.NoError(t, json.Unmarshal(call.Body, &body))
	assert.EqualValues(t, 1850, body["sweat_loss_ml"])
	assert.InDelta(t, 31.0, body["temperature_c"], 0.001)
	_, hasDist := body["distance_m"]
	assert.False(t, hasDist, "unset ingestion field absent from PATCH body")
}

func TestPatchWorkout_OmitsRPEAndGIWhenAbsent(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["patch_workout"].Build(json.RawMessage(`{"id":"w1","notes":"felt strong"}`))
	require.NoError(t, err)
	body := string(call.Body)
	assert.NotContains(t, body, `"rpe"`)
	assert.NotContains(t, body, `"gi_distress_score"`)
	assert.Contains(t, body, `"notes":"felt strong"`)
}

func TestDeleteWorkout_BuildShape(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["delete_workout"].Build(json.RawMessage(`{"id":"w1"}`))
	require.NoError(t, err)
	assert.Equal(t, "DELETE", call.Method)
	assert.Equal(t, "/workouts/w1", call.Path)
	assert.Empty(t, call.Body)
}

func TestFulfillWorkout_PostsCompletedIDBody(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["fulfill_workout"].Build(json.RawMessage(`{"planned_id":"p1","completed_id":"c2"}`))
	require.NoError(t, err)
	assert.Equal(t, "POST", call.Method)
	assert.Equal(t, "/workouts/p1/fulfill", call.Path)
	var body map[string]any
	require.NoError(t, json.Unmarshal(call.Body, &body))
	assert.Equal(t, "c2", body["completed_id"])
	_, hasPlanned := body["planned_id"]
	assert.False(t, hasPlanned, "planned_id is a path segment, not part of the body")
}

func TestUnfulfillWorkout_PostsNoBody(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["unfulfill_workout"].Build(json.RawMessage(`{"id":"w1"}`))
	require.NoError(t, err)
	assert.Equal(t, "POST", call.Method)
	assert.Equal(t, "/workouts/w1/unfulfill", call.Path)
	assert.Empty(t, call.Body)
}

func TestWorkoutFuelingSummary_BuildShape(t *testing.T) {
	specs := ByName(MCPRegistry())

	// No windows → no query keys.
	call, err := specs["workout_fueling_summary"].Build(json.RawMessage(`{"workout_id":"w1"}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/workouts/w1/fueling", call.Path)
	assert.False(t, call.Query.Has("pre_window_min"))
	assert.False(t, call.Query.Has("post_window_min"))

	// With windows → integer-formatted query keys.
	call, err = specs["workout_fueling_summary"].Build(json.RawMessage(`{"workout_id":"w1","pre_window_min":180,"post_window_min":90}`))
	require.NoError(t, err)
	assert.Equal(t, "180", call.Query.Get("pre_window_min"))
	assert.Equal(t, "90", call.Query.Get("post_window_min"))
}
