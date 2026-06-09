package dailycontext_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/nutrition-api/internal/auth"
	"github.com/vinzenzs/nutrition-api/internal/dailycontext"
)

func init() {
	gin.SetMode(gin.TestMode)
}

const (
	mobileToken = "mobile-token-aaaaaaaaaaaaaa"
	agentToken  = "agent-token-bbbbbbbbbbbbbbbb"
)

func setupHandler(t *testing.T, withAuth bool) *gin.Engine {
	t.Helper()
	f := setup(t)
	r := gin.New()
	rg := r.Group("/")
	if withAuth {
		rg.Use(auth.Middleware(auth.Config{MobileToken: mobileToken, AgentToken: agentToken}))
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dailycontext.NewHandlers(f.svc, "UTC", logger).Register(rg)
	return r
}

func TestHandler_MissingDate(t *testing.T) {
	r := setupHandler(t, false)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/context/daily", nil))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"date_invalid"}`, rec.Body.String())
}

func TestHandler_InvalidDateFormat(t *testing.T) {
	r := setupHandler(t, false)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/context/daily?date=07/24/2026", nil))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"date_invalid"}`, rec.Body.String())
}

func TestHandler_InvalidTZ(t *testing.T) {
	r := setupHandler(t, false)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/context/daily?date=2026-07-15&tz=NowhereLand", nil))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"tz_invalid"}`, rec.Body.String())
}

func TestHandler_HappyPath_EmptyDayReturns200WithFullShape(t *testing.T) {
	r := setupHandler(t, false)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/context/daily?date=2026-07-15", nil))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var resp dailycontext.DailyContext
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "2026-07-15", resp.Date)
	assert.Equal(t, "UTC", resp.TZ)
	// Empty-day invariants.
	require.NotNil(t, resp.Workouts)
	require.NotNil(t, resp.WorkoutFuel)
	assert.False(t, resp.GoalOverride.Present)
}

func TestHandler_AuthRequired(t *testing.T) {
	r := setupHandler(t, true)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/context/daily?date=2026-07-15", nil))
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestHandler_DefaultTZAppliedWhenOmitted(t *testing.T) {
	// Construct with a non-UTC default TZ via the handler ctor.
	f := setup(t)
	r := gin.New()
	rg := r.Group("/")
	dailycontext.NewHandlers(f.svc, "Europe/Berlin", slog.New(slog.NewTextHandler(io.Discard, nil))).Register(rg)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/context/daily?date=2026-07-15", nil))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp dailycontext.DailyContext
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "Europe/Berlin", resp.TZ)
}

func TestHandler_EmptyDayJSON_HasWorkoutsArrayNotNull(t *testing.T) {
	r := setupHandler(t, false)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/context/daily?date=2026-07-15", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	// Verify the raw JSON shape: arrays are `[]`, not `null`.
	assert.Contains(t, body, `"workouts":[]`)
	assert.Contains(t, body, `"workout_fuel":[]`)
	// Verify weight/phase are null (no entry), goal_override.present=false.
	assert.Contains(t, body, `"weight":null`)
	assert.Contains(t, body, `"phase":null`)
	assert.Contains(t, body, `"goal_override":{"present":false,"goals":null}`)
}
