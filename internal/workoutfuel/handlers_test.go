package workoutfuel_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/nutrition-api/internal/auth"
	"github.com/vinzenzs/nutrition-api/internal/idempotency"
	"github.com/vinzenzs/nutrition-api/internal/store/storetest"
	"github.com/vinzenzs/nutrition-api/internal/workoutfuel"
	"github.com/vinzenzs/nutrition-api/internal/workouts"
)

func init() {
	gin.SetMode(gin.TestMode)
}

const (
	mobileToken = "mobile-token-aaaaaaaaaaaaaa"
	agentToken  = "agent-token-bbbbbbbbbbbbbbbb"
)

type fixture struct {
	r            *gin.Engine
	workoutsRepo *workouts.Repo
}

// setup mounts handlers without auth/idem. SetWorkoutsRepo is wired so the
// workout_id link validation tests have something to hit.
func setup(t *testing.T) *fixture {
	t.Helper()
	pool := storetest.NewPool(t)
	repo := workoutfuel.NewRepo(pool)
	svc := workoutfuel.NewService(repo)
	wRepo := workouts.NewRepo(pool)
	svc.SetWorkoutsRepo(wRepo)
	r := gin.New()
	rg := r.Group("/")
	workoutfuel.NewHandlers(svc).Register(rg)
	return &fixture{r: r, workoutsRepo: wRepo}
}

func setupWithMiddleware(t *testing.T) *fixture {
	t.Helper()
	pool := storetest.NewPool(t)
	repo := workoutfuel.NewRepo(pool)
	svc := workoutfuel.NewService(repo)
	wRepo := workouts.NewRepo(pool)
	svc.SetWorkoutsRepo(wRepo)
	idemRepo := idempotency.NewRepo(pool)
	r := gin.New()
	r.Use(auth.Middleware(auth.Config{MobileToken: mobileToken, AgentToken: agentToken}))
	r.Use(idempotency.Middleware(idemRepo, time.Hour))
	rg := r.Group("/")
	workoutfuel.NewHandlers(svc).Register(rg)
	return &fixture{r: r, workoutsRepo: wRepo}
}

func doRequest(t *testing.T, r *gin.Engine, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Buffer
	if body != "" {
		reader = bytes.NewBufferString(body)
	} else {
		reader = bytes.NewBuffer(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

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

func mustCreate(t *testing.T, r *gin.Engine, body string) *workoutfuel.Entry {
	t.Helper()
	rec := doRequest(t, r, http.MethodPost, "/workout-fuel", body, nil)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var e workoutfuel.Entry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &e))
	return &e
}

// ============================================================================
// POST /workout-fuel — happy paths
// ============================================================================

func TestCreate_GelHappyPath(t *testing.T) {
	f := setup(t)
	rec := doRequest(t, f.r, http.MethodPost, "/workout-fuel",
		`{"name":"Maurten Gel 100","logged_at":"2026-06-07T08:45:00Z","carbs_g":25,"sodium_mg":0,"caffeine_mg":100}`, nil)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var e workoutfuel.Entry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &e))
	assert.Equal(t, "Maurten Gel 100", e.Name)
	require.NotNil(t, e.CarbsG)
	assert.Equal(t, 25.0, *e.CarbsG)
	require.NotNil(t, e.SodiumMg)
	assert.Equal(t, 0.0, *e.SodiumMg, "explicit zero should round-trip distinct from nil")
	require.NotNil(t, e.CaffeineMg)
	assert.Equal(t, 100.0, *e.CaffeineMg)
	assert.Nil(t, e.QuantityMl)
	assert.Nil(t, e.PotassiumMg)
}

func TestCreate_ElectrolyteDrinkHappyPath(t *testing.T) {
	f := setup(t)
	rec := doRequest(t, f.r, http.MethodPost, "/workout-fuel",
		`{"name":"Skratch","logged_at":"2026-06-07T08:30:00Z","quantity_ml":500,"carbs_g":20,"sodium_mg":380}`, nil)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var e workoutfuel.Entry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &e))
	require.NotNil(t, e.QuantityMl)
	assert.Equal(t, 500.0, *e.QuantityMl)
	require.NotNil(t, e.SodiumMg)
	assert.Equal(t, 380.0, *e.SodiumMg)
}

