package raceprep_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/auth"
	"github.com/vinzenzs/kazper/internal/raceprep"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// fixedNow is the simulated wall-clock used across handler tests. All
// race_date_in_past / race_date_today behaviour is anchored to this date.
var fixedNow = time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)

func setup(t *testing.T) *gin.Engine {
	t.Helper()
	svc := raceprep.NewService(
		func() time.Time { return fixedNow },
		time.UTC,
		nil, // Plan path doesn't need a pool; apply tests build their own service with a real pool.
	)
	r := gin.New()
	rg := r.Group("/")
	raceprep.NewHandlers(svc).Register(rg)
	return r
}

func doGet(r *gin.Engine, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// ----- happy paths -----

func TestCarbLoad_DefaultsExactFixture(t *testing.T) {
	r := setup(t)
	rec := doGet(r, "/race-prep/carb-load?race_date=2026-07-24&body_weight_kg=70")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	want := `{
        "race_date": "2026-07-24",
        "body_weight_kg": 70,
        "params": {"days_before": 3, "carbs_per_kg_per_day": 10, "race_day_carbs_per_kg": 2},
        "schedule": [
            {"date": "2026-07-21", "days_before": 3, "target_carbs_g": 700, "rationale": "carb-load day 1 of 3"},
            {"date": "2026-07-22", "days_before": 2, "target_carbs_g": 700, "rationale": "carb-load day 2 of 3"},
            {"date": "2026-07-23", "days_before": 1, "target_carbs_g": 700, "rationale": "carb-load day 3 of 3"},
            {"date": "2026-07-24", "days_before": 0, "target_carbs_g": 140, "rationale": "race morning, pre-race meal ~3-4h before start"}
        ]
    }`
	assert.JSONEq(t, want, rec.Body.String())
}

func TestCarbLoad_CustomParamsHonoured(t *testing.T) {
	r := setup(t)
	rec := doGet(r, "/race-prep/carb-load?race_date=2026-07-24&body_weight_kg=80&days_before=2&carbs_per_kg_per_day=8&race_day_carbs_per_kg=2.5")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	want := `{
        "race_date": "2026-07-24",
        "body_weight_kg": 80,
        "params": {"days_before": 2, "carbs_per_kg_per_day": 8, "race_day_carbs_per_kg": 2.5},
        "schedule": [
            {"date": "2026-07-22", "days_before": 2, "target_carbs_g": 640, "rationale": "carb-load day 1 of 2"},
            {"date": "2026-07-23", "days_before": 1, "target_carbs_g": 640, "rationale": "carb-load day 2 of 2"},
            {"date": "2026-07-24", "days_before": 0, "target_carbs_g": 200, "rationale": "race morning, pre-race meal ~3-4h before start"}
        ]
    }`
	assert.JSONEq(t, want, rec.Body.String())
}

// ----- validation failures -----

func TestCarbLoad_MissingRaceDate(t *testing.T) {
	r := setup(t)
	rec := doGet(r, "/race-prep/carb-load?body_weight_kg=70")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"race_date_required"}`, rec.Body.String())
}

func TestCarbLoad_MissingBodyWeight(t *testing.T) {
	r := setup(t)
	rec := doGet(r, "/race-prep/carb-load?race_date=2026-07-24")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"body_weight_kg_required"}`, rec.Body.String())
}

func TestCarbLoad_InvalidRaceDateFormat(t *testing.T) {
	r := setup(t)
	rec := doGet(r, "/race-prep/carb-load?race_date=07/24/2026&body_weight_kg=70")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"race_date_invalid"}`, rec.Body.String())
}

func TestCarbLoad_RaceDateInPast(t *testing.T) {
	r := setup(t)
	// fixedNow is 2026-06-07; 2026-06-06 is strictly in the past.
	rec := doGet(r, "/race-prep/carb-load?race_date=2026-06-06&body_weight_kg=70")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"race_date_in_past"}`, rec.Body.String())
}

