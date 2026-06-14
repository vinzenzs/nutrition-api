package summary_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/goals"
	"github.com/vinzenzs/kazper/internal/summary"
)

// Helpers (`setupSummary`, `makeProductForSummary`, `logMeal`, `logMealAt`,
// `doGet`, `ptrFlt`) are defined in handlers_test.go in the same _test package.

// ============================================================================
// Happy path
// ============================================================================

func TestRolling_HappyPath_SevenDayWindow(t *testing.T) {
	f := setupSummary(t, "UTC")
	pid := makeProductForSummary(t, f.pRepo, "p1", 100.0) // 100 kcal/100g

	// Log 250g/day from 2026-06-02 through 2026-06-08 = 250 kcal/day
	for day := 2; day <= 8; day++ {
		ts := time.Date(2026, 6, day, 12, 0, 0, 0, time.UTC).Format(time.RFC3339)
		logMealAt(t, f.r, pid, ts, 250, "")
	}

	rec := doGet(t, f.r, "/summary/rolling?anchor_date=2026-06-08&window_days=7&tz=UTC")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var out summary.Rolling
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, "2026-06-08", out.AnchorDate)
	assert.Equal(t, 7, out.WindowDays)
	assert.Equal(t, "UTC", out.TZ)
	assert.Equal(t, 7, out.DaysWithData)
	assert.Equal(t, 7, out.TotalDays)
	require.Len(t, out.Days, 7)
	// First day is anchor - 6 = 2026-06-02; last is anchor itself.
	assert.Equal(t, "2026-06-02", out.Days[0].Date)
	assert.Equal(t, "2026-06-08", out.Days[6].Date)
	for _, d := range out.Days {
		assert.True(t, d.HasData, "every day in this fixture should have data")
		assert.InDelta(t, 250.0, d.Totals.Kcal, 0.05)
	}
	assert.InDelta(t, 250.0, out.Averages.Kcal, 0.05)
}

// ============================================================================
// Sparse-divisor rule (the load-bearing decision)
// ============================================================================

func TestRolling_Sparse_DividesByDaysWithDataOnly(t *testing.T) {
	f := setupSummary(t, "UTC")
	pid := makeProductForSummary(t, f.pRepo, "p1", 100.0) // 100 kcal/100g

	// Log on 5 of 7 days. Each day: 2520 kcal (2520g of a 100kcal/100g product → 2520 kcal).
	// Total = 12600 kcal across 5 days. divisor=5 → 2520; divisor=7 (wrong) → 1800.
	for _, day := range []int{2, 3, 5, 6, 8} { // skip 4 and 7
		ts := time.Date(2026, 6, day, 12, 0, 0, 0, time.UTC).Format(time.RFC3339)
		logMealAt(t, f.r, pid, ts, 2520, "")
	}

	rec := doGet(t, f.r, "/summary/rolling?anchor_date=2026-06-08&window_days=7&tz=UTC")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var out summary.Rolling
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, 5, out.DaysWithData)
	assert.Equal(t, 7, out.TotalDays)
	assert.InDelta(t, 2520.0, out.Averages.Kcal, 0.1,
		"divisor MUST be days_with_data=5, not total_days=7")

	// Per-day rows: 7 entries, two with has_data=false
	hadData := 0
	for _, d := range out.Days {
		if d.HasData {
			hadData++
		}
	}
	assert.Equal(t, 5, hadData)
}

// ============================================================================
// Empty window
// ============================================================================

func TestRolling_EmptyWindow_ZeroDivisorAndNoDataAdherence(t *testing.T) {
	f := setupSummary(t, "UTC")
	// Goals exist so adherence keys exist; meals do NOT.
	require.NoError(t, f.goalsRepo.Upsert(context.Background(), &goals.Goals{
		Kcal: &goals.Range{Min: ptrFlt(2090), Max: ptrFlt(2310)},
	}))

	rec := doGet(t, f.r, "/summary/rolling?anchor_date=2026-06-08&window_days=7&tz=UTC")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var out summary.Rolling
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, 0, out.DaysWithData)
	assert.Equal(t, 7, out.TotalDays)
	assert.Equal(t, 0.0, out.Averages.Kcal)
	require.Len(t, out.Days, 7)
	for _, d := range out.Days {
		assert.False(t, d.HasData)
	}
	// Adherence: kcal should be no_data with actual=nil.
	require.NotNil(t, out.Adherence)
	require.Contains(t, out.Adherence, "kcal")
	assert.Equal(t, "no_data", out.Adherence["kcal"].Status)
	assert.Nil(t, out.Adherence["kcal"].Actual)
}

// ============================================================================
// Zero-kcal logged day is NOT flagged as missing
// ============================================================================

