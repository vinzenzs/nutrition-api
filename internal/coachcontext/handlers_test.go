package coachcontext_test

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

	"github.com/vinzenzs/kazper/internal/coachcontext"
)

func init() { gin.SetMode(gin.TestMode) }

func handler(t *testing.T) *gin.Engine {
	t.Helper()
	f := setup(t)
	r := gin.New()
	coachcontext.NewHandlers(f.svc, "UTC", slog.New(slog.NewTextHandler(io.Discard, nil))).Register(r.Group("/"))
	return r
}

func TestHandler_Training_DefaultsToTodayAnd200(t *testing.T) {
	r := handler(t)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/context/training", nil)) // no date → today
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp coachcontext.TrainingContext
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, 14, resp.LookbackDays)
	// Empty-history invariants: arrays present, not null.
	assert.Contains(t, rec.Body.String(), `"recent_workouts":[]`)
	assert.Contains(t, rec.Body.String(), `"upcoming_workouts":[]`)
	assert.Contains(t, rec.Body.String(), `"phase":null`)
}

func TestHandler_Training_InvalidTZ(t *testing.T) {
	r := handler(t)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/context/training?tz=NowhereLand", nil))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "tz_invalid")
}

func TestHandler_Training_InvalidDate(t *testing.T) {
	r := handler(t)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/context/training?date=07/15/2026", nil))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "date_invalid")
}

func TestHandler_Recovery_200ShapeAndDays(t *testing.T) {
	r := handler(t)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/context/recovery?date=2026-07-15&days=999", nil))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp coachcontext.RecoveryContext
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "2026-07-15", resp.Date)
	assert.Equal(t, 90, resp.Days, "days clamped to max")
	assert.Contains(t, rec.Body.String(), `"recent":[]`)
	assert.Contains(t, rec.Body.String(), `"latest":null`)
}
