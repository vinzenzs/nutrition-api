package bodyweight_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/nutrition-api/internal/auth"
	"github.com/vinzenzs/nutrition-api/internal/bodyweight"
	"github.com/vinzenzs/nutrition-api/internal/idempotency"
	"github.com/vinzenzs/nutrition-api/internal/store/storetest"
)

func init() {
	gin.SetMode(gin.TestMode)
}

const (
	mobileToken = "mobile-token-aaaaaaaaaaaaaa"
	agentToken  = "agent-token-bbbbbbbbbbbbbbbb"
)

type fixture struct {
	r      *gin.Engine
	logBuf *bytes.Buffer
}

func setup(t *testing.T, defaultTZ string) *fixture {
	t.Helper()
	pool := storetest.NewPool(t)
	repo := bodyweight.NewRepo(pool)
	svc := bodyweight.NewService(repo)
	logBuf := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(logBuf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	r := gin.New()
	rg := r.Group("/")
	bodyweight.NewHandlers(svc, defaultTZ, logger).Register(rg)
	return &fixture{r: r, logBuf: logBuf}
}

func setupWithMiddleware(t *testing.T) *fixture {
	t.Helper()
	pool := storetest.NewPool(t)
	repo := bodyweight.NewRepo(pool)
	svc := bodyweight.NewService(repo)
	idemRepo := idempotency.NewRepo(pool)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	r := gin.New()
	r.Use(auth.Middleware(auth.Config{MobileToken: mobileToken, AgentToken: agentToken}))
	r.Use(idempotency.Middleware(idemRepo, time.Hour))
	rg := r.Group("/")
	bodyweight.NewHandlers(svc, "UTC", logger).Register(rg)
	return &fixture{r: r}
}

func doReq(t *testing.T, r *gin.Engine, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
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

// ============================================================================
// POST /weight
// ============================================================================

func TestCreate_HappyPath(t *testing.T) {
	f := setup(t, "UTC")
	rec := doReq(t, f.r, http.MethodPost, "/weight",
		`{"weight_kg":72.5,"logged_at":"2026-06-07T07:00:00Z"}`, nil)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var e bodyweight.Entry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &e))
	assert.Equal(t, 72.5, e.WeightKg)
	assert.Equal(t, time.Date(2026, 6, 7, 7, 0, 0, 0, time.UTC), e.LoggedAt.UTC())
	assert.Nil(t, e.BodyFatPct)
	assert.Nil(t, e.Note)
}

func TestCreate_WithBodyFatAndNote(t *testing.T) {
	f := setup(t, "UTC")
	rec := doReq(t, f.r, http.MethodPost, "/weight",
		`{"weight_kg":72.5,"body_fat_pct":14.2,"logged_at":"2026-06-07T07:00:00Z","note":"morning, fasted"}`, nil)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var e bodyweight.Entry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &e))
	require.NotNil(t, e.BodyFatPct)
	assert.Equal(t, 14.2, *e.BodyFatPct)
	require.NotNil(t, e.Note)
	assert.Equal(t, "morning, fasted", *e.Note)
}

func TestCreate_MissingWeightRejected(t *testing.T) {
	f := setup(t, "UTC")
	rec := doReq(t, f.r, http.MethodPost, "/weight",
		`{"logged_at":"2026-06-07T07:00:00Z"}`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"weight_kg_invalid"}`, rec.Body.String())
}

func TestCreate_ZeroWeightRejected(t *testing.T) {
	f := setup(t, "UTC")
	rec := doReq(t, f.r, http.MethodPost, "/weight",
		`{"weight_kg":0,"logged_at":"2026-06-07T07:00:00Z"}`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"weight_kg_invalid"}`, rec.Body.String())
}

func TestCreate_NegativeWeightRejected(t *testing.T) {
	f := setup(t, "UTC")
	rec := doReq(t, f.r, http.MethodPost, "/weight",
		`{"weight_kg":-72,"logged_at":"2026-06-07T07:00:00Z"}`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"weight_kg_invalid"}`, rec.Body.String())
}

func TestCreate_BodyFatBelowZeroRejected(t *testing.T) {
	f := setup(t, "UTC")
	rec := doReq(t, f.r, http.MethodPost, "/weight",
		`{"weight_kg":72,"body_fat_pct":-1,"logged_at":"2026-06-07T07:00:00Z"}`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"body_fat_pct_invalid"}`, rec.Body.String())
}

