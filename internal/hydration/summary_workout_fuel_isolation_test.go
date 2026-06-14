package hydration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/hydration"
	"github.com/vinzenzs/kazper/internal/store/storetest"
	"github.com/vinzenzs/kazper/internal/workoutfuel"
)

// TestHydrationDailySummary_ExcludesWorkoutFuel covers the unit-isolation rule
// from `add-workout-fuel`: a 500 ml electrolyte drink logged as workout-fuel
// MUST NOT contribute to the daily hydration total, only the 300 ml plain-water
// hydration entry should.
func TestHydrationDailySummary_ExcludesWorkoutFuel(t *testing.T) {
	pool := storetest.NewPool(t)
	hRepo := hydration.NewRepo(pool)
	hSvc := hydration.NewService(hRepo)
	fuelRepo := workoutfuel.NewRepo(pool)
	logBuf := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(logBuf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	r := gin.New()
	rg := r.Group("/")
	hydration.NewHandlers(hSvc).Register(rg)
	hydration.NewSummaryHandlers(hSvc, "UTC", logger).Register(rg)

	// 300 ml plain water via hydration.
	require.NoError(t, hRepo.Insert(context.Background(), &hydration.Entry{
		LoggedAt:   time.Date(2026, 6, 7, 10, 0, 0, 0, time.UTC),
		QuantityMl: 300,
	}))

	// 500 ml electrolyte drink via workout-fuel on the SAME date.
	ml := 500.0
	sodium := 380.0
	require.NoError(t, fuelRepo.Insert(context.Background(), &workoutfuel.Entry{
		LoggedAt:   time.Date(2026, 6, 7, 8, 30, 0, 0, time.UTC),
		Name:       "Skratch",
		QuantityMl: &ml,
		SodiumMg:   &sodium,
	}))

	req := httptest.NewRequest(http.MethodGet,
		"/summary/hydration/daily?date=2026-06-07&tz=UTC", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var d hydration.Daily
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &d))
	assert.Equal(t, 300.0, d.TotalMl,
		"daily hydration total must exclude workout-fuel ml (unit-isolation rule)")
	assert.Equal(t, 1, d.EntryCount,
		"only the hydration_entries row contributes to entry_count")
}
