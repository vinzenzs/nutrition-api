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
	"github.com/jackc/pgx/v5/pgxpool"
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
	pool *pgxpool.Pool
}

func setup(t *testing.T) *fixture {
	t.Helper()
	pool := storetest.NewPool(t)
	repo := workouts.NewRepo(pool)
	svc := workouts.NewService(repo, pool, "UTC")
	r := gin.New()
	workouts.NewHandlers(svc).Register(r.Group("/"))
	return &fixture{r: r, repo: repo, pool: pool}
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
	rows, err := f.repo.List(context.Background(), from, to, nil, nil)
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

// ============================================================================
// RPE + GI distress score (rehearsal-outcome fields)
// ============================================================================

func TestPost_WithRPEAndGIDistressScore_Returns201AndEchoesFields(t *testing.T) {
	f := setup(t)
	body := `{
        "source":"manual","sport":"bike",
        "started_at":"2026-06-07T08:00:00Z","ended_at":"2026-06-07T09:30:00Z",
        "rpe":7,"gi_distress_score":2
    }`
	rec := doReq(t, f.r, http.MethodPost, "/workouts", body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var w workouts.Workout
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &w))
	require.NotNil(t, w.RPE)
	assert.Equal(t, 7, *w.RPE)
	require.NotNil(t, w.GIDistressScore)
	assert.Equal(t, 2, *w.GIDistressScore)
}

func TestPost_WithoutRPEOrGI_OmitsFromResponse(t *testing.T) {
	f := setup(t)
	body := `{"source":"manual","sport":"bike","started_at":"2026-06-07T08:00:00Z","ended_at":"2026-06-07T09:30:00Z"}`
	rec := doReq(t, f.r, http.MethodPost, "/workouts", body)
	require.Equal(t, http.StatusCreated, rec.Code)
	assert.NotContains(t, rec.Body.String(), `"rpe"`)
	assert.NotContains(t, rec.Body.String(), `"gi_distress_score"`)
}

func TestPost_RPEOutOfRange_Returns400WithRangeHint(t *testing.T) {
	f := setup(t)
	for _, bad := range []int{0, 11, -1} {
		body := fmt.Sprintf(`{"source":"manual","sport":"bike","started_at":"2026-06-07T08:00:00Z","ended_at":"2026-06-07T09:30:00Z","rpe":%d}`, bad)
		rec := doReq(t, f.r, http.MethodPost, "/workouts", body)
		require.Equal(t, http.StatusBadRequest, rec.Code, "rpe=%d", bad)
		assert.JSONEq(t, `{"error":"rpe_invalid","range":{"min":1,"max":10}}`, rec.Body.String())
	}
}

func TestPost_GIOutOfRange_Returns400WithRangeHint(t *testing.T) {
	f := setup(t)
	for _, bad := range []int{0, 6, -2, 100} {
		body := fmt.Sprintf(`{"source":"manual","sport":"bike","started_at":"2026-06-07T08:00:00Z","ended_at":"2026-06-07T09:30:00Z","gi_distress_score":%d}`, bad)
		rec := doReq(t, f.r, http.MethodPost, "/workouts", body)
		require.Equal(t, http.StatusBadRequest, rec.Code, "gi=%d", bad)
		assert.JSONEq(t, `{"error":"gi_distress_score_invalid","range":{"min":1,"max":5}}`, rec.Body.String())
	}
}

func TestPost_NonIntegerRPE_Returns400_RpeInvalid(t *testing.T) {
	f := setup(t)
	// String, float — both rejected with the precise per-field code.
	body := `{"source":"manual","sport":"bike","started_at":"2026-06-07T08:00:00Z","ended_at":"2026-06-07T09:30:00Z","rpe":"seven"}`
	rec := doReq(t, f.r, http.MethodPost, "/workouts", body)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"rpe_invalid","range":{"min":1,"max":10}}`, rec.Body.String())

	body = `{"source":"manual","sport":"bike","started_at":"2026-06-07T08:00:00Z","ended_at":"2026-06-07T09:30:00Z","rpe":7.5}`
	rec = doReq(t, f.r, http.MethodPost, "/workouts", body)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"rpe_invalid","range":{"min":1,"max":10}}`, rec.Body.String())
}