func TestCreate_BodyFatAboveHundredRejected(t *testing.T) {
	f := setup(t, "UTC")
	rec := doReq(t, f.r, http.MethodPost, "/weight",
		`{"weight_kg":72,"body_fat_pct":120,"logged_at":"2026-06-07T07:00:00Z"}`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"body_fat_pct_invalid"}`, rec.Body.String())
}

func TestCreate_NoteTooLongRejected(t *testing.T) {
	f := setup(t, "UTC")
	longNote := strings.Repeat("a", 501)
	body := fmt.Sprintf(`{"weight_kg":72,"logged_at":"2026-06-07T07:00:00Z","note":%q}`, longNote)
	rec := doReq(t, f.r, http.MethodPost, "/weight", body, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"note_too_long"}`, rec.Body.String())
}

func TestCreate_LoggedAtFarFutureRejected(t *testing.T) {
	f := setup(t, "UTC")
	farFuture := time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339)
	body := fmt.Sprintf(`{"weight_kg":72,"logged_at":%q}`, farFuture)
	rec := doReq(t, f.r, http.MethodPost, "/weight", body, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"logged_at_too_far_future"}`, rec.Body.String())
}

func TestCreate_InvalidLoggedAtRejected(t *testing.T) {
	f := setup(t, "UTC")
	rec := doReq(t, f.r, http.MethodPost, "/weight",
		`{"weight_kg":72,"logged_at":"not-a-timestamp"}`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"logged_at_invalid"}`, rec.Body.String())
}

// ============================================================================
// GET /weight (list)
// ============================================================================

func TestList_WindowFiltersEntries(t *testing.T) {
	f := setup(t, "UTC")
	mustCreate(t, f.r, `{"weight_kg":72.3,"logged_at":"2026-06-07T05:00:00Z"}`)
	mustCreate(t, f.r, `{"weight_kg":72.6,"logged_at":"2026-06-07T20:00:00Z"}`)
	mustCreate(t, f.r, `{"weight_kg":72.9,"logged_at":"2026-06-08T05:00:00Z"}`) // outside

	rec := doReq(t, f.r, http.MethodGet,
		"/weight?from=2026-06-07T00:00:00Z&to=2026-06-08T00:00:00Z", "", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body struct {
		Entries []bodyweight.Entry `json:"entries"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Entries, 2)
}

func TestList_MissingWindowRejected(t *testing.T) {
	f := setup(t, "UTC")
	rec := doReq(t, f.r, http.MethodGet, "/weight", "", nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"window_required"}`, rec.Body.String())
}

func TestList_InvertedWindowRejected(t *testing.T) {
	f := setup(t, "UTC")
	rec := doReq(t, f.r, http.MethodGet,
		"/weight?from=2026-06-08T00:00:00Z&to=2026-06-07T00:00:00Z", "", nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"window_invalid"}`, rec.Body.String())
}

func TestList_RangeTooLargeRejected(t *testing.T) {
	f := setup(t, "UTC")
	rec := doReq(t, f.r, http.MethodGet,
		"/weight?from=2026-01-01T00:00:00Z&to=2026-12-31T00:00:00Z", "", nil)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "range_too_large", body["error"])
	assert.EqualValues(t, 92, body["max_days"])
}

// ============================================================================
// PATCH /weight/{id}
// ============================================================================

func TestPatch_UpdatesOnlySuppliedFields(t *testing.T) {
	f := setup(t, "UTC")
	created := mustCreate(t, f.r, `{"weight_kg":72.5,"body_fat_pct":14.2,"logged_at":"2026-06-07T07:00:00Z"}`)
	rec := doReq(t, f.r, http.MethodPatch, "/weight/"+created.ID.String(),
		`{"body_fat_pct":13.8}`, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var e bodyweight.Entry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &e))
	require.NotNil(t, e.BodyFatPct)
	assert.Equal(t, 13.8, *e.BodyFatPct)
	assert.Equal(t, 72.5, e.WeightKg, "weight_kg untouched")
}

func TestPatch_InvalidWeightRejected(t *testing.T) {
	f := setup(t, "UTC")
	created := mustCreate(t, f.r, `{"weight_kg":72,"logged_at":"2026-06-07T07:00:00Z"}`)
	rec := doReq(t, f.r, http.MethodPatch, "/weight/"+created.ID.String(),
		`{"weight_kg":-1}`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"weight_kg_invalid"}`, rec.Body.String())
}

