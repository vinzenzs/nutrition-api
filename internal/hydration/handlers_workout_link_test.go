package hydration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/nutrition-api/internal/hydration"
	"github.com/vinzenzs/nutrition-api/internal/store/storetest"
	"github.com/vinzenzs/nutrition-api/internal/workouts"
)

type hydLinkedFixture struct {
	r            *gin.Engine
	workoutsRepo *workouts.Repo
}

func setupHydrationLinked(t *testing.T) *hydLinkedFixture {
	t.Helper()
	pool := storetest.NewPool(t)
	hRepo := hydration.NewRepo(pool)
	wRepo := workouts.NewRepo(pool)
	svc := hydration.NewService(hRepo)
	svc.SetWorkoutsRepo(wRepo)
	r := gin.New()
	rg := r.Group("/")
	hydration.NewHandlers(svc).Register(rg)
	// SummaryHandlers attached too for the §5.4 assertion below.
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelWarn}))
	hydration.NewSummaryHandlers(svc, "UTC", logger).Register(rg)
	return &hydLinkedFixture{r: r, workoutsRepo: wRepo}
}

func seedHydWorkout(t *testing.T, repo *workouts.Repo) uuid.UUID {
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

func doHydReq(t *testing.T, r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	return doRequest(t, r, method, path, body, nil)
}

func TestHydration_CreateWithWorkoutID(t *testing.T) {
	f := setupHydrationLinked(t)
	wid := seedHydWorkout(t, f.workoutsRepo)

	body := fmt.Sprintf(`{"quantity_ml":500,"logged_at":"2026-06-07T08:30:00Z","workout_id":%q}`, wid)
	rec := doHydReq(t, f.r, http.MethodPost, "/hydration", body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var e hydration.Entry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &e))
	require.NotNil(t, e.WorkoutID)
	assert.Equal(t, wid, *e.WorkoutID)
}

func TestHydration_CreateWithUnknownWorkoutID_400(t *testing.T) {
	f := setupHydrationLinked(t)
	rec := doHydReq(t, f.r, http.MethodPost, "/hydration",
		`{"quantity_ml":500,"logged_at":"2026-06-07T08:30:00Z","workout_id":"00000000-0000-0000-0000-000000000000"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"workout_not_found"}`, rec.Body.String())
}

func TestHydration_PatchSetClearNoTouchWorkoutID(t *testing.T) {
	f := setupHydrationLinked(t)
	wid := seedHydWorkout(t, f.workoutsRepo)

	rec := doHydReq(t, f.r, http.MethodPost, "/hydration",
		`{"quantity_ml":500,"logged_at":"2026-06-07T08:30:00Z"}`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var e hydration.Entry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &e))

	// Set.
	patchSet := fmt.Sprintf(`{"workout_id":%q}`, wid)
	rec = doHydReq(t, f.r, http.MethodPatch, "/hydration/"+e.ID.String(), patchSet)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var afterSet hydration.Entry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &afterSet))
	require.NotNil(t, afterSet.WorkoutID)
	assert.Equal(t, wid, *afterSet.WorkoutID)

	// No-touch: patch quantity only.
	rec = doHydReq(t, f.r, http.MethodPatch, "/hydration/"+e.ID.String(), `{"quantity_ml":600}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var afterNoTouch hydration.Entry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &afterNoTouch))
	require.NotNil(t, afterNoTouch.WorkoutID)
	assert.Equal(t, wid, *afterNoTouch.WorkoutID)

	// Clear via "".
	rec = doHydReq(t, f.r, http.MethodPatch, "/hydration/"+e.ID.String(), `{"workout_id":""}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var afterClear hydration.Entry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &afterClear))
	assert.Nil(t, afterClear.WorkoutID)
}

func TestHydration_DeleteWorkoutCascadesNull(t *testing.T) {
	f := setupHydrationLinked(t)
	wid := seedHydWorkout(t, f.workoutsRepo)

	body := fmt.Sprintf(`{"quantity_ml":500,"logged_at":"2026-06-07T08:30:00Z","workout_id":%q}`, wid)
	rec := doHydReq(t, f.r, http.MethodPost, "/hydration", body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var e hydration.Entry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &e))

	require.NoError(t, f.workoutsRepo.Delete(context.Background(), wid))

	// Re-fetch via the list endpoint.
	rec = doHydReq(t, f.r, http.MethodGet,
		"/hydration?from=2026-06-07T00:00:00Z&to=2026-06-08T00:00:00Z", "")
	require.Equal(t, http.StatusOK, rec.Code)
	var body2 struct {
		Entries []hydration.Entry `json:"entries"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body2))
	require.Len(t, body2.Entries, 1)
	assert.Nil(t, body2.Entries[0].WorkoutID, "workout deletion should cascade SET NULL on the hydration entry")
}

// TestHydration_DailySummaryIncludesBothTaggedAndUntagged covers task 5.4:
// the daily hydration aggregation must include every entry on the date,
// regardless of whether `workout_id` is set.
func TestHydration_DailySummaryIncludesBothTaggedAndUntagged(t *testing.T) {
	f := setupHydrationLinked(t)
	wid := seedHydWorkout(t, f.workoutsRepo)

	// One tagged, one untagged, both on 2026-06-07 UTC.
	tagged := fmt.Sprintf(`{"quantity_ml":500,"logged_at":"2026-06-07T08:30:00Z","workout_id":%q}`, wid)
	require.Equal(t, http.StatusCreated,
		doHydReq(t, f.r, http.MethodPost, "/hydration", tagged).Code)
	untagged := `{"quantity_ml":300,"logged_at":"2026-06-07T10:00:00Z"}`
	require.Equal(t, http.StatusCreated,
		doHydReq(t, f.r, http.MethodPost, "/hydration", untagged).Code)

	rec := doHydReq(t, f.r, http.MethodGet,
		"/summary/hydration/daily?date=2026-06-07&tz=UTC", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var d hydration.Daily
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &d))
	assert.Equal(t, 800.0, d.TotalMl,
		"daily total must include both tagged and untagged entries")
	assert.Equal(t, 2, d.EntryCount)
}