func TestPatch_SetRPEAndGI_OnExistingRow(t *testing.T) {
	f := setup(t)
	// Create a workout without rehearsal fields, then PATCH them in.
	postBody := `{"source":"manual","sport":"bike","started_at":"2026-06-07T08:00:00Z","ended_at":"2026-06-07T09:30:00Z"}`
	postRec := doReq(t, f.r, http.MethodPost, "/workouts", postBody)
	require.Equal(t, http.StatusCreated, postRec.Code)
	var w workouts.Workout
	require.NoError(t, json.Unmarshal(postRec.Body.Bytes(), &w))

	patchBody := `{"rpe":7,"gi_distress_score":2}`
	patchRec := doReq(t, f.r, http.MethodPatch, "/workouts/"+w.ID.String(), patchBody)
	require.Equal(t, http.StatusOK, patchRec.Code, patchRec.Body.String())
	var got workouts.Workout
	require.NoError(t, json.Unmarshal(patchRec.Body.Bytes(), &got))
	require.NotNil(t, got.RPE)
	assert.Equal(t, 7, *got.RPE)
	require.NotNil(t, got.GIDistressScore)
	assert.Equal(t, 2, *got.GIDistressScore)
}

func TestPatch_ClearRPEViaJSONNull(t *testing.T) {
	f := setup(t)
	// Create with both, then PATCH rpe to null.
	postBody := `{"source":"manual","sport":"bike","started_at":"2026-06-07T08:00:00Z","ended_at":"2026-06-07T09:30:00Z","rpe":7,"gi_distress_score":2}`
	postRec := doReq(t, f.r, http.MethodPost, "/workouts", postBody)
	require.Equal(t, http.StatusCreated, postRec.Code)
	var w workouts.Workout
	require.NoError(t, json.Unmarshal(postRec.Body.Bytes(), &w))

	patchRec := doReq(t, f.r, http.MethodPatch, "/workouts/"+w.ID.String(), `{"rpe":null}`)
	require.Equal(t, http.StatusOK, patchRec.Code, patchRec.Body.String())
	// Body omits rpe, keeps gi_distress_score=2.
	body := patchRec.Body.String()
	assert.NotContains(t, body, `"rpe"`)
	assert.Contains(t, body, `"gi_distress_score":2`)
}

func TestPatch_RangeViolationDoesNotPersistOtherField(t *testing.T) {
	f := setup(t)
	postRec := doReq(t, f.r, http.MethodPost, "/workouts",
		`{"source":"manual","sport":"bike","started_at":"2026-06-07T08:00:00Z","ended_at":"2026-06-07T09:30:00Z"}`)
	require.Equal(t, http.StatusCreated, postRec.Code)
	var w workouts.Workout
	require.NoError(t, json.Unmarshal(postRec.Body.Bytes(), &w))

	// rpe=11 invalid; gi_distress_score=3 valid — neither should be written.
	patchRec := doReq(t, f.r, http.MethodPatch, "/workouts/"+w.ID.String(), `{"rpe":11,"gi_distress_score":3}`)
	require.Equal(t, http.StatusBadRequest, patchRec.Code)
	assert.JSONEq(t, `{"error":"rpe_invalid","range":{"min":1,"max":10}}`, patchRec.Body.String())

	// Verify nothing was written.
	getRec := doReq(t, f.r, http.MethodGet, "/workouts/"+w.ID.String(), "")
	require.Equal(t, http.StatusOK, getRec.Code)
	body := getRec.Body.String()
	assert.NotContains(t, body, `"rpe"`)
	assert.NotContains(t, body, `"gi_distress_score"`)
}

