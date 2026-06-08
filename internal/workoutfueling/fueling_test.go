package workoutfueling_test

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

	"github.com/vinzenzs/nutrition-api/internal/hydration"
	"github.com/vinzenzs/nutrition-api/internal/meals"
	"github.com/vinzenzs/nutrition-api/internal/products"
	"github.com/vinzenzs/nutrition-api/internal/store/storetest"
	"github.com/vinzenzs/nutrition-api/internal/workoutfuel"
	"github.com/vinzenzs/nutrition-api/internal/workoutfueling"
	"github.com/vinzenzs/nutrition-api/internal/workouts"
)

func init() {
	gin.SetMode(gin.TestMode)
}

type fixture struct {
	r            *gin.Engine
	productsRepo *products.Repo
	mealsRepo    *meals.Repo
	hydRepo      *hydration.Repo
	fuelRepo     *workoutfuel.Repo
	workoutsRepo *workouts.Repo
}

func setup(t *testing.T) *fixture {
	t.Helper()
	pool := storetest.NewPool(t)
	pRepo := products.NewRepo(pool)
	mRepo := meals.NewRepo(pool)
	hRepo := hydration.NewRepo(pool)
	wRepo := workouts.NewRepo(pool)
	fRepo := workoutfuel.NewRepo(pool)
	svc := workoutfueling.NewService(wRepo, mRepo, hRepo, fRepo)
	r := gin.New()
	rg := r.Group("/")
	workoutfueling.NewHandlers(svc).Register(rg)
	return &fixture{
		r: r, productsRepo: pRepo, mealsRepo: mRepo, hydRepo: hRepo, fuelRepo: fRepo, workoutsRepo: wRepo,
	}
}

func makeProduct(t *testing.T, repo *products.Repo, kcalPer100g, carbsPer100g float64) uuid.UUID {
	t.Helper()
	k := kcalPer100g
	c := carbsPer100g
	p := &products.Product{
		Name:   "test-product",
		Source: products.SourceManual,
		Nutriments: products.Nutriments{
			KcalPer100g:   &k,
			CarbsGPer100g: &c,
		},
	}
	require.NoError(t, repo.Insert(context.Background(), p))
	return p.ID
}

// makeWorkout: a Z2 bike from 08:00 to 09:30 UTC by default.
func makeWorkout(t *testing.T, repo *workouts.Repo) uuid.UUID {
	t.Helper()
	w := &workouts.Workout{
		Source:    workouts.SourceManual,
		Sport:     workouts.SportBike,
		StartedAt: time.Date(2026, 6, 7, 8, 0, 0, 0, time.UTC),
		EndedAt:   time.Date(2026, 6, 7, 9, 30, 0, 0, time.UTC),
	}
	_, err := repo.Upsert(context.Background(), w)
	require.NoError(t, err)
	return w.ID
}

func insertMeal(t *testing.T, repo *meals.Repo, pid uuid.UUID, at time.Time, qty float64) {
	t.Helper()
	_, err := repo.Insert(context.Background(), meals.InsertParams{
		ProductID: &pid,
		LoggedAt:  at,
		QuantityG: qty,
	})
	require.NoError(t, err)
}

func insertHydration(t *testing.T, repo *hydration.Repo, at time.Time, ml float64, workoutID *uuid.UUID) {
	t.Helper()
	e := &hydration.Entry{LoggedAt: at, QuantityMl: ml, WorkoutID: workoutID}
	require.NoError(t, repo.Insert(context.Background(), e))
}

type fuelOpts struct {
	carbs     *float64
	sodium    *float64
	potass    *float64
	caffeine  *float64
	ml        *float64
	workoutID *uuid.UUID
	name      string
}

func insertFuel(t *testing.T, repo *workoutfuel.Repo, at time.Time, opts fuelOpts) {
	t.Helper()
	name := opts.name
	if name == "" {
		name = "test-fuel"
	}
	e := &workoutfuel.Entry{
		LoggedAt:    at,
		Name:        name,
		QuantityMl:  opts.ml,
		CarbsG:      opts.carbs,
		SodiumMg:    opts.sodium,
		PotassiumMg: opts.potass,
		CaffeineMg:  opts.caffeine,
		WorkoutID:   opts.workoutID,
	}
	require.NoError(t, repo.Insert(context.Background(), e))
}

