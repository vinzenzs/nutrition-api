package summary_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/nutrition-api/internal/goals"
	"github.com/vinzenzs/nutrition-api/internal/meals"
	"github.com/vinzenzs/nutrition-api/internal/products"
	"github.com/vinzenzs/nutrition-api/internal/store/storetest"
	"github.com/vinzenzs/nutrition-api/internal/summary"
)

func init() {
	gin.SetMode(gin.TestMode)
}

type fix struct {
	r             *gin.Engine
	pRepo         *products.Repo
	mealsSvc      *meals.Service
	goalsRepo     *goals.Repo
	overridesRepo *goals.OverridesRepo
	logBuf        *bytes.Buffer
}

func setupSummary(t *testing.T, defaultTZ string) *fix {
	t.Helper()
	pool := storetest.NewPool(t)
	pRepo := products.NewRepo(pool)
	mRepo := meals.NewRepo(pool)
	mSvc := meals.NewService(pool, mRepo, pRepo)
	gRepo := goals.NewRepo(pool)
	goRepo := goals.NewOverridesRepo(pool)
	resolver := goals.NewResolver(gRepo, goRepo)
	sSvc := summary.NewService(pool, mRepo, resolver)
	logBuf := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(logBuf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	r := gin.New()
	rg := r.Group("/")
	summary.NewHandlers(sSvc, defaultTZ, logger).Register(rg)
	// Also mount meals to seed data via the API for realism.
	meals.NewHandlers(mSvc).Register(rg)
	return &fix{r: r, pRepo: pRepo, mealsSvc: mSvc, goalsRepo: gRepo, overridesRepo: goRepo, logBuf: logBuf}
}

func makeProductForSummary(t *testing.T, repo *products.Repo, name string, kcal float64) uuid.UUID {
	t.Helper()
	k := kcal
	p := &products.Product{
		Name:       name,
		Source:     products.SourceManual,
		Nutriments: products.Nutriments{KcalPer100g: &k},
	}
	require.NoError(t, repo.Insert(context.Background(), p))
	return p.ID
}

func logMeal(t *testing.T, r *gin.Engine, pid uuid.UUID, ts string, qty float64) {
	t.Helper()
	body := fmt.Sprintf(`{"product_id":%q,"quantity_g":%g,"logged_at":%q}`, pid, qty, ts)
	req := httptest.NewRequest(http.MethodPost, "/meals", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
}

func doGet(t *testing.T, r *gin.Engine, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// ============================================================================
// Daily
// ============================================================================

func TestDaily_TotalsFromEffectiveNutriments(t *testing.T) {
	f := setupSummary(t, "UTC")
	pid := makeProductForSummary(t, f.pRepo, "P1", 100.0) // 100 kcal/100g

	// Two meals on 2026-06-06 in Europe/Berlin: 200g + 50g = 250 kcal
	logMeal(t, f.r, pid, "2026-06-06T08:00:00+02:00", 200)
	logMeal(t, f.r, pid, "2026-06-06T20:00:00+02:00", 50)

	rec := doGet(t, f.r, "/summary/daily?date=2026-06-06&tz=Europe/Berlin")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out summary.Daily
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, "2026-06-06", out.Date)
	assert.Equal(t, "Europe/Berlin", out.TZ)
	assert.InDelta(t, 250.0, out.Totals.Kcal, 0.001)
	assert.Len(t, out.Entries, 2)
}

func TestDaily_DefaultTZFallsBackAndWarns(t *testing.T) {
	f := setupSummary(t, "Europe/Berlin")
	pid := makeProductForSummary(t, f.pRepo, "P1", 100.0)
	logMeal(t, f.r, pid, "2026-06-06T01:00:00Z", 100) // 03:00 Berlin = 2026-06-06

	rec := doGet(t, f.r, "/summary/daily?date=2026-06-06")
	require.Equal(t, http.StatusOK, rec.Code)
	var out summary.Daily
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, "Europe/Berlin", out.TZ)
	assert.Contains(t, f.logBuf.String(), "default_tz=Europe/Berlin")
}

func TestDaily_InvalidTZReturns400(t *testing.T) {
	f := setupSummary(t, "UTC")
	rec := doGet(t, f.r, "/summary/daily?date=2026-06-06&tz=Mars%2FOlympus")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"tz_invalid"}`, rec.Body.String())
}

func TestDaily_InvalidDateReturns400(t *testing.T) {
	f := setupSummary(t, "UTC")
	rec := doGet(t, f.r, "/summary/daily?date=2026-13-99&tz=UTC")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"date_invalid"}`, rec.Body.String())
}