func TestPatch_LeaveUnchangedWhenAbsent(t *testing.T) {
	f := setup(t)
	postRec := doReq(t, f.r, http.MethodPost, "/workouts",
		`{"source":"manual","sport":"bike","started_at":"2026-06-07T08:00:00Z","ended_at":"2026-06-07T09:30:00Z","rpe":7,"gi_distress_score":2}`)
	require.Equal(t, http.StatusCreated, postRec.Code)
	var w workouts.Workout
	require.NoError(t, json.Unmarshal(postRec.Body.Bytes(), &w))

	// PATCH notes only — RPE/GI absent, must remain.
	patchRec := doReq(t, f.r, http.MethodPatch, "/workouts/"+w.ID.String(), `{"notes":"felt strong"}`)
	require.Equal(t, http.StatusOK, patchRec.Code, patchRec.Body.String())
	var got workouts.Workout
	require.NoError(t, json.Unmarshal(patchRec.Body.Bytes(), &got))
	require.NotNil(t, got.RPE)
	assert.Equal(t, 7, *got.RPE)
	require.NotNil(t, got.GIDistressScore)
	assert.Equal(t, 2, *got.GIDistressScore)
}

// ============================================================================
// Ingestion metrics (distance / power / temperature / sweat / session_group)
// ============================================================================

func TestPost_WithIngestionMetrics_Returns201AndEchoesFields(t *testing.T) {
	f := setup(t)
	body := `{
        "external_id":"garmin:555","source":"garmin","sport":"bike",
        "started_at":"2026-06-07T08:00:00Z","ended_at":"2026-06-07T11:00:00Z",
        "distance_m":80500,"avg_power_w":182,"temperature_c":27.5,
        "sweat_loss_ml":2400,"session_group":"garmin:554"
    }`
	rec := doReq(t, f.r, http.MethodPost, "/workouts", body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var w workouts.Workout
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &w))
	require.NotNil(t, w.DistanceM)
	assert.InDelta(t, 80500, *w.DistanceM, 0.05)
	require.NotNil(t, w.AvgPowerW)
	assert.Equal(t, 182, *w.AvgPowerW)
	require.NotNil(t, w.TemperatureC)
	assert.InDelta(t, 27.5, *w.TemperatureC, 0.05)
	require.NotNil(t, w.SweatLossML)
	assert.InDelta(t, 2400, *w.SweatLossML, 0.05)
	require.NotNil(t, w.SessionGroup)
	assert.Equal(t, "garmin:554", *w.SessionGroup)
}

func TestPost_WithoutIngestionMetrics_OmitsFromResponse(t *testing.T) {
	f := setup(t)
	body := `{"source":"manual","sport":"strength","started_at":"2026-06-07T18:00:00Z","ended_at":"2026-06-07T19:00:00Z"}`
	rec := doReq(t, f.r, http.MethodPost, "/workouts", body)
	require.Equal(t, http.StatusCreated, rec.Code)
	s := rec.Body.String()
	assert.NotContains(t, s, `"distance_m"`)
	assert.NotContains(t, s, `"avg_power_w"`)
	assert.NotContains(t, s, `"temperature_c"`)
	assert.NotContains(t, s, `"sweat_loss_ml"`)
	assert.NotContains(t, s, `"session_group"`)
}

func TestPost_IngestionInvalidValues_Return400(t *testing.T) {
	f := setup(t)
	base := `"source":"manual","sport":"bike","started_at":"2026-06-07T08:00:00Z","ended_at":"2026-06-07T09:30:00Z"`
	cases := []struct {
		field, frag, wantErr string
	}{
		{"distance_m", `"distance_m":-100`, "distance_m_invalid"},
		{"distance_m_zero", `"distance_m":0`, "distance_m_invalid"},
		{"avg_power_w", `"avg_power_w":0`, "avg_power_w_invalid"},
		{"avg_power_w_noninteger", `"avg_power_w":"strong"`, "avg_power_w_invalid"},
		{"sweat_loss_ml", `"sweat_loss_ml":-1`, "sweat_loss_ml_invalid"},
		{"session_group_empty", `"session_group":""`, "session_group_invalid"},
	}
	for _, tc := range cases {
		body := fmt.Sprintf("{%s,%s}", base, tc.frag)
		rec := doReq(t, f.r, http.MethodPost, "/workouts", body)
		require.Equal(t, http.StatusBadRequest, rec.Code, "%s: %s", tc.field, rec.Body.String())
		var got map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
		assert.Equal(t, tc.wantErr, got["error"], tc.field)
	}
}