func ptr(f float64) *float64 { return &f }

func doGet(t *testing.T, r *gin.Engine, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// ============================================================================

func TestFueling_DefaultWindowsBucketCorrectly(t *testing.T) {
	f := setup(t)
	wid := makeWorkout(t, f.workoutsRepo)
	pid := makeProduct(t, f.productsRepo, 100, 25) // 100 kcal/100g, 25g carbs/100g

	// Pre-window (07:00, 1h before start): 200g banana = 200 kcal, 50g carbs.
	insertMeal(t, f.mealsRepo, pid, time.Date(2026, 6, 7, 7, 0, 0, 0, time.UTC), 200)
	// Intra: hydration 500ml at 08:30.
	insertHydration(t, f.hydRepo, time.Date(2026, 6, 7, 8, 30, 0, 0, time.UTC), 500, nil)
	// Post: 100g recovery snack at 09:45 = 100 kcal, 25g carbs.
	insertMeal(t, f.mealsRepo, pid, time.Date(2026, 6, 7, 9, 45, 0, 0, time.UTC), 100)

	rec := doGet(t, f.r, "/workouts/"+wid.String()+"/fueling")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out workoutfueling.WorkoutFueling
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))

	// Pre.
	assert.Equal(t, 240, out.PreWindow.Minutes)
	assert.Equal(t, 1, out.PreWindow.Nutrition.EntryCount)
	assert.Equal(t, 200.0, out.PreWindow.Nutrition.Totals.Kcal)
	assert.Equal(t, 50.0, out.PreWindow.Nutrition.Totals.CarbsG)
	assert.Equal(t, 0.0, out.PreWindow.Hydration.TotalMl)
	// Intra.
	assert.Equal(t, 90, out.IntraWindow.Minutes, "08:00-09:30 = 90 min")
	assert.Equal(t, 0, out.IntraWindow.Nutrition.EntryCount)
	assert.Equal(t, 500.0, out.IntraWindow.Hydration.TotalMl)
	assert.Equal(t, 1, out.IntraWindow.Hydration.EntryCount)
	// Post.
	assert.Equal(t, 60, out.PostWindow.Minutes)
	assert.Equal(t, 1, out.PostWindow.Nutrition.EntryCount)
	assert.Equal(t, 100.0, out.PostWindow.Nutrition.Totals.Kcal)
}

func TestFueling_CustomWindowsHonored(t *testing.T) {
	f := setup(t)
	wid := makeWorkout(t, f.workoutsRepo)
	pid := makeProduct(t, f.productsRepo, 100, 25)

	// 3h before start (out of default 4h pre, in of 6h pre).
	insertMeal(t, f.mealsRepo, pid, time.Date(2026, 6, 7, 5, 0, 0, 0, time.UTC), 100)

	rec := doGet(t, f.r, "/workouts/"+wid.String()+"/fueling?pre_window_min=360")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out workoutfueling.WorkoutFueling
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, 360, out.PreWindow.Minutes)
	assert.Equal(t, 1, out.PreWindow.Nutrition.EntryCount)
}

func TestFueling_ZeroWindowReturnsEmptyBucket(t *testing.T) {
	f := setup(t)
	wid := makeWorkout(t, f.workoutsRepo)
	pid := makeProduct(t, f.productsRepo, 100, 25)
	insertMeal(t, f.mealsRepo, pid, time.Date(2026, 6, 7, 7, 30, 0, 0, time.UTC), 100)

	rec := doGet(t, f.r, "/workouts/"+wid.String()+"/fueling?pre_window_min=0")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out workoutfueling.WorkoutFueling
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, 0, out.PreWindow.Minutes)
	assert.Equal(t, 0, out.PreWindow.Nutrition.EntryCount)
}

func TestFueling_BoundaryAtStartedAtBelongsToIntra(t *testing.T) {
	f := setup(t)
	wid := makeWorkout(t, f.workoutsRepo)
	pid := makeProduct(t, f.productsRepo, 100, 25)
	// Exactly at started_at.
	insertMeal(t, f.mealsRepo, pid, time.Date(2026, 6, 7, 8, 0, 0, 0, time.UTC), 100)

	rec := doGet(t, f.r, "/workouts/"+wid.String()+"/fueling")
	require.Equal(t, http.StatusOK, rec.Code)
	var out workoutfueling.WorkoutFueling
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, 0, out.PreWindow.Nutrition.EntryCount, "boundary belongs to intra, not pre")
	assert.Equal(t, 1, out.IntraWindow.Nutrition.EntryCount)
}

