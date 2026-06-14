package hydration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/auth"
	"github.com/vinzenzs/kazper/internal/hydration"
	"github.com/vinzenzs/kazper/internal/idempotency"
	"github.com/vinzenzs/kazper/internal/store/storetest"
)

func init() {
	gin.SetMode(gin.TestMode)
}

const (
	mobileToken = "mobile-token-aaaaaaaaaaaaaa"
	agentToken  = "agent-token-bbbbbbbbbbbbbbbb"
)

type fixture struct {
	r *gin.Engine
}

// setup mounts the hydration handlers directly without auth/idem so tests can
// stay focused on per-endpoint behaviour. Idempotency is exercised by setupWithMiddleware.
func setup(t *testing.T) *fixture {
	t.Helper()
	pool := storetest.NewPool(t)
	repo := hydration.NewRepo(pool)
	svc := hydration.NewService(repo)
	r := gin.New()
	rg := r.Group("/")
	hydration.NewHandlers(svc).Register(rg)
	return &fixture{r: r}
}

// setupWithMiddleware mounts the full auth + idempotency middleware in front
// of the hydration handlers, mirroring how the prod router wires them.
func setupWithMiddleware(t *testing.T) *fixture {
	t.Helper()
	pool := storetest.NewPool(t)
	repo := hydration.NewRepo(pool)
	svc := hydration.NewService(repo)
	idemRepo := idempotency.NewRepo(pool)

	r := gin.New()
	r.Use(auth.Middleware(auth.Config{MobileToken: mobileToken, AgentToken: agentToken}))
	r.Use(idempotency.Middleware(idemRepo, time.Hour))
	rg := r.Group("/")
	hydration.NewHandlers(svc).Register(rg)
	return &fixture{r: r}
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

// ============================================================================
// POST /hydration
// ============================================================================

func TestCreate_HappyPath(t *testing.T) {
	f := setup(t)
	rec := doRequest(t, f.r, http.MethodPost, "/hydration",
		`{"quantity_ml":500,"logged_at":"2026-06-07T08:00:00Z"}`, nil)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var e hydration.Entry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &e))
	assert.Equal(t, 500.0, e.QuantityMl)
	assert.Equal(t, time.Date(2026, 6, 7, 8, 0, 0, 0, time.UTC), e.LoggedAt.UTC())
	assert.Nil(t, e.Note)
	assert.NotEmpty(t, e.ID)
}

func TestCreate_WithNote(t *testing.T) {
	f := setup(t)
	rec := doRequest(t, f.r, http.MethodPost, "/hydration",
		`{"quantity_ml":250,"logged_at":"2026-06-07T08:00:00Z","note":"iced coffee"}`, nil)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var e hydration.Entry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &e))
	require.NotNil(t, e.Note)
	assert.Equal(t, "iced coffee", *e.Note)
}

func TestCreate_MissingQuantityRejected(t *testing.T) {
	f := setup(t)
	rec := doRequest(t, f.r, http.MethodPost, "/hydration",
		`{"logged_at":"2026-06-07T08:00:00Z"}`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"quantity_ml_invalid"}`, rec.Body.String())
}

func TestCreate_ZeroQuantityRejected(t *testing.T) {
	f := setup(t)
	rec := doRequest(t, f.r, http.MethodPost, "/hydration",
		`{"quantity_ml":0,"logged_at":"2026-06-07T08:00:00Z"}`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"quantity_ml_invalid"}`, rec.Body.String())
}

func TestCreate_NegativeQuantityRejected(t *testing.T) {
	f := setup(t)
	rec := doRequest(t, f.r, http.MethodPost, "/hydration",
		`{"quantity_ml":-100,"logged_at":"2026-06-07T08:00:00Z"}`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"quantity_ml_invalid"}`, rec.Body.String())
}

func TestCreate_NoteTooLongRejected(t *testing.T) {
	f := setup(t)
	longNote := strings.Repeat("a", 501)
	body := fmt.Sprintf(`{"quantity_ml":500,"logged_at":"2026-06-07T08:00:00Z","note":%q}`, longNote)
	rec := doRequest(t, f.r, http.MethodPost, "/hydration", body, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"note_too_long"}`, rec.Body.String())
}