func TestRolling_ZeroKcalMeal_DayCountsAsHasData(t *testing.T) {
	f := setupSummary(t, "UTC")
	pid := makeProductForSummary(t, f.pRepo, "zero-kcal", 0.0) // 0 kcal/100g

	// Single 100g meal on the anchor day; produces 0 kcal but the user logged something.
	logMealAt(t, f.r, pid, "2026-06-08T12:00:00Z", 100, "")

	rec := doGet(t, f.r, "/summary/rolling?anchor_date=2026-06-08&window_days=2&tz=UTC")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var out summary.Rolling
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Len(t, out.Days, 2)
	// out.Days[0] = 2026-06-07 (empty), out.Days[1] = 2026-06-08 (zero-kcal meal)
	assert.False(t, out.Days[0].HasData)
	assert.True(t, out.Days[1].HasData, "logged-zero day is distinct from no-meals day")
	assert.Equal(t, 0.0, out.Days[1].Totals.Kcal)
	assert.Equal(t, 1, out.DaysWithData)
}

// ============================================================================
// TZ boundaries
// ============================================================================

func TestRolling_TZ_MealAt2230Z_AppearsOnLocalNextDay(t *testing.T) {
	f := setupSummary(t, "UTC")
	pid := makeProductForSummary(t, f.pRepo, "p1", 100.0)

	// Meal at 22:30Z on June 7. In Europe/Berlin (UTC+2 in summer) this is
	// 00:30 local on June 8.
	logMealAt(t, f.r, pid, "2026-06-07T22:30:00Z", 250, "")

	rec := doGet(t, f.r, "/summary/rolling?anchor_date=2026-06-08&window_days=2&tz=Europe/Berlin")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var out summary.Rolling
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Len(t, out.Days, 2)
	// out.Days[0]=2026-06-07 should be empty; the meal moved to June 8 LOCAL.
	assert.False(t, out.Days[0].HasData,
		"the 22:30Z meal must NOT appear on the local June 7 row")
	assert.True(t, out.Days[1].HasData,
		"the 22:30Z meal must appear on the local June 8 row")
	assert.Equal(t, "2026-06-07", out.Days[0].Date)
	assert.Equal(t, "2026-06-08", out.Days[1].Date)
}

// DST: Europe/Berlin spring-forward in 2026 is the night of March 28/29.
// A 7-day window anchored at 2026-03-31 covers 2026-03-25..2026-03-31 and
// must still produce 7 day rows even though one of those days is 23h long.
func TestRolling_DST_SpringForward_StillSevenDays(t *testing.T) {
	f := setupSummary(t, "Europe/Berlin")

	rec := doGet(t, f.r, "/summary/rolling?anchor_date=2026-03-31&window_days=7&tz=Europe/Berlin")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var out summary.Rolling
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Len(t, out.Days, 7, "DST does not shrink the calendar-day count")
	assert.Equal(t, "2026-03-25", out.Days[0].Date)
	assert.Equal(t, "2026-03-31", out.Days[6].Date)
}

// ============================================================================
// window_days bounds
// ============================================================================

func TestRolling_WindowDays_TwoIsMinAccepted(t *testing.T) {
	f := setupSummary(t, "UTC")
	rec := doGet(t, f.r, "/summary/rolling?anchor_date=2026-06-08&window_days=2&tz=UTC")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out summary.Rolling
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Len(t, out.Days, 2)
}

func TestRolling_WindowDays_ThirtyIsMaxAccepted(t *testing.T) {
	f := setupSummary(t, "UTC")
	rec := doGet(t, f.r, "/summary/rolling?anchor_date=2026-06-08&window_days=30&tz=UTC")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out summary.Rolling
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Len(t, out.Days, 30)
}

func TestRolling_WindowDays_OneRejected(t *testing.T) {
	f := setupSummary(t, "UTC")
	rec := doGet(t, f.r, "/summary/rolling?anchor_date=2026-06-08&window_days=1&tz=UTC")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "window_days_invalid", body["error"])
	rng, ok := body["range"].(map[string]any)
	require.True(t, ok)
	assert.EqualValues(t, 2, rng["min"])
	assert.EqualValues(t, 30, rng["max"])
}

func TestRolling_WindowDays_ThirtyOneRejected(t *testing.T) {
	f := setupSummary(t, "UTC")
	rec := doGet(t, f.r, "/summary/rolling?anchor_date=2026-06-08&window_days=31&tz=UTC")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "window_days_invalid", body["error"])
}

