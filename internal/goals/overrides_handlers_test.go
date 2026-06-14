package goals_test

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

	"github.com/vinzenzs/kazper/internal/auth"
	"github.com/vinzenzs/kazper/internal/goals"
	"github.com/vinzenzs/kazper/internal/idempotency"
	"github.com/vinzenzs/kazper/internal/store/storetest"
)

const (
	mobileToken = "mobile-token-aaaaaaaaaaaaaa"
	agentToken  = "agent-token-bbbbbbbbbbbbbbbb"
)

func setupOverrides(t *testing.T) (*gin.Engine, *goals.OverridesRepo) {
	t.Helper()
	pool := storetest.NewPool(t)
	repo := goals.NewOverridesRepo(pool)
	r := gin.New()
	rg := r.Group("/")
	goals.NewOverridesHandlers(repo).Register(rg)
	return r, repo
}

func setupOverridesWithMiddleware(t *testing.T) *gin.Engine {
	t.Helper()
	pool := storetest.NewPool(t)
	repo := goals.NewOverridesRepo(pool)
	idemRepo := idempotency.NewRepo(pool)
	r := gin.New()
	r.Use(auth.Middleware(auth.Config{MobileToken: mobileToken, AgentToken: agentToken}))
	r.Use(idempotency.Middleware(idemRepo, time.Hour))
	rg := r.Group("/")
	goals.NewOverridesHandlers(repo).Register(rg)
	return r
}

func doReq(r *gin.Engine, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
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

// ============================================================================
// PUT /goals/overrides/{date}
// ============================================================================

func TestOverridePut_HappyPath(t *testing.T) {
	r, _ := setupOverrides(t)
	body := `{"kcal":{"min":2280,"max":2520},"protein_g":{"min":160,"max":200}}`
	rec := doReq(r, http.MethodPut, "/goals/overrides/2026-06-15", body, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var resp struct {
		Goals *goals.Goals `json:"goals"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.NotNil(t, resp.Goals)
	require.NotNil(t, resp.Goals.Kcal)
	assert.InDelta(t, 2280, *resp.Goals.Kcal.Min, 0.001)
	assert.InDelta(t, 2520, *resp.Goals.Kcal.Max, 0.001)
}

func TestOverridePut_ReplacesPreviousValues(t *testing.T) {
	r, _ := setupOverrides(t)
	first := `{"kcal":{"min":2000,"max":2200},"protein_g":{"min":150}}`
	doReq(r, http.MethodPut, "/goals/overrides/2026-06-15", first, nil)

	second := `{"kcal":{"min":2400,"max":2600}}`
	rec := doReq(r, http.MethodPut, "/goals/overrides/2026-06-15", second, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	getRec := doReq(r, http.MethodGet, "/goals/overrides/2026-06-15", "", nil)
	require.Equal(t, http.StatusOK, getRec.Code)
	assert.NotContains(t, getRec.Body.String(), "protein_g",
		"second PUT must clear previously-set fields")
	assert.Contains(t, getRec.Body.String(), `"kcal":{"min":2400,"max":2600}`)
}

func TestOverridePut_InvalidDateRejected(t *testing.T) {
	r, _ := setupOverrides(t)
	rec := doReq(r, http.MethodPut, "/goals/overrides/2026-13-99",
		`{"kcal":{"min":2000}}`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"date_invalid"}`, rec.Body.String())
}

func TestOverridePut_LegacyKcalTargetRejected(t *testing.T) {
	r, _ := setupOverrides(t)
	rec := doReq(r, http.MethodPut, "/goals/overrides/2026-06-15",
		`{"kcal_target":2200}`, nil)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "goal_value_invalid", body["error"])
	assert.Equal(t, "kcal_target", body["field"])
}

func TestOverridePut_EmptyRangeObjectRejected(t *testing.T) {
	r, _ := setupOverrides(t)
	rec := doReq(r, http.MethodPut, "/goals/overrides/2026-06-15",
		`{"kcal":{}}`, nil)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "goal_value_invalid", body["error"])
	assert.Equal(t, "kcal", body["field"])
}

func TestOverridePut_InvertedMinMaxRejected(t *testing.T) {
	r, _ := setupOverrides(t)
	rec := doReq(r, http.MethodPut, "/goals/overrides/2026-06-15",
		`{"kcal":{"min":2500,"max":2000}}`, nil)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "goal_range_invalid", body["error"])
}