func TestDaily_EmptyDayReturnsZeroTotals(t *testing.T) {
	f := setupSummary(t, "UTC")
	rec := doGet(t, f.r, "/summary/daily?date=2026-06-06&tz=UTC")
	require.Equal(t, http.StatusOK, rec.Code)
	var out summary.Daily
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.InDelta(t, 0.0, out.Totals.Kcal, 0.001)
	assert.NotNil(t, out.Entries)
	assert.Len(t, out.Entries, 0)
}

// TestDaily_DoesNotLeakHydrationFields guards the unit-isolation rule from the
// hydration capability: the nutrition daily summary must not carry hydration
// keys regardless of whether hydration entries exist (they live in a separate
// table and a separate endpoint).
func TestDaily_DoesNotLeakHydrationFields(t *testing.T) {
	f := setupSummary(t, "UTC")
	pid := makeProductForSummary(t, f.pRepo, "P1", 100.0)
	logMeal(t, f.r, pid, "2026-06-06T08:00:00Z", 150)
	rec := doGet(t, f.r, "/summary/daily?date=2026-06-06&tz=UTC")
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.NotContains(t, body, "total_ml", "nutrition summary must not include hydration fields")
	assert.NotContains(t, body, "hydration")
}

// TestDaily_DoesNotLeakBodyWeightFields guards the same unit-isolation rule for
// the body-weight capability (add-weight-log).
func TestDaily_DoesNotLeakBodyWeightFields(t *testing.T) {
	f := setupSummary(t, "UTC")
	pid := makeProductForSummary(t, f.pRepo, "P1", 100.0)
	logMeal(t, f.r, pid, "2026-06-06T08:00:00Z", 150)
	rec := doGet(t, f.r, "/summary/daily?date=2026-06-06&tz=UTC")
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.NotContains(t, body, "weight_kg", "nutrition summary must not include body-weight fields")
	assert.NotContains(t, body, "body_fat_pct")
}

// ============================================================================
// Daily — goal_source resolution (add-date-varying-goals)
// ============================================================================

func TestDaily_GoalSourceDefault(t *testing.T) {
	f := setupSummary(t, "UTC")
	pid := makeProductForSummary(t, f.pRepo, "x", 100.0)
	logMealAt(t, f.r, pid, "2026-06-06T08:00:00Z", 100, "")

	require.NoError(t, f.goalsRepo.Upsert(context.Background(), &goals.Goals{
		Kcal: &goals.Range{Min: ptrFlt(2090), Max: ptrFlt(2310)},
	}))

	rec := doGet(t, f.r, "/summary/daily?date=2026-06-06&tz=UTC")
	require.Equal(t, http.StatusOK, rec.Code)
	var out summary.Daily
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, "default", out.GoalSource)
}

func TestDaily_GoalSourceOverride(t *testing.T) {
	f := setupSummary(t, "UTC")
	pid := makeProductForSummary(t, f.pRepo, "x", 100.0)
	logMealAt(t, f.r, pid, "2026-06-06T08:00:00Z", 100, "")

	// Default exists.
	require.NoError(t, f.goalsRepo.Upsert(context.Background(), &goals.Goals{
		Kcal: &goals.Range{Min: ptrFlt(2090), Max: ptrFlt(2310)},
	}))
	// Override for the queried date — different bounds.
	require.NoError(t, f.overridesRepo.Upsert(context.Background(),
		time.Date(2026, 6, 6, 0, 0, 0, 0, time.UTC),
		&goals.Goals{Kcal: &goals.Range{Min: ptrFlt(2280), Max: ptrFlt(2520)}}))

	rec := doGet(t, f.r, "/summary/daily?date=2026-06-06&tz=UTC")
	require.Equal(t, http.StatusOK, rec.Code)
	var out summary.Daily
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, "override", out.GoalSource)
	// Adherence reflects override bounds (2280..2520), not default (2090..2310).
	require.NotNil(t, out.Adherence)
	entry, ok := out.Adherence["kcal"]
	require.True(t, ok)
	require.NotNil(t, entry.Target.Min)
	assert.InDelta(t, 2280, *entry.Target.Min, 0.001)
}