func TestCreate_LoggedAtFarFutureRejected(t *testing.T) {
	f := setup(t)
	farFuture := time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339)
	body := fmt.Sprintf(`{"quantity_ml":500,"logged_at":%q}`, farFuture)
	rec := doRequest(t, f.r, http.MethodPost, "/hydration", body, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"logged_at_too_far_future"}`, rec.Body.String())
}

func TestCreate_InvalidLoggedAtRejected(t *testing.T) {
	f := setup(t)
	rec := doRequest(t, f.r, http.MethodPost, "/hydration",
		`{"quantity_ml":500,"logged_at":"not-a-timestamp"}`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"logged_at_invalid"}`, rec.Body.String())
}

// ============================================================================
// GET /hydration?from=…&to=…
// ============================================================================

func TestList_WindowFiltersEntries(t *testing.T) {
	f := setup(t)
	// Log three entries.
	mustCreate(t, f.r, `{"quantity_ml":500,"logged_at":"2026-06-07T05:00:00Z"}`)
	mustCreate(t, f.r, `{"quantity_ml":300,"logged_at":"2026-06-07T12:00:00Z"}`)
	mustCreate(t, f.r, `{"quantity_ml":700,"logged_at":"2026-06-08T05:00:00Z"}`) // outside window

	rec := doRequest(t, f.r, http.MethodGet,
		"/hydration?from=2026-06-07T00:00:00Z&to=2026-06-08T00:00:00Z", "", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body struct {
		Entries []hydration.Entry `json:"entries"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Entries, 2)
	// Ordering: 05:00 then 12:00.
	assert.Equal(t, 500.0, body.Entries[0].QuantityMl)
	assert.Equal(t, 300.0, body.Entries[1].QuantityMl)
}

func TestList_MissingWindowRejected(t *testing.T) {
	f := setup(t)
	rec := doRequest(t, f.r, http.MethodGet, "/hydration", "", nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"window_required"}`, rec.Body.String())
}

func TestList_InvertedWindowRejected(t *testing.T) {
	f := setup(t)
	rec := doRequest(t, f.r, http.MethodGet,
		"/hydration?from=2026-06-08T00:00:00Z&to=2026-06-07T00:00:00Z", "", nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"window_invalid"}`, rec.Body.String())
}

func TestList_InvalidWindowRejected(t *testing.T) {
	f := setup(t)
	rec := doRequest(t, f.r, http.MethodGet,
		"/hydration?from=garbage&to=2026-06-07T00:00:00Z", "", nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"window_invalid"}`, rec.Body.String())
}

func TestList_RangeTooLargeRejected(t *testing.T) {
	f := setup(t)
	rec := doRequest(t, f.r, http.MethodGet,
		"/hydration?from=2026-01-01T00:00:00Z&to=2026-12-31T00:00:00Z", "", nil)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "range_too_large", body["error"])
	assert.EqualValues(t, 92, body["max_days"])
}

// ============================================================================
// PATCH /hydration/{id}
// ============================================================================

func TestPatch_UpdatesOnlySuppliedFields(t *testing.T) {
	f := setup(t)
	created := mustCreate(t, f.r, `{"quantity_ml":500,"logged_at":"2026-06-07T08:00:00Z","note":"water"}`)

	rec := doRequest(t, f.r, http.MethodPatch, "/hydration/"+created.ID.String(),
		`{"quantity_ml":250}`, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var e hydration.Entry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &e))
	assert.Equal(t, 250.0, e.QuantityMl)
	// Note unchanged.
	require.NotNil(t, e.Note)
	assert.Equal(t, "water", *e.Note)
}