// ============================================================================
// POST /workout-fuel — validation
// ============================================================================

func TestCreate_MissingName(t *testing.T) {
	f := setup(t)
	rec := doRequest(t, f.r, http.MethodPost, "/workout-fuel",
		`{"logged_at":"2026-06-07T08:45:00Z","carbs_g":25}`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"name_required"}`, rec.Body.String())
}

func TestCreate_EmptyNameRejected(t *testing.T) {
	f := setup(t)
	rec := doRequest(t, f.r, http.MethodPost, "/workout-fuel",
		`{"name":"   ","logged_at":"2026-06-07T08:45:00Z","carbs_g":25}`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"name_required"}`, rec.Body.String())
}

func TestCreate_EmptyEntry(t *testing.T) {
	f := setup(t)
	rec := doRequest(t, f.r, http.MethodPost, "/workout-fuel",
		`{"name":"Some gel","logged_at":"2026-06-07T08:45:00Z"}`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"empty_entry"}`, rec.Body.String())
}

func TestCreate_QuantityZeroRejected(t *testing.T) {
	f := setup(t)
	rec := doRequest(t, f.r, http.MethodPost, "/workout-fuel",
		`{"name":"Bottle","logged_at":"2026-06-07T08:45:00Z","quantity_ml":0,"sodium_mg":100}`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"quantity_ml_invalid"}`, rec.Body.String())
}

func TestCreate_QuantityNegativeRejected(t *testing.T) {
	f := setup(t)
	rec := doRequest(t, f.r, http.MethodPost, "/workout-fuel",
		`{"name":"Bottle","logged_at":"2026-06-07T08:45:00Z","quantity_ml":-100,"sodium_mg":100}`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"quantity_ml_invalid"}`, rec.Body.String())
}

func TestCreate_NutrientFieldInvalid(t *testing.T) {
	f := setup(t)
	cases := []struct {
		field    string
		code     string
		body     string
	}{
		{"carbs_g", "carbs_g_invalid",
			`{"name":"Gel","logged_at":"2026-06-07T08:45:00Z","carbs_g":-1}`},
		{"sodium_mg", "sodium_mg_invalid",
			`{"name":"Gel","logged_at":"2026-06-07T08:45:00Z","sodium_mg":-5}`},
		{"potassium_mg", "potassium_mg_invalid",
			`{"name":"Gel","logged_at":"2026-06-07T08:45:00Z","potassium_mg":-5}`},
		{"caffeine_mg", "caffeine_mg_invalid",
			`{"name":"Gel","logged_at":"2026-06-07T08:45:00Z","caffeine_mg":-1}`},
	}
	for _, tc := range cases {
		t.Run(tc.field, func(t *testing.T) {
			rec := doRequest(t, f.r, http.MethodPost, "/workout-fuel", tc.body, nil)
			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.JSONEq(t, fmt.Sprintf(`{"error":%q}`, tc.code), rec.Body.String())
		})
	}
}

func TestCreate_NutrientZeroAccepted(t *testing.T) {
	f := setup(t)
	rec := doRequest(t, f.r, http.MethodPost, "/workout-fuel",
		`{"name":"Decaf shot","logged_at":"2026-06-07T08:45:00Z","caffeine_mg":0,"carbs_g":5}`, nil)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var e workoutfuel.Entry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &e))
	require.NotNil(t, e.CaffeineMg, "explicit zero must persist as non-nil")
	assert.Equal(t, 0.0, *e.CaffeineMg)
}

func TestCreate_LoggedAtFarFuture(t *testing.T) {
	f := setup(t)
	farFuture := time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339)
	body := fmt.Sprintf(`{"name":"Gel","logged_at":%q,"carbs_g":25}`, farFuture)
	rec := doRequest(t, f.r, http.MethodPost, "/workout-fuel", body, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"logged_at_too_far_future"}`, rec.Body.String())
}

