package recoverymetrics_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/nutrition-api/internal/recoverymetrics"
	"github.com/vinzenzs/nutrition-api/internal/store/storetest"
)

func init() { gin.SetMode(gin.TestMode) }

func setup(t *testing.T) *gin.Engine {
	t.Helper()
	pool := storetest.NewPool(t)
	svc := recoverymetrics.NewService(recoverymetrics.NewRepo(pool))
	r := gin.New()
	recoverymetrics.NewHandlers(svc).Register(r.Group("/"))
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

func TestUpsert_InsertThenUpdateInPlace(t *testing.T) {
	r := setup(t)
	body := `{"date":"2026-06-09","sleep_seconds":27000,"sleep_score":82,"hrv_ms":61.0,"resting_hr":48,"stress_avg":28,"training_readiness":74}`
	rec := do(t, r, http.MethodPost, "/recovery-metrics", body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var s recoverymetrics.Snapshot
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &s))
	assert.Equal(t, "2026-06-09", s.Date)
	require.NotNil(t, s.RestingHR)
	assert.Equal(t, 48, *s.RestingHR)

	// Second POST for same date with fewer fields → 200, full-replace nulls the rest.
	rec2 := do(t, r, http.MethodPost, "/recovery-metrics", `{"date":"2026-06-09","resting_hr":46}`)
	require.Equal(t, http.StatusOK, rec2.Code, rec2.Body.String())
	var s2 recoverymetrics.Snapshot
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &s2))
	assert.Equal(t, 46, *s2.RestingHR)
	assert.Nil(t, s2.SleepSeconds, "full-replace upsert nulls omitted fields")
	assert.Nil(t, s2.TrainingReadiness)
}

func TestUpsert_OmittedFieldsOmittedFromResponse(t *testing.T) {
	r := setup(t)
	rec := do(t, r, http.MethodPost, "/recovery-metrics", `{"date":"2026-06-09","sleep_seconds":27000}`)
	require.Equal(t, http.StatusCreated, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, `"sleep_seconds":27000`)
	for _, k := range []string{`"hrv_ms"`, `"resting_hr"`, `"stress_avg"`, `"training_readiness"`, `"body_battery_charged"`} {
		assert.NotContains(t, body, k)
	}
}

func TestUpsert_MissingOrInvalidDate(t *testing.T) {
	r := setup(t)
	for _, b := range []string{`{"sleep_seconds":100}`, `{"date":"2026-13-99","sleep_seconds":100}`, `{"date":"June 9"}`} {
		rec := do(t, r, http.MethodPost, "/recovery-metrics", b)
		require.Equal(t, http.StatusBadRequest, rec.Code, b)
		assert.JSONEq(t, `{"error":"date_invalid"}`, rec.Body.String())
	}
}

func TestUpsert_OutOfRangeMetrics(t *testing.T) {
	r := setup(t)
	cases := map[string]string{
		`{"date":"2026-06-09","sleep_score":120}`:        "sleep_score_invalid",
		`{"date":"2026-06-09","stress_avg":-1}`:          "stress_avg_invalid",
		`{"date":"2026-06-09","resting_hr":0}`:           "resting_hr_invalid",
		`{"date":"2026-06-09","training_readiness":101}`: "training_readiness_invalid",
		`{"date":"2026-06-09","hrv_ms":0}`:               "hrv_ms_invalid",
	}
	for body, want := range cases {
		rec := do(t, r, http.MethodPost, "/recovery-metrics", body)
		require.Equal(t, http.StatusBadRequest, rec.Code, body)
		var got map[string]string
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
		assert.Equal(t, want, got["error"], body)
	}
}

func TestList_WindowAndCaps(t *testing.T) {
	r := setup(t)
	for _, d := range []string{"2026-06-01", "2026-06-15", "2026-07-05"} {
		rec := do(t, r, http.MethodPost, "/recovery-metrics", `{"date":"`+d+`","resting_hr":48}`)
		require.Equal(t, http.StatusCreated, rec.Code)
	}
	rec := do(t, r, http.MethodGet, "/recovery-metrics?from=2026-06-01&to=2026-06-30", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out struct {
		RecoveryMetrics []recoverymetrics.Snapshot `json:"recovery_metrics"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Len(t, out.RecoveryMetrics, 2)
	assert.Equal(t, "2026-06-01", out.RecoveryMetrics[0].Date)
	assert.Equal(t, "2026-06-15", out.RecoveryMetrics[1].Date)

	// missing window
	rec = do(t, r, http.MethodGet, "/recovery-metrics?from=2026-06-01", "")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"window_required"}`, rec.Body.String())

	// > 92 days
	rec = do(t, r, http.MethodGet, "/recovery-metrics?from=2026-01-01&to=2026-12-31", "")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	var m map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &m))
	assert.Equal(t, "range_too_large", m["error"])
}

