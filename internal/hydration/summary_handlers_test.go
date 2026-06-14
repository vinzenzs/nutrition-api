package hydration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/hydration"
	"github.com/vinzenzs/kazper/internal/store/storetest"
)

type summaryFixture struct {
	r      *gin.Engine
	repo   *hydration.Repo
	logBuf *bytes.Buffer
}

func setupSummary(t *testing.T, defaultTZ string) *summaryFixture {
	t.Helper()
	pool := storetest.NewPool(t)
	repo := hydration.NewRepo(pool)
	svc := hydration.NewService(repo)
	logBuf := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(logBuf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	r := gin.New()
	rg := r.Group("/")
	// Also mount the regular handlers so we can seed data via the API.
	hydration.NewHandlers(svc).Register(rg)
	hydration.NewSummaryHandlers(svc, defaultTZ, logger).Register(rg)
	return &summaryFixture{r: r, repo: repo, logBuf: logBuf}
}

func doGet(t *testing.T, r *gin.Engine, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestDailySummary_SumsEntriesInWindow(t *testing.T) {
	f := setupSummary(t, "UTC")
	// Three entries on 2026-06-07 in Europe/Berlin: 500 + 750 + 1000 = 2250
	mustCreate(t, f.r, `{"quantity_ml":500,"logged_at":"2026-06-07T08:00:00+02:00"}`)
	mustCreate(t, f.r, `{"quantity_ml":750,"logged_at":"2026-06-07T13:00:00+02:00"}`)
	mustCreate(t, f.r, `{"quantity_ml":1000,"logged_at":"2026-06-07T20:00:00+02:00"}`)
	// One entry on the next day in Berlin — should NOT count.
	mustCreate(t, f.r, `{"quantity_ml":400,"logged_at":"2026-06-08T08:00:00+02:00"}`)

	rec := doGet(t, f.r, "/summary/hydration/daily?date=2026-06-07&tz=Europe/Berlin")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var d hydration.Daily
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &d))
	assert.Equal(t, "2026-06-07", d.Date)
	assert.Equal(t, "Europe/Berlin", d.TZ)
	assert.Equal(t, 2250.0, d.TotalMl)
	assert.Equal(t, 3, d.EntryCount)
	require.Len(t, d.Entries, 3)
}

func TestDailySummary_EmptyDay(t *testing.T) {
	f := setupSummary(t, "UTC")
	rec := doGet(t, f.r, "/summary/hydration/daily?date=2026-06-07&tz=UTC")
	require.Equal(t, http.StatusOK, rec.Code)
	var d hydration.Daily
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &d))
	assert.Equal(t, 0.0, d.TotalMl)
	assert.Equal(t, 0, d.EntryCount)
	assert.NotNil(t, d.Entries)
	assert.Len(t, d.Entries, 0)
}

func TestDailySummary_DefaultTZFallsBackAndWarns(t *testing.T) {
	f := setupSummary(t, "Europe/Berlin")
	mustCreate(t, f.r, `{"quantity_ml":500,"logged_at":"2026-06-07T01:00:00Z"}`)

	rec := doGet(t, f.r, "/summary/hydration/daily?date=2026-06-07")
	require.Equal(t, http.StatusOK, rec.Code)
	var d hydration.Daily
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &d))
	assert.Equal(t, "Europe/Berlin", d.TZ)
	assert.Contains(t, f.logBuf.String(), "default_tz=Europe/Berlin")
}

func TestDailySummary_InvalidTZReturns400(t *testing.T) {
	f := setupSummary(t, "UTC")
	rec := doGet(t, f.r, "/summary/hydration/daily?date=2026-06-07&tz=Mars%2FOlympus")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"tz_invalid"}`, rec.Body.String())
}

func TestDailySummary_InvalidDateReturns400(t *testing.T) {
	f := setupSummary(t, "UTC")
	rec := doGet(t, f.r, "/summary/hydration/daily?date=2026-13-99&tz=UTC")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"date_invalid"}`, rec.Body.String())
}

func TestDailySummary_MissingDateReturns400(t *testing.T) {
	f := setupSummary(t, "UTC")
	rec := doGet(t, f.r, "/summary/hydration/daily?tz=UTC")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"date_invalid"}`, rec.Body.String())
}

// TestDailySummary_TotalRoundedTo1dp covers the rounding rule: NUMERIC(10,1)
// in the column means storage already rounds at 0.1 granularity, but the
// service-side numfmt.Round1 guards against float artefacts during summation.
// We seed three values whose sum is 0.3 (in ml, 0.1 + 0.1 + 0.1) — the
// straightforward float sum would be 0.30000000000000004, but the response
// must surface 0.3.
func TestDailySummary_TotalRoundedTo1dp(t *testing.T) {
	f := setupSummary(t, "UTC")
	pool := storetest.NewPool // ensures import; unused warning guard
	_ = pool

	// Insert three entries with quantity that sums with float noise; using
	// numbers below NUMERIC(10,1) precision is fine — column stores as 0.1.
	mustCreate(t, f.r, `{"quantity_ml":0.1,"logged_at":"2026-06-07T08:00:00Z"}`)
	mustCreate(t, f.r, `{"quantity_ml":0.1,"logged_at":"2026-06-07T09:00:00Z"}`)
	mustCreate(t, f.r, `{"quantity_ml":0.1,"logged_at":"2026-06-07T10:00:00Z"}`)

	rec := doGet(t, f.r, "/summary/hydration/daily?date=2026-06-07&tz=UTC")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var d hydration.Daily
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &d))
	assert.Equal(t, 0.3, d.TotalMl, "0.1 + 0.1 + 0.1 should round cleanly to 0.3")
}

// silence unused import guards.
var _ = context.Background