func TestCreate_NoteTooLong(t *testing.T) {
	f := setup(t)
	longNote := strings.Repeat("a", 501)
	body := fmt.Sprintf(`{"name":"Gel","logged_at":"2026-06-07T08:45:00Z","carbs_g":25,"note":%q}`, longNote)
	rec := doRequest(t, f.r, http.MethodPost, "/workout-fuel", body, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"note_too_long"}`, rec.Body.String())
}

func TestCreate_WithWorkoutID(t *testing.T) {
	f := setup(t)
	wid := seedWorkout(t, f.workoutsRepo)
	body := fmt.Sprintf(`{"name":"Gel","logged_at":"2026-06-07T08:45:00Z","carbs_g":25,"workout_id":%q}`, wid)
	rec := doRequest(t, f.r, http.MethodPost, "/workout-fuel", body, nil)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var e workoutfuel.Entry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &e))
	require.NotNil(t, e.WorkoutID)
	assert.Equal(t, wid, *e.WorkoutID)
}

func TestCreate_UnknownWorkoutIDRejected(t *testing.T) {
	f := setup(t)
	rec := doRequest(t, f.r, http.MethodPost, "/workout-fuel",
		`{"name":"Gel","logged_at":"2026-06-07T08:45:00Z","carbs_g":25,"workout_id":"00000000-0000-0000-0000-000000000000"}`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"workout_not_found"}`, rec.Body.String())
}

func TestCreate_WorkoutIDInvalid(t *testing.T) {
	f := setup(t)
	rec := doRequest(t, f.r, http.MethodPost, "/workout-fuel",
		`{"name":"Gel","logged_at":"2026-06-07T08:45:00Z","carbs_g":25,"workout_id":"not-a-uuid"}`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"workout_id_invalid"}`, rec.Body.String())
}

// ============================================================================
// GET /workout-fuel?from=…&to=…
// ============================================================================

func TestList_WindowFilters(t *testing.T) {
	f := setup(t)
	mustCreate(t, f.r, `{"name":"A","logged_at":"2026-06-07T05:00:00Z","carbs_g":25}`)
	mustCreate(t, f.r, `{"name":"B","logged_at":"2026-06-07T12:00:00Z","carbs_g":40}`)
	mustCreate(t, f.r, `{"name":"C","logged_at":"2026-06-08T05:00:00Z","carbs_g":25}`) // outside

	rec := doRequest(t, f.r, http.MethodGet,
		"/workout-fuel?from=2026-06-07T00:00:00Z&to=2026-06-08T00:00:00Z", "", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body struct {
		Entries []workoutfuel.Entry `json:"entries"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Entries, 2)
	assert.Equal(t, "A", body.Entries[0].Name)
	assert.Equal(t, "B", body.Entries[1].Name)
}

func TestList_MissingWindow(t *testing.T) {
	f := setup(t)
	rec := doRequest(t, f.r, http.MethodGet, "/workout-fuel", "", nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"window_required"}`, rec.Body.String())
}

func TestList_InvertedWindow(t *testing.T) {
	f := setup(t)
	rec := doRequest(t, f.r, http.MethodGet,
		"/workout-fuel?from=2026-06-08T00:00:00Z&to=2026-06-07T00:00:00Z", "", nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"window_invalid"}`, rec.Body.String())
}

func TestList_RangeTooLarge(t *testing.T) {
	f := setup(t)
	rec := doRequest(t, f.r, http.MethodGet,
		"/workout-fuel?from=2026-01-01T00:00:00Z&to=2026-12-31T00:00:00Z", "", nil)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "range_too_large", body["error"])
	assert.EqualValues(t, 92, body["max_days"])
}

// ============================================================================
// PATCH /workout-fuel/{id}
// ============================================================================

