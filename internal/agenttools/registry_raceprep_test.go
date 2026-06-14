package agenttools

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The race-prep domain contributes exactly these two MCP tools with the tiers
// the bespoke registrations implied: plan_carb_load is a write (apply branch
// POSTs and keyed in the bespoke handler), recommend_workout_fuel is read-only.
func TestRacePrep_SurfaceAndTiers(t *testing.T) {
	specs := ByName(MCPRegistry())
	wantTier := map[string]Tier{
		"plan_carb_load":         TierWriteAuto,
		"recommend_workout_fuel": TierRead,
	}
	for name, tier := range wantTier {
		s, ok := specs[name]
		require.Truef(t, ok, "missing MCP tool %s", name)
		assert.Equalf(t, tier, s.Tier, "tier wrong for %s", name)
		assert.NotNilf(t, s.SchemaType, "tool %s must carry a SchemaType for MCP schema reflection", name)
		assert.Truef(t, s.MCPExposed, "tool %s should be MCP-exposed", name)
	}
}

// plan_carb_load with apply omitted → pure-compute GET /race-prep/carb-load
// with required params only; unset optionals are not sent; no body.
func TestRacePrep_PlanCarbLoad_ComputeRequiredOnly(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["plan_carb_load"].Build(json.RawMessage(
		`{"race_date":"2026-07-24","body_weight_kg":70}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/race-prep/carb-load", call.Path)
	assert.Equal(t, "2026-07-24", call.Query.Get("race_date"))
	assert.Equal(t, "70", call.Query.Get("body_weight_kg"))
	assert.Empty(t, call.Query.Get("days_before"), "unset optionals must not be sent")
	assert.Empty(t, call.Query.Get("carbs_per_kg_per_day"))
	assert.Empty(t, call.Query.Get("race_day_carbs_per_kg"))
	assert.Empty(t, call.Body, "GET sends no body")
}

// Optional params are forwarded in the query for the compute path with the
// exact string formatting the bespoke handler produced.
func TestRacePrep_PlanCarbLoad_ComputeOptionalsForwarded(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["plan_carb_load"].Build(json.RawMessage(
		`{"race_date":"2026-07-24","body_weight_kg":70,"days_before":2,` +
			`"carbs_per_kg_per_day":8,"race_day_carbs_per_kg":2.5}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "2", call.Query.Get("days_before"))
	assert.Equal(t, "8", call.Query.Get("carbs_per_kg_per_day"))
	assert.Equal(t, "2.5", call.Query.Get("race_day_carbs_per_kg"))
}

// apply=false stays on the read path (GET), matching the bespoke switch.
func TestRacePrep_PlanCarbLoad_ApplyFalseHitsGET(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["plan_carb_load"].Build(json.RawMessage(
		`{"race_date":"2026-07-24","body_weight_kg":70,"apply":false}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/race-prep/carb-load", call.Path)
	assert.Empty(t, call.Body)
}

// apply=true switches to POST /race-prep/carb-load/apply with the apply body;
// the apply switch and idempotency_key are NOT forwarded; query is empty.
func TestRacePrep_PlanCarbLoad_ApplyTrueHitsPOST(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["plan_carb_load"].Build(json.RawMessage(
		`{"race_date":"2026-07-24","body_weight_kg":70,"apply":true,"idempotency_key":"k1"}`))
	require.NoError(t, err)
	assert.Equal(t, "POST", call.Method)
	assert.Equal(t, "/race-prep/carb-load/apply", call.Path)
	assert.Empty(t, call.Query)

	var body map[string]any
	require.NoError(t, json.Unmarshal(call.Body, &body))
	assert.Equal(t, "2026-07-24", body["race_date"])
	assert.InDelta(t, 70.0, body["body_weight_kg"], 0.001)
	_, hasApply := body["apply"]
	assert.False(t, hasApply, "wrapper consumed `apply`; do not forward to backend")
	_, hasKey := body["idempotency_key"]
	assert.False(t, hasKey, "idempotency_key is a header, not a body field")
	// Unset optionals are omitted via omitempty on applyBody.
	assert.NotContains(t, body, "days_before")
	assert.NotContains(t, body, "carbs_per_kg_per_day")
	assert.NotContains(t, body, "race_day_carbs_per_kg")
}

// apply=true forwards optional params in the body when supplied.
func TestRacePrep_PlanCarbLoad_ApplyTrueOptionalsInBody(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["plan_carb_load"].Build(json.RawMessage(
		`{"race_date":"2026-07-24","body_weight_kg":70,"apply":true,` +
			`"days_before":2,"carbs_per_kg_per_day":8,"race_day_carbs_per_kg":2.5}`))
	require.NoError(t, err)
	var body map[string]any
	require.NoError(t, json.Unmarshal(call.Body, &body))
	assert.InDelta(t, 2.0, body["days_before"], 0.001)
	assert.InDelta(t, 8.0, body["carbs_per_kg_per_day"], 0.001)
	assert.InDelta(t, 2.5, body["race_day_carbs_per_kg"], 0.001)
}

// recommend_workout_fuel in workout mode forwards only workout_id in the query.
func TestRacePrep_RecommendWorkoutFuel_WorkoutMode(t *testing.T) {
	specs := ByName(MCPRegistry())
	wid := "11111111-1111-1111-1111-111111111111"
	call, err := specs["recommend_workout_fuel"].Build(json.RawMessage(
		`{"workout_id":"` + wid + `"}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/race-prep/recommend-workout-fuel", call.Path)
	assert.Equal(t, wid, call.Query.Get("workout_id"))
	assert.Empty(t, call.Query.Get("sport"))
	assert.Empty(t, call.Query.Get("duration_min"))
	assert.Empty(t, call.Query.Get("intensity_zone"))
	assert.Empty(t, call.Query.Get("body_weight_kg"))
	assert.Empty(t, call.Body)
}

// recommend_workout_fuel in explicit mode forwards the sport/duration/zone
// triplet (and body_weight_kg when supplied) with the bespoke string formats.
func TestRacePrep_RecommendWorkoutFuel_ExplicitMode(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["recommend_workout_fuel"].Build(json.RawMessage(
		`{"sport":"bike","duration_min":90,"intensity_zone":3,"body_weight_kg":72.5}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "bike", call.Query.Get("sport"))
	assert.Equal(t, "90", call.Query.Get("duration_min"))
	assert.Equal(t, "3", call.Query.Get("intensity_zone"))
	assert.Equal(t, "72.5", call.Query.Get("body_weight_kg"))
	assert.Empty(t, call.Query.Get("workout_id"))
}