func TestFueling_BoundaryAtEndedAtBelongsToPost(t *testing.T) {
	f := setup(t)
	wid := makeWorkout(t, f.workoutsRepo)
	pid := makeProduct(t, f.productsRepo, 100, 25)
	// Exactly at ended_at.
	insertMeal(t, f.mealsRepo, pid, time.Date(2026, 6, 7, 9, 30, 0, 0, time.UTC), 100)

	rec := doGet(t, f.r, "/workouts/"+wid.String()+"/fueling")
	require.Equal(t, http.StatusOK, rec.Code)
	var out workoutfueling.WorkoutFueling
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, 0, out.IntraWindow.Nutrition.EntryCount, "boundary belongs to post, not intra")
	assert.Equal(t, 1, out.PostWindow.Nutrition.EntryCount)
}

func TestFueling_TaggedButOutsideWindowIsExcluded(t *testing.T) {
	f := setup(t)
	wid := makeWorkout(t, f.workoutsRepo)
	// Hydration 8h before, tagged with the workout. Outside the default 240min pre-window.
	insertHydration(t, f.hydRepo,
		time.Date(2026, 6, 7, 0, 0, 0, 0, time.UTC), 500, &wid)

	rec := doGet(t, f.r, "/workouts/"+wid.String()+"/fueling")
	require.Equal(t, http.StatusOK, rec.Code)
	var out workoutfueling.WorkoutFueling
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, 0.0, out.PreWindow.Hydration.TotalMl,
		"tagged-but-outside-window must not appear in any window total")
	assert.Equal(t, 0.0, out.IntraWindow.Hydration.TotalMl)
	assert.Equal(t, 0.0, out.PostWindow.Hydration.TotalMl)
}

func TestFueling_UntaggedInsideWindowIsIncluded(t *testing.T) {
	f := setup(t)
	wid := makeWorkout(t, f.workoutsRepo)
	// 30 min before start, untagged.
	insertHydration(t, f.hydRepo,
		time.Date(2026, 6, 7, 7, 30, 0, 0, time.UTC), 750, nil)

	rec := doGet(t, f.r, "/workouts/"+wid.String()+"/fueling")
	require.Equal(t, http.StatusOK, rec.Code)
	var out workoutfueling.WorkoutFueling
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, 750.0, out.PreWindow.Hydration.TotalMl,
		"untagged-but-in-window entry must be aggregated (time-window matching, not tag)")
}

func TestFueling_EmptyWorkoutReturnsZeros(t *testing.T) {
	f := setup(t)
	wid := makeWorkout(t, f.workoutsRepo)

	rec := doGet(t, f.r, "/workouts/"+wid.String()+"/fueling")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out workoutfueling.WorkoutFueling
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, 0, out.PreWindow.Nutrition.EntryCount)
	assert.Equal(t, 0, out.IntraWindow.Hydration.EntryCount)
	assert.Equal(t, 0, out.PostWindow.Nutrition.EntryCount)
	assert.Equal(t, 0.0, out.IntraWindow.Nutrition.Totals.Kcal)
}

func TestFueling_404OnUnknownWorkout(t *testing.T) {
	f := setup(t)
	rec := doGet(t, f.r, "/workouts/00000000-0000-0000-0000-000000000000/fueling")
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.JSONEq(t, `{"error":"workout_not_found"}`, rec.Body.String())
}