func TestPatch_PartialUpdate(t *testing.T) {
	f := setup(t)
	created := mustCreate(t, f.r,
		`{"name":"Skratch","logged_at":"2026-06-07T08:30:00Z","quantity_ml":500,"carbs_g":20,"sodium_mg":380}`)

	rec := doRequest(t, f.r, http.MethodPatch, "/workout-fuel/"+created.ID.String(),
		`{"sodium_mg":420}`, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var e workoutfuel.Entry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &e))
	require.NotNil(t, e.SodiumMg)
	assert.Equal(t, 420.0, *e.SodiumMg)
	require.NotNil(t, e.QuantityMl)
	assert.Equal(t, 500.0, *e.QuantityMl, "untouched field must be preserved")
}

func TestPatch_WorkoutIDSetClearNoTouch(t *testing.T) {
	f := setup(t)
	wid := seedWorkout(t, f.workoutsRepo)
	created := mustCreate(t, f.r,
		`{"name":"Gel","logged_at":"2026-06-07T08:30:00Z","carbs_g":25}`)

	// Set.
	rec := doRequest(t, f.r, http.MethodPatch, "/workout-fuel/"+created.ID.String(),
		fmt.Sprintf(`{"workout_id":%q}`, wid), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var afterSet workoutfuel.Entry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &afterSet))
	require.NotNil(t, afterSet.WorkoutID)
	assert.Equal(t, wid, *afterSet.WorkoutID)

	// No-touch.
	rec = doRequest(t, f.r, http.MethodPatch, "/workout-fuel/"+created.ID.String(),
		`{"carbs_g":30}`, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var afterNoTouch workoutfuel.Entry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &afterNoTouch))
	require.NotNil(t, afterNoTouch.WorkoutID)
	assert.Equal(t, wid, *afterNoTouch.WorkoutID)

	// Clear via empty string.
	rec = doRequest(t, f.r, http.MethodPatch, "/workout-fuel/"+created.ID.String(),
		`{"workout_id":""}`, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var afterClear workoutfuel.Entry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &afterClear))
	assert.Nil(t, afterClear.WorkoutID)
}

func TestPatch_WorkoutIDUnknownRejected(t *testing.T) {
	f := setup(t)
	created := mustCreate(t, f.r, `{"name":"Gel","logged_at":"2026-06-07T08:30:00Z","carbs_g":25}`)
	rec := doRequest(t, f.r, http.MethodPatch, "/workout-fuel/"+created.ID.String(),
		`{"workout_id":"00000000-0000-0000-0000-000000000000"}`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"workout_not_found"}`, rec.Body.String())
}

func TestPatch_NullClearsField(t *testing.T) {
	f := setup(t)
	created := mustCreate(t, f.r,
		`{"name":"Skratch","logged_at":"2026-06-07T08:30:00Z","sodium_mg":380,"potassium_mg":100,"carbs_g":20}`)

	rec := doRequest(t, f.r, http.MethodPatch, "/workout-fuel/"+created.ID.String(),
		`{"sodium_mg":null}`, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var e workoutfuel.Entry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &e))
	assert.Nil(t, e.SodiumMg, "explicit null in PATCH must clear the column")
	require.NotNil(t, e.PotassiumMg, "untouched field must remain")
	assert.Equal(t, 100.0, *e.PotassiumMg)
}

func TestPatch_ToEmptyRejected(t *testing.T) {
	f := setup(t)
	created := mustCreate(t, f.r,
		`{"name":"Gel","logged_at":"2026-06-07T08:30:00Z","carbs_g":25}`)

	rec := doRequest(t, f.r, http.MethodPatch, "/workout-fuel/"+created.ID.String(),
		`{"carbs_g":null}`, nil)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"empty_entry"}`, rec.Body.String())

	// Confirm row is unchanged.
	listRec := doRequest(t, f.r, http.MethodGet,
		"/workout-fuel?from=2026-06-07T00:00:00Z&to=2026-06-08T00:00:00Z", "", nil)
	require.Equal(t, http.StatusOK, listRec.Code)
	var body struct {
		Entries []workoutfuel.Entry `json:"entries"`
	}
	require.NoError(t, json.Unmarshal(listRec.Body.Bytes(), &body))
	require.Len(t, body.Entries, 1)
	require.NotNil(t, body.Entries[0].CarbsG)
	assert.Equal(t, 25.0, *body.Entries[0].CarbsG, "rejected patch must not partially apply")
}

