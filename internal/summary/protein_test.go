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
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/bodyweight"
	"github.com/vinzenzs/kazper/internal/goals"
	"github.com/vinzenzs/kazper/internal/meals"
	"github.com/vinzenzs/kazper/internal/products"
	"github.com/vinzenzs/kazper/internal/store/storetest"
	"github.com/vinzenzs/kazper/internal/summary"
)

type proteinFix struct {
	r      *gin.Engine
	pRepo  *products.Repo
	bwRepo *bodyweight.Repo
}

// setupProtein wires the summary service with body-weight resolution enabled.
// Sibling to setupSummary; needs the body-weight repo wired for the resolver
// to find stored weights.
func setupProtein(t *testing.T, defaultTZ string) *proteinFix {
	t.Helper()
	pool := storetest.NewPool(t)
	pRepo := products.NewRepo(pool)
	mRepo := meals.NewRepo(pool)
	mSvc := meals.NewService(pool, mRepo, pRepo)
	gRepo := goals.NewRepo(pool)
	goRepo := goals.NewOverridesRepo(pool)
	resolver := goals.NewResolver(gRepo, goRepo, nil, nil)
	sSvc := summary.NewService(pool, mRepo, resolver)
	bwRepo := bodyweight.NewRepo(pool)
	sSvc.SetBodyWeightRepo(bwRepo)
	logBuf := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(logBuf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	r := gin.New()
	rg := r.Group("/")
	summary.NewHandlers(sSvc, defaultTZ, logger).Register(rg)
	meals.NewHandlers(mSvc).Register(rg)
	return &proteinFix{r: r, pRepo: pRepo, bwRepo: bwRepo}
}

// makeProteinProduct creates a product whose only nutriment is protein.
// kcal defaults to 0 — irrelevant for protein-distribution tests.
func makeProteinProduct(t *testing.T, repo *products.Repo, name string, proteinPer100g float64) uuid.UUID {
	t.Helper()
	p := &products.Product{
		Name:   name,
		Source: products.SourceManual,
		Nutriments: products.Nutriments{
			ProteinGPer100g: &proteinPer100g,
		},
	}
	require.NoError(t, repo.Insert(context.Background(), p))
	return p.ID
}

func logProteinMeal(t *testing.T, r *gin.Engine, pid uuid.UUID, ts string, qty float64) {
	t.Helper()
	body := fmt.Sprintf(`{"product_id":%q,"quantity_g":%g,"logged_at":%q}`, pid, qty, ts)
	req := httptest.NewRequest(http.MethodPost, "/meals", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
}

func insertWeight(t *testing.T, repo *bodyweight.Repo, at time.Time, kg float64) {
	t.Helper()
	require.NoError(t, repo.Insert(context.Background(), &bodyweight.Entry{
		LoggedAt: at,
		WeightKg: kg,
	}))
}

func doGetProtein(t *testing.T, r *gin.Engine, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// ============================================================================
// Happy path
// ============================================================================

func TestProtein_HappyPath_AllMealsAboveThreshold(t *testing.T) {
	f := setupProtein(t, "UTC")
	// 100 g protein per 100 g product → 1 g protein per gram quantity.
	pid := makeProteinProduct(t, f.pRepo, "iso-whey", 100.0)
	// Body weight 72.5 → threshold = 21.75 g/meal.
	insertWeight(t, f.bwRepo, time.Date(2026, 6, 9, 7, 0, 0, 0, time.UTC), 72.5)

	// Four meals: 28, 25, 30, 40 g protein — all ≥ 21.75 → mps_effective.
	for i, qty := range []float64{28, 25, 30, 40} {
		ts := time.Date(2026, 6, 9, 8+i*3, 0, 0, 0, time.UTC).Format(time.RFC3339)
		logProteinMeal(t, f.r, pid, ts, qty)
	}

	rec := doGetProtein(t, f.r,
		"/summary/protein-distribution?date=2026-06-09&tz=UTC&body_weight_kg=72.5")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var out summary.ProteinDistribution
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, "2026-06-09", out.Date)
	assert.Equal(t, "UTC", out.TZ)
	assert.Equal(t, 72.5, out.BodyWeightKg)
	assert.Equal(t, summary.BodyWeightSourceExplicit, out.BodyWeightSource)
	// 0.3 × 72.5 = 21.75 → Round1 (half-away-from-zero) → 21.8.
	assert.Equal(t, 21.8, out.MPSThresholdG)
	assert.Equal(t, 123.0, out.TotalProteinG)
	assert.Equal(t, 4, out.MealCount)
	assert.Equal(t, 4, out.MPSEffectiveMealCount)
	require.Len(t, out.Meals, 4)
	for _, m := range out.Meals {
		assert.True(t, m.MPSEffective, "meal at %s should be effective", m.LoggedAt)
	}
	assert.Nil(t, out.Meals[0].GapMinutesSincePrevious, "first meal gap is null")
	require.NotNil(t, out.Meals[1].GapMinutesSincePrevious)
	assert.Equal(t, 180, *out.Meals[1].GapMinutesSincePrevious, "3 hours = 180 min")
}

// ============================================================================
// Mixed effectiveness
// ============================================================================

func TestProtein_MixedEffectiveness(t *testing.T) {
	f := setupProtein(t, "UTC")
	pid := makeProteinProduct(t, f.pRepo, "iso-whey", 100.0)
	insertWeight(t, f.bwRepo, time.Date(2026, 6, 9, 7, 0, 0, 0, time.UTC), 72.5)

	// 28 (eff) / 18 (under) / 25 (eff) / 14 (under)
	for i, qty := range []float64{28, 18, 25, 14} {
		ts := time.Date(2026, 6, 9, 8+i*3, 0, 0, 0, time.UTC).Format(time.RFC3339)
		logProteinMeal(t, f.r, pid, ts, qty)
	}

	rec := doGetProtein(t, f.r,
		"/summary/protein-distribution?date=2026-06-09&tz=UTC&body_weight_kg=72.5")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var out summary.ProteinDistribution
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, 2, out.MPSEffectiveMealCount)
	assert.Equal(t, 85.0, out.TotalProteinG)
	require.Len(t, out.Meals, 4)
	assert.True(t, out.Meals[0].MPSEffective)
	assert.False(t, out.Meals[1].MPSEffective)
	assert.True(t, out.Meals[2].MPSEffective)
	assert.False(t, out.Meals[3].MPSEffective)
}

// ============================================================================
// Boundary at exactly the threshold (closed-low: 21.75 → effective)
// ============================================================================

func TestProtein_BoundaryAtExactThreshold(t *testing.T) {
	f := setupProtein(t, "UTC")
	pid := makeProteinProduct(t, f.pRepo, "iso-whey", 100.0)
	insertWeight(t, f.bwRepo, time.Date(2026, 6, 9, 7, 0, 0, 0, time.UTC), 72.5)

	logProteinMeal(t, f.r, pid, "2026-06-09T08:00:00Z", 21.75)

	rec := doGetProtein(t, f.r,
		"/summary/protein-distribution?date=2026-06-09&tz=UTC&body_weight_kg=72.5")
	require.Equal(t, http.StatusOK, rec.Code)
	var out summary.ProteinDistribution
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Len(t, out.Meals, 1)
	assert.True(t, out.Meals[0].MPSEffective, "exactly at threshold → effective (inclusive)")
}

// 21.7 with threshold 21.75 — the meal stores as 21.7g (DB rounds to 1dp),
// the threshold check uses unrounded 0.3 × 72.5 = 21.75, so 21.7 < 21.75 → false.
func TestProtein_JustBelowThreshold(t *testing.T) {
	f := setupProtein(t, "UTC")
	pid := makeProteinProduct(t, f.pRepo, "iso-whey", 100.0)
	insertWeight(t, f.bwRepo, time.Date(2026, 6, 9, 7, 0, 0, 0, time.UTC), 72.5)

	logProteinMeal(t, f.r, pid, "2026-06-09T08:00:00Z", 21.7)

	rec := doGetProtein(t, f.r,
		"/summary/protein-distribution?date=2026-06-09&tz=UTC&body_weight_kg=72.5")
	require.Equal(t, http.StatusOK, rec.Code)
	var out summary.ProteinDistribution
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Len(t, out.Meals, 1)
	assert.False(t, out.Meals[0].MPSEffective)
}

// ============================================================================
// One row per meal_entries row — no implicit grouping
// ============================================================================

func TestProtein_OneRowPerMealEntry_SameMinute(t *testing.T) {
	f := setupProtein(t, "UTC")
	pid := makeProteinProduct(t, f.pRepo, "iso-whey", 100.0)
	insertWeight(t, f.bwRepo, time.Date(2026, 6, 9, 7, 0, 0, 0, time.UTC), 72.5)

	// Three breakfast components logged within the same minute.
	logProteinMeal(t, f.r, pid, "2026-06-09T07:30:00Z", 25)
	logProteinMeal(t, f.r, pid, "2026-06-09T07:30:15Z", 15)
	logProteinMeal(t, f.r, pid, "2026-06-09T07:30:30Z", 10)

	rec := doGetProtein(t, f.r,
		"/summary/protein-distribution?date=2026-06-09&tz=UTC&body_weight_kg=72.5")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out summary.ProteinDistribution
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Len(t, out.Meals, 3, "three rows — no implicit grouping")
	assert.Nil(t, out.Meals[0].GapMinutesSincePrevious)
	require.NotNil(t, out.Meals[1].GapMinutesSincePrevious)
	assert.Equal(t, 0, *out.Meals[1].GapMinutesSincePrevious, "15 sec < 1 min → 0")
	require.NotNil(t, out.Meals[2].GapMinutesSincePrevious)
	assert.Equal(t, 0, *out.Meals[2].GapMinutesSincePrevious)
}

// ============================================================================
// logged_at_hour reflects requested tz
// ============================================================================

func TestProtein_LoggedAtHour_InTZ(t *testing.T) {
	f := setupProtein(t, "UTC")
	pid := makeProteinProduct(t, f.pRepo, "iso-whey", 100.0)
	// Body weight needed; place on June 10 so the in-window rolling avg finds it.
	insertWeight(t, f.bwRepo, time.Date(2026, 6, 10, 7, 0, 0, 0, time.UTC), 72.5)

	// 22:30Z on June 9 = 00:30 Berlin on June 10 (UTC+2 summer).
	logProteinMeal(t, f.r, pid, "2026-06-09T22:30:00Z", 25)

	rec := doGetProtein(t, f.r,
		"/summary/protein-distribution?date=2026-06-10&tz=Europe/Berlin&body_weight_kg=72.5")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out summary.ProteinDistribution
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Len(t, out.Meals, 1, "the 22:30Z meal must appear on local June 10")
	assert.Equal(t, 0, out.Meals[0].LoggedAtHour, "00:30 Berlin → hour 0")

	// And: querying June 9 in Berlin → the meal is NOT there.
	rec = doGetProtein(t, f.r,
		"/summary/protein-distribution?date=2026-06-09&tz=Europe/Berlin&body_weight_kg=72.5")
	require.Equal(t, http.StatusOK, rec.Code)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, 0, out.MealCount, "the 22:30Z meal moved to June 10 LOCAL")
}

