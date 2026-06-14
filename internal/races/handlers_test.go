package races_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/races"
	"github.com/vinzenzs/kazper/internal/store/storetest"
)

func init() { gin.SetMode(gin.TestMode) }

func setup(t *testing.T) *gin.Engine {
	t.Helper()
	pool := storetest.NewPool(t)
	svc := races.NewService(pool, races.NewRepo(pool))
	r := gin.New()
	races.NewHandlers(svc).Register(r.Group("/"))
	return r
}

func do(t *testing.T, r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var rdr *bytes.Buffer
	if body != "" {
		rdr = bytes.NewBufferString(body)
	} else {
		rdr = bytes.NewBufferString("")
	}
	req := httptest.NewRequest(method, path, rdr)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

const sprintBody = `{
  "name": "Allgäu Sprint",
  "race_date": "2026-07-24",
  "race_type": "sprint",
  "legs": [
    {"ordinal": 1, "discipline": "swim", "expected_duration_min": 15},
    {"ordinal": 2, "discipline": "bike", "expected_duration_min": 90},
    {"ordinal": 3, "discipline": "run", "expected_duration_min": 50}
  ]
}`

func createSprint(t *testing.T, r *gin.Engine) races.Race {
	w := do(t, r, http.MethodPost, "/races", sprintBody)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var race races.Race
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &race))
	return race
}

func TestCreateAndGetRoundTrip(t *testing.T) {
	r := setup(t)
	race := createSprint(t, r)
	require.NotEqual(t, "", race.ID.String())
	require.Len(t, race.Legs, 3)
	assert.Equal(t, "2026-07-24", race.RaceDate)
	assert.Equal(t, races.DisciplineSwim, race.Legs[0].Discipline)
	assert.Equal(t, races.DisciplineRun, race.Legs[2].Discipline)

	w := do(t, r, http.MethodGet, "/races/"+race.ID.String(), "")
	require.Equal(t, http.StatusOK, w.Code)
	var got races.Race
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Len(t, got.Legs, 3)
}

func TestDeleteCascadesLegs(t *testing.T) {
	r := setup(t)
	race := createSprint(t, r)

	w := do(t, r, http.MethodDelete, "/races/"+race.ID.String(), "")
	require.Equal(t, http.StatusNoContent, w.Code)

	w = do(t, r, http.MethodGet, "/races/"+race.ID.String(), "")
	require.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "race_not_found")
}

func TestDuplicateOrdinalRejected(t *testing.T) {
	r := setup(t)
	body := `{"name":"X","race_date":"2026-07-24","legs":[
        {"ordinal":1,"discipline":"bike","expected_duration_min":60},
        {"ordinal":1,"discipline":"run","expected_duration_min":30}]}`
	w := do(t, r, http.MethodPost, "/races", body)
	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "leg_ordinal_duplicate")
}

func TestInvalidDisciplineRejected(t *testing.T) {
	r := setup(t)
	body := `{"name":"X","race_date":"2026-07-24","legs":[
        {"ordinal":1,"discipline":"kayak","expected_duration_min":60}]}`
	w := do(t, r, http.MethodPost, "/races", body)
	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "leg_discipline_invalid")
}

func TestFuelingPlanHappyPath(t *testing.T) {
	r := setup(t)
	race := createSprint(t, r)

	w := do(t, r, http.MethodGet, "/races/"+race.ID.String()+"/fueling-plan?body_weight_kg=70", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var plan races.FuelingPlan
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &plan))

	// Total 155 min → 90 g/hr baseline.
	require.Len(t, plan.Legs, 3)
	assert.Equal(t, 155, plan.TotalDurationMin)
	assert.Equal(t, float64(0), plan.Legs[0].CarbsGPerHr)  // swim
	assert.Equal(t, float64(90), plan.Legs[1].CarbsGPerHr) // bike
	assert.Equal(t, float64(63), plan.Legs[2].CarbsGPerHr) // run 0.7×90
	// Default sweat rate → 600/600 on the bike leg, flagged.
	assert.Equal(t, float64(600), plan.Legs[1].FluidMlPerHr)
	assert.Equal(t, float64(600), plan.Legs[1].SodiumMgPerHr)
	assert.Contains(t, plan.Legs[1].Rationale, "default sweat rate")
}

func TestFuelingPlanUnitIsolation(t *testing.T) {
	r := setup(t)
	race := createSprint(t, r)
	w := do(t, r, http.MethodGet, "/races/"+race.ID.String()+"/fueling-plan?body_weight_kg=70&sweat_rate_ml_per_hr=900", "")
	require.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	// Distinct unit families present...
	assert.Contains(t, body, "carbs_g_per_hr")
	assert.Contains(t, body, "sodium_mg_per_hr")
	assert.Contains(t, body, "fluid_ml_per_hr")
	// ...and never cross-wired into the wrong unit.
	assert.NotContains(t, body, "carbs_ml")
	assert.NotContains(t, body, "carbs_mg")
	assert.NotContains(t, body, "sodium_g_")
	assert.NotContains(t, body, "fluid_g")
}

func TestFuelingPlanValidation(t *testing.T) {
	r := setup(t)
	race := createSprint(t, r)
	id := race.ID.String()

	w := do(t, r, http.MethodGet, "/races/"+id+"/fueling-plan", "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "body_weight_kg_required")

	w = do(t, r, http.MethodGet, "/races/"+id+"/fueling-plan?body_weight_kg=15", "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "body_weight_kg_out_of_range")

	w = do(t, r, http.MethodGet, "/races/11111111-1111-1111-1111-111111111111/fueling-plan?body_weight_kg=70", "")
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "race_not_found")
}

func TestListRaces(t *testing.T) {
	r := setup(t)
	createSprint(t, r)
	w := do(t, r, http.MethodGet, "/races", "")
	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Races []races.Race `json:"races"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.GreaterOrEqual(t, len(resp.Races), 1)
}
