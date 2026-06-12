package trainingplan_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// paceOverride is a slot target override that runs the "active" step at 7:15/km.
const paceOverride = `"target_overrides":[{"intent":"active","target":{"kind":"pace","low_sec_per_km":435,"high_sec_per_km":435}}]`

func TestSlot_TargetOverridesRoundTripInNestedGet(t *testing.T) {
	r := setup(t)
	templateID := createTemplate(t, r)
	planID := mustID(t, do(t, r, http.MethodPost, "/training-plans", `{"name":"p","start_date":"2026-06-01"}`))
	weekID := mustID(t, do(t, r, http.MethodPost, "/training-plans/"+planID+"/weeks", `{"ordinal":1}`))
	require.Equal(t, http.StatusCreated, do(t, r, http.MethodPost, "/training-plans/"+planID+"/weeks/"+weekID+"/slots",
		`{"weekday":0,"ordinal":0,"template_id":"`+templateID+`",`+paceOverride+`}`).Code)

	rec := do(t, r, http.MethodGet, "/training-plans/"+planID, "")
	require.Equal(t, http.StatusOK, rec.Code)
	slot := firstSlot(t, rec.Body.Bytes())
	ov := slot["target_overrides"].([]any)
	require.Len(t, ov, 1)
	entry := ov[0].(map[string]any)
	assert.Equal(t, "active", entry["intent"])
	assert.Equal(t, "pace", entry["target"].(map[string]any)["kind"])
	assert.Equal(t, float64(435), entry["target"].(map[string]any)["low_sec_per_km"])
}

func TestSlot_PatchOverridesReplaceClearOmit(t *testing.T) {
	r := setup(t)
	planID, _, slotID, _ := buildPlan(t, r)

	// Replace: set an override list.
	require.Equal(t, http.StatusOK,
		do(t, r, http.MethodPatch, "/training-plans/"+planID+"/slots/"+slotID, `{`+paceOverride+`}`).Code)
	require.Len(t, slotOverrides(t, r, planID), 1)

	// Omit: a patch of an unrelated field leaves overrides unchanged.
	require.Equal(t, http.StatusOK,
		do(t, r, http.MethodPatch, "/training-plans/"+planID+"/slots/"+slotID, `{"ordinal":3}`).Code)
	require.Len(t, slotOverrides(t, r, planID), 1)

	// Clear: empty list removes all overrides.
	require.Equal(t, http.StatusOK,
		do(t, r, http.MethodPatch, "/training-plans/"+planID+"/slots/"+slotID, `{"target_overrides":[]}`).Code)
	require.Empty(t, slotOverrides(t, r, planID))
}

func TestSlot_InvalidOverridesRejected(t *testing.T) {
	r := setup(t)
	templateID := createTemplate(t, r)
	planID := mustID(t, do(t, r, http.MethodPost, "/training-plans", `{"name":"p","start_date":"2026-06-01"}`))
	weekID := mustID(t, do(t, r, http.MethodPost, "/training-plans/"+planID+"/weeks", `{"ordinal":1}`))
	base := "/training-plans/" + planID + "/weeks/" + weekID + "/slots"

	cases := map[string]string{
		"unknown_intent":    `{"intent":"sprint","target":{"kind":"none"}}`,
		"duplicate_intent":  `{"intent":"active","target":{"kind":"none"}},{"intent":"active","target":{"kind":"pace","low_sec_per_km":300,"high_sec_per_km":320}}`,
		"inverted_pace":     `{"intent":"active","target":{"kind":"pace","low_sec_per_km":440,"high_sec_per_km":420}}`,
		"out_of_range_zone": `{"intent":"active","target":{"kind":"hr_zone","low":0,"high":9}}`,
		"unknown_kind":      `{"intent":"active","target":{"kind":"bananas"}}`,
	}
	for name, entries := range cases {
		t.Run(name, func(t *testing.T) {
			body := `{"weekday":0,"ordinal":0,"template_id":"` + templateID + `","target_overrides":[` + entries + `]}`
			rec := do(t, r, http.MethodPost, base, body)
			assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		})
	}
}

func TestWorkoutProgram_OverrideAppliedToMatchingIntentOnly(t *testing.T) {
	r := setup(t)
	templateID := createTemplate(t, r)
	planID := mustID(t, do(t, r, http.MethodPost, "/training-plans", `{"name":"p","start_date":"2026-06-01"}`))
	weekID := mustID(t, do(t, r, http.MethodPost, "/training-plans/"+planID+"/weeks", `{"ordinal":1}`))
	require.Equal(t, http.StatusCreated, do(t, r, http.MethodPost, "/training-plans/"+planID+"/weeks/"+weekID+"/slots",
		`{"weekday":0,"ordinal":0,"template_id":"`+templateID+`",`+paceOverride+`}`).Code)
	ws := materialize(t, r, planID, `{"scope":"all"}`)
	require.Len(t, ws, 1)
	workoutID := ws[0]["id"].(string)

	prog := program(t, r, workoutID)
	steps := prog["steps"].([]any)
	require.Len(t, steps, 1)
	target := steps[0].(map[string]any)["target"].(map[string]any)
	assert.Equal(t, "pace", target["kind"], "the active step's target was overridden")
	assert.Equal(t, float64(435), target["low_sec_per_km"])
}

func TestWorkoutProgram_NoOverrideYieldsTemplateVerbatim(t *testing.T) {
	r := setup(t)
	planID, _, _, _ := buildPlan(t, r) // slot has no overrides
	ws := materialize(t, r, planID, `{"scope":"all"}`)
	require.Len(t, ws, 1)

	prog := program(t, r, ws[0]["id"].(string))
	steps := prog["steps"].([]any)
	require.Len(t, steps, 1)
	target := steps[0].(map[string]any)["target"].(map[string]any)
	assert.Equal(t, "hr_zone", target["kind"], "unchanged from the template")
}

func TestWorkoutProgram_TemplateLessReturnsMetadataOnly(t *testing.T) {
	r := setup(t)
	// A manually-created workout has no template_id.
	rec := do(t, r, http.MethodPost, "/workouts",
		`{"source":"manual","sport":"swim","name":"Open swim","started_at":"2026-06-01T06:00:00Z","ended_at":"2026-06-01T07:00:00Z","status":"completed"}`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var w map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &w))

	prog := program(t, r, w["id"].(string))
	assert.Equal(t, "swim", prog["sport"])
	assert.Empty(t, prog["steps"].([]any), "no template → no steps")
}

// ----- helpers -----

func program(t *testing.T, r *gin.Engine, workoutID string) map[string]any {
	t.Helper()
	rec := do(t, r, http.MethodGet, "/workouts/"+workoutID+"/program", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var m map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &m))
	return m
}

// slotOverrides returns the first slot's target_overrides from the plan's
// nested GET (nil when absent).
func slotOverrides(t *testing.T, r *gin.Engine, planID string) []any {
	t.Helper()
	rec := do(t, r, http.MethodGet, "/training-plans/"+planID, "")
	require.Equal(t, http.StatusOK, rec.Code)
	slot := firstSlot(t, rec.Body.Bytes())
	if ov, ok := slot["target_overrides"].([]any); ok {
		return ov
	}
	return nil
}

func firstSlot(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var plan map[string]any
	require.NoError(t, json.Unmarshal(body, &plan))
	weeks := plan["weeks"].([]any)
	require.NotEmpty(t, weeks)
	slots := weeks[0].(map[string]any)["slots"].([]any)
	require.NotEmpty(t, slots)
	return slots[0].(map[string]any)
}