func TestCarbLoad_BodyWeightUnderMin(t *testing.T) {
	r := setup(t)
	rec := doGet(r, "/race-prep/carb-load?race_date=2026-07-24&body_weight_kg=25")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"body_weight_kg_invalid","range":{"min":30,"max":200}}`, rec.Body.String())
}

func TestCarbLoad_BodyWeightOverMax(t *testing.T) {
	r := setup(t)
	rec := doGet(r, "/race-prep/carb-load?race_date=2026-07-24&body_weight_kg=250")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"body_weight_kg_invalid","range":{"min":30,"max":200}}`, rec.Body.String())
}

func TestCarbLoad_BodyWeightNonNumeric(t *testing.T) {
	r := setup(t)
	rec := doGet(r, "/race-prep/carb-load?race_date=2026-07-24&body_weight_kg=heavy")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"body_weight_kg_invalid","range":{"min":30,"max":200}}`, rec.Body.String())
}

func TestCarbLoad_DaysBeforeNegative(t *testing.T) {
	r := setup(t)
	rec := doGet(r, "/race-prep/carb-load?race_date=2026-07-24&body_weight_kg=70&days_before=-1")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"days_before_invalid","range":{"min":0,"max":7}}`, rec.Body.String())
}

func TestCarbLoad_DaysBeforeOverMax(t *testing.T) {
	r := setup(t)
	rec := doGet(r, "/race-prep/carb-load?race_date=2026-07-24&body_weight_kg=70&days_before=8")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"days_before_invalid","range":{"min":0,"max":7}}`, rec.Body.String())
}

func TestCarbLoad_CarbsPerKgUnderMin(t *testing.T) {
	r := setup(t)
	rec := doGet(r, "/race-prep/carb-load?race_date=2026-07-24&body_weight_kg=70&carbs_per_kg_per_day=0.5")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"carbs_per_kg_per_day_invalid","range":{"min":1,"max":20}}`, rec.Body.String())
}

func TestCarbLoad_CarbsPerKgOverMax(t *testing.T) {
	r := setup(t)
	rec := doGet(r, "/race-prep/carb-load?race_date=2026-07-24&body_weight_kg=70&carbs_per_kg_per_day=25")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"carbs_per_kg_per_day_invalid","range":{"min":1,"max":20}}`, rec.Body.String())
}

func TestCarbLoad_RaceDayCarbsUnderMin(t *testing.T) {
	r := setup(t)
	rec := doGet(r, "/race-prep/carb-load?race_date=2026-07-24&body_weight_kg=70&race_day_carbs_per_kg=-1")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"race_day_carbs_per_kg_invalid","range":{"min":0,"max":10}}`, rec.Body.String())
}

func TestCarbLoad_RaceDayCarbsOverMax(t *testing.T) {
	r := setup(t)
	rec := doGet(r, "/race-prep/carb-load?race_date=2026-07-24&body_weight_kg=70&race_day_carbs_per_kg=11")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"race_day_carbs_per_kg_invalid","range":{"min":0,"max":10}}`, rec.Body.String())
}

// ----- non-numeric optional params -----

func TestCarbLoad_DaysBeforeNonNumeric(t *testing.T) {
	r := setup(t)
	rec := doGet(r, "/race-prep/carb-load?race_date=2026-07-24&body_weight_kg=70&days_before=lots")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"days_before_invalid","range":{"min":0,"max":7}}`, rec.Body.String())
}

// ----- auth -----

func TestCarbLoad_MissingAuthReturns401(t *testing.T) {
	svc := raceprep.NewService(func() time.Time { return fixedNow }, time.UTC, nil)
	r := gin.New()
	r.Use(auth.Middleware(auth.Config{
		MobileToken: "mobile-token-aaaaaaaaaaaaaa",
		AgentToken:  "agent-token-bbbbbbbbbbbbbbbb",
	}))
	rg := r.Group("/")
	raceprep.NewHandlers(svc).Register(rg)

	rec := doGet(r, "/race-prep/carb-load?race_date=2026-07-24&body_weight_kg=70")
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}