func TestRolling_WindowDays_MissingRejected(t *testing.T) {
	f := setupSummary(t, "UTC")
	rec := doGet(t, f.r, "/summary/rolling?anchor_date=2026-06-08&tz=UTC")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"window_days_required"}`, rec.Body.String())
}

func TestRolling_WindowDays_MalformedRejected(t *testing.T) {
	f := setupSummary(t, "UTC")
	rec := doGet(t, f.r, "/summary/rolling?anchor_date=2026-06-08&window_days=lots&tz=UTC")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "window_days_invalid", body["error"])
}

func TestRolling_AnchorDate_MissingRejected(t *testing.T) {
	f := setupSummary(t, "UTC")
	rec := doGet(t, f.r, "/summary/rolling?window_days=7&tz=UTC")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"anchor_date_required"}`, rec.Body.String())
}

func TestRolling_AnchorDate_MalformedRejected(t *testing.T) {
	f := setupSummary(t, "UTC")
	rec := doGet(t, f.r, "/summary/rolling?anchor_date=2026-13-99&window_days=7&tz=UTC")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"anchor_date_invalid"}`, rec.Body.String())
}

func TestRolling_TZ_Invalid(t *testing.T) {
	f := setupSummary(t, "UTC")
	rec := doGet(t, f.r, "/summary/rolling?anchor_date=2026-06-08&window_days=7&tz=Mars%2FOlympus")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"tz_invalid"}`, rec.Body.String())
}

func TestRolling_TZ_DefaultsWhenOmitted(t *testing.T) {
	f := setupSummary(t, "Europe/Berlin")
	rec := doGet(t, f.r, "/summary/rolling?anchor_date=2026-06-08&window_days=7")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out summary.Rolling
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, "Europe/Berlin", out.TZ)
}

// ============================================================================
// Adherence at the anchor
// ============================================================================

func TestRolling_Adherence_GoalSourceDefault(t *testing.T) {
	f := setupSummary(t, "UTC")
	pid := makeProductForSummary(t, f.pRepo, "p1", 100.0)
	logMealAt(t, f.r, pid, "2026-06-08T12:00:00Z", 2200, "") // 2200 kcal

	require.NoError(t, f.goalsRepo.Upsert(context.Background(), &goals.Goals{
		Kcal: &goals.Range{Min: ptrFlt(2090), Max: ptrFlt(2310)},
	}))

	rec := doGet(t, f.r, "/summary/rolling?anchor_date=2026-06-08&window_days=7&tz=UTC")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out summary.Rolling
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, "default", out.GoalSource)
	require.Contains(t, out.Adherence, "kcal")
	// avg kcal over 1 logged day = 2200, target [2090, 2310] → "on"
	assert.Equal(t, "on", out.Adherence["kcal"].Status)
}

func TestRolling_Adherence_GoalSourceOverrideAtAnchor(t *testing.T) {
	f := setupSummary(t, "UTC")
	pid := makeProductForSummary(t, f.pRepo, "p1", 100.0)
	logMealAt(t, f.r, pid, "2026-06-08T12:00:00Z", 2400, "")

	require.NoError(t, f.goalsRepo.Upsert(context.Background(), &goals.Goals{
		Kcal: &goals.Range{Min: ptrFlt(2090), Max: ptrFlt(2310)},
	}))
	// Override only on the anchor.
	require.NoError(t, f.overridesRepo.Upsert(context.Background(),
		time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC),
		&goals.Goals{Kcal: &goals.Range{Min: ptrFlt(2280), Max: ptrFlt(2520)}}))

	rec := doGet(t, f.r, "/summary/rolling?anchor_date=2026-06-08&window_days=7&tz=UTC")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out summary.Rolling
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, "override", out.GoalSource,
		"goal source resolves at the anchor, not the average across the window")
	// 2400 falls in [2280, 2520] → "on" — the override "saves" what would
	// have been "over" against the default.
	assert.Equal(t, "on", out.Adherence["kcal"].Status)
}

// ============================================================================
// Rounding
// ============================================================================

func TestRolling_Rounding_AveragesAtResponseBoundary(t *testing.T) {
	f := setupSummary(t, "UTC")
	pid := makeProductForSummary(t, f.pRepo, "p1", 100.0)

	// 3 days, each with 25.5556 kcal (mass 25.5556g of a 100kcal/100g product).
	// DB rounds NUMERIC(10,1) so each meal stores 25.6g → 25.6 kcal → avg 25.6.
	// We just want to confirm rounding lands at 1dp.
	for _, day := range []int{6, 7, 8} {
		ts := time.Date(2026, 6, day, 12, 0, 0, 0, time.UTC).Format(time.RFC3339)
		logMealAt(t, f.r, pid, ts, 25.5556, "")
	}

	rec := doGet(t, f.r, "/summary/rolling?anchor_date=2026-06-08&window_days=3&tz=UTC")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var out summary.Rolling
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, 3, out.DaysWithData)
	// numfmt.Round1 round-half-up; avg of three 25.6's = 25.6.
	assert.Equal(t, 25.6, out.Averages.Kcal)
}