func TestGetAndDeleteByDate(t *testing.T) {
	r := setup(t)
	require.Equal(t, http.StatusCreated, do(t, r, http.MethodPost, "/recovery-metrics", `{"date":"2026-06-09","resting_hr":48}`).Code)

	rec := do(t, r, http.MethodGet, "/recovery-metrics/2026-06-09", "")
	require.Equal(t, http.StatusOK, rec.Code)

	rec = do(t, r, http.MethodGet, "/recovery-metrics/2026-06-10", "")
	require.Equal(t, http.StatusNotFound, rec.Code)
	assert.JSONEq(t, `{"error":"recovery_metrics_not_found"}`, rec.Body.String())

	rec = do(t, r, http.MethodDelete, "/recovery-metrics/2026-06-09", "")
	require.Equal(t, http.StatusNoContent, rec.Code)
	rec = do(t, r, http.MethodGet, "/recovery-metrics/2026-06-09", "")
	require.Equal(t, http.StatusNotFound, rec.Code)
	rec = do(t, r, http.MethodDelete, "/recovery-metrics/2026-06-09", "")
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestUpsert_ExtendedFieldsRoundTrip(t *testing.T) {
	r := setup(t)
	body := `{"date":"2026-06-09","sleep_seconds":27000,"spo2_avg":95,"spo2_lowest":89,"respiration_avg":13.4,"respiration_lowest":9.8,"deep_sleep_seconds":6000,"light_sleep_seconds":15000,"rem_sleep_seconds":5400,"awake_seconds":600}`
	rec := do(t, r, http.MethodPost, "/recovery-metrics", body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	rec = do(t, r, http.MethodGet, "/recovery-metrics/2026-06-09", "")
	require.Equal(t, http.StatusOK, rec.Code)
	var s recoverymetrics.Snapshot
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &s))
	require.NotNil(t, s.Spo2Avg)
	assert.Equal(t, 95, *s.Spo2Avg)
	require.NotNil(t, s.Spo2Lowest)
	assert.Equal(t, 89, *s.Spo2Lowest)
	require.NotNil(t, s.RespirationAvg)
	assert.InDelta(t, 13.4, *s.RespirationAvg, 0.05)
	require.NotNil(t, s.RespirationLowest)
	assert.InDelta(t, 9.8, *s.RespirationLowest, 0.05)
	require.NotNil(t, s.DeepSleepSeconds)
	assert.Equal(t, 6000, *s.DeepSleepSeconds)
	require.NotNil(t, s.AwakeSeconds)
	assert.Equal(t, 600, *s.AwakeSeconds)
}

func TestUpsert_ExtendedFieldsOmittedWhenAbsent(t *testing.T) {
	r := setup(t)
	rec := do(t, r, http.MethodPost, "/recovery-metrics", `{"date":"2026-06-09","sleep_seconds":27000}`)
	require.Equal(t, http.StatusCreated, rec.Code)
	body := rec.Body.String()
	for _, k := range []string{
		`"spo2_avg"`, `"spo2_lowest"`, `"respiration_avg"`, `"respiration_lowest"`,
		`"deep_sleep_seconds"`, `"light_sleep_seconds"`, `"rem_sleep_seconds"`, `"awake_seconds"`,
	} {
		assert.NotContains(t, body, k)
	}
}

func TestUpsert_ExtendedOutOfRange(t *testing.T) {
	r := setup(t)
	cases := map[string]string{
		`{"date":"2026-06-09","spo2_avg":120}`:          "spo2_avg_invalid",
		`{"date":"2026-06-09","spo2_lowest":-1}`:        "spo2_lowest_invalid",
		`{"date":"2026-06-09","respiration_avg":0}`:     "respiration_avg_invalid",
		`{"date":"2026-06-09","respiration_lowest":-1}`: "respiration_lowest_invalid",
		`{"date":"2026-06-09","deep_sleep_seconds":-1}`: "deep_sleep_seconds_invalid",
		`{"date":"2026-06-09","awake_seconds":-1}`:      "awake_seconds_invalid",
	}
	for body, want := range cases {
		rec := do(t, r, http.MethodPost, "/recovery-metrics", body)
		require.Equal(t, http.StatusBadRequest, rec.Code, body)
		var got map[string]string
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
		assert.Equal(t, want, got["error"], body)
	}
}

func TestUnitIsolation_NoForeignFields(t *testing.T) {
	r := setup(t)
	rec := do(t, r, http.MethodPost, "/recovery-metrics", `{"date":"2026-06-09","sleep_seconds":27000}`)
	require.Equal(t, http.StatusCreated, rec.Code)
	body := rec.Body.String()
	for _, k := range []string{`"kcal"`, `"vo2max`, `"weight_kg"`} {
		assert.NotContains(t, body, k)
	}
}