func TestPatch_InvalidValueRejected(t *testing.T) {
	f := setup(t)
	created := mustCreate(t, f.r,
		`{"name":"Gel","logged_at":"2026-06-07T08:30:00Z","carbs_g":25}`)

	rec := doRequest(t, f.r, http.MethodPatch, "/workout-fuel/"+created.ID.String(),
		`{"carbs_g":-1}`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"carbs_g_invalid"}`, rec.Body.String())
}

func TestPatch_UnknownIDReturns404(t *testing.T) {
	f := setup(t)
	rec := doRequest(t, f.r, http.MethodPatch,
		"/workout-fuel/00000000-0000-0000-0000-000000000000",
		`{"carbs_g":25}`, nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.JSONEq(t, `{"error":"workout_fuel_not_found"}`, rec.Body.String())
}

// ============================================================================
// DELETE /workout-fuel/{id}
// ============================================================================

func TestDelete_HappyPath(t *testing.T) {
	f := setup(t)
	created := mustCreate(t, f.r, `{"name":"Gel","logged_at":"2026-06-07T08:30:00Z","carbs_g":25}`)

	rec := doRequest(t, f.r, http.MethodDelete, "/workout-fuel/"+created.ID.String(), "", nil)
	assert.Equal(t, http.StatusNoContent, rec.Code)

	listRec := doRequest(t, f.r, http.MethodGet,
		"/workout-fuel?from=2026-06-07T00:00:00Z&to=2026-06-08T00:00:00Z", "", nil)
	require.Equal(t, http.StatusOK, listRec.Code)
	var body struct {
		Entries []workoutfuel.Entry `json:"entries"`
	}
	require.NoError(t, json.Unmarshal(listRec.Body.Bytes(), &body))
	assert.Len(t, body.Entries, 0)
}

func TestDelete_UnknownReturns404(t *testing.T) {
	f := setup(t)
	rec := doRequest(t, f.r, http.MethodDelete,
		"/workout-fuel/00000000-0000-0000-0000-000000000000", "", nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.JSONEq(t, `{"error":"workout_fuel_not_found"}`, rec.Body.String())
}

// ============================================================================
// Idempotency
// ============================================================================

func TestIdempotency_SameKeySameBodyReplays(t *testing.T) {
	f := setupWithMiddleware(t)
	body := `{"name":"Gel","logged_at":"2026-06-07T08:30:00Z","carbs_g":25}`
	headers := map[string]string{
		"Authorization":   "Bearer " + mobileToken,
		"Idempotency-Key": "wfuel-1",
	}
	first := doRequest(t, f.r, http.MethodPost, "/workout-fuel", body, headers)
	require.Equal(t, http.StatusCreated, first.Code, first.Body.String())
	second := doRequest(t, f.r, http.MethodPost, "/workout-fuel", body, headers)
	require.Equal(t, http.StatusCreated, second.Code)
	assert.Equal(t, first.Body.String(), second.Body.String())
}

func TestIdempotency_SameKeyDifferentBodyReturns409(t *testing.T) {
	f := setupWithMiddleware(t)
	headers := map[string]string{
		"Authorization":   "Bearer " + mobileToken,
		"Idempotency-Key": "wfuel-2",
	}
	first := doRequest(t, f.r, http.MethodPost, "/workout-fuel",
		`{"name":"Gel","logged_at":"2026-06-07T08:30:00Z","carbs_g":25}`, headers)
	require.Equal(t, http.StatusCreated, first.Code, first.Body.String())
	conflict := doRequest(t, f.r, http.MethodPost, "/workout-fuel",
		`{"name":"Gel","logged_at":"2026-06-07T08:30:00Z","carbs_g":50}`, headers)
	require.Equal(t, http.StatusConflict, conflict.Code)
	assert.JSONEq(t, `{"error":"idempotency_key_conflict"}`, conflict.Body.String())
}