// ============================================================================
// Empty day still resolves the threshold
// ============================================================================

func TestProtein_EmptyDay_WithWeightDataReturnsThreshold(t *testing.T) {
	f := setupProtein(t, "UTC")
	insertWeight(t, f.bwRepo, time.Date(2026, 6, 9, 7, 0, 0, 0, time.UTC), 72.5)

	// No meals.
	rec := doGetProtein(t, f.r,
		"/summary/protein-distribution?date=2026-06-09&tz=UTC")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var out summary.ProteinDistribution
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, 0, out.MealCount)
	assert.Equal(t, 0, out.MPSEffectiveMealCount)
	assert.Equal(t, 0.0, out.TotalProteinG)
	assert.Empty(t, out.Meals)
	assert.Equal(t, 21.8, out.MPSThresholdG, "threshold still computed from weight (rounded)")
	assert.Equal(t, summary.BodyWeightSourceRolling7dAvg, out.BodyWeightSource)
}

// ============================================================================
// Body-weight resolution paths
// ============================================================================

func TestProtein_Resolution_Rolling7dAvg(t *testing.T) {
	f := setupProtein(t, "UTC")
	pid := makeProteinProduct(t, f.pRepo, "iso-whey", 100.0)
	// Three entries in the 7d window ending at 2026-06-09:
	// June 4 (73 kg), June 6 (72 kg), June 8 (71 kg) → mean = 72.
	insertWeight(t, f.bwRepo, time.Date(2026, 6, 4, 7, 0, 0, 0, time.UTC), 73)
	insertWeight(t, f.bwRepo, time.Date(2026, 6, 6, 7, 0, 0, 0, time.UTC), 72)
	insertWeight(t, f.bwRepo, time.Date(2026, 6, 8, 7, 0, 0, 0, time.UTC), 71)

	logProteinMeal(t, f.r, pid, "2026-06-09T08:00:00Z", 30)

	rec := doGetProtein(t, f.r,
		"/summary/protein-distribution?date=2026-06-09&tz=UTC")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out summary.ProteinDistribution
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, 72.0, out.BodyWeightKg, "mean of 73+72+71")
	assert.Equal(t, summary.BodyWeightSourceRolling7dAvg, out.BodyWeightSource)
	assert.Equal(t, 21.6, out.MPSThresholdG, "0.3 × 72 = 21.6")
}