func TestPost_TemperatureOutOfRange_Returns400WithRangeHint(t *testing.T) {
	f := setup(t)
	for _, bad := range []string{"-41", "61", "98.6"} {
		body := fmt.Sprintf(`{"source":"manual","sport":"bike","started_at":"2026-06-07T08:00:00Z","ended_at":"2026-06-07T09:30:00Z","temperature_c":%s}`, bad)
		rec := doReq(t, f.r, http.MethodPost, "/workouts", body)
		require.Equal(t, http.StatusBadRequest, rec.Code, "temp=%s", bad)
		assert.JSONEq(t, `{"error":"temperature_c_invalid","range":{"min":-40,"max":60}}`, rec.Body.String())
	}
}

func TestPost_TemperatureNegativeInRange_Accepted(t *testing.T) {
	f := setup(t)
	body := `{"source":"manual","sport":"run","started_at":"2026-01-07T08:00:00Z","ended_at":"2026-01-07T09:00:00Z","temperature_c":-5.5}`
	rec := doReq(t, f.r, http.MethodPost, "/workouts", body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var w workouts.Workout
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &w))
	require.NotNil(t, w.TemperatureC)
	assert.InDelta(t, -5.5, *w.TemperatureC, 0.05)
}

func TestPost_BrickLegsShareSessionGroup(t *testing.T) {
	f := setup(t)
	post := func(extID, sport, started, ended string) {
		t.Helper()
		body := fmt.Sprintf(`{"external_id":%q,"source":"garmin","sport":%q,"started_at":%q,"ended_at":%q,"session_group":"garmin:9876543"}`, extID, sport, started, ended)
		rec := doReq(t, f.r, http.MethodPost, "/workouts", body)
		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	}
	post("g:bike", "bike", "2026-06-07T08:00:00Z", "2026-06-07T09:00:00Z")
	post("g:run", "run", "2026-06-07T09:05:00Z", "2026-06-07T09:35:00Z")

	rec := doReq(t, f.r, http.MethodGet, "/workouts?from=2026-06-07T00:00:00Z&to=2026-06-08T00:00:00Z&session_group=garmin:9876543", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out struct {
		Workouts []workouts.Workout `json:"workouts"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Len(t, out.Workouts, 2)
	// Each leg keeps its own real sport — no merged pseudo-workout.
	assert.Equal(t, workouts.SportBike, out.Workouts[0].Sport)
	assert.Equal(t, workouts.SportRun, out.Workouts[1].Sport)
}

func TestList_SessionGroupFilterExcludesNonMatching(t *testing.T) {
	f := setup(t)
	post := func(extID, sport string, group string) {
		t.Helper()
		grpFrag := ""
		if group != "" {
			grpFrag = fmt.Sprintf(`,"session_group":%q`, group)
		}
		body := fmt.Sprintf(`{"external_id":%q,"source":"garmin","sport":%q,"started_at":"2026-06-07T08:00:00Z","ended_at":"2026-06-07T08:30:00Z"%s}`, extID, sport, grpFrag)
		rec := doReq(t, f.r, http.MethodPost, "/workouts", body)
		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	}
	post("g:leg", "bike", "garmin:1")
	post("g:loose", "swim", "")

	rec := doReq(t, f.r, http.MethodGet, "/workouts?from=2026-06-07T00:00:00Z&to=2026-06-08T00:00:00Z&session_group=garmin:1", "")
	require.Equal(t, http.StatusOK, rec.Code)
	var out struct {
		Workouts []workouts.Workout `json:"workouts"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Len(t, out.Workouts, 1)
	assert.Equal(t, "g:leg", *out.Workouts[0].ExternalID)
}

func TestList_SessionGroupWithoutWindowStillRequiresWindow(t *testing.T) {
	f := setup(t)
	rec := doReq(t, f.r, http.MethodGet, "/workouts?session_group=garmin:1", "")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"window_required"}`, rec.Body.String())
}

func TestPatch_SetIngestionMetrics_OnExistingRow(t *testing.T) {
	f := setup(t)
	postBody := `{"source":"manual","sport":"bike","started_at":"2026-06-07T08:00:00Z","ended_at":"2026-06-07T09:30:00Z"}`
	postRec := doReq(t, f.r, http.MethodPost, "/workouts", postBody)
	require.Equal(t, http.StatusCreated, postRec.Code)
	var w workouts.Workout
	require.NoError(t, json.Unmarshal(postRec.Body.Bytes(), &w))

	patchRec := doReq(t, f.r, http.MethodPatch, "/workouts/"+w.ID.String(), `{"sweat_loss_ml":1850,"temperature_c":31}`)
	require.Equal(t, http.StatusOK, patchRec.Code, patchRec.Body.String())
	var got workouts.Workout
	require.NoError(t, json.Unmarshal(patchRec.Body.Bytes(), &got))
	require.NotNil(t, got.SweatLossML)
	assert.InDelta(t, 1850, *got.SweatLossML, 0.05)
	require.NotNil(t, got.TemperatureC)
	assert.InDelta(t, 31, *got.TemperatureC, 0.05)
	assert.Nil(t, got.DistanceM, "untouched ingestion field stays NULL")
}

func TestPatch_ClearSessionGroupViaJSONNull(t *testing.T) {
	f := setup(t)
	postBody := `{"source":"manual","sport":"bike","started_at":"2026-06-07T08:00:00Z","ended_at":"2026-06-07T09:30:00Z","session_group":"garmin:9876543","sweat_loss_ml":2400}`
	postRec := doReq(t, f.r, http.MethodPost, "/workouts", postBody)
	require.Equal(t, http.StatusCreated, postRec.Code)
	var w workouts.Workout
	require.NoError(t, json.Unmarshal(postRec.Body.Bytes(), &w))

	patchRec := doReq(t, f.r, http.MethodPatch, "/workouts/"+w.ID.String(), `{"session_group":null}`)
	require.Equal(t, http.StatusOK, patchRec.Code, patchRec.Body.String())
	body := patchRec.Body.String()
	assert.NotContains(t, body, `"session_group"`)
	assert.Contains(t, body, `"sweat_loss_ml":2400`, "other ingestion field unchanged")
}

func TestPatch_IngestionValidationMatchesPost(t *testing.T) {
	f := setup(t)
	postRec := doReq(t, f.r, http.MethodPost, "/workouts",
		`{"source":"manual","sport":"bike","started_at":"2026-06-07T08:00:00Z","ended_at":"2026-06-07T09:30:00Z"}`)
	require.Equal(t, http.StatusCreated, postRec.Code)
	var w workouts.Workout
	require.NoError(t, json.Unmarshal(postRec.Body.Bytes(), &w))

	// temperature out of range → range hint
	rec := doReq(t, f.r, http.MethodPatch, "/workouts/"+w.ID.String(), `{"temperature_c":98.6}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"temperature_c_invalid","range":{"min":-40,"max":60}}`, rec.Body.String())

	// negative distance → plain code
	rec = doReq(t, f.r, http.MethodPatch, "/workouts/"+w.ID.String(), `{"distance_m":-100}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "distance_m_invalid", decodeErr(t, rec))

	// empty session_group → plain code
	rec = doReq(t, f.r, http.MethodPatch, "/workouts/"+w.ID.String(), `{"session_group":""}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "session_group_invalid", decodeErr(t, rec))
}

func decodeErr(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var got map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	s, _ := got["error"].(string)
	return s
}

// ============================================================================
// Planned/completed status lifecycle
// ============================================================================

func TestPost_PlannedFutureWorkoutAccepted(t *testing.T) {
	f := setup(t)
	// 3 weeks out — would be rejected for a completed workout, allowed for planned.
	start := time.Now().Add(21 * 24 * time.Hour).UTC().Format(time.RFC3339)
	end := time.Now().Add(21*24*time.Hour + 2*time.Hour).UTC().Format(time.RFC3339)
	body := fmt.Sprintf(`{"source":"garmin","sport":"bike","status":"planned","started_at":%q,"ended_at":%q}`, start, end)
	rec := doReq(t, f.r, http.MethodPost, "/workouts", body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var w workouts.Workout
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &w))
	assert.Equal(t, workouts.StatusPlanned, w.Status)
}

func TestPost_CompletedFutureStillRejected(t *testing.T) {
	f := setup(t)
	start := time.Now().Add(21 * 24 * time.Hour).UTC().Format(time.RFC3339)
	end := time.Now().Add(21*24*time.Hour + 2*time.Hour).UTC().Format(time.RFC3339)
	// status omitted → defaults to completed → far-future guard fires.
	body := fmt.Sprintf(`{"source":"garmin","sport":"bike","started_at":%q,"ended_at":%q}`, start, end)
	rec := doReq(t, f.r, http.MethodPost, "/workouts", body)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"started_at_too_far_future"}`, rec.Body.String())
}

