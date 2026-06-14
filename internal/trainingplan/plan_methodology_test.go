package trainingplan_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func planMethodology(t *testing.T, body []byte) (methodology, notes *string) {
	t.Helper()
	var plan map[string]any
	require.NoError(t, json.Unmarshal(body, &plan))
	if m, ok := plan["methodology"].(string); ok {
		methodology = &m
	}
	if n, ok := plan["notes"].(string); ok {
		notes = &n
	}
	return
}

func TestPlan_MethodologyPatchSetsAndGetReturns(t *testing.T) {
	r := setup(t)
	planID := mustID(t, do(t, r, http.MethodPost, "/training-plans", `{"name":"18wk","start_date":"2026-06-01","notes":"A-race build"}`))

	// PATCH sets plan-level methodology.
	require.Equal(t, http.StatusOK,
		do(t, r, http.MethodPatch, "/training-plans/"+planID, `{"methodology":"## Key Principles\nPolarized 80/20 (Seiler)."}`).Code)

	rec := do(t, r, http.MethodGet, "/training-plans/"+planID, "")
	require.Equal(t, http.StatusOK, rec.Code)
	methodology, notes := planMethodology(t, rec.Body.Bytes())
	require.NotNil(t, methodology)
	assert.Contains(t, *methodology, "Seiler")
	// notes left untouched by the methodology patch.
	require.NotNil(t, notes)
	assert.Equal(t, "A-race build", *notes)
}

func TestPlan_AbsentMethodologyIsNull(t *testing.T) {
	r := setup(t)
	planID := mustID(t, do(t, r, http.MethodPost, "/training-plans", `{"name":"p","start_date":"2026-06-01"}`))
	rec := do(t, r, http.MethodGet, "/training-plans/"+planID, "")
	require.Equal(t, http.StatusOK, rec.Code)
	var plan map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &plan))
	assert.Nil(t, plan["methodology"], "absent methodology reads as null/unset")
}