func TestFueling_WindowInvalid(t *testing.T) {
	f := setup(t)
	wid := makeWorkout(t, f.workoutsRepo)

	rec := doGet(t, f.r, "/workouts/"+wid.String()+"/fueling?pre_window_min=-1")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "window_invalid", body["error"])

	rec = doGet(t, f.r, "/workouts/"+wid.String()+"/fueling?post_window_min=721")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestFueling_Rounding(t *testing.T) {
	f := setup(t)
	wid := makeWorkout(t, f.workoutsRepo)
	// 100 kcal/100g × 100.001g = 100.001 kcal. After 1dp rounding: 100.0.
	// Compose values that sum to a .x666 fraction: 100.0 kcal/100g × 41.999g = 41.999.
	// Two entries: 41.999 + 41.999 + 41.999 ≈ 125.997 → rounds to 126.0.
	// Simpler: use 100 kcal/100g × 33.333g = 33.333; three of them = 99.999 → rounds to 100.0.
	pid := makeProduct(t, f.productsRepo, 100, 0)
	for i, at := range []time.Time{
		time.Date(2026, 6, 7, 8, 10, 0, 0, time.UTC),
		time.Date(2026, 6, 7, 8, 20, 0, 0, time.UTC),
		time.Date(2026, 6, 7, 8, 30, 0, 0, time.UTC),
	} {
		_ = i
		insertMeal(t, f.mealsRepo, pid, at, 33.333)
	}

	rec := doGet(t, f.r, fmt.Sprintf("/workouts/%s/fueling", wid))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out workoutfueling.WorkoutFueling
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	// 100 × 99.999/100 = 99.999 → 100.0
	assert.Equal(t, 100.0, out.IntraWindow.Nutrition.Totals.Kcal,
		"rounding to 1dp at the response boundary")
}

// ============================================================================
// workout_fuel sub-object (added by add-workout-fuel)
// ============================================================================

// Intra fuel entry contributes to intra_window.workout_fuel.totals.
func TestFueling_WorkoutFuelIntraContributes(t *testing.T) {
	f := setup(t)
	wid := makeWorkout(t, f.workoutsRepo)
	insertFuel(t, f.fuelRepo, time.Date(2026, 6, 7, 8, 30, 0, 0, time.UTC), fuelOpts{
		name: "Gel A", carbs: ptr(25), sodium: ptr(100),
	})
	insertFuel(t, f.fuelRepo, time.Date(2026, 6, 7, 8, 45, 0, 0, time.UTC), fuelOpts{
		name: "Drink B", ml: ptr(500), carbs: ptr(25), sodium: ptr(200),
	})

	rec := doGet(t, f.r, "/workouts/"+wid.String()+"/fueling")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out workoutfueling.WorkoutFueling
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, 2, out.IntraWindow.WorkoutFuel.EntryCount)
	require.NotNil(t, out.IntraWindow.WorkoutFuel.Totals.CarbsG)
	assert.Equal(t, 50.0, *out.IntraWindow.WorkoutFuel.Totals.CarbsG)
	require.NotNil(t, out.IntraWindow.WorkoutFuel.Totals.SodiumMg)
	assert.Equal(t, 300.0, *out.IntraWindow.WorkoutFuel.Totals.SodiumMg)
	require.NotNil(t, out.IntraWindow.WorkoutFuel.Totals.QuantityMl,
		"sum across nil+500 contributors should yield 500, not nil")
	assert.Equal(t, 500.0, *out.IntraWindow.WorkoutFuel.Totals.QuantityMl)
}

// Boundary at started_at lands in intra, not pre.
func TestFueling_WorkoutFuelBoundaryStartedAtIntoIntra(t *testing.T) {
	f := setup(t)
	wid := makeWorkout(t, f.workoutsRepo)
	insertFuel(t, f.fuelRepo, time.Date(2026, 6, 7, 8, 0, 0, 0, time.UTC), fuelOpts{
		name: "Boundary gel", carbs: ptr(25),
	})

	rec := doGet(t, f.r, "/workouts/"+wid.String()+"/fueling")
	require.Equal(t, http.StatusOK, rec.Code)
	var out workoutfueling.WorkoutFueling
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, 0, out.PreWindow.WorkoutFuel.EntryCount)
	assert.Equal(t, 1, out.IntraWindow.WorkoutFuel.EntryCount)
}

// Boundary at ended_at lands in post, not intra.
func TestFueling_WorkoutFuelBoundaryEndedAtIntoPost(t *testing.T) {
	f := setup(t)
	wid := makeWorkout(t, f.workoutsRepo)
	insertFuel(t, f.fuelRepo, time.Date(2026, 6, 7, 9, 30, 0, 0, time.UTC), fuelOpts{
		name: "Recovery shake", carbs: ptr(20),
	})

	rec := doGet(t, f.r, "/workouts/"+wid.String()+"/fueling")
	require.Equal(t, http.StatusOK, rec.Code)
	var out workoutfueling.WorkoutFueling
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, 0, out.IntraWindow.WorkoutFuel.EntryCount)
	assert.Equal(t, 1, out.PostWindow.WorkoutFuel.EntryCount)
}

