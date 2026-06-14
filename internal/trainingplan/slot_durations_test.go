package trainingplan_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// activeDurOverride extends the "active" step to 80 minutes (4800s).
const activeDurOverride = `"duration_overrides":[{"intent":"active","duration":{"kind":"time","seconds":4800}}]`

func TestSlot_DurationOverridesRoundTripInNestedGet(t *testing.T) {
	r := setup(t)
	templateID := createTemplate(t, r)
	planID := mustID(t, do(t, r, http.MethodPost, "/training-plans", `{"name":"p","start_date":"2026-06-01"}`))
	weekID := mustID(t, do(t, r, http.MethodPost, "/training-plans/"+planID+"/weeks", `{"ordinal":1}`))
	// Carry both override lists to prove they round-trip together.
	require.Equal(t, http.StatusCreated, do(t, r, http.MethodPost, "/training-plans/"+planID+"/weeks/"+weekID+"/slots",
		`{"weekday":0,"ordinal":0,"template_id":"`+templateID+`",`+paceOverride+`,`+activeDurOverride+`}`).Code)

	rec := do(t, r, http.MethodGet, "/training-plans/"+planID, "")
	require.Equal(t, http.StatusOK, rec.Code)
	slot := firstSlot(t, rec.Body.Bytes())
	require.Len(t, slot["target_overrides"].([]any), 1)
	dov := slot["duration_overrides"].([]any)
	require.Len(t, dov, 1)
	entry := dov[0].(map[string]any)
	assert.Equal(t, "active", entry["intent"])
	dur := entry["duration"].(map[string]any)
	assert.Equal(t, "time", dur["kind"])
	assert.Equal(t, float64(4800), dur["seconds"])
}

func TestSlot_PatchDurationOverridesReplaceClearOmit(t *testing.T) {
	r := setup(t)
	planID, _, slotID, _ := buildPlan(t, r)

	// Replace.
	require.Equal(t, http.StatusOK,
		do(t, r, http.MethodPatch, "/training-plans/"+planID+"/slots/"+slotID, `{`+activeDurOverride+`}`).Code)
	require.Len(t, slotDurationOverrides(t, r, planID), 1)

	// Omit: an unrelated patch leaves duration overrides unchanged.
	require.Equal(t, http.StatusOK,
		do(t, r, http.MethodPatch, "/training-plans/"+planID+"/slots/"+slotID, `{"ordinal":3}`).Code)
	require.Len(t, slotDurationOverrides(t, r, planID), 1)

	// Clear.
	require.Equal(t, http.StatusOK,
		do(t, r, http.MethodPatch, "/training-plans/"+planID+"/slots/"+slotID, `{"duration_overrides":[]}`).Code)
	require.Empty(t, slotDurationOverrides(t, r, planID))
}

func TestSlot_InvalidDurationOverridesRejected(t *testing.T) {
	r := setup(t)
	templateID := createTemplate(t, r)
	planID := mustID(t, do(t, r, http.MethodPost, "/training-plans", `{"name":"p","start_date":"2026-06-01"}`))
	weekID := mustID(t, do(t, r, http.MethodPost, "/training-plans/"+planID+"/weeks", `{"ordinal":1}`))
	base := "/training-plans/" + planID + "/weeks/" + weekID + "/slots"

	cases := map[string]string{
		"unknown_intent":   `{"intent":"sprint","duration":{"kind":"time","seconds":600}}`,
		"duplicate_intent": `{"intent":"active","duration":{"kind":"time","seconds":600}},{"intent":"active","duration":{"kind":"time","seconds":700}}`,
		"open_kind":        `{"intent":"active","duration":{"kind":"open"}}`,
		"lap_button_kind":  `{"intent":"active","duration":{"kind":"lap_button"}}`,
		"non_positive":     `{"intent":"active","duration":{"kind":"time","seconds":0}}`,
		"unknown_kind":     `{"intent":"active","duration":{"kind":"forever"}}`,
	}
	for name, entries := range cases {
		t.Run(name, func(t *testing.T) {
			body := `{"weekday":0,"ordinal":0,"template_id":"` + templateID + `","duration_overrides":[` + entries + `]}`
			rec := do(t, r, http.MethodPost, base, body)
			assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		})
	}
}