func TestProtein_Resolution_LastBeforeDate(t *testing.T) {
	f := setupProtein(t, "UTC")
	pid := makeProteinProduct(t, f.pRepo, "iso-whey", 100.0)
	// Only entry is 14 days before — outside the 7d rolling window.
	insertWeight(t, f.bwRepo, time.Date(2026, 5, 26, 7, 0, 0, 0, time.UTC), 75.0)

	logProteinMeal(t, f.r, pid, "2026-06-09T08:00:00Z", 30)

	rec := doGetProtein(t, f.r,
		"/summary/protein-distribution?date=2026-06-09&tz=UTC")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out summary.ProteinDistribution
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, 75.0, out.BodyWeightKg)
	assert.Equal(t, summary.BodyWeightSourceLastBeforeDate, out.BodyWeightSource)
}

func TestProtein_Resolution_ExplicitWinsOverStored(t *testing.T) {
	f := setupProtein(t, "UTC")
	pid := makeProteinProduct(t, f.pRepo, "iso-whey", 100.0)
	// Stored weight 72.5 would otherwise be used.
	insertWeight(t, f.bwRepo, time.Date(2026, 6, 8, 7, 0, 0, 0, time.UTC), 72.5)

	logProteinMeal(t, f.r, pid, "2026-06-09T08:00:00Z", 30)

	rec := doGetProtein(t, f.r,
		"/summary/protein-distribution?date=2026-06-09&tz=UTC&body_weight_kg=80")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out summary.ProteinDistribution
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, 80.0, out.BodyWeightKg)
	assert.Equal(t, summary.BodyWeightSourceExplicit, out.BodyWeightSource)
	assert.Equal(t, 24.0, out.MPSThresholdG, "0.3 × 80")
}

