package raceprep_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/bodyweight"
	"github.com/vinzenzs/kazper/internal/raceprep"
	"github.com/vinzenzs/kazper/internal/store/storetest"
	"github.com/vinzenzs/kazper/internal/workouts"
)

// fixedNowRec is the simulated wall-clock for recommend tests. June 9 2026,
// which lines up with the protein-distribution / energy fixtures so the same
// body-weight rows can be reused mentally.
var fixedNowRec = time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)

func setupRecommend(t *testing.T) (*gin.Engine, *bodyweight.Repo, *workouts.Repo) {
	t.Helper()
	pool := storetest.NewPool(t)
	bwRepo := bodyweight.NewRepo(pool)
	wRepo := workouts.NewRepo(pool)
	svc := raceprep.NewService(
		func() time.Time { return fixedNowRec },
		time.UTC,
		pool,
	)
	svc.SetBodyWeightRepo(bwRepo)
	svc.SetWorkoutsRepo(wRepo)
	r := gin.New()
	rg := r.Group("/")
	raceprep.NewHandlers(svc).Register(rg)
	return r, bwRepo, wRepo
}

func doGetRec(r *gin.Engine, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func insertRecWeight(t *testing.T, repo *bodyweight.Repo, at time.Time, kg float64) {
	t.Helper()
	require.NoError(t, repo.Insert(context.Background(), &bodyweight.Entry{
		LoggedAt: at,
		WeightKg: kg,
	}))
}

// ============================================================================
// Explicit mode — table-driven literature bands
// ============================================================================

func TestRecommend_Explicit_BikeZ3_90min(t *testing.T) {
	r, _, _ := setupRecommend(t)
	rec := doGetRec(r,
		"/race-prep/recommend-workout-fuel?sport=bike&duration_min=90&intensity_zone=3&body_weight_kg=72")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var out raceprep.FuelRecommendation
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))

	// Inputs
	assert.Equal(t, "bike", out.Inputs.Sport)
	assert.Equal(t, 90, out.Inputs.DurationMin)
	assert.Equal(t, 3, out.Inputs.IntensityZone)
	assert.Equal(t, 72.0, out.Inputs.BodyWeightKg)
	assert.Equal(t, bodyweight.SourceExplicit, out.Inputs.BodyWeightSource)
	assert.Nil(t, out.Inputs.WorkoutID, "explicit mode → no workout_id echoed")

	// Pre: Z3 → 1.5 g/kg, [60, 120]
	assert.Equal(t, [2]int{60, 120}, out.PreWorkout.WindowMinutesBefore)
	assert.Equal(t, 1.5, out.PreWorkout.CarbsGPerKg)
	assert.Equal(t, 108.0, out.PreWorkout.CarbsG)

	// Intra: 90-min Z3 bike → 60 g/hr, total 90, fluid 700, sodium 600
	require.True(t, out.IntraWorkout.Applicable)
	require.NotNil(t, out.IntraWorkout.CarbsGPerHour)
	assert.Equal(t, 60.0, *out.IntraWorkout.CarbsGPerHour)
	require.NotNil(t, out.IntraWorkout.CarbsGTotal)
	assert.Equal(t, 90.0, *out.IntraWorkout.CarbsGTotal)
	require.NotNil(t, out.IntraWorkout.FluidMlPerHour)
	assert.Equal(t, 700.0, *out.IntraWorkout.FluidMlPerHour)
	require.NotNil(t, out.IntraWorkout.SodiumMgPerHour)
	assert.Equal(t, 600.0, *out.IntraWorkout.SodiumMgPerHour)

	// Post: 1.0/0.3 × 72 = 72/21.6, [0, 60]
	assert.Equal(t, [2]int{0, 60}, out.PostWorkout.WindowMinutesAfter)
	assert.Equal(t, 72.0, out.PostWorkout.CarbsG)
	assert.Equal(t, 21.6, out.PostWorkout.ProteinG)

	// Notes — at least the three baseline ones.
	require.GreaterOrEqual(t, len(out.Notes), 3)
	joined := strings.Join(out.Notes, " ")
	assert.Contains(t, joined, "sodium")
	assert.Contains(t, joined, "plan_carb_load")
}

func TestRecommend_Explicit_RunCapAt60(t *testing.T) {
	r, _, _ := setupRecommend(t)
	// 240-min Z2 run — bike would get 90 g/hr in the > 180 min bucket; run caps at 60.
	rec := doGetRec(r,
		"/race-prep/recommend-workout-fuel?sport=run&duration_min=240&intensity_zone=2&body_weight_kg=72")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var out raceprep.FuelRecommendation
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.True(t, out.IntraWorkout.Applicable)
	require.NotNil(t, out.IntraWorkout.CarbsGPerHour)
	assert.Equal(t, 60.0, *out.IntraWorkout.CarbsGPerHour, "run cap applies")
	assert.Contains(t, out.IntraWorkout.Rationale, "run-specific cap")

	// Notes should include the run-cap disclosure for > 180 min runs.
	joined := strings.Join(out.Notes, " ")
	assert.Contains(t, joined, "Run-specific cap")
}

