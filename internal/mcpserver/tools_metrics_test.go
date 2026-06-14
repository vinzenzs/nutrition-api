package mcpserver

import (
	"context"
	"encoding/json"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Recovery-metrics, fitness-metrics, and hydration-balance tools were ported
// onto the shared agenttools registry (unify-mcp-tool-registry); their Build
// shapes are now asserted in internal/agenttools/registry_*_test.go and the
// idempotency-key attachment generically in registry_dispatch_test.go. The
// weight and workouts handlers below stay until those domains are ported.

func TestLogWeight_ForwardsBiometrics(t *testing.T) {
	c, recs := newWorkoutRecorder(t, 201, `{"id":"w1"}`)
	_ = handleLogWeight(context.Background(), c, LogWeightArgs{
		WeightKg: 72.5, LoggedAt: "2026-06-09T07:00:00Z",
		MuscleMassKg: ptrFloat(58.4), BMI: ptrFloat(22.4),
	})
	require.Len(t, *recs, 1)
	var body map[string]any
	require.NoError(t, json.Unmarshal((*recs)[0].body, &body))
	assert.InDelta(t, 58.4, body["muscle_mass_kg"], 0.001)
	assert.InDelta(t, 22.4, body["bmi"], 0.001)
	_, hasWater := body["body_water_pct"]
	assert.False(t, hasWater, "omitted biometric absent from body")
}

func TestLogWorkout_ForwardsStatus(t *testing.T) {
	c, recs := newWorkoutRecorder(t, 201, `{"id":"w1"}`)
	_ = handleLogWorkout(context.Background(), c, LogWorkoutArgs{
		Source: "garmin", Sport: "bike", Status: "planned",
		StartedAt: "2026-09-01T08:00:00Z", EndedAt: "2026-09-01T10:00:00Z",
	})
	require.Len(t, *recs, 1)
	var body map[string]any
	require.NoError(t, json.Unmarshal((*recs)[0].body, &body))
	assert.Equal(t, "planned", body["status"])
}

func TestListWorkouts_ForwardsStatusFilter(t *testing.T) {
	c, recs := newWorkoutRecorder(t, 200, `{"workouts":[]}`)
	st := "planned"
	_ = handleListWorkouts(context.Background(), c, ListWorkoutsArgs{
		From: "2026-06-01T00:00:00Z", To: "2026-06-30T00:00:00Z", Status: &st,
	})
	require.Len(t, *recs, 1)
	values, err := url.ParseQuery((*recs)[0].rawQuery)
	require.NoError(t, err)
	assert.Equal(t, "planned", values.Get("status"))
}

func TestPatchWorkout_ForwardsStatus(t *testing.T) {
	c, recs := newWorkoutRecorder(t, 200, `{"id":"w1"}`)
	st := "completed"
	_ = handlePatchWorkout(context.Background(), c, PatchWorkoutArgs{ID: "w1", Status: &st})
	require.Len(t, *recs, 1)
	var body map[string]any
	require.NoError(t, json.Unmarshal((*recs)[0].body, &body))
	assert.Equal(t, "completed", body["status"])
}

func ptrInt(v int) *int           { return &v }
func ptrFloat(v float64) *float64 { return &v }
