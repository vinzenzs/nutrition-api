package bodyweight_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/nutrition-api/internal/bodyweight"
)

func TestTrendEndpoint_ThreeConsecutiveDays(t *testing.T) {
	f := setup(t, "UTC")
	mustCreate(t, f.r, `{"weight_kg":73.1,"logged_at":"2026-06-05T07:00:00Z"}`)
	mustCreate(t, f.r, `{"weight_kg":72.4,"logged_at":"2026-06-06T07:00:00Z"}`)
	mustCreate(t, f.r, `{"weight_kg":73.6,"logged_at":"2026-06-07T07:00:00Z"}`)

	rec := doReq(t, f.r, http.MethodGet,
		"/weight/trend?from=2026-06-07&to=2026-06-07&window_days=3&tz=UTC", "", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var tr bodyweight.Trend
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &tr))
	require.Len(t, tr.Points, 1)
	assert.Equal(t, 3, tr.Points[0].SampleCount)
	require.NotNil(t, tr.Points[0].RollingAvgKg)
	assert.InDelta(t, 73.0, *tr.Points[0].RollingAvgKg, 0.001)
}

func TestTrendEndpoint_SparseWindow(t *testing.T) {
	f := setup(t, "UTC")
	mustCreate(t, f.r, `{"weight_kg":72.5,"logged_at":"2026-06-07T07:00:00Z"}`)

	rec := doReq(t, f.r, http.MethodGet,
		"/weight/trend?from=2026-06-07&to=2026-06-07&tz=UTC", "", nil) // default window_days=7
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var tr bodyweight.Trend
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &tr))
	require.Len(t, tr.Points, 1)
	assert.Equal(t, 1, tr.Points[0].SampleCount)
	assert.Equal(t, 7, tr.WindowDays, "window_days should default to 7")
}

func TestTrendEndpoint_GapDayHasNullAvg(t *testing.T) {
	f := setup(t, "UTC")
	mustCreate(t, f.r, `{"weight_kg":72.0,"logged_at":"2026-06-05T07:00:00Z"}`)
	mustCreate(t, f.r, `{"weight_kg":72.6,"logged_at":"2026-06-07T07:00:00Z"}`)

	rec := doReq(t, f.r, http.MethodGet,
		"/weight/trend?from=2026-06-05&to=2026-06-07&window_days=1&tz=UTC", "", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	var tr bodyweight.Trend
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &tr))
	require.Len(t, tr.Points, 3)
	assert.Equal(t, "2026-06-05", tr.Points[0].Date)
	assert.Equal(t, 1, tr.Points[0].SampleCount)
	assert.Equal(t, "2026-06-06", tr.Points[1].Date)
	assert.Equal(t, 0, tr.Points[1].SampleCount)
	assert.Nil(t, tr.Points[1].RollingAvgKg)
	assert.Equal(t, "2026-06-07", tr.Points[2].Date)
	assert.Equal(t, 1, tr.Points[2].SampleCount)
}

func TestTrendEndpoint_WindowDaysOutOfRange(t *testing.T) {
	f := setup(t, "UTC")
	rec := doReq(t, f.r, http.MethodGet,
		"/weight/trend?from=2026-06-07&to=2026-06-07&window_days=0&tz=UTC", "", nil)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "window_days_invalid", body["error"])

	rec2 := doReq(t, f.r, http.MethodGet,
		"/weight/trend?from=2026-06-07&to=2026-06-07&window_days=31&tz=UTC", "", nil)
	require.Equal(t, http.StatusBadRequest, rec2.Code)
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &body))
	assert.Equal(t, "window_days_invalid", body["error"])
}

func TestTrendEndpoint_RangeTooLarge(t *testing.T) {
	f := setup(t, "UTC")
	// 367-day span (2024-01-01 to 2025-01-02 inclusive)
	rec := doReq(t, f.r, http.MethodGet,
		"/weight/trend?from=2024-01-01&to=2025-01-02&tz=UTC", "", nil)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "range_too_large", body["error"])
	assert.EqualValues(t, 366, body["max_days"])
}

func TestTrendEndpoint_MissingRangeRejected(t *testing.T) {
	f := setup(t, "UTC")
	rec := doReq(t, f.r, http.MethodGet, "/weight/trend?tz=UTC", "", nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"range_required"}`, rec.Body.String())
}

func TestTrendEndpoint_InvalidDateRejected(t *testing.T) {
	f := setup(t, "UTC")
	rec := doReq(t, f.r, http.MethodGet,
		"/weight/trend?from=2026-13-99&to=2026-06-07&tz=UTC", "", nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"date_invalid"}`, rec.Body.String())
}

func TestTrendEndpoint_InvertedRangeRejected(t *testing.T) {
	f := setup(t, "UTC")
	rec := doReq(t, f.r, http.MethodGet,
		"/weight/trend?from=2026-06-10&to=2026-06-05&tz=UTC", "", nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"range_invalid"}`, rec.Body.String())
}

func TestTrendEndpoint_DefaultTZFallsBackAndWarns(t *testing.T) {
	f := setup(t, "Europe/Berlin")
	mustCreate(t, f.r, `{"weight_kg":72.5,"logged_at":"2026-06-07T01:00:00Z"}`) // 03:00 Berlin

	rec := doReq(t, f.r, http.MethodGet,
		"/weight/trend?from=2026-06-07&to=2026-06-07&window_days=1", "", nil) // no tz
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var tr bodyweight.Trend
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &tr))
	assert.Equal(t, "Europe/Berlin", tr.TZ)
	assert.Contains(t, f.logBuf.String(), "default_tz=Europe/Berlin")
}

func TestTrendEndpoint_InvalidTZRejected(t *testing.T) {
	f := setup(t, "UTC")
	rec := doReq(t, f.r, http.MethodGet,
		"/weight/trend?from=2026-06-07&to=2026-06-07&tz=Mars%2FOlympus", "", nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"tz_invalid"}`, rec.Body.String())
}
