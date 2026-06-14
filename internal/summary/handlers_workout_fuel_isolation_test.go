package summary_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/goals"
	"github.com/vinzenzs/kazper/internal/meals"
	"github.com/vinzenzs/kazper/internal/products"
	"github.com/vinzenzs/kazper/internal/store/storetest"
	"github.com/vinzenzs/kazper/internal/summary"
	"github.com/vinzenzs/kazper/internal/workoutfuel"
)

// TestDailySummary_ExcludesWorkoutFuelCarbs covers the unit-isolation rule
// from `add-workout-fuel`: 80g of gel carbs logged as workout-fuel MUST NOT
// contribute to the daily nutrition macro totals or to macro adherence.
// Only the 50g of meal carbs should appear.
func TestDailySummary_ExcludesWorkoutFuelCarbs(t *testing.T) {
	pool := storetest.NewPool(t)
	pRepo := products.NewRepo(pool)
	mRepo := meals.NewRepo(pool)
	mSvc := meals.NewService(pool, mRepo, pRepo)
	gRepo := goals.NewRepo(pool)
	goRepo := goals.NewOverridesRepo(pool)
	resolver := goals.NewResolver(gRepo, goRepo, nil, nil)
	sSvc := summary.NewService(pool, mRepo, resolver)
	fuelRepo := workoutfuel.NewRepo(pool)

	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelWarn}))
	r := gin.New()
	rg := r.Group("/")
	summary.NewHandlers(sSvc, "UTC", logger).Register(rg)
	meals.NewHandlers(mSvc).Register(rg)

	// 100 kcal/100g, 25g carbs/100g. 200g × 25/100 = 50g carbs.
	kcal := 100.0
	carbsPer100g := 25.0
	p := &products.Product{
		Name:   "Banana",
		Source: products.SourceManual,
		Nutriments: products.Nutriments{
			KcalPer100g:   &kcal,
			CarbsGPer100g: &carbsPer100g,
		},
	}
	require.NoError(t, pRepo.Insert(context.Background(), p))
	body := fmt.Sprintf(`{"product_id":%q,"quantity_g":200,"logged_at":"2026-06-07T08:00:00Z"}`, p.ID)
	req := httptest.NewRequest(http.MethodPost, "/meals", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	// 80g of carbs via workout-fuel on the same day — MUST be ignored by the
	// nutrition daily summary.
	carbs := 80.0
	require.NoError(t, fuelRepo.Insert(context.Background(), &workoutfuel.Entry{
		LoggedAt: time.Date(2026, 6, 7, 9, 0, 0, 0, time.UTC),
		Name:     "Gel",
		CarbsG:   &carbs,
	}))

	req = httptest.NewRequest(http.MethodGet,
		"/summary/daily?date=2026-06-07&tz=UTC", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var d summary.Daily
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &d))
	assert.Equal(t, 50.0, d.Totals.CarbsG,
		"daily nutrition totals must exclude workout-fuel carbs (unit-isolation rule)")
	assert.Equal(t, 200.0, d.Totals.Kcal,
		"workout-fuel has no kcal field; daily kcal must reflect only the meal")
}
