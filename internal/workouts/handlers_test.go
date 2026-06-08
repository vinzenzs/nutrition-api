package workouts_test

import (
	"bytes"
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

	"github.com/vinzenzs/nutrition-api/internal/store/storetest"
	"github.com/vinzenzs/nutrition-api/internal/workouts"
)

func init() {
	gin.SetMode(gin.TestMode)
}

type fixture struct {
	r    *gin.Engine
	repo *workouts.Repo
}

func setup(t *testing.T) *fixture {
	t.Helper()
	pool := storetest.NewPool(t)
	repo := workouts.NewRepo(pool)
	svc := workouts.NewService(repo)
	r := gin.New()
	workouts.NewHandlers(svc).Register(r.Group("/"))
	return &fixture{r: r, repo: repo}
}

func doReq(t *testing.T, r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var rdr *bytes.Buffer
	if body != "" {
		rdr = bytes.NewBufferString(body)
	} else {
		rdr = bytes.NewBuffer(nil)
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// ============================================================================
// POST /workouts
// ============================================================================

func TestPost_HappyPathFullBodyReturns201(t *testing.T) {
	f := setup(t)
	body := `{
        "external_id":"garmin:1",
        "source":"garmin",
        "sport":"bike",
        "name":"Morning Z2",
        "started_at":"2026-06-07T08:00:00Z",
        "ended_at":"2026-06-07T09:30:00Z",
        "kcal_burned":850,
        "avg_hr":135,
        "tss":78,
        "notes":"felt easy"
    }`
	rec := doReq(t, f.r, http.MethodPost, "/workouts", body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var w workouts.Workout
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &w))
	require.NotNil(t, w.ExternalID)
	assert.Equal(t, "garmin:1", *w.ExternalID)
	assert.Equal(t, workouts.SourceGarmin, w.Source)
	assert.Equal(t, workouts.SportBike, w.Sport)
	require.NotNil(t, w.KcalBurned)
	assert.InDelta(t, 850, *w.KcalBurned, 0.001)
}

func TestPost_NoExternalIDAlwaysInsertsNewRow(t *testing.T) {
	f := setup(t)
	body := `{"source":"manual","sport":"strength","started_at":"2026-06-07T18:00:00Z","ended_at":"2026-06-07T19:00:00Z","name":"Push day"}`

	rec1 := doReq(t, f.r, http.MethodPost, "/workouts", body)
	require.Equal(t, http.StatusCreated, rec1.Code)
	rec2 := doReq(t, f.r, http.MethodPost, "/workouts", body)
	require.Equal(t, http.StatusCreated, rec2.Code)

	var a, b workouts.Workout
	require.NoError(t, json.Unmarshal(rec1.Body.Bytes(), &a))
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &b))
	assert.NotEqual(t, a.ID, b.ID, "two NULL-external_id POSTs create two distinct rows")
}

func TestPost_ExistingExternalIDReturns200Update(t *testing.T) {
	f := setup(t)
	first := `{"external_id":"garmin:2","source":"garmin","sport":"bike","started_at":"2026-06-07T08:00:00Z","ended_at":"2026-06-07T09:30:00Z","kcal_burned":800}`
	second := `{"external_id":"garmin:2","source":"garmin","sport":"bike","started_at":"2026-06-07T08:00:00Z","ended_at":"2026-06-07T09:30:00Z","kcal_burned":900}`

	rec1 := doReq(t, f.r, http.MethodPost, "/workouts", first)
	require.Equal(t, http.StatusCreated, rec1.Code)

	rec2 := doReq(t, f.r, http.MethodPost, "/workouts", second)
	require.Equal(t, http.StatusOK, rec2.Code, rec2.Body.String())

	var w workouts.Workout
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &w))
	require.NotNil(t, w.KcalBurned)
	assert.InDelta(t, 900, *w.KcalBurned, 0.001)
}

func TestPost_InvalidSource(t *testing.T) {
	f := setup(t)
	body := `{"source":"strava","sport":"run","started_at":"2026-06-07T08:00:00Z","ended_at":"2026-06-07T09:00:00Z"}`
	rec := doReq(t, f.r, http.MethodPost, "/workouts", body)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"source_invalid"}`, rec.Body.String())
}

func TestPost_InvalidSport(t *testing.T) {
	f := setup(t)
	body := `{"source":"manual","sport":"yoga","started_at":"2026-06-07T08:00:00Z","ended_at":"2026-06-07T09:00:00Z"}`
	rec := doReq(t, f.r, http.MethodPost, "/workouts", body)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"sport_invalid"}`, rec.Body.String())
}