func TestDaily_GoalSourceNoneWhenNoGoals(t *testing.T) {
	f := setupSummary(t, "UTC")
	rec := doGet(t, f.r, "/summary/daily?date=2026-06-06&tz=UTC")
	require.Equal(t, http.StatusOK, rec.Code)
	var out summary.Daily
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, "none", out.GoalSource)
	assert.Nil(t, out.Adherence)
}

func TestDaily_GoalSourceOmittedWithMealTypeFilter(t *testing.T) {
	f := setupSummary(t, "UTC")
	pid := makeProductForSummary(t, f.pRepo, "x", 100.0)
	logMealAt(t, f.r, pid, "2026-06-06T08:00:00Z", 100, "breakfast")

	require.NoError(t, f.goalsRepo.Upsert(context.Background(), &goals.Goals{
		Kcal: &goals.Range{Min: ptrFlt(2090), Max: ptrFlt(2310)},
	}))

	rec := doGet(t, f.r, "/summary/daily?date=2026-06-06&tz=UTC&meal_type=breakfast")
	require.Equal(t, http.StatusOK, rec.Code)
	// goal_source uses omitempty; meal_type filter suppresses adherence and the source field too.
	assert.NotContains(t, rec.Body.String(), `"goal_source"`)
	assert.NotContains(t, rec.Body.String(), `"adherence"`)
}

func TestRange_GoalSourceSwitchesDayByDay(t *testing.T) {
	f := setupSummary(t, "UTC")
	pid := makeProductForSummary(t, f.pRepo, "x", 100.0)
	// Use safely-past dates so logged_at validation accepts them.
	logMealAt(t, f.r, pid, "2026-06-04T08:00:00Z", 100, "")
	logMealAt(t, f.r, pid, "2026-06-05T08:00:00Z", 100, "")
	logMealAt(t, f.r, pid, "2026-06-06T08:00:00Z", 100, "")

	require.NoError(t, f.goalsRepo.Upsert(context.Background(), &goals.Goals{
		Kcal: &goals.Range{Min: ptrFlt(2090), Max: ptrFlt(2310)},
	}))
	require.NoError(t, f.overridesRepo.Upsert(context.Background(),
		time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC),
		&goals.Goals{Kcal: &goals.Range{Min: ptrFlt(2280), Max: ptrFlt(2520)}}))

	rec := doGet(t, f.r, "/summary/range?from=2026-06-04&to=2026-06-06&tz=UTC")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out summary.Range
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Len(t, out.Days, 3)
	assert.Equal(t, "default", out.Days[0].GoalSource)
	assert.Equal(t, "override", out.Days[1].GoalSource)
	assert.Equal(t, "default", out.Days[2].GoalSource)
	// June 5's adherence row uses override bounds.
	require.NotNil(t, out.Days[1].Adherence)
	entry, ok := out.Days[1].Adherence["kcal"]
	require.True(t, ok)
	require.NotNil(t, entry.Target.Min)
	assert.InDelta(t, 2280, *entry.Target.Min, 0.001)
}

// ============================================================================
// Range
// ============================================================================

func TestRange_PerDayBreakdownIncludesEmptyDays(t *testing.T) {
	f := setupSummary(t, "UTC")
	pid := makeProductForSummary(t, f.pRepo, "P1", 100.0)
	// One meal on June 2 in Berlin
	logMeal(t, f.r, pid, "2026-06-02T10:00:00+02:00", 100) // 100 kcal

	rec := doGet(t, f.r, "/summary/range?from=2026-06-01&to=2026-06-07&tz=Europe/Berlin")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out summary.Range
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, "Europe/Berlin", out.TZ)
	require.Len(t, out.Days, 7, "should have 7 days inclusive")
	dayMap := map[string]float64{}
	for _, d := range out.Days {
		dayMap[d.Date] = d.Totals.Kcal
	}
	assert.InDelta(t, 0.0, dayMap["2026-06-01"], 0.001)
	assert.InDelta(t, 100.0, dayMap["2026-06-02"], 0.001)
	assert.InDelta(t, 0.0, dayMap["2026-06-07"], 0.001)
}

