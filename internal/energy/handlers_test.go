package energy_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/bodyweight"
	"github.com/vinzenzs/kazper/internal/energy"
	"github.com/vinzenzs/kazper/internal/meals"
	"github.com/vinzenzs/kazper/internal/products"
	"github.com/vinzenzs/kazper/internal/store/storetest"
	"github.com/vinzenzs/kazper/internal/workouts"
)

func init() {
	gin.SetMode(gin.TestMode)
}

type fixture struct {
	r            *gin.Engine
	mealsRepo    *meals.Repo
	workoutsRepo *workouts.Repo
	bwRepo       *bodyweight.Repo
	productsRepo *products.Repo
}

func setupHandlers(t *testing.T) *fixture {
	t.Helper()
	pool := storetest.NewPool(t)
	pRepo := products.NewRepo(pool)
	mRepo := meals.NewRepo(pool)
	wRepo := workouts.NewRepo(pool)
	bwRepo := bodyweight.NewRepo(pool)
	svc := energy.NewService(mRepo, wRepo, bwRepo)

	r := gin.New()
	rg := r.Group("/")
	energy.NewHandlers(svc, "UTC").Register(rg)
	return &fixture{r: r, mealsRepo: mRepo, workoutsRepo: wRepo, bwRepo: bwRepo, productsRepo: pRepo}
}

func makeProduct(t *testing.T, repo *products.Repo, kcalPer100g float64) uuid.UUID {
	t.Helper()
	k := kcalPer100g
	p := &products.Product{
		Name:       "test-product",
		Source:     products.SourceManual,
		Nutriments: products.Nutriments{KcalPer100g: &k},
	}
	require.NoError(t, repo.Insert(context.Background(), p))
	return p.ID
}

func insertMeal(t *testing.T, repo *meals.Repo, pid uuid.UUID, at time.Time, gramsForOneKcal float64) {
	t.Helper()
	_, err := repo.Insert(context.Background(), meals.InsertParams{
		ProductID: &pid,
		LoggedAt:  at,
		QuantityG: gramsForOneKcal,
	})
	require.NoError(t, err)
}

func insertWorkout(t *testing.T, repo *workouts.Repo, startedAt, endedAt time.Time, kcal *float64) uuid.UUID {
	t.Helper()
	w := &workouts.Workout{
		Source:     workouts.SourceManual,
		Sport:      workouts.SportBike,
		StartedAt:  startedAt,
		EndedAt:    endedAt,
		KcalBurned: kcal,
	}
	_, err := repo.Upsert(context.Background(), w)
	require.NoError(t, err)
	return w.ID
}

// insertPlannedWorkout inserts a status=planned workout (no kcal_burned). Used to
// assert planned sessions never distort energy-availability aggregates.
func insertPlannedWorkout(t *testing.T, repo *workouts.Repo, startedAt, endedAt time.Time) uuid.UUID {
	t.Helper()
	w := &workouts.Workout{
		Source:    workouts.SourceManual,
		Sport:     workouts.SportBike,
		Status:    workouts.StatusPlanned,
		StartedAt: startedAt,
		EndedAt:   endedAt,
	}
	_, err := repo.Upsert(context.Background(), w)
	require.NoError(t, err)
	return w.ID
}

func insertBW(t *testing.T, repo *bodyweight.Repo, at time.Time, kg float64, bf *float64) {
	t.Helper()
	require.NoError(t, repo.Insert(context.Background(), &bodyweight.Entry{
		LoggedAt:   at,
		WeightKg:   kg,
		BodyFatPct: bf,
	}))
}

func ptrF(v float64) *float64 { return &v }

func doGet(t *testing.T, r *gin.Engine, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// ============================================================================
// Happy paths
// ============================================================================

func TestHappy_HighIntakeAdequate(t *testing.T) {
	f := setupHandlers(t)
	// Single calendar day: 2026-06-04 UTC.
	from := "2026-06-04T00:00:00Z"
	to := "2026-06-05T00:00:00Z"

	// 100 kcal/100g product. 3000g → 3000 kcal intake.
	pid := makeProduct(t, f.productsRepo, 100)
	insertMeal(t, f.mealsRepo, pid, time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC), 3000)

	// Workout with 600 kcal burned.
	insertWorkout(t, f.workoutsRepo,
		time.Date(2026, 6, 4, 8, 0, 0, 0, time.UTC),
		time.Date(2026, 6, 4, 9, 30, 0, 0, time.UTC),
		ptrF(600))

	rec := doGet(t, f.r,
		fmt.Sprintf("/energy/availability?from=%s&to=%s&lean_mass_kg=60", from, to))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var out energy.Availability
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Len(t, out.Days, 1)
	d := out.Days[0]
	assert.Equal(t, "2026-06-04", d.Date)
	assert.Equal(t, 3000.0, d.IntakeKcal)
	assert.Equal(t, 600.0, d.ExerciseEnergyKcal)
	assert.InDelta(t, 40.0, d.EA, 0.05) // (3000-600)/60
	assert.Equal(t, energy.BandSubOptimal, d.Band)
	assert.Empty(t, d.MissingBurnWorkoutIDs)
	assert.True(t, d.CompleteData)

	assert.Equal(t, 60.0, out.Composition.FFMKg)
	assert.Equal(t, energy.SourceExplicitLeanMass, out.Composition.Source)
}