func TestPost_MissingWindow(t *testing.T) {
	f := setup(t)
	body := `{"source":"manual","sport":"run"}`
	rec := doReq(t, f.r, http.MethodPost, "/workouts", body)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"window_invalid"}`, rec.Body.String())
}

func TestPost_EndedBeforeStartedRejected(t *testing.T) {
	f := setup(t)
	body := `{"source":"manual","sport":"run","started_at":"2026-06-07T09:00:00Z","ended_at":"2026-06-07T08:00:00Z"}`
	rec := doReq(t, f.r, http.MethodPost, "/workouts", body)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"window_invalid"}`, rec.Body.String())
}

func TestPost_StartedFarFutureRejected(t *testing.T) {
	f := setup(t)
	body := `{"source":"manual","sport":"run","started_at":"2099-06-07T08:00:00Z","ended_at":"2099-06-07T09:00:00Z"}`
	rec := doReq(t, f.r, http.MethodPost, "/workouts", body)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"started_at_too_far_future"}`, rec.Body.String())
}

func TestPost_NegativeKcalRejected(t *testing.T) {
	f := setup(t)
	body := `{"source":"manual","sport":"run","started_at":"2026-06-07T08:00:00Z","ended_at":"2026-06-07T09:00:00Z","kcal_burned":-1}`
	rec := doReq(t, f.r, http.MethodPost, "/workouts", body)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"kcal_burned_invalid"}`, rec.Body.String())
}

func TestPost_NegativeHRRejected(t *testing.T) {
	f := setup(t)
	body := `{"source":"manual","sport":"run","started_at":"2026-06-07T08:00:00Z","ended_at":"2026-06-07T09:00:00Z","avg_hr":-10}`
	rec := doReq(t, f.r, http.MethodPost, "/workouts", body)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"avg_hr_invalid"}`, rec.Body.String())
}

func TestPost_NegativeTSSRejected(t *testing.T) {
	f := setup(t)
	body := `{"source":"manual","sport":"run","started_at":"2026-06-07T08:00:00Z","ended_at":"2026-06-07T09:00:00Z","tss":-1}`
	rec := doReq(t, f.r, http.MethodPost, "/workouts", body)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"tss_invalid"}`, rec.Body.String())
}

// ============================================================================
// GET /workouts (list)
// ============================================================================

