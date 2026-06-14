package healthvitals_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/healthvitals"
	"github.com/vinzenzs/kazper/internal/store/storetest"
)

func init() { gin.SetMode(gin.TestMode) }

func setup(t *testing.T) *gin.Engine {
	t.Helper()
	pool := storetest.NewPool(t)
	svc := healthvitals.NewService(healthvitals.NewRepo(pool))
	r := gin.New()
	healthvitals.NewHandlers(svc).Register(r.Group("/"))
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

func decode(t *testing.T, b []byte) healthvitals.Snapshot {
	t.Helper()
	var s healthvitals.Snapshot
	require.NoError(t, json.Unmarshal(b, &s))
	return s
}

const full = `{"date":"2026-06-09","bp_systolic":118,"bp_diastolic":74,"bp_pulse":52,"resting_hr":48,"max_hr":171,"stress_avg":26}`

func TestUpsert_InsertThenUpdateNullsOmitted(t *testing.T) {
	r := setup(t)
	rec := do(t, r, http.MethodPost, "/health-vitals", full)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	s := decode(t, rec.Body.Bytes())
	require.NotNil(t, s.BPSystolic)
	assert.Equal(t, 118, *s.BPSystolic)

	rec2 := do(t, r, http.MethodPost, "/health-vitals", `{"date":"2026-06-09","resting_hr":46}`)
	require.Equal(t, http.StatusOK, rec2.Code, rec2.Body.String())
	s2 := decode(t, rec2.Body.Bytes())
	require.NotNil(t, s2.RestingHR)
	assert.Equal(t, 46, *s2.RestingHR)
	assert.Nil(t, s2.BPSystolic, "full-replace nulls omitted fields")
}

func TestUpsert_Validation(t *testing.T) {
	r := setup(t)
	cases := map[string]string{
		`{"bp_systolic":118}`:                    "date_invalid",
		`{"date":"2026-06-09","bp_systolic":0}`:  "bp_systolic_invalid",
		`{"date":"2026-06-09","stress_avg":120}`: "stress_avg_invalid",
		`{"date":"2026-06-09","max_hr":-1}`:      "max_hr_invalid",
	}
	for body, code := range cases {
		rec := do(t, r, http.MethodPost, "/health-vitals", body)
		require.Equal(t, http.StatusBadRequest, rec.Code, body)
		assert.JSONEq(t, `{"error":"`+code+`"}`, rec.Body.String(), body)
	}
}

func TestList_WindowAndGet(t *testing.T) {
	r := setup(t)
	require.Equal(t, http.StatusCreated, do(t, r, http.MethodPost, "/health-vitals", full).Code)

	missing := do(t, r, http.MethodGet, "/health-vitals", "")
	require.Equal(t, http.StatusBadRequest, missing.Code)
	assert.JSONEq(t, `{"error":"window_required"}`, missing.Body.String())

	tooBig := do(t, r, http.MethodGet, "/health-vitals?from=2026-01-01&to=2026-12-31", "")
	require.Equal(t, http.StatusBadRequest, tooBig.Code)
	assert.Contains(t, tooBig.Body.String(), "range_too_large")

	win := do(t, r, http.MethodGet, "/health-vitals?from=2026-06-01&to=2026-06-30", "")
	require.Equal(t, http.StatusOK, win.Code)
	var out struct {
		HealthVitals []healthvitals.Snapshot `json:"health_vitals"`
	}
	require.NoError(t, json.Unmarshal(win.Body.Bytes(), &out))
	require.Len(t, out.HealthVitals, 1)

	one := do(t, r, http.MethodGet, "/health-vitals/2026-06-09", "")
	require.Equal(t, http.StatusOK, one.Code)
	miss := do(t, r, http.MethodGet, "/health-vitals/2026-06-10", "")
	require.Equal(t, http.StatusNotFound, miss.Code)
	assert.JSONEq(t, `{"error":"health_vitals_not_found"}`, miss.Body.String())
}

func TestUnitIsolation(t *testing.T) {
	r := setup(t)
	rec := do(t, r, http.MethodPost, "/health-vitals", full)
	require.Equal(t, http.StatusCreated, rec.Code)
	body := rec.Body.String()
	for _, foreign := range []string{"kcal", "sleep_seconds", "hrv_ms", "vo2max", "weight_kg", "total_ml"} {
		assert.NotContains(t, body, foreign)
	}
}