// ============================================================================
// Missing-burn flagging
// ============================================================================

func TestMissingBurn_DayFlaggedAndExcludedFromWindow(t *testing.T) {
	f := setupHandlers(t)
	from := "2026-06-01T00:00:00Z"
	to := "2026-06-04T00:00:00Z" // 3 days

	// 3000 kcal/day intake.
	pid := makeProduct(t, f.productsRepo, 100)
	for day := 1; day <= 3; day++ {
		insertMeal(t, f.mealsRepo, pid, time.Date(2026, 6, day, 12, 0, 0, 0, time.UTC), 3000)
	}
	// Day 1: workout with kcal_burned. Day 2: workout without (missing). Day 3: no workout.
	insertWorkout(t, f.workoutsRepo,
		time.Date(2026, 6, 1, 8, 0, 0, 0, time.UTC),
		time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC), ptrF(600))
	missingID := insertWorkout(t, f.workoutsRepo,
		time.Date(2026, 6, 2, 8, 0, 0, 0, time.UTC),
		time.Date(2026, 6, 2, 9, 0, 0, 0, time.UTC), nil)

	rec := doGet(t, f.r,
		fmt.Sprintf("/energy/availability?from=%s&to=%s&lean_mass_kg=60", from, to))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var out energy.Availability
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Len(t, out.Days, 3)

	assert.True(t, out.Days[0].CompleteData, "day with kcal_burned set is complete")
	assert.Empty(t, out.Days[0].MissingBurnWorkoutIDs)

	assert.False(t, out.Days[1].CompleteData, "day with missing kcal_burned is incomplete")
	require.Len(t, out.Days[1].MissingBurnWorkoutIDs, 1)
	assert.Equal(t, missingID, out.Days[1].MissingBurnWorkoutIDs[0])
	assert.Equal(t, 0.0, out.Days[1].ExerciseEnergyKcal, "missing burn contributes zero")

	assert.True(t, out.Days[2].CompleteData, "day with no workouts has nothing missing")

	// Window aggregate: only days 1 and 3 are complete.
	assert.Equal(t, 3, out.Window.TotalDays)
	assert.Equal(t, 2, out.Window.DaysWithCompleteData)
	require.NotNil(t, out.Window.AvgEA)
	// Day 1 EA = (3000-600)/60 = 40.0. Day 3 EA = 3000/60 = 50.0. Mean = 45.0.
	assert.InDelta(t, 45.0, *out.Window.AvgEA, 0.05)
}

// A planned workout that overlaps the EA window must NOT mark its day
// incomplete — EA only counts completed sessions (add-garmin-daily-metrics).
func TestPlannedWorkout_ExcludedFromEnergyAvailability(t *testing.T) {
	f := setupHandlers(t)
	from := "2026-06-01T00:00:00Z"
	to := "2026-06-02T00:00:00Z" // single day

	pid := makeProduct(t, f.productsRepo, 100)
	insertMeal(t, f.mealsRepo, pid, time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC), 3000)
	// A planned session on the same day, no kcal_burned. Past-dated planned is allowed.
	insertPlannedWorkout(t, f.workoutsRepo,
		time.Date(2026, 6, 1, 8, 0, 0, 0, time.UTC),
		time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC))

	rec := doGet(t, f.r,
		fmt.Sprintf("/energy/availability?from=%s&to=%s&lean_mass_kg=60", from, to))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out energy.Availability
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Len(t, out.Days, 1)
	assert.True(t, out.Days[0].CompleteData, "planned workout must not flag the day incomplete")
	assert.Empty(t, out.Days[0].MissingBurnWorkoutIDs, "planned workout is not a missing-burn workout")
	assert.Equal(t, 0.0, out.Days[0].ExerciseEnergyKcal)
	require.NotNil(t, out.Window.AvgEA)
	// EA = 3000/60 = 50.0 (no exercise energy from the planned session).
	assert.InDelta(t, 50.0, *out.Window.AvgEA, 0.05)
}