func TestPost_PlannedMoreThanAYearOutRejected(t *testing.T) {
	f := setup(t)
	start := time.Now().AddDate(2, 0, 0).UTC().Format(time.RFC3339)
	end := time.Now().AddDate(2, 0, 0).Add(2 * time.Hour).UTC().Format(time.RFC3339)
	body := fmt.Sprintf(`{"source":"garmin","sport":"bike","status":"planned","started_at":%q,"ended_at":%q}`, start, end)
	rec := doReq(t, f.r, http.MethodPost, "/workouts", body)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"started_at_too_far_future"}`, rec.Body.String())
}

func TestPost_InvalidStatusRejected(t *testing.T) {
	f := setup(t)
	body := `{"source":"manual","sport":"run","status":"scheduled","started_at":"2026-06-07T08:00:00Z","ended_at":"2026-06-07T09:00:00Z"}`
	rec := doReq(t, f.r, http.MethodPost, "/workouts", body)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"status_invalid"}`, rec.Body.String())
}

func TestPost_DefaultStatusCompleted(t *testing.T) {
	f := setup(t)
	body := `{"source":"manual","sport":"run","started_at":"2026-06-07T08:00:00Z","ended_at":"2026-06-07T09:00:00Z"}`
	rec := doReq(t, f.r, http.MethodPost, "/workouts", body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var w workouts.Workout
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &w))
	assert.Equal(t, workouts.StatusCompleted, w.Status)
}