func TestList_FiltersByWindow(t *testing.T) {
	f := setup(t)
	postSample := func(extID, started, ended string) {
		t.Helper()
		body := fmt.Sprintf(`{"external_id":%q,"source":"garmin","sport":"run","started_at":%q,"ended_at":%q}`, extID, started, ended)
		rec := doReq(t, f.r, http.MethodPost, "/workouts", body)
		require.Equal(t, http.StatusCreated, rec.Code)
	}
	postSample("g:in1", "2026-06-07T08:00:00Z", "2026-06-07T09:00:00Z")
	postSample("g:in2", "2026-06-07T18:00:00Z", "2026-06-07T19:00:00Z")
	postSample("g:out", "2026-05-30T08:00:00Z", "2026-05-30T09:00:00Z")

	rec := doReq(t, f.r, http.MethodGet, "/workouts?from=2026-06-01T00:00:00Z&to=2026-06-08T00:00:00Z", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var out struct {
		Workouts []workouts.Workout `json:"workouts"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Len(t, out.Workouts, 2)
}

func TestList_MissingWindow(t *testing.T) {
	f := setup(t)
	rec := doReq(t, f.r, http.MethodGet, "/workouts", "")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"window_required"}`, rec.Body.String())
}

func TestList_InvertedWindow(t *testing.T) {
	f := setup(t)
	rec := doReq(t, f.r, http.MethodGet, "/workouts?from=2026-06-08T00:00:00Z&to=2026-06-01T00:00:00Z", "")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"window_invalid"}`, rec.Body.String())
}

func TestList_RangeTooLarge(t *testing.T) {
	f := setup(t)
	rec := doReq(t, f.r, http.MethodGet, "/workouts?from=2026-01-01T00:00:00Z&to=2026-12-31T00:00:00Z", "")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "range_too_large", body["error"])
	assert.EqualValues(t, 92, body["max_days"])
}

// ============================================================================
// GET /workouts/{id}
// ============================================================================

func TestGet_HappyPath(t *testing.T) {
	f := setup(t)
	w := sample(ptrStr("garmin:get"), 500)
	_, err := f.repo.Upsert(context.Background(), w)
	require.NoError(t, err)

	rec := doReq(t, f.r, http.MethodGet, "/workouts/"+w.ID.String(), "")
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestGet_NotFound(t *testing.T) {
	f := setup(t)
	rec := doReq(t, f.r, http.MethodGet, "/workouts/"+uuid.New().String(), "")
	require.Equal(t, http.StatusNotFound, rec.Code)
	assert.JSONEq(t, `{"error":"workout_not_found"}`, rec.Body.String())
}

// ============================================================================
// PATCH /workouts/{id}
// ============================================================================

func TestPatch_HappyPath(t *testing.T) {
	f := setup(t)
	w := sample(ptrStr("garmin:patch"), 800)
	_, err := f.repo.Upsert(context.Background(), w)
	require.NoError(t, err)

	rec := doReq(t, f.r, http.MethodPatch, "/workouts/"+w.ID.String(), `{"tss":85,"notes":"FTP updated"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var got workouts.Workout
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.NotNil(t, got.TSS)
	assert.InDelta(t, 85, *got.TSS, 0.001)
}

func TestPatch_ImmutableFieldsRejected(t *testing.T) {
	f := setup(t)
	w := sample(ptrStr("garmin:imm"), 800)
	_, err := f.repo.Upsert(context.Background(), w)
	require.NoError(t, err)

	for _, field := range []string{"source", "external_id", "sport", "started_at", "ended_at"} {
		body := fmt.Sprintf(`{%q:"whatever"}`, field)
		rec := doReq(t, f.r, http.MethodPatch, "/workouts/"+w.ID.String(), body)
		require.Equal(t, http.StatusBadRequest, rec.Code, "field=%s", field)
		var b map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &b))
		assert.Equal(t, "field_immutable", b["error"])
		assert.Equal(t, field, b["field"])
	}
}

func TestPatch_NegativeTSSRejected(t *testing.T) {
	f := setup(t)
	w := sample(ptrStr("garmin:tss"), 800)
	_, err := f.repo.Upsert(context.Background(), w)
	require.NoError(t, err)

	rec := doReq(t, f.r, http.MethodPatch, "/workouts/"+w.ID.String(), `{"tss":-1}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"tss_invalid"}`, rec.Body.String())
}

func TestPatch_NotFound(t *testing.T) {
	f := setup(t)
	rec := doReq(t, f.r, http.MethodPatch, "/workouts/"+uuid.New().String(), `{"tss":50}`)
	require.Equal(t, http.StatusNotFound, rec.Code)
	assert.JSONEq(t, `{"error":"workout_not_found"}`, rec.Body.String())
}

// ============================================================================
// DELETE /workouts/{id}
// ============================================================================

func TestDelete_HappyPath(t *testing.T) {
	f := setup(t)
	w := sample(nil, 100)
	_, err := f.repo.Upsert(context.Background(), w)
	require.NoError(t, err)

	rec := doReq(t, f.r, http.MethodDelete, "/workouts/"+w.ID.String(), "")
	require.Equal(t, http.StatusNoContent, rec.Code)

	rec2 := doReq(t, f.r, http.MethodGet, "/workouts/"+w.ID.String(), "")
	require.Equal(t, http.StatusNotFound, rec2.Code)
}

func TestDelete_NotFound(t *testing.T) {
	f := setup(t)
	rec := doReq(t, f.r, http.MethodDelete, "/workouts/"+uuid.New().String(), "")
	require.Equal(t, http.StatusNotFound, rec.Code)
	assert.JSONEq(t, `{"error":"workout_not_found"}`, rec.Body.String())
}

// ============================================================================
// POST /workouts/bulk
// ============================================================================

