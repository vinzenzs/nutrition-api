package workouttemplates_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/nutrition-api/internal/store/storetest"
	"github.com/vinzenzs/nutrition-api/internal/workouttemplates"
)

func init() { gin.SetMode(gin.TestMode) }

func setup(t *testing.T) *gin.Engine {
	t.Helper()
	pool := storetest.NewPool(t)
	svc := workouttemplates.NewService(workouttemplates.NewRepo(pool))
	r := gin.New()
	workouttemplates.NewHandlers(svc).Register(r.Group("/"))
	return r
}

func do(t *testing.T, r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	rdr := bytes.NewBuffer(nil)
	if body != "" {
		rdr = bytes.NewBufferString(body)
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// validBody is the design-doc example template, as JSON.
const validBody = `{
  "sport":"run","name":"VO2 intervals","estimated_duration_sec":1800,
  "steps":[
    {"type":"step","intent":"warmup","duration":{"kind":"time","seconds":600},"target":{"kind":"hr_zone","low":1,"high":2}},
    {"type":"repeat","count":5,"steps":[
      {"type":"step","intent":"interval","duration":{"kind":"time","seconds":180},"target":{"kind":"power_zone","low":4,"high":4}},
      {"type":"step","intent":"recovery","duration":{"kind":"time","seconds":120},"target":{"kind":"hr_zone","low":1}}
    ]},
    {"type":"step","intent":"cooldown","duration":{"kind":"time","seconds":300},"target":{"kind":"hr_zone","low":1}}
  ]
}`

func createValid(t *testing.T, r *gin.Engine) workouttemplates.Template {
	t.Helper()
	rec := do(t, r, http.MethodPost, "/workout-templates", validBody)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var got workouttemplates.Template
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	return got
}

func TestCreateGetRoundTrip(t *testing.T) {
	r := setup(t)
	created := createValid(t, r)
	assert.NotEmpty(t, created.ID)
	require.Len(t, created.Steps, 3)
	assert.Equal(t, "repeat", created.Steps[1].Type)
	require.Len(t, created.Steps[1].Steps, 2)

	rec := do(t, r, http.MethodGet, "/workout-templates/"+created.ID, "")
	require.Equal(t, http.StatusOK, rec.Code)
	var got workouttemplates.Template
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, "VO2 intervals", got.Name)
	require.NotNil(t, got.EstimatedDurationSec)
	assert.Equal(t, 1800, *got.EstimatedDurationSec)
	// Steps round-trip verbatim.
	require.Len(t, got.Steps, 3)
	require.NotNil(t, got.Steps[0].Duration.Seconds)
	assert.Equal(t, 600, *got.Steps[0].Duration.Seconds)
}

func TestListFiltersBySport(t *testing.T) {
	r := setup(t)
	createValid(t, r) // run
	swim := `{"sport":"swim","name":"Easy swim","steps":[{"type":"step","intent":"active","duration":{"kind":"distance","meters":1000},"target":{"kind":"none"}}]}`
	require.Equal(t, http.StatusCreated, do(t, r, http.MethodPost, "/workout-templates", swim).Code)

	rec := do(t, r, http.MethodGet, "/workout-templates?sport=swim", "")
	require.Equal(t, http.StatusOK, rec.Code)
	var resp struct {
		WorkoutTemplates []workouttemplates.Template `json:"workout_templates"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.WorkoutTemplates, 1)
	assert.Equal(t, "swim", resp.WorkoutTemplates[0].Sport)
}

func TestPatchReplacesStepsAsAUnit(t *testing.T) {
	r := setup(t)
	created := createValid(t, r)
	patch := `{"steps":[{"type":"step","intent":"active","duration":{"kind":"open"},"target":{"kind":"none"}}]}`
	rec := do(t, r, http.MethodPatch, "/workout-templates/"+created.ID, patch)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var got workouttemplates.Template
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Len(t, got.Steps, 1)
	assert.Equal(t, "open", got.Steps[0].Duration.Kind)
	// Other fields unchanged.
	assert.Equal(t, "VO2 intervals", got.Name)
	assert.Equal(t, "run", got.Sport)
}

func TestPatchLeavesOmittedFieldsUnchanged(t *testing.T) {
	r := setup(t)
	created := createValid(t, r)
	rec := do(t, r, http.MethodPatch, "/workout-templates/"+created.ID, `{"name":"Renamed"}`)
	require.Equal(t, http.StatusOK, rec.Code)
	var got workouttemplates.Template
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "Renamed", got.Name)
	assert.Equal(t, "run", got.Sport)
	require.Len(t, got.Steps, 3, "steps unchanged when omitted")
	require.NotNil(t, got.EstimatedDurationSec)
	assert.Equal(t, 1800, *got.EstimatedDurationSec)
}

func TestPatchNullClearsNullable(t *testing.T) {
	r := setup(t)
	created := createValid(t, r)
	rec := do(t, r, http.MethodPatch, "/workout-templates/"+created.ID, `{"estimated_duration_sec":null}`)
	require.Equal(t, http.StatusOK, rec.Code)
	var got workouttemplates.Template
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Nil(t, got.EstimatedDurationSec, "present null clears the nullable")
}

func TestDeleteThen404(t *testing.T) {
	r := setup(t)
	created := createValid(t, r)
	require.Equal(t, http.StatusNoContent, do(t, r, http.MethodDelete, "/workout-templates/"+created.ID, "").Code)
	assert.Equal(t, http.StatusNotFound, do(t, r, http.MethodGet, "/workout-templates/"+created.ID, "").Code)
}

func TestGetMissingReturns404(t *testing.T) {
	r := setup(t)
	rec := do(t, r, http.MethodGet, "/workout-templates/00000000-0000-0000-0000-000000000000", "")
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "workout_template_not_found")
}

func TestMalformedStepsRejectedAtBoundary(t *testing.T) {
	r := setup(t)
	cases := map[string]string{
		"empty steps":   `{"sport":"run","name":"x","steps":[]}`,
		"nested repeat": `{"sport":"run","name":"x","steps":[{"type":"repeat","count":2,"steps":[{"type":"repeat","count":2,"steps":[{"type":"step","intent":"active","duration":{"kind":"open"},"target":{"kind":"none"}}]}]}]}`,
		"bad zone":      `{"sport":"run","name":"x","steps":[{"type":"step","intent":"interval","duration":{"kind":"time","seconds":60},"target":{"kind":"hr_zone","low":1,"high":9}}]}`,
		"unknown kind":  `{"sport":"run","name":"x","steps":[{"type":"step","intent":"active","duration":{"kind":"forever"},"target":{"kind":"none"}}]}`,
		"bad sport":     `{"sport":"chess","name":"x","steps":[{"type":"step","intent":"active","duration":{"kind":"open"},"target":{"kind":"none"}}]}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			rec := do(t, r, http.MethodPost, "/workout-templates", body)
			assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		})
	}
}