// All three sub-objects populated in the same intra window.
func TestFueling_AllThreeSubObjectsCoexist(t *testing.T) {
	f := setup(t)
	wid := makeWorkout(t, f.workoutsRepo)
	pid := makeProduct(t, f.productsRepo, 100, 25) // 100 kcal/100g, 25g carbs/100g

	// Intra meal: 200g banana = 200 kcal, 50g carbs
	insertMeal(t, f.mealsRepo, pid, time.Date(2026, 6, 7, 8, 15, 0, 0, time.UTC), 200)
	// Intra hydration: 500ml
	insertHydration(t, f.hydRepo, time.Date(2026, 6, 7, 8, 30, 0, 0, time.UTC), 500, nil)
	// Intra fuel: gel
	insertFuel(t, f.fuelRepo, time.Date(2026, 6, 7, 8, 45, 0, 0, time.UTC), fuelOpts{
		name: "Gel", carbs: ptr(25), caffeine: ptr(100),
	})

	rec := doGet(t, f.r, "/workouts/"+wid.String()+"/fueling")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out workoutfueling.WorkoutFueling
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, 1, out.IntraWindow.Nutrition.EntryCount)
	assert.Equal(t, 200.0, out.IntraWindow.Nutrition.Totals.Kcal)
	assert.Equal(t, 500.0, out.IntraWindow.Hydration.TotalMl)
	assert.Equal(t, 1, out.IntraWindow.WorkoutFuel.EntryCount)
	require.NotNil(t, out.IntraWindow.WorkoutFuel.Totals.CaffeineMg)
	assert.Equal(t, 100.0, *out.IntraWindow.WorkoutFuel.Totals.CaffeineMg)
}

// Empty workout: workout_fuel.entry_count = 0; totals nullable fields stay nil.
func TestFueling_WorkoutFuelEmptyWindow(t *testing.T) {
	f := setup(t)
	wid := makeWorkout(t, f.workoutsRepo)
	rec := doGet(t, f.r, "/workouts/"+wid.String()+"/fueling")
	require.Equal(t, http.StatusOK, rec.Code)
	var out workoutfueling.WorkoutFueling
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, 0, out.PreWindow.WorkoutFuel.EntryCount)
	assert.Equal(t, 0, out.IntraWindow.WorkoutFuel.EntryCount)
	assert.Equal(t, 0, out.PostWindow.WorkoutFuel.EntryCount)
	assert.Nil(t, out.IntraWindow.WorkoutFuel.Totals.CarbsG,
		"no contributors → all totals stay nil")
	assert.Nil(t, out.IntraWindow.WorkoutFuel.Totals.SodiumMg)
}

// Rounding: assemble values that produce a .x666 fraction.
func TestFueling_WorkoutFuelRoundingAtResponseBoundary(t *testing.T) {
	f := setup(t)
	wid := makeWorkout(t, f.workoutsRepo)
	// Three contributions: 25.5556, 25.5556, 25.5556 → 76.6668 → 76.7.
	for _, at := range []time.Time{
		time.Date(2026, 6, 7, 8, 10, 0, 0, time.UTC),
		time.Date(2026, 6, 7, 8, 20, 0, 0, time.UTC),
		time.Date(2026, 6, 7, 8, 30, 0, 0, time.UTC),
	} {
		insertFuel(t, f.fuelRepo, at, fuelOpts{
			name: "Gel", carbs: ptr(25.5556),
		})
	}

	rec := doGet(t, f.r, "/workouts/"+wid.String()+"/fueling")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out workoutfueling.WorkoutFueling
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	// DB rounds to NUMERIC(10,1) on insert: 25.5556 → 25.6. Three of them: 76.8.
	require.NotNil(t, out.IntraWindow.WorkoutFuel.Totals.CarbsG)
	assert.Equal(t, 76.8, *out.IntraWindow.WorkoutFuel.Totals.CarbsG)
}
