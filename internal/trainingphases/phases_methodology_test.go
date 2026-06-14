package trainingphases_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/trainingphases"
)

func TestPhase_MethodologyRoundTripsOnCreate(t *testing.T) {
	f := setupHandlers(t, false)
	body := `{"name":"build","type":"build","start_date":"2026-04-01","end_date":"2026-05-15","methodology":"## Build\nPolarized per Seiler — 80/20."}`
	rec := f.do(http.MethodPost, "/phases", body, nil)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var resp struct {
		Phase *trainingphases.Phase `json:"phase"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.NotNil(t, resp.Phase.Methodology)
	assert.Contains(t, *resp.Phase.Methodology, "Seiler")

	// And the dedicated GET returns it too.
	get := f.do(http.MethodGet, "/phases/"+resp.Phase.ID.String(), "", nil)
	require.Equal(t, http.StatusOK, get.Code)
	require.NoError(t, json.Unmarshal(get.Body.Bytes(), &resp))
	require.NotNil(t, resp.Phase.Methodology)
	assert.Contains(t, *resp.Phase.Methodology, "Seiler")
}

func TestPhase_MethodologyIndependentOfNotes(t *testing.T) {
	f := setupHandlers(t, false)
	create := f.do(http.MethodPost, "/phases",
		`{"name":"base","type":"base","start_date":"2026-01-01","end_date":"2026-03-01","notes":"keep it easy","methodology":"## Base"}`, nil)
	require.Equal(t, http.StatusCreated, create.Code, create.Body.String())
	var resp struct {
		Phase *trainingphases.Phase `json:"phase"`
	}
	require.NoError(t, json.Unmarshal(create.Body.Bytes(), &resp))
	id := resp.Phase.ID.String()

	// Replace methodology only; notes must survive untouched.
	patch := f.do(http.MethodPatch, "/phases/"+id, `{"methodology":"## Base (revised)\nSan Millán Z2."}`, nil)
	require.Equal(t, http.StatusOK, patch.Code, patch.Body.String())
	require.NoError(t, json.Unmarshal(patch.Body.Bytes(), &resp))
	require.NotNil(t, resp.Phase.Methodology)
	assert.Contains(t, *resp.Phase.Methodology, "San Millán")
	require.NotNil(t, resp.Phase.Notes)
	assert.Equal(t, "keep it easy", *resp.Phase.Notes)
}

func TestPhase_AbsentMethodologyIsNull(t *testing.T) {
	f := setupHandlers(t, false)
	rec := f.do(http.MethodPost, "/phases",
		`{"name":"peak","type":"peak","start_date":"2026-06-01","end_date":"2026-06-21"}`, nil)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var raw map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &raw))
	phase := raw["phase"].(map[string]any)
	assert.Nil(t, phase["methodology"], "absent methodology reads as null/unset")
}