func TestOverridePut_IdempotencyKeyRejected(t *testing.T) {
	r := setupOverridesWithMiddleware(t)
	rec := doReq(r, http.MethodPut, "/goals/overrides/2026-06-15",
		`{"kcal":{"min":2000,"max":2200}}`,
		map[string]string{
			"Authorization":   "Bearer " + mobileToken,
			"Idempotency-Key": "should-be-rejected",
		})
	require.Equal(t, http.StatusBadRequest, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "idempotency_unsupported_for_put", body["error"])
}

// ============================================================================
// GET /goals/overrides/{date}
// ============================================================================

func TestOverrideGet_ExistingRow(t *testing.T) {
	r, _ := setupOverrides(t)
	doReq(r, http.MethodPut, "/goals/overrides/2026-06-15",
		`{"kcal":{"min":2280,"max":2520}}`, nil)

	rec := doReq(r, http.MethodGet, "/goals/overrides/2026-06-15", "", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"kcal":{"min":2280,"max":2520}`)
}

func TestOverrideGet_MissingReturns404(t *testing.T) {
	r, _ := setupOverrides(t)
	rec := doReq(r, http.MethodGet, "/goals/overrides/2026-06-15", "", nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.JSONEq(t, `{"error":"override_not_found"}`, rec.Body.String())
}

func TestOverrideGet_InvalidDateRejected(t *testing.T) {
	r, _ := setupOverrides(t)
	rec := doReq(r, http.MethodGet, "/goals/overrides/bad-date", "", nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"date_invalid"}`, rec.Body.String())
}

// ============================================================================
// DELETE /goals/overrides/{date}
// ============================================================================

func TestOverrideDelete_Happy(t *testing.T) {
	r, _ := setupOverrides(t)
	doReq(r, http.MethodPut, "/goals/overrides/2026-06-15",
		`{"kcal":{"min":2000}}`, nil)

	rec := doReq(r, http.MethodDelete, "/goals/overrides/2026-06-15", "", nil)
	assert.Equal(t, http.StatusNoContent, rec.Code)

	getRec := doReq(r, http.MethodGet, "/goals/overrides/2026-06-15", "", nil)
	assert.Equal(t, http.StatusNotFound, getRec.Code)
}

func TestOverrideDelete_UnknownReturns404(t *testing.T) {
	r, _ := setupOverrides(t)
	rec := doReq(r, http.MethodDelete, "/goals/overrides/2026-06-15", "", nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.JSONEq(t, `{"error":"override_not_found"}`, rec.Body.String())
}

// ============================================================================
// GET /goals/overrides?from=&to=
// ============================================================================

func TestOverrideList_Ordering(t *testing.T) {
	r, _ := setupOverrides(t)
	for i, d := range []string{"2026-06-17", "2026-06-15", "2026-06-16"} {
		body := `{"kcal":{"min":` + []string{"2100", "2200", "2300"}[i] + `}}`
		doReq(r, http.MethodPut, "/goals/overrides/"+d, body, nil)
	}

	rec := doReq(r, http.MethodGet, "/goals/overrides?from=2026-06-01&to=2026-06-30", "", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body struct {
		Overrides []struct {
			Date  string       `json:"date"`
			Goals *goals.Goals `json:"goals"`
		} `json:"overrides"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Overrides, 3)
	assert.Equal(t, "2026-06-15", body.Overrides[0].Date)
	assert.Equal(t, "2026-06-16", body.Overrides[1].Date)
	assert.Equal(t, "2026-06-17", body.Overrides[2].Date)
}

func TestOverrideList_MissingRangeRejected(t *testing.T) {
	r, _ := setupOverrides(t)
	rec := doReq(r, http.MethodGet, "/goals/overrides", "", nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"range_required"}`, rec.Body.String())
}

func TestOverrideList_InvalidDateRejected(t *testing.T) {
	r, _ := setupOverrides(t)
	rec := doReq(r, http.MethodGet, "/goals/overrides?from=bad&to=2026-06-30", "", nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"date_invalid"}`, rec.Body.String())
}

func TestOverrideList_InvertedRangeRejected(t *testing.T) {
	r, _ := setupOverrides(t)
	rec := doReq(r, http.MethodGet, "/goals/overrides?from=2026-06-30&to=2026-06-01", "", nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"range_invalid"}`, rec.Body.String())
}

func TestOverrideList_RangeTooLargeRejected(t *testing.T) {
	r, _ := setupOverrides(t)
	rec := doReq(r, http.MethodGet, "/goals/overrides?from=2024-01-01&to=2026-12-31", "", nil)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "range_too_large", body["error"])
	assert.EqualValues(t, 366, body["max_days"])
}