func TestList_FilterByStatus(t *testing.T) {
	f := setup(t)
	planStart := time.Now().Add(14 * 24 * time.Hour).UTC()
	post := func(extID, status, started, ended string) {
		t.Helper()
		body := fmt.Sprintf(`{"external_id":%q,"source":"garmin","sport":"bike","status":%q,"started_at":%q,"ended_at":%q}`, extID, status, started, ended)
		rec := doReq(t, f.r, http.MethodPost, "/workouts", body)
		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	}
	post("g:done", "completed", "2026-06-07T08:00:00Z", "2026-06-07T09:00:00Z")
	post("g:plan", "planned", planStart.Format(time.RFC3339), planStart.Add(time.Hour).Format(time.RFC3339))

	// Wide window covering both.
	from := "2026-06-01T00:00:00Z"
	to := planStart.Add(48 * time.Hour).UTC().Format(time.RFC3339)
	url := "/workouts?from=" + from + "&to=" + to + "&status=planned"
	rec := doReq(t, f.r, http.MethodGet, url, "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out struct {
		Workouts []workouts.Workout `json:"workouts"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Len(t, out.Workouts, 1)
	assert.Equal(t, "g:plan", *out.Workouts[0].ExternalID)
}

func TestPatch_PromotePlannedToCompleted(t *testing.T) {
	f := setup(t)
	planStart := time.Now().Add(7 * 24 * time.Hour).UTC()
	body := fmt.Sprintf(`{"source":"garmin","sport":"run","status":"planned","started_at":%q,"ended_at":%q}`, planStart.Format(time.RFC3339), planStart.Add(time.Hour).Format(time.RFC3339))
	rec := doReq(t, f.r, http.MethodPost, "/workouts", body)
	require.Equal(t, http.StatusCreated, rec.Code)
	var w workouts.Workout
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &w))

	rec = doReq(t, f.r, http.MethodPatch, "/workouts/"+w.ID.String(), `{"status":"completed"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var got workouts.Workout
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, workouts.StatusCompleted, got.Status)

	// Invalid status on patch → 400.
	rec = doReq(t, f.r, http.MethodPatch, "/workouts/"+w.ID.String(), `{"status":"bogus"}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"status_invalid"}`, rec.Body.String())
}
