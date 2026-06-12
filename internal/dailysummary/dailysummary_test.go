package dailysummary_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/nutrition-api/internal/dailysummary"
	"github.com/vinzenzs/nutrition-api/internal/store/storetest"
)

func init() { gin.SetMode(gin.TestMode) }

func setup(t *testing.T) *gin.Engine {
	t.Helper()
	pool := storetest.NewPool(t)
	svc := dailysummary.NewService(dailysummary.NewRepo(pool))
	r := gin.New()
	dailysummary.NewHandlers(svc).Register(r.Group("/"))
	return r
}

func do(t *testing.T, r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
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

func decode(t *testing.T, b []byte) dailysummary.Snapshot {
	t.Helper()
	var s dailysummary.Snapshot
	require.NoError(t, json.Unmarshal(b, &s))
	return s
}

const fullBody = `{"date":"2026-06-11","active_kcal":820,"resting_kcal":1650,"total_kcal":2470,"steps":12400,"floors":14,"moderate_intensity_minutes":35,"vigorous_intensity_minutes":48,"distance_m":9320.5}`

func TestUpsert_InsertThenUpdateInPlaceNullsOmitted(t *testing.T) {
	r := setup(t)
	rec := do(t, r, http.MethodPost, "/daily-summary", fullBody)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	s := decode(t, rec.Body.Bytes())
	assert.Equal(t, "2026-06-11", s.Date)
	require.NotNil(t, s.TotalKcal)
	assert.Equal(t, 2470, *s.TotalKcal)
	require.NotNil(t, s.Steps)
	assert.Equal(t, 12400, *s.Steps)

	// Second POST same date with only steps → 200, full-replace nulls the rest.
	rec2 := do(t, r, http.MethodPost, "/daily-summary", `{"date":"2026-06-11","steps":13100}`)
	require.Equal(t, http.StatusOK, rec2.Code, rec2.Body.String())
	s2 := decode(t, rec2.Body.Bytes())
	require.NotNil(t, s2.Steps)
	assert.Equal(t, 13100, *s2.Steps)
	assert.Nil(t, s2.TotalKcal, "full-replace upsert nulls omitted fields")
	assert.Nil(t, s2.DistanceM)

	// No duplicate row for the date.
	list := do(t, r, http.MethodGet, "/daily-summary?from=2026-06-11&to=2026-06-11", "")
	require.Equal(t, http.StatusOK, list.Code)
	var out struct {
		DailySummary []dailysummary.Snapshot `json:"daily_summary"`
	}
	require.NoError(t, json.Unmarshal(list.Body.Bytes(), &out))
	assert.Len(t, out.DailySummary, 1)
}

func TestUpsert_OmittedFieldsOmittedFromResponse(t *testing.T) {
	r := setup(t)
	rec := do(t, r, http.MethodPost, "/daily-summary", `{"date":"2026-06-11","total_kcal":2100}`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	// NULL metrics omitted via omitempty.
	body := rec.Body.String()
	assert.Contains(t, body, "total_kcal")
	assert.NotContains(t, body, "active_kcal")
	assert.NotContains(t, body, "steps")
	assert.NotContains(t, body, "distance_m")
}

func TestUpsert_ZeroIsValid(t *testing.T) {
	r := setup(t)
	rec := do(t, r, http.MethodPost, "/daily-summary", `{"date":"2026-06-11","floors":0,"steps":0}`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	s := decode(t, rec.Body.Bytes())
	require.NotNil(t, s.Floors)
	assert.Equal(t, 0, *s.Floors)
}

func TestUpsert_InvalidDateRejected(t *testing.T) {
	r := setup(t)
	for _, body := range []string{
		`{"total_kcal":2000}`,   // missing date
		`{"date":"2026-13-40"}`, // not a real date
		`{"date":"06/11/2026"}`, // wrong format
	} {
		rec := do(t, r, http.MethodPost, "/daily-summary", body)
		require.Equal(t, http.StatusBadRequest, rec.Code, body)
		assert.JSONEq(t, `{"error":"date_invalid"}`, rec.Body.String())
	}
}

func TestUpsert_NegativeMetricRejected(t *testing.T) {
	r := setup(t)
	cases := map[string]string{
		`{"date":"2026-06-11","total_kcal":-1}`:   "total_kcal_invalid",
		`{"date":"2026-06-11","steps":-5}`:        "steps_invalid",
		`{"date":"2026-06-11","distance_m":-0.1}`: "distance_m_invalid",
	}
	for body, code := range cases {
		rec := do(t, r, http.MethodPost, "/daily-summary", body)
		require.Equal(t, http.StatusBadRequest, rec.Code, body)
		assert.JSONEq(t, `{"error":"`+code+`"}`, rec.Body.String())
	}
	// Nothing persisted.
	list := do(t, r, http.MethodGet, "/daily-summary?from=2026-06-11&to=2026-06-11", "")
	var out struct {
		DailySummary []dailysummary.Snapshot `json:"daily_summary"`
	}
	require.NoError(t, json.Unmarshal(list.Body.Bytes(), &out))
	assert.Empty(t, out.DailySummary)
}

func TestList_WindowRequiredAndRangeCap(t *testing.T) {
	r := setup(t)
	rec := do(t, r, http.MethodGet, "/daily-summary", "")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"window_required"}`, rec.Body.String())

	rec = do(t, r, http.MethodGet, "/daily-summary?from=2026-01-01&to=2026-12-31", "")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "range_too_large")
}

func TestList_WindowFilteringOrdered(t *testing.T) {
	r := setup(t)
	for _, d := range []string{"2026-06-09", "2026-06-11", "2026-06-13"} {
		rec := do(t, r, http.MethodPost, "/daily-summary", `{"date":"`+d+`","steps":1000}`)
		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	}
	rec := do(t, r, http.MethodGet, "/daily-summary?from=2026-06-10&to=2026-06-12", "")
	require.Equal(t, http.StatusOK, rec.Code)
	var out struct {
		DailySummary []dailysummary.Snapshot `json:"daily_summary"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Len(t, out.DailySummary, 1)
	assert.Equal(t, "2026-06-11", out.DailySummary[0].Date)
}

func TestGet_SingleAndNotFound(t *testing.T) {
	r := setup(t)
	require.Equal(t, http.StatusCreated,
		do(t, r, http.MethodPost, "/daily-summary", fullBody).Code)

	rec := do(t, r, http.MethodGet, "/daily-summary/2026-06-11", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	s := decode(t, rec.Body.Bytes())
	assert.Equal(t, "2026-06-11", s.Date)

	miss := do(t, r, http.MethodGet, "/daily-summary/2026-06-12", "")
	require.Equal(t, http.StatusNotFound, miss.Code)
	assert.JSONEq(t, `{"error":"daily_summary_not_found"}`, miss.Body.String())
}

func TestDelete_204ThenNotFound(t *testing.T) {
	r := setup(t)
	require.Equal(t, http.StatusCreated,
		do(t, r, http.MethodPost, "/daily-summary", fullBody).Code)

	del := do(t, r, http.MethodDelete, "/daily-summary/2026-06-11", "")
	require.Equal(t, http.StatusNoContent, del.Code)

	// Gone.
	require.Equal(t, http.StatusNotFound, do(t, r, http.MethodGet, "/daily-summary/2026-06-11", "").Code)
	// Deleting again → 404.
	del2 := do(t, r, http.MethodDelete, "/daily-summary/2026-06-11", "")
	require.Equal(t, http.StatusNotFound, del2.Code)
	assert.JSONEq(t, `{"error":"daily_summary_not_found"}`, del2.Body.String())
}

func TestDistanceRoundedAtBoundary(t *testing.T) {
	r := setup(t)
	rec := do(t, r, http.MethodPost, "/daily-summary", `{"date":"2026-06-11","distance_m":9320.4789}`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	s := decode(t, rec.Body.Bytes())
	require.NotNil(t, s.DistanceM)
	assert.InDelta(t, 9320.5, *s.DistanceM, 0.0001, "distance_m rounded to 1dp at the boundary")
}