func TestBulk_MixedBatchPersistsValidAndReportsInvalid(t *testing.T) {
	f := setup(t)

	// Seed an existing row so item-2 will be an UPDATE.
	existing := sample(ptrStr("garmin:bulk-existing"), 700)
	_, err := f.repo.Upsert(context.Background(), existing)
	require.NoError(t, err)
	existingID := existing.ID

	body := `{
        "workouts": [
            {"external_id":"garmin:bulk-new","source":"garmin","sport":"run","started_at":"2026-06-07T08:00:00Z","ended_at":"2026-06-07T09:00:00Z","kcal_burned":600},
            {"external_id":"garmin:bulk-existing","source":"garmin","sport":"bike","started_at":"2026-06-07T08:00:00Z","ended_at":"2026-06-07T09:30:00Z","kcal_burned":900},
            {"source":"manual","sport":"yoga","started_at":"2026-06-07T07:00:00Z","ended_at":"2026-06-07T07:45:00Z"}
        ]
    }`
	rec := doReq(t, f.r, http.MethodPost, "/workouts/bulk", body)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var out struct {
		Results []map[string]any `json:"results"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Len(t, out.Results, 3)

	// Item 0 — new insert.
	assert.EqualValues(t, 0, out.Results[0]["index"])
	assert.Equal(t, true, out.Results[0]["created"])

	// Item 1 — update of existing.
	assert.EqualValues(t, 1, out.Results[1]["index"])
	assert.Equal(t, false, out.Results[1]["created"])
	assert.Equal(t, existingID.String(), out.Results[1]["id"])

	// Item 2 — sport_invalid.
	assert.EqualValues(t, 2, out.Results[2]["index"])
	assert.Equal(t, "sport_invalid", out.Results[2]["error"])

	// Verify existing row was updated.
	got, err := f.repo.GetByID(context.Background(), existingID)
	require.NoError(t, err)
	require.NotNil(t, got.KcalBurned)
	assert.InDelta(t, 900, *got.KcalBurned, 0.001)
}

func TestBulk_EmptyArray(t *testing.T) {
	f := setup(t)
	rec := doReq(t, f.r, http.MethodPost, "/workouts/bulk", `{"workouts":[]}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"bulk_empty"}`, rec.Body.String())
}

func TestBulk_TooLarge(t *testing.T) {
	f := setup(t)
	// Build 101 valid items.
	items := make([]string, 101)
	for i := range items {
		items[i] = fmt.Sprintf(`{"external_id":"g:bulk%d","source":"garmin","sport":"run","started_at":"2026-06-07T08:00:00Z","ended_at":"2026-06-07T09:00:00Z"}`, i)
	}
	body := `{"workouts":[` + joinCSV(items) + `]}`
	rec := doReq(t, f.r, http.MethodPost, "/workouts/bulk", body)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	var b map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &b))
	assert.Equal(t, "bulk_too_large", b["error"])
	assert.EqualValues(t, 100, b["max"])
}

func TestBulk_MissingWorkoutsField(t *testing.T) {
	f := setup(t)
	rec := doReq(t, f.r, http.MethodPost, "/workouts/bulk", `{"items":[]}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"bulk_invalid"}`, rec.Body.String())
}

func TestBulk_DuplicateExternalIDInBatchLastWriteWins(t *testing.T) {
	f := setup(t)
	body := `{
        "workouts": [
            {"external_id":"garmin:dup","source":"garmin","sport":"run","started_at":"2026-06-07T08:00:00Z","ended_at":"2026-06-07T09:00:00Z","kcal_burned":500},
            {"external_id":"garmin:dup","source":"garmin","sport":"run","started_at":"2026-06-07T08:00:00Z","ended_at":"2026-06-07T09:00:00Z","kcal_burned":600}
        ]
    }`
	rec := doReq(t, f.r, http.MethodPost, "/workouts/bulk", body)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// Verify only one row exists with that external_id, and its kcal is 600.
	from, _ := time.Parse(time.RFC3339, "2026-06-07T07:00:00Z")
	to, _ := time.Parse(time.RFC3339, "2026-06-07T10:00:00Z")
	rows, err := f.repo.List(context.Background(), from, to)
	require.NoError(t, err)
	var dup []*workouts.Workout
	for _, r := range rows {
		if r.ExternalID != nil && *r.ExternalID == "garmin:dup" {
			dup = append(dup, r)
		}
	}
	require.Len(t, dup, 1)
	require.NotNil(t, dup[0].KcalBurned)
	assert.InDelta(t, 600, *dup[0].KcalBurned, 0.001)
}

// joinCSV joins strings with commas. Pulled out to keep test bodies tidy.
func joinCSV(parts []string) string {
	var buf bytes.Buffer
	for i, p := range parts {
		if i > 0 {
			buf.WriteString(",")
		}
		buf.WriteString(p)
	}
	return buf.String()
}