func TestRange_ExceedingMaxRangeReturns400(t *testing.T) {
	f := setupSummary(t, "UTC")
	rec := doGet(t, f.r, "/summary/range?from=2026-01-01&to=2026-12-31&tz=UTC")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "range_too_large", body["error"])
	assert.EqualValues(t, 92, body["max_days"])
}

func TestRange_InvertedReturns400(t *testing.T) {
	f := setupSummary(t, "UTC")
	rec := doGet(t, f.r, "/summary/range?from=2026-06-07&to=2026-06-01&tz=UTC")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"range_invalid"}`, rec.Body.String())
}

func TestRange_InvalidTZReturns400(t *testing.T) {
	f := setupSummary(t, "UTC")
	rec := doGet(t, f.r, "/summary/range?from=2026-06-01&to=2026-06-07&tz=Not%2FAReal")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"tz_invalid"}`, rec.Body.String())
}

// silence import warning when these are unused as we iterate.
var _ = io.Discard

// ============================================================================
// Section 7: micros / adherence / meal_type filter (daily-use-essentials)
// ============================================================================

func ptrFlt(v float64) *float64 { return &v }

// makeProductWithNutriments inserts a product carrying arbitrary nutriments.
func makeProductWithNutriments(t *testing.T, repo *products.Repo, name string, n products.Nutriments) uuid.UUID {
	t.Helper()
	p := &products.Product{Name: name, Source: products.SourceManual, Nutriments: n}
	require.NoError(t, repo.Insert(context.Background(), p))
	return p.ID
}

// logMealAt logs a meal entry of given qty at the given ts (RFC3339), with an
// optional meal_type.
func logMealAt(t *testing.T, r *gin.Engine, pid uuid.UUID, ts string, qty float64, mealType string) {
	t.Helper()
	var body string
	if mealType == "" {
		body = fmt.Sprintf(`{"product_id":%q,"quantity_g":%g,"logged_at":%q}`, pid, qty, ts)
	} else {
		body = fmt.Sprintf(`{"product_id":%q,"quantity_g":%g,"logged_at":%q,"meal_type":%q}`, pid, qty, ts, mealType)
	}
	req := httptest.NewRequest(http.MethodPost, "/meals", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
}

func TestDaily_MicrosInTotalsWhenContributorsExist(t *testing.T) {
	f := setupSummary(t, "UTC")
	pid := makeProductWithNutriments(t, f.pRepo, "fortified", products.Nutriments{
		KcalPer100g: ptrFlt(100),
		IronMgPer100g: ptrFlt(5),
		PotassiumMgPer100g: ptrFlt(200),
	})
	logMealAt(t, f.r, pid, "2026-06-06T08:00:00Z", 200, "")

	rec := doGet(t, f.r, "/summary/daily?date=2026-06-06&tz=UTC")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out summary.Daily
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))

	// Macros computed normally.
	assert.InDelta(t, 200.0, out.Totals.Kcal, 0.001)
	// Micros with contributors are present; expected: 5 mg/100g * 200g/100 = 10 mg.
	require.NotNil(t, out.Totals.IronMg)
	assert.InDelta(t, 10.0, *out.Totals.IronMg, 0.001)
	require.NotNil(t, out.Totals.PotassiumMg)
	assert.InDelta(t, 400.0, *out.Totals.PotassiumMg, 0.001)
	// No-fake-zero: micros with no contributor are omitted.
	assert.Nil(t, out.Totals.CalciumMg)
	assert.Nil(t, out.Totals.VitaminB12Mcg)
}

func TestDaily_NoFakeZeroForMicrosOnEmptyDay(t *testing.T) {
	f := setupSummary(t, "UTC")
	rec := doGet(t, f.r, "/summary/daily?date=2026-06-06&tz=UTC")
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	// Macros zero is OK; micros must be absent from the JSON.
	assert.Contains(t, body, `"kcal":0`)
	assert.NotContains(t, body, `"iron_mg"`)
	assert.NotContains(t, body, `"potassium_mg"`)
	// Adherence omitted (no goals set).
	assert.NotContains(t, body, `"adherence"`)
}