func TestMissingBurn_WindowAggregateNullWhenAllDaysIncomplete(t *testing.T) {
	f := setupHandlers(t)
	from := "2026-06-01T00:00:00Z"
	to := "2026-06-03T00:00:00Z" // 2 days

	for day := 1; day <= 2; day++ {
		insertWorkout(t, f.workoutsRepo,
			time.Date(2026, 6, day, 8, 0, 0, 0, time.UTC),
			time.Date(2026, 6, day, 9, 0, 0, 0, time.UTC), nil)
	}

	rec := doGet(t, f.r,
		fmt.Sprintf("/energy/availability?from=%s&to=%s&lean_mass_kg=60", from, to))
	require.Equal(t, http.StatusOK, rec.Code)

	var out energy.Availability
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, 0, out.Window.DaysWithCompleteData)
	assert.Nil(t, out.Window.AvgEA, "headline number omitted when no day qualifies")
	assert.Nil(t, out.Window.Band)
}

// ============================================================================
// Empty days
// ============================================================================

func TestEmptyDay_AppearsWithZeros(t *testing.T) {
	f := setupHandlers(t)
	from := "2026-06-01T00:00:00Z"
	to := "2026-06-04T00:00:00Z" // 3 days, only day 2 has data

	pid := makeProduct(t, f.productsRepo, 100)
	insertMeal(t, f.mealsRepo, pid, time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC), 2500)

	rec := doGet(t, f.r,
		fmt.Sprintf("/energy/availability?from=%s&to=%s&lean_mass_kg=60", from, to))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var out energy.Availability
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Len(t, out.Days, 3)

	assert.Equal(t, 0.0, out.Days[0].IntakeKcal)
	assert.Equal(t, 0.0, out.Days[0].EA)
	assert.Equal(t, energy.BandLow, out.Days[0].Band)
	assert.True(t, out.Days[0].CompleteData, "empty day has nothing missing")

	assert.Equal(t, 2500.0, out.Days[1].IntakeKcal)
}

// ============================================================================
// TZ boundaries
// ============================================================================

func TestTZ_MealLogged2230ZBerlinTimeAppearsOnLocalDay(t *testing.T) {
	f := setupHandlers(t)
	// Berlin is UTC+2 in summer. 22:30Z on June 7 = 00:30 local on June 8.
	pid := makeProduct(t, f.productsRepo, 100)
	insertMeal(t, f.mealsRepo, pid, time.Date(2026, 6, 7, 22, 30, 0, 0, time.UTC), 1000)

	// Query the window covering June 8 in Berlin time only — June 7 UTC 22:00 ≈ June 7 00:00 (LOL no — let's pick something safer).
	// To keep test predictable, query a window in UTC RFC3339 that covers both potential days.
	from := "2026-06-07T00:00:00Z"
	to := "2026-06-09T00:00:00Z"

	rec := doGet(t, f.r,
		fmt.Sprintf("/energy/availability?from=%s&to=%s&tz=Europe/Berlin&lean_mass_kg=60", from, to))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var out energy.Availability
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, "Europe/Berlin", out.TZ)
	// Expected: the meal lands on the calendar day 2026-06-08 in Berlin.
	dayMap := map[string]float64{}
	for _, d := range out.Days {
		dayMap[d.Date] = d.IntakeKcal
	}
	assert.Equal(t, 1000.0, dayMap["2026-06-08"],
		"meal logged at 22:30Z falls on the LOCAL date (Berlin = June 8)")
	assert.Equal(t, 0.0, dayMap["2026-06-07"],
		"the LOCAL June 7 has no meals (the 22:30Z meal moved to June 8 local)")
}

func TestTZ_WorkoutSpansMidnight_AttributedToStartDay(t *testing.T) {
	f := setupHandlers(t)
	// Berlin local: workout starts 23:45 June 7, ends 01:15 June 8. UTC: 21:45 / 23:15.
	insertWorkout(t, f.workoutsRepo,
		time.Date(2026, 6, 7, 21, 45, 0, 0, time.UTC),
		time.Date(2026, 6, 7, 23, 15, 0, 0, time.UTC),
		ptrF(700))

	from := "2026-06-07T00:00:00Z"
	to := "2026-06-09T00:00:00Z"

	rec := doGet(t, f.r,
		fmt.Sprintf("/energy/availability?from=%s&to=%s&tz=Europe/Berlin&lean_mass_kg=60", from, to))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var out energy.Availability
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	dayMap := map[string]float64{}
	for _, d := range out.Days {
		dayMap[d.Date] = d.ExerciseEnergyKcal
	}
	assert.Equal(t, 700.0, dayMap["2026-06-07"],
		"workout attributed by start day (Loucks convention); the full 700 kcal lands on June 7 local")
	assert.Equal(t, 0.0, dayMap["2026-06-08"])
}