func TestRecommend_Explicit_BikeLongGets90(t *testing.T) {
	r, _, _ := setupRecommend(t)
	rec := doGetRec(r,
		"/race-prep/recommend-workout-fuel?sport=bike&duration_min=240&intensity_zone=2&body_weight_kg=72")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var out raceprep.FuelRecommendation
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.NotNil(t, out.IntraWorkout.CarbsGPerHour)
	assert.Equal(t, 90.0, *out.IntraWorkout.CarbsGPerHour, "bike has no cap")
}

func TestRecommend_Explicit_StrengthIntraNotApplicable(t *testing.T) {
	r, _, _ := setupRecommend(t)
	rec := doGetRec(r,
		"/race-prep/recommend-workout-fuel?sport=strength&duration_min=60&intensity_zone=4&body_weight_kg=72")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var out raceprep.FuelRecommendation
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.False(t, out.IntraWorkout.Applicable)
	assert.Nil(t, out.IntraWorkout.CarbsGPerHour)
	assert.Nil(t, out.IntraWorkout.CarbsGTotal)
	assert.Nil(t, out.IntraWorkout.FluidMlPerHour)
	assert.Nil(t, out.IntraWorkout.SodiumMgPerHour)

	// Strength pre-workout: 0.5 g/kg [30, 90]
	assert.Equal(t, [2]int{30, 90}, out.PreWorkout.WindowMinutesBefore)
	assert.Equal(t, 0.5, out.PreWorkout.CarbsGPerKg)
}

func TestRecommend_Explicit_ShortSessionIntraNotApplicable(t *testing.T) {
	r, _, _ := setupRecommend(t)
	rec := doGetRec(r,
		"/race-prep/recommend-workout-fuel?sport=bike&duration_min=40&intensity_zone=3&body_weight_kg=72")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var out raceprep.FuelRecommendation
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.False(t, out.IntraWorkout.Applicable, "sub-45-min → not applicable")
}

func TestRecommend_Explicit_SwimUnder2hIntraNotApplicable(t *testing.T) {
	r, _, _ := setupRecommend(t)
	rec := doGetRec(r,
		"/race-prep/recommend-workout-fuel?sport=swim&duration_min=90&intensity_zone=3&body_weight_kg=72")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var out raceprep.FuelRecommendation
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.False(t, out.IntraWorkout.Applicable, "swim ≤ 120 min → not applicable")
}

// ============================================================================
// Workout-mode
// ============================================================================

func seedWorkout(t *testing.T, repo *workouts.Repo, sport workouts.Sport, started, ended time.Time, tss *float64) workouts.Workout {
	t.Helper()
	w := &workouts.Workout{
		Source:     workouts.SourceManual,
		Sport:      sport,
		StartedAt:  started,
		EndedAt:    ended,
		TSS:        tss,
	}
	_, err := repo.Upsert(context.Background(), w)
	require.NoError(t, err)
	return *w
}

func TestRecommend_WorkoutMode_DerivesIntensityFromTSS(t *testing.T) {
	r, bwRepo, wRepo := setupRecommend(t)
	insertRecWeight(t, bwRepo, time.Date(2026, 6, 8, 7, 0, 0, 0, time.UTC), 72)
	// 90-min bike with TSS=70 → IF=sqrt(70/(1.5*100))=sqrt(0.4667)≈0.683 → Z2
	tss := 70.0
	w := seedWorkout(t, wRepo, workouts.SportBike,
		time.Date(2026, 6, 9, 8, 0, 0, 0, time.UTC),
		time.Date(2026, 6, 9, 9, 30, 0, 0, time.UTC),
		&tss)

	rec := doGetRec(r,
		fmt.Sprintf("/race-prep/recommend-workout-fuel?workout_id=%s", w.ID))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var out raceprep.FuelRecommendation
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, "bike", out.Inputs.Sport)
	assert.Equal(t, 90, out.Inputs.DurationMin)
	assert.Equal(t, 2, out.Inputs.IntensityZone)
	require.NotNil(t, out.Inputs.WorkoutID)
	assert.Equal(t, w.ID, *out.Inputs.WorkoutID)
}

func TestRecommend_WorkoutMode_DefaultsToZ2WhenTSSAbsent(t *testing.T) {
	r, bwRepo, wRepo := setupRecommend(t)
	insertRecWeight(t, bwRepo, time.Date(2026, 6, 8, 7, 0, 0, 0, time.UTC), 72)
	w := seedWorkout(t, wRepo, workouts.SportBike,
		time.Date(2026, 6, 9, 8, 0, 0, 0, time.UTC),
		time.Date(2026, 6, 9, 9, 30, 0, 0, time.UTC),
		nil)

	rec := doGetRec(r,
		fmt.Sprintf("/race-prep/recommend-workout-fuel?workout_id=%s", w.ID))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var out raceprep.FuelRecommendation
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, 2, out.Inputs.IntensityZone)
	// The disclosure note must be present.
	joined := strings.Join(out.Notes, " ")
	assert.Contains(t, joined, "Intensity defaulted to Z2")
}