func TestDaily_MealTypeFilterScopesAndOmitsAdherence(t *testing.T) {
	f := setupSummary(t, "UTC")
	pid := makeProductForSummary(t, f.pRepo, "X", 100.0)
	logMealAt(t, f.r, pid, "2026-06-06T08:00:00Z", 200, "breakfast") // 200 kcal
	logMealAt(t, f.r, pid, "2026-06-06T13:00:00Z", 300, "lunch")    // 300 kcal

	// Even with goals set, meal_type filter must omit adherence.
	require.NoError(t, f.goalsRepo.Upsert(context.Background(), &goals.Goals{Kcal: &goals.Range{Min: ptrFlt(2090), Max: ptrFlt(2310)}}))

	rec := doGet(t, f.r, "/summary/daily?date=2026-06-06&tz=UTC&meal_type=breakfast")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out summary.Daily
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))

	assert.InDelta(t, 200.0, out.Totals.Kcal, 0.001) // breakfast only
	require.NotNil(t, out.MealType)
	assert.Equal(t, "breakfast", *out.MealType)
	assert.Nil(t, out.Adherence, "filter mode must omit adherence")
	assert.Len(t, out.Entries, 1)
}

func TestDaily_MealTypeInvalidReturns400(t *testing.T) {
	f := setupSummary(t, "UTC")
	rec := doGet(t, f.r, "/summary/daily?date=2026-06-06&tz=UTC&meal_type=brunch")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"meal_type_invalid"}`, rec.Body.String())
}

func TestDaily_AdherenceKcalOnUnderOver(t *testing.T) {
	cases := []struct {
		name    string
		qty     float64 // qty * 1 kcal/g = total kcal
		want    string
	}{
		{"on", 2150, "on"},  // -2.3% vs 2200 → on
		{"under", 1900, "under"},
		{"over", 2400, "over"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := setupSummary(t, "UTC")
			pid := makeProductForSummary(t, f.pRepo, "x", 100.0) // 100 kcal / 100g
			// quantity_g = tc.qty grams → tc.qty kcal total
			logMealAt(t, f.r, pid, "2026-06-06T08:00:00Z", tc.qty, "")

			require.NoError(t, f.goalsRepo.Upsert(context.Background(), &goals.Goals{Kcal: &goals.Range{Min: ptrFlt(2090), Max: ptrFlt(2310)}}))

			rec := doGet(t, f.r, "/summary/daily?date=2026-06-06&tz=UTC")
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			var out summary.Daily
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
			require.NotNil(t, out.Adherence)
			entry, ok := out.Adherence["kcal"]
			require.True(t, ok)
			assert.Equal(t, tc.want, entry.Status)
		})
	}
}

func TestDaily_AdherenceRangeProtein(t *testing.T) {
	f := setupSummary(t, "UTC")
	pid := makeProductWithNutriments(t, f.pRepo, "high-p", products.Nutriments{
		KcalPer100g: ptrFlt(100), ProteinGPer100g: ptrFlt(20),
	})
	// 800g → 160g protein
	logMealAt(t, f.r, pid, "2026-06-06T08:00:00Z", 800, "")

	require.NoError(t, f.goalsRepo.Upsert(context.Background(), &goals.Goals{
		ProteinG: &goals.Range{Min: ptrFlt(150), Max: ptrFlt(190)},
	}))

	rec := doGet(t, f.r, "/summary/daily?date=2026-06-06&tz=UTC")
	require.Equal(t, http.StatusOK, rec.Code)
	var out summary.Daily
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	entry, ok := out.Adherence["protein_g"]
	require.True(t, ok)
	assert.Equal(t, "on", entry.Status)
}

func TestDaily_AdherenceMaxOnlyNeverUnder(t *testing.T) {
	f := setupSummary(t, "UTC")
	pid := makeProductWithNutriments(t, f.pRepo, "low-sugar", products.Nutriments{
		KcalPer100g: ptrFlt(100), SugarGPer100g: ptrFlt(1),
	})
	logMealAt(t, f.r, pid, "2026-06-06T08:00:00Z", 100, "") // 1g sugar

	require.NoError(t, f.goalsRepo.Upsert(context.Background(), &goals.Goals{
		SugarG: &goals.Range{Max: ptrFlt(50)},
	}))

	rec := doGet(t, f.r, "/summary/daily?date=2026-06-06&tz=UTC")
	require.Equal(t, http.StatusOK, rec.Code)
	var out summary.Daily
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	entry, ok := out.Adherence["sugar_g"]
	require.True(t, ok)
	assert.Equal(t, "on", entry.Status, "max-only goals never produce under")
}

func TestDaily_AdherenceMinOnlyNeverOver(t *testing.T) {
	f := setupSummary(t, "UTC")
	pid := makeProductWithNutriments(t, f.pRepo, "high-fiber", products.Nutriments{
		KcalPer100g: ptrFlt(100), FiberGPer100g: ptrFlt(10),
	})
	logMealAt(t, f.r, pid, "2026-06-06T08:00:00Z", 1000, "") // 100g fiber

	require.NoError(t, f.goalsRepo.Upsert(context.Background(), &goals.Goals{
		FiberG: &goals.Range{Min: ptrFlt(30)},
	}))

	rec := doGet(t, f.r, "/summary/daily?date=2026-06-06&tz=UTC")
	require.Equal(t, http.StatusOK, rec.Code)
	var out summary.Daily
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	entry, ok := out.Adherence["fiber_g"]
	require.True(t, ok)
	assert.Equal(t, "on", entry.Status, "min-only goals never produce over")
}

func TestDaily_AdherenceMicroWithoutContributorReturnsNoData(t *testing.T) {
	f := setupSummary(t, "UTC")
	pid := makeProductForSummary(t, f.pRepo, "no-iron", 100.0)
	logMealAt(t, f.r, pid, "2026-06-06T08:00:00Z", 100, "")

	require.NoError(t, f.goalsRepo.Upsert(context.Background(), &goals.Goals{
		IronMg: &goals.Range{Min: ptrFlt(14)},
	}))

	rec := doGet(t, f.r, "/summary/daily?date=2026-06-06&tz=UTC")
	require.Equal(t, http.StatusOK, rec.Code)
	var out summary.Daily
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	entry, ok := out.Adherence["iron_mg"]
	require.True(t, ok, "iron_mg row should be present (configured goal)")
	assert.Equal(t, "no_data", entry.Status, "no contributing entry → no_data, not silent omission")
	assert.Nil(t, entry.Actual)
}

func TestRange_GroupByMealType(t *testing.T) {
	f := setupSummary(t, "UTC")
	pid := makeProductForSummary(t, f.pRepo, "X", 100.0)
	logMealAt(t, f.r, pid, "2026-06-06T08:00:00Z", 200, "breakfast") // 200 kcal
	logMealAt(t, f.r, pid, "2026-06-06T13:00:00Z", 300, "lunch")    // 300 kcal

	rec := doGet(t, f.r, "/summary/range?from=2026-06-06&to=2026-06-06&tz=UTC&group_by=meal_type")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out summary.Range
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.NotNil(t, out.GroupBy)
	assert.Equal(t, "meal_type", *out.GroupBy)
	require.Len(t, out.Days, 1)
	day := out.Days[0]
	assert.Nil(t, day.Totals, "totals omitted when group_by is set")
	assert.Nil(t, day.Adherence, "adherence omitted in group_by mode")
	require.NotNil(t, day.ByMealType)
	assert.InDelta(t, 200.0, day.ByMealType["breakfast"].Kcal, 0.001)
	assert.InDelta(t, 300.0, day.ByMealType["lunch"].Kcal, 0.001)
	_, hasDinner := day.ByMealType["dinner"]
	assert.False(t, hasDinner, "meal types with no entries are omitted from by_meal_type")
}

func TestRange_GroupByInvalidReturns400(t *testing.T) {
	f := setupSummary(t, "UTC")
	rec := doGet(t, f.r, "/summary/range?from=2026-06-06&to=2026-06-06&tz=UTC&group_by=weekday")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"group_by_invalid"}`, rec.Body.String())
}