func TestWorkoutProgram_DurationOverrideReflected(t *testing.T) {
	r := setup(t)
	templateID := createTemplate(t, r)
	planID := mustID(t, do(t, r, http.MethodPost, "/training-plans", `{"name":"p","start_date":"2026-06-01"}`))
	weekID := mustID(t, do(t, r, http.MethodPost, "/training-plans/"+planID+"/weeks", `{"ordinal":1}`))
	require.Equal(t, http.StatusCreated, do(t, r, http.MethodPost, "/training-plans/"+planID+"/weeks/"+weekID+"/slots",
		`{"weekday":0,"ordinal":0,"template_id":"`+templateID+`",`+activeDurOverride+`}`).Code)
	ws := materialize(t, r, planID, `{"scope":"all"}`)
	require.Len(t, ws, 1)

	prog := program(t, r, ws[0]["id"].(string))
	steps := prog["steps"].([]any)
	require.Len(t, steps, 1)
	dur := steps[0].(map[string]any)["duration"].(map[string]any)
	assert.Equal(t, float64(4800), dur["seconds"], "the active step's duration was overridden")
}

func TestMaterialize_DurationOverrideMovesWindow(t *testing.T) {
	r := setup(t)
	templateID := createTemplate(t, r) // single 3600s active step, est 3600
	planID := mustID(t, do(t, r, http.MethodPost, "/training-plans", `{"name":"p","start_date":"2026-06-01"}`))
	weekID := mustID(t, do(t, r, http.MethodPost, "/training-plans/"+planID+"/weeks", `{"ordinal":1}`))

	// No override → 60-minute window (template's own duration).
	slotID := mustID(t, do(t, r, http.MethodPost, "/training-plans/"+planID+"/weeks/"+weekID+"/slots",
		`{"weekday":0,"ordinal":0,"template_id":"`+templateID+`"}`))
	ws := materialize(t, r, planID, `{"scope":"all"}`)
	require.Len(t, ws, 1)
	assert.Equal(t, 60, windowMinutes(t, ws[0]))

	// Add an 80-minute override and re-materialize → window grows to 80min.
	require.Equal(t, http.StatusOK,
		do(t, r, http.MethodPatch, "/training-plans/"+planID+"/slots/"+slotID, `{`+activeDurOverride+`}`).Code)
	ws = materialize(t, r, planID, `{"scope":"all"}`)
	require.Len(t, ws, 1)
	assert.Equal(t, 80, windowMinutes(t, ws[0]))
}

func TestMaterialize_NonTimeProgramFallsBackToEstimated(t *testing.T) {
	r := setup(t)
	// A distance-bounded template: the effective program is not time-summable,
	// so materialize falls back to estimated_duration_sec (3000s = 50min).
	tmplBody := `{"sport":"run","name":"Distance run","estimated_duration_sec":3000,"steps":[{"type":"step","intent":"active","duration":{"kind":"distance","meters":10000},"target":{"kind":"hr_zone","low":1,"high":2}}]}`
	templateID := mustID(t, do(t, r, http.MethodPost, "/workout-templates", tmplBody))
	planID := mustID(t, do(t, r, http.MethodPost, "/training-plans", `{"name":"p","start_date":"2026-06-01"}`))
	weekID := mustID(t, do(t, r, http.MethodPost, "/training-plans/"+planID+"/weeks", `{"ordinal":1}`))
	require.Equal(t, http.StatusCreated, do(t, r, http.MethodPost, "/training-plans/"+planID+"/weeks/"+weekID+"/slots",
		`{"weekday":0,"ordinal":0,"template_id":"`+templateID+`"}`).Code)

	ws := materialize(t, r, planID, `{"scope":"all"}`)
	require.Len(t, ws, 1)
	assert.Equal(t, 50, windowMinutes(t, ws[0]))
}

// ----- helpers -----

func slotDurationOverrides(t *testing.T, r *gin.Engine, planID string) []any {
	t.Helper()
	rec := do(t, r, http.MethodGet, "/training-plans/"+planID, "")
	require.Equal(t, http.StatusOK, rec.Code)
	slot := firstSlot(t, rec.Body.Bytes())
	if ov, ok := slot["duration_overrides"].([]any); ok {
		return ov
	}
	return nil
}

// windowMinutes parses a materialized workout's started_at/ended_at and returns
// the span in whole minutes.
func windowMinutes(t *testing.T, w map[string]any) int {
	t.Helper()
	start, err := time.Parse(time.RFC3339, w["started_at"].(string))
	require.NoError(t, err)
	end, err := time.Parse(time.RFC3339, w["ended_at"].(string))
	require.NoError(t, err)
	return int(end.Sub(start).Minutes())
}
