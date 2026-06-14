package athleteconfig_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/athleteconfig"
	"github.com/vinzenzs/kazper/internal/idempotency"
	"github.com/vinzenzs/kazper/internal/store/storetest"
)

func init() { gin.SetMode(gin.TestMode) }

// setup wires the handlers behind the idempotency middleware exactly as
// server.go does, so the PUT-rejects-Idempotency-Key behavior is exercised.
func setup(t *testing.T) *gin.Engine {
	t.Helper()
	pool := storetest.NewPool(t)
	svc := athleteconfig.NewService(athleteconfig.NewRepo(pool))
	r := gin.New()
	api := r.Group("/")
	api.Use(idempotency.Middleware(idempotency.NewRepo(pool), time.Hour))
	athleteconfig.NewHandlers(svc).Register(api)
	return r
}

func do(t *testing.T, r *gin.Engine, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
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
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func decode(t *testing.T, b []byte) *athleteconfig.AthleteConfig {
	t.Helper()
	var out struct {
		AthleteConfig *athleteconfig.AthleteConfig `json:"athlete_config"`
	}
	require.NoError(t, json.Unmarshal(b, &out))
	return out.AthleteConfig
}

func TestGet_NullBeforeAnyWrite(t *testing.T) {
	r := setup(t)
	rec := do(t, r, http.MethodGet, "/athlete-config", "", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"athlete_config":null}`, rec.Body.String())
}

func TestPut_CreatesThenFullReplaceClearsOmitted(t *testing.T) {
	r := setup(t)
	rec := do(t, r, http.MethodPut, "/athlete-config",
		`{"ftp_watts":265,"threshold_hr":168,"max_hr":188}`, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	cfg := decode(t, rec.Body.Bytes())
	require.NotNil(t, cfg)
	require.NotNil(t, cfg.FtpWatts)
	assert.Equal(t, 265, *cfg.FtpWatts)

	// Second PUT with only max_hr → full-replace nulls ftp/threshold.
	rec2 := do(t, r, http.MethodPut, "/athlete-config", `{"max_hr":190}`, nil)
	require.Equal(t, http.StatusOK, rec2.Code, rec2.Body.String())
	cfg2 := decode(t, rec2.Body.Bytes())
	require.NotNil(t, cfg2.MaxHR)
	assert.Equal(t, 190, *cfg2.MaxHR)
	assert.Nil(t, cfg2.FtpWatts, "full-replace cleared omitted field")
	assert.Nil(t, cfg2.ThresholdHR)

	// GET reflects the stored singleton.
	got := decode(t, do(t, r, http.MethodGet, "/athlete-config", "", nil).Body.Bytes())
	require.NotNil(t, got.MaxHR)
	assert.Equal(t, 190, *got.MaxHR)
}

func TestPut_PartialConfigOmitsNulls(t *testing.T) {
	r := setup(t)
	rec := do(t, r, http.MethodPut, "/athlete-config", `{"ftp_watts":265,"max_hr":188}`, nil)
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "ftp_watts")
	assert.Contains(t, body, "max_hr")
	assert.NotContains(t, body, "threshold_hr")
	assert.NotContains(t, body, "hr_zone_1_max")
}

func TestPut_HRZoneRoundTripPowerZonesOmitted(t *testing.T) {
	r := setup(t)
	rec := do(t, r, http.MethodPut, "/athlete-config",
		`{"hr_zone_1_max":120,"hr_zone_2_max":140,"hr_zone_3_max":155,"hr_zone_4_max":168,"hr_zone_5_max":182}`, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	cfg := decode(t, rec.Body.Bytes())
	require.NotNil(t, cfg.HRZone1Max)
	assert.Equal(t, 120, *cfg.HRZone1Max)
	require.NotNil(t, cfg.HRZone5Max)
	assert.Equal(t, 182, *cfg.HRZone5Max)
	// No power zones supplied → omitted.
	assert.NotContains(t, rec.Body.String(), "power_zone_1_max")
}

func TestPut_NegativeValueRejected(t *testing.T) {
	r := setup(t)
	rec := do(t, r, http.MethodPut, "/athlete-config", `{"ftp_watts":-10}`, nil)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"athlete_config_value_invalid","field":"ftp_watts"}`, rec.Body.String())
	// Not applied: still null.
	assert.JSONEq(t, `{"athlete_config":null}`, do(t, r, http.MethodGet, "/athlete-config", "", nil).Body.String())
}

func TestPut_IdempotencyKeyRejected(t *testing.T) {
	r := setup(t)
	rec := do(t, r, http.MethodPut, "/athlete-config", `{"ftp_watts":265}`,
		map[string]string{"Idempotency-Key": "abc123"})
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "idempotency_unsupported_for_put")
	// No row created.
	assert.JSONEq(t, `{"athlete_config":null}`, do(t, r, http.MethodGet, "/athlete-config", "", nil).Body.String())
}

func TestPut_FloatRoundedAtBoundary(t *testing.T) {
	r := setup(t)
	rec := do(t, r, http.MethodPut, "/athlete-config", `{"threshold_pace_sec_per_km":258.04}`, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	cfg := decode(t, rec.Body.Bytes())
	require.NotNil(t, cfg.ThresholdPaceSecPerKm)
	assert.InDelta(t, 258.0, *cfg.ThresholdPaceSecPerKm, 0.0001, "rounded to 1dp at the boundary")
}