// ============================================================================
// Error codes
// ============================================================================

func TestProtein_Error_WeightDataMissing(t *testing.T) {
	f := setupProtein(t, "UTC")
	pid := makeProteinProduct(t, f.pRepo, "iso-whey", 100.0)
	logProteinMeal(t, f.r, pid, "2026-06-09T08:00:00Z", 30)

	// No weight entries, no override.
	rec := doGetProtein(t, f.r,
		"/summary/protein-distribution?date=2026-06-09&tz=UTC")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"weight_data_missing"}`, rec.Body.String())
}

func TestProtein_Error_BodyWeightKgInvalid(t *testing.T) {
	f := setupProtein(t, "UTC")
	for _, v := range []string{"0", "-1", "not-a-number"} {
		rec := doGetProtein(t, f.r,
			fmt.Sprintf("/summary/protein-distribution?date=2026-06-09&tz=UTC&body_weight_kg=%s", v))
		assert.Equal(t, http.StatusBadRequest, rec.Code, "value=%s", v)
		assert.JSONEq(t, `{"error":"body_weight_kg_invalid"}`, rec.Body.String(), "value=%s", v)
	}
}

func TestProtein_Error_DateRequired(t *testing.T) {
	f := setupProtein(t, "UTC")
	rec := doGetProtein(t, f.r,
		"/summary/protein-distribution?tz=UTC&body_weight_kg=72.5")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"date_required"}`, rec.Body.String())
}

func TestProtein_Error_DateInvalid(t *testing.T) {
	f := setupProtein(t, "UTC")
	rec := doGetProtein(t, f.r,
		"/summary/protein-distribution?date=2026-13-99&tz=UTC&body_weight_kg=72.5")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"date_invalid"}`, rec.Body.String())
}

func TestProtein_Error_TZInvalid(t *testing.T) {
	f := setupProtein(t, "UTC")
	rec := doGetProtein(t, f.r,
		"/summary/protein-distribution?date=2026-06-09&tz=Mars%2FOlympus&body_weight_kg=72.5")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"tz_invalid"}`, rec.Body.String())
}

// ============================================================================
// Rounding at response boundary
// ============================================================================

func TestProtein_Rounding_AtBoundary(t *testing.T) {
	f := setupProtein(t, "UTC")
	pid := makeProteinProduct(t, f.pRepo, "iso-whey", 100.0)
	// Body weight 72.5556 → threshold 21.7666... → rounds to 21.8.
	rec := doGetProtein(t, f.r,
		"/summary/protein-distribution?date=2026-06-09&tz=UTC&body_weight_kg=72.5556")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out summary.ProteinDistribution
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, 21.8, out.MPSThresholdG, "round to 1dp at the response boundary")
	assert.Equal(t, 72.6, out.BodyWeightKg, "body weight also rounded for presentation")

	// And: a meal whose unrounded protein crosses the unrounded threshold
	// gets mps_effective=true. Use a 22g entry → above the 21.7666... threshold.
	logProteinMeal(t, f.r, pid, "2026-06-09T08:00:00Z", 22)
	rec = doGetProtein(t, f.r,
		"/summary/protein-distribution?date=2026-06-09&tz=UTC&body_weight_kg=72.5556")
	require.Equal(t, http.StatusOK, rec.Code)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Len(t, out.Meals, 1)
	assert.True(t, out.Meals[0].MPSEffective)
	_ = pid // shut up "unused" if the build path differs
	_ = uuid.UUID{}
}