// ============================================================================
// Error codes
// ============================================================================

func TestError_WindowRequired(t *testing.T) {
	f := setupHandlers(t)
	rec := doGet(t, f.r, "/energy/availability?lean_mass_kg=60")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"window_required"}`, rec.Body.String())
}

func TestError_WindowInvalid_Inverted(t *testing.T) {
	f := setupHandlers(t)
	rec := doGet(t, f.r,
		"/energy/availability?from=2026-06-08T00:00:00Z&to=2026-06-01T00:00:00Z&lean_mass_kg=60")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"window_invalid"}`, rec.Body.String())
}

func TestError_WindowInvalid_Malformed(t *testing.T) {
	f := setupHandlers(t)
	rec := doGet(t, f.r,
		"/energy/availability?from=garbage&to=2026-06-08T00:00:00Z&lean_mass_kg=60")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"window_invalid"}`, rec.Body.String())
}

func TestError_RangeTooLarge(t *testing.T) {
	f := setupHandlers(t)
	rec := doGet(t, f.r,
		"/energy/availability?from=2026-01-01T00:00:00Z&to=2026-12-31T00:00:00Z&lean_mass_kg=60")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "range_too_large", body["error"])
	assert.EqualValues(t, 92, body["max_days"])
}

func TestError_TZInvalid(t *testing.T) {
	f := setupHandlers(t)
	rec := doGet(t, f.r,
		"/energy/availability?from=2026-06-01T00:00:00Z&to=2026-06-08T00:00:00Z&tz=Not/A_Real_TZ&lean_mass_kg=60")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"tz_invalid"}`, rec.Body.String())
}

func TestError_LeanMassInvalid(t *testing.T) {
	f := setupHandlers(t)
	for _, v := range []string{"0", "-1", "not-a-number"} {
		rec := doGet(t, f.r,
			fmt.Sprintf("/energy/availability?from=2026-06-01T00:00:00Z&to=2026-06-08T00:00:00Z&lean_mass_kg=%s", v))
		assert.Equal(t, http.StatusBadRequest, rec.Code, "value=%s", v)
		assert.JSONEq(t, `{"error":"lean_mass_kg_invalid"}`, rec.Body.String(), "value=%s", v)
	}
}

func TestError_BodyFatInvalid(t *testing.T) {
	f := setupHandlers(t)
	// Need a body-weight entry so the composition tries to compute (otherwise
	// weight_data_missing wins before body-fat validation runs).
	insertBW(t, f.bwRepo, time.Date(2026, 6, 4, 7, 0, 0, 0, time.UTC), 72.0, nil)

	for _, v := range []string{"-1", "100", "150"} {
		rec := doGet(t, f.r,
			fmt.Sprintf("/energy/availability?from=2026-06-01T00:00:00Z&to=2026-06-08T00:00:00Z&body_fat_pct=%s", v))
		assert.Equal(t, http.StatusBadRequest, rec.Code, "value=%s", v)
		assert.JSONEq(t, `{"error":"body_fat_pct_invalid"}`, rec.Body.String(), "value=%s", v)
	}
}

func TestError_WeightDataMissing(t *testing.T) {
	f := setupHandlers(t)
	rec := doGet(t, f.r,
		"/energy/availability?from=2026-06-01T00:00:00Z&to=2026-06-08T00:00:00Z")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"weight_data_missing"}`, rec.Body.String())
}

// ============================================================================
// Composition source paths via the wire
// ============================================================================

func TestSource_StoredBodyFat(t *testing.T) {
	f := setupHandlers(t)
	insertBW(t, f.bwRepo, time.Date(2026, 6, 4, 7, 0, 0, 0, time.UTC), 80.0, ptrF(15.0))

	rec := doGet(t, f.r,
		"/energy/availability?from=2026-06-01T00:00:00Z&to=2026-06-08T00:00:00Z")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var out energy.Availability
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, energy.SourceStoredBodyFat, out.Composition.Source)
	assert.InDelta(t, 80.0*(1-0.15), out.Composition.FFMKg, 0.05)
}

func TestSource_Fallback85pct_FlagSet(t *testing.T) {
	f := setupHandlers(t)
	// Weight entry with no body_fat_pct → falls back to 85%.
	insertBW(t, f.bwRepo, time.Date(2026, 6, 4, 7, 0, 0, 0, time.UTC), 80.0, nil)

	rec := doGet(t, f.r,
		"/energy/availability?from=2026-06-01T00:00:00Z&to=2026-06-08T00:00:00Z")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var out energy.Availability
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, energy.SourceEstimated85pct, out.Composition.Source)
	assert.True(t, out.Composition.CompositionEstimated)
}
