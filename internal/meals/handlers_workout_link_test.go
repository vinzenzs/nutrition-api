package meals_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/meals"
	"github.com/vinzenzs/kazper/internal/products"
	"github.com/vinzenzs/kazper/internal/store/storetest"
	"github.com/vinzenzs/kazper/internal/workouts"
)

// linkedFixture wires meals with workouts existence-validation enabled.
type linkedFixture struct {
	r            *gin.Engine
	productsRepo *products.Repo
	workoutsRepo *workouts.Repo
}

func setupMealsLinked(t *testing.T) *linkedFixture {
	t.Helper()
	p := storetest.NewPool(t)
	pRepo := products.NewRepo(p)
	mRepo := meals.NewRepo(p)
	wRepo := workouts.NewRepo(p)
	svc := meals.NewService(p, mRepo, pRepo)
	svc.SetWorkoutsRepo(wRepo)
	r := gin.New()
	rg := r.Group("/")
	meals.NewHandlers(svc).Register(rg)
	return &linkedFixture{r: r, productsRepo: pRepo, workoutsRepo: wRepo}
}

// seedWorkout inserts a manual workout and returns its id.
func seedWorkout(t *testing.T, repo *workouts.Repo) uuid.UUID {
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

func TestCreateMeal_WithWorkoutID_Success(t *testing.T) {
	f := setupMealsLinked(t)
	pid := makeProduct(t, f.productsRepo)
	wid := seedWorkout(t, f.workoutsRepo)

	body := fmt.Sprintf(
		`{"product_id":%q,"quantity_g":150,"logged_at":"2026-06-07T08:30:00Z","workout_id":%q}`,
		pid, wid)
	rec := doRequest(t, f.r, http.MethodPost, "/meals", body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var m meals.MealEntry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &m))
	require.NotNil(t, m.WorkoutID)
	assert.Equal(t, wid, *m.WorkoutID)
}

func TestCreateMeal_WithUnknownWorkoutID_400(t *testing.T) {
	f := setupMealsLinked(t)
	pid := makeProduct(t, f.productsRepo)

	body := fmt.Sprintf(
		`{"product_id":%q,"quantity_g":150,"logged_at":"2026-06-07T08:30:00Z","workout_id":"00000000-0000-0000-0000-000000000000"}`,
		pid)
	rec := doRequest(t, f.r, http.MethodPost, "/meals", body)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"workout_not_found"}`, rec.Body.String())
}

func TestCreateMeal_WorkoutIDOmittedPersistsAsNull(t *testing.T) {
	f := setupMealsLinked(t)
	pid := makeProduct(t, f.productsRepo)

	body := fmt.Sprintf(
		`{"product_id":%q,"quantity_g":150,"logged_at":"2026-06-07T08:30:00Z"}`, pid)
	rec := doRequest(t, f.r, http.MethodPost, "/meals", body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var m meals.MealEntry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &m))
	assert.Nil(t, m.WorkoutID)
	// JSON should omit the field via omitempty.
	assert.NotContains(t, rec.Body.String(), "workout_id")
}

func TestPatchMeal_SetWorkoutID(t *testing.T) {
	f := setupMealsLinked(t)
	pid := makeProduct(t, f.productsRepo)
	wid := seedWorkout(t, f.workoutsRepo)

	// Create without a link.
	body := fmt.Sprintf(`{"product_id":%q,"quantity_g":150,"logged_at":"2026-06-07T08:30:00Z"}`, pid)
	rec := doRequest(t, f.r, http.MethodPost, "/meals", body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var created meals.MealEntry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))

	// PATCH to set the link.
	patchBody := fmt.Sprintf(`{"workout_id":%q}`, wid)
	rec = doRequest(t, f.r, http.MethodPatch, "/meals/"+created.ID.String(), patchBody)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var patched meals.MealEntry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &patched))
	require.NotNil(t, patched.WorkoutID)
	assert.Equal(t, wid, *patched.WorkoutID)
}

func TestPatchMeal_ClearWorkoutID_EmptyString(t *testing.T) {
	f := setupMealsLinked(t)
	pid := makeProduct(t, f.productsRepo)
	wid := seedWorkout(t, f.workoutsRepo)

	// Create with a link.
	body := fmt.Sprintf(
		`{"product_id":%q,"quantity_g":150,"logged_at":"2026-06-07T08:30:00Z","workout_id":%q}`,
		pid, wid)
	rec := doRequest(t, f.r, http.MethodPost, "/meals", body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var created meals.MealEntry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	require.NotNil(t, created.WorkoutID)

	// PATCH with "" to clear.
	rec = doRequest(t, f.r, http.MethodPatch, "/meals/"+created.ID.String(), `{"workout_id":""}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var patched meals.MealEntry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &patched))
	assert.Nil(t, patched.WorkoutID)
	assert.NotContains(t, rec.Body.String(), "workout_id")
}

func TestPatchMeal_OmittedWorkoutIDLeavesUnchanged(t *testing.T) {
	f := setupMealsLinked(t)
	pid := makeProduct(t, f.productsRepo)
	wid := seedWorkout(t, f.workoutsRepo)

	body := fmt.Sprintf(
		`{"product_id":%q,"quantity_g":150,"logged_at":"2026-06-07T08:30:00Z","workout_id":%q}`,
		pid, wid)
	rec := doRequest(t, f.r, http.MethodPost, "/meals", body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var created meals.MealEntry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))

	// PATCH only the quantity; workout_id absent → preserved.
	rec = doRequest(t, f.r, http.MethodPatch, "/meals/"+created.ID.String(), `{"quantity_g":200}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var patched meals.MealEntry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &patched))
	require.NotNil(t, patched.WorkoutID, "workout_id should be preserved")
	assert.Equal(t, wid, *patched.WorkoutID)
}

func TestPatchMeal_UnknownWorkoutID_400(t *testing.T) {
	f := setupMealsLinked(t)
	pid := makeProduct(t, f.productsRepo)

	body := fmt.Sprintf(`{"product_id":%q,"quantity_g":150,"logged_at":"2026-06-07T08:30:00Z"}`, pid)
	rec := doRequest(t, f.r, http.MethodPost, "/meals", body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var created meals.MealEntry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))

	rec = doRequest(t, f.r, http.MethodPatch, "/meals/"+created.ID.String(),
		`{"workout_id":"00000000-0000-0000-0000-000000000000"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"workout_not_found"}`, rec.Body.String())
}

func TestDeleteWorkout_CascadesNullOnLinkedMeal(t *testing.T) {
	f := setupMealsLinked(t)
	pid := makeProduct(t, f.productsRepo)
	wid := seedWorkout(t, f.workoutsRepo)

	// Create a meal linked to the workout.
	body := fmt.Sprintf(
		`{"product_id":%q,"quantity_g":150,"logged_at":"2026-06-07T08:30:00Z","workout_id":%q}`,
		pid, wid)
	rec := doRequest(t, f.r, http.MethodPost, "/meals", body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var created meals.MealEntry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))

	// Delete the workout via the repo (simulates DELETE /workouts/{id}).
	require.NoError(t, f.workoutsRepo.Delete(context.Background(), wid))

	// Re-fetch the meal; workout_id should now be NULL.
	rec = doRequest(t, f.r, http.MethodGet, "/meals/"+created.ID.String(), "")
	require.Equal(t, http.StatusOK, rec.Code)
	var refetched meals.MealEntry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &refetched))
	assert.Nil(t, refetched.WorkoutID, "workout deletion should cascade SET NULL on the meal")
}