func TestPatch_InvalidQuantityRejected(t *testing.T) {
	f := setup(t)
	created := mustCreate(t, f.r, `{"quantity_ml":500,"logged_at":"2026-06-07T08:00:00Z"}`)
	rec := doRequest(t, f.r, http.MethodPatch, "/hydration/"+created.ID.String(),
		`{"quantity_ml":-1}`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"quantity_ml_invalid"}`, rec.Body.String())
}

func TestPatch_UnknownIdReturns404(t *testing.T) {
	f := setup(t)
	rec := doRequest(t, f.r, http.MethodPatch,
		"/hydration/00000000-0000-0000-0000-000000000000",
		`{"quantity_ml":250}`, nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.JSONEq(t, `{"error":"hydration_not_found"}`, rec.Body.String())
}

// ============================================================================
// DELETE /hydration/{id}
// ============================================================================

func TestDelete_HappyPath(t *testing.T) {
	f := setup(t)
	created := mustCreate(t, f.r, `{"quantity_ml":500,"logged_at":"2026-06-07T08:00:00Z"}`)

	rec := doRequest(t, f.r, http.MethodDelete, "/hydration/"+created.ID.String(), "", nil)
	assert.Equal(t, http.StatusNoContent, rec.Code)

	// Re-list to confirm it's gone.
	listRec := doRequest(t, f.r, http.MethodGet,
		"/hydration?from=2026-06-07T00:00:00Z&to=2026-06-08T00:00:00Z", "", nil)
	require.Equal(t, http.StatusOK, listRec.Code)
	var body struct {
		Entries []hydration.Entry `json:"entries"`
	}
	require.NoError(t, json.Unmarshal(listRec.Body.Bytes(), &body))
	assert.Len(t, body.Entries, 0)
}

func TestDelete_UnknownIdReturns404(t *testing.T) {
	f := setup(t)
	rec := doRequest(t, f.r, http.MethodDelete,
		"/hydration/00000000-0000-0000-0000-000000000000", "", nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.JSONEq(t, `{"error":"hydration_not_found"}`, rec.Body.String())
}

// ============================================================================
// Idempotency middleware integration
// ============================================================================

func TestIdempotency_SameKeyAndBodyReplays(t *testing.T) {
	f := setupWithMiddleware(t)
	body := `{"quantity_ml":500,"logged_at":"2026-06-07T08:00:00Z"}`
	headers := map[string]string{
		"Authorization":   "Bearer " + mobileToken,
		"Idempotency-Key": "key-1",
	}
	first := doRequest(t, f.r, http.MethodPost, "/hydration", body, headers)
	require.Equal(t, http.StatusCreated, first.Code, first.Body.String())

	second := doRequest(t, f.r, http.MethodPost, "/hydration", body, headers)
	require.Equal(t, http.StatusCreated, second.Code)
	assert.Equal(t, first.Body.String(), second.Body.String(),
		"replay should return the original body byte-for-byte")
}

func TestIdempotency_SameKeyDifferentBodyReturns409(t *testing.T) {
	f := setupWithMiddleware(t)
	headers := map[string]string{
		"Authorization":   "Bearer " + mobileToken,
		"Idempotency-Key": "key-2",
	}
	first := doRequest(t, f.r, http.MethodPost, "/hydration",
		`{"quantity_ml":500,"logged_at":"2026-06-07T08:00:00Z"}`, headers)
	require.Equal(t, http.StatusCreated, first.Code, first.Body.String())

	conflict := doRequest(t, f.r, http.MethodPost, "/hydration",
		`{"quantity_ml":250,"logged_at":"2026-06-07T08:00:00Z"}`, headers)
	require.Equal(t, http.StatusConflict, conflict.Code)
	assert.JSONEq(t, `{"error":"idempotency_key_conflict"}`, conflict.Body.String())
}

// ============================================================================
// Helpers
// ============================================================================

func mustCreate(t *testing.T, r *gin.Engine, body string) *hydration.Entry {
	t.Helper()
	rec := doRequest(t, r, http.MethodPost, "/hydration", body, nil)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var e hydration.Entry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &e))
	return &e
}