func TestPatch_UnknownIdReturns404(t *testing.T) {
	f := setup(t, "UTC")
	rec := doReq(t, f.r, http.MethodPatch,
		"/weight/00000000-0000-0000-0000-000000000000",
		`{"weight_kg":72}`, nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.JSONEq(t, `{"error":"weight_not_found"}`, rec.Body.String())
}

// ============================================================================
// DELETE /weight/{id}
// ============================================================================

func TestDelete_HappyPath(t *testing.T) {
	f := setup(t, "UTC")
	created := mustCreate(t, f.r, `{"weight_kg":72.5,"logged_at":"2026-06-07T07:00:00Z"}`)
	rec := doReq(t, f.r, http.MethodDelete, "/weight/"+created.ID.String(), "", nil)
	assert.Equal(t, http.StatusNoContent, rec.Code)

	listRec := doReq(t, f.r, http.MethodGet,
		"/weight?from=2026-06-07T00:00:00Z&to=2026-06-08T00:00:00Z", "", nil)
	require.Equal(t, http.StatusOK, listRec.Code)
	var body struct {
		Entries []bodyweight.Entry `json:"entries"`
	}
	require.NoError(t, json.Unmarshal(listRec.Body.Bytes(), &body))
	assert.Len(t, body.Entries, 0)
}

func TestDelete_UnknownIdReturns404(t *testing.T) {
	f := setup(t, "UTC")
	rec := doReq(t, f.r, http.MethodDelete,
		"/weight/00000000-0000-0000-0000-000000000000", "", nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.JSONEq(t, `{"error":"weight_not_found"}`, rec.Body.String())
}

// ============================================================================
// Idempotency middleware integration
// ============================================================================

func TestIdempotency_SameKeyAndBodyReplays(t *testing.T) {
	f := setupWithMiddleware(t)
	body := `{"weight_kg":72.5,"logged_at":"2026-06-07T07:00:00Z"}`
	headers := map[string]string{
		"Authorization":   "Bearer " + mobileToken,
		"Idempotency-Key": "weight-key-1",
	}
	first := doReq(t, f.r, http.MethodPost, "/weight", body, headers)
	require.Equal(t, http.StatusCreated, first.Code, first.Body.String())

	second := doReq(t, f.r, http.MethodPost, "/weight", body, headers)
	require.Equal(t, http.StatusCreated, second.Code)
	assert.Equal(t, first.Body.String(), second.Body.String(),
		"replay should return the original body byte-for-byte")
}

func TestIdempotency_SameKeyDifferentBodyReturns409(t *testing.T) {
	f := setupWithMiddleware(t)
	headers := map[string]string{
		"Authorization":   "Bearer " + mobileToken,
		"Idempotency-Key": "weight-key-2",
	}
	first := doReq(t, f.r, http.MethodPost, "/weight",
		`{"weight_kg":72.5,"logged_at":"2026-06-07T07:00:00Z"}`, headers)
	require.Equal(t, http.StatusCreated, first.Code, first.Body.String())

	conflict := doReq(t, f.r, http.MethodPost, "/weight",
		`{"weight_kg":73.0,"logged_at":"2026-06-07T07:00:00Z"}`, headers)
	require.Equal(t, http.StatusConflict, conflict.Code)
	assert.JSONEq(t, `{"error":"idempotency_key_conflict"}`, conflict.Body.String())
}

// ============================================================================
// Helpers
// ============================================================================

func mustCreate(t *testing.T, r *gin.Engine, body string) *bodyweight.Entry {
	t.Helper()
	rec := doReq(t, r, http.MethodPost, "/weight", body, nil)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var e bodyweight.Entry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &e))
	return &e
}