func TestRecommend_WorkoutMode_NotFound(t *testing.T) {
	r, bwRepo, _ := setupRecommend(t)
	insertRecWeight(t, bwRepo, time.Date(2026, 6, 8, 7, 0, 0, 0, time.UTC), 72)
	rec := doGetRec(r,
		"/race-prep/recommend-workout-fuel?workout_id=00000000-0000-0000-0000-000000000000")
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.JSONEq(t, `{"error":"workout_not_found"}`, rec.Body.String())
}

// ============================================================================
// Mode exclusivity + validation errors
// ============================================================================

func TestRecommend_Error_NeitherMode(t *testing.T) {
	r, _, _ := setupRecommend(t)
	rec := doGetRec(r, "/race-prep/recommend-workout-fuel?body_weight_kg=72")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"input_required"}`, rec.Body.String())
}

func TestRecommend_Error_BothModes(t *testing.T) {
	r, bwRepo, wRepo := setupRecommend(t)
	insertRecWeight(t, bwRepo, time.Date(2026, 6, 8, 7, 0, 0, 0, time.UTC), 72)
	tss := 70.0
	w := seedWorkout(t, wRepo, workouts.SportBike,
		time.Date(2026, 6, 9, 8, 0, 0, 0, time.UTC),
		time.Date(2026, 6, 9, 9, 0, 0, 0, time.UTC),
		&tss)

	rec := doGetRec(r,
		fmt.Sprintf("/race-prep/recommend-workout-fuel?workout_id=%s&sport=bike", w.ID))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"input_conflict"}`, rec.Body.String())
}

func TestRecommend_Error_PartialExplicit(t *testing.T) {
	r, _, _ := setupRecommend(t)
	// sport without duration_min/intensity_zone → first-missing-wins = duration_min_required
	rec := doGetRec(r, "/race-prep/recommend-workout-fuel?sport=bike&body_weight_kg=72")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"duration_min_required"}`, rec.Body.String())

	// duration_min alone → sport_required
	rec = doGetRec(r, "/race-prep/recommend-workout-fuel?duration_min=90&body_weight_kg=72")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"sport_required"}`, rec.Body.String())

	// sport + duration_min → intensity_zone_required
	rec = doGetRec(r,
		"/race-prep/recommend-workout-fuel?sport=bike&duration_min=90&body_weight_kg=72")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"intensity_zone_required"}`, rec.Body.String())
}

func TestRecommend_Error_InvalidValues(t *testing.T) {
	r, _, _ := setupRecommend(t)
	cases := []struct {
		name string
		path string
		code string
	}{
		{"unknown sport", "/race-prep/recommend-workout-fuel?sport=elliptical&duration_min=60&intensity_zone=3&body_weight_kg=72", "sport_invalid"},
		{"zone 0", "/race-prep/recommend-workout-fuel?sport=bike&duration_min=60&intensity_zone=0&body_weight_kg=72", "intensity_zone_invalid"},
		{"zone 6", "/race-prep/recommend-workout-fuel?sport=bike&duration_min=60&intensity_zone=6&body_weight_kg=72", "intensity_zone_invalid"},
		{"duration 0", "/race-prep/recommend-workout-fuel?sport=bike&duration_min=0&intensity_zone=3&body_weight_kg=72", "duration_min_invalid"},
		{"bad body weight", "/race-prep/recommend-workout-fuel?sport=bike&duration_min=60&intensity_zone=3&body_weight_kg=0", "body_weight_kg_invalid"},
		{"bad workout id", "/race-prep/recommend-workout-fuel?workout_id=not-a-uuid", "workout_id_invalid"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := doGetRec(r, tc.path)
			assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			var body map[string]any
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			assert.Equal(t, tc.code, body["error"])
		})
	}
}

func TestRecommend_Error_WeightDataMissing(t *testing.T) {
	r, _, _ := setupRecommend(t)
	// Explicit mode without override AND no stored weight → 400 weight_data_missing.
	rec := doGetRec(r,
		"/race-prep/recommend-workout-fuel?sport=bike&duration_min=90&intensity_zone=3")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"weight_data_missing"}`, rec.Body.String())
}

// ============================================================================
// MPS-threshold reuse: round-trip with protein-distribution's literature constant
// ============================================================================

func TestRecommend_PostProteinUsesSameMPSThreshold(t *testing.T) {
	r, _, _ := setupRecommend(t)
	// Body weight 72.5 — same fixture as the protein-distribution rounding case.
	// Expected post-protein = 0.3 × 72.5 = 21.75 → Round1(half-away-from-zero) = 21.8.
	rec := doGetRec(r,
		"/race-prep/recommend-workout-fuel?sport=bike&duration_min=90&intensity_zone=3&body_weight_kg=72.5")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var out raceprep.FuelRecommendation
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, 21.8, out.PostWorkout.ProteinG,
		"post-protein recommendation must match the MPS threshold from add-protein-distribution (single literature constant across endpoints)")
	assert.Equal(t, 72.5, out.PostWorkout.CarbsG)
}
