package mcpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// log_meal / log_meal_freeform / patch_meal forward workout_id
// ============================================================================

func TestLogMeal_ForwardsWorkoutID(t *testing.T) {
	c, recs := newMealRecorder(t, 201, `{"id":"m1"}`)
	_ = handleLogMeal(context.Background(), c, LogMealArgs{
		ProductID: "p1", QuantityG: 100, LoggedAt: "2026-06-07T08:30:00Z",
		WorkoutID: "11111111-1111-1111-1111-111111111111",
	})
	require.Len(t, *recs, 1)
	var got map[string]any
	require.NoError(t, json.Unmarshal((*recs)[0].body, &got))
	assert.Equal(t, "11111111-1111-1111-1111-111111111111", got["workout_id"])
}

func TestLogMeal_OmitsWorkoutIDWhenUnset(t *testing.T) {
	c, recs := newMealRecorder(t, 201, `{"id":"m1"}`)
	_ = handleLogMeal(context.Background(), c, LogMealArgs{
		ProductID: "p1", QuantityG: 100, LoggedAt: "2026-06-07T08:30:00Z",
	})
	require.Len(t, *recs, 1)
	assert.NotContains(t, string((*recs)[0].body), "workout_id")
}

func TestLogMealFreeform_ForwardsWorkoutID(t *testing.T) {
	c, recs := newMealRecorder(t, 201, `{"id":"m1"}`)
	_ = handleLogMealFreeform(context.Background(), c, LogMealFreeformArgs{
		Name: "banana", QuantityG: 100, LoggedAt: "2026-06-07T08:30:00Z",
		WorkoutID: "22222222-2222-2222-2222-222222222222",
	})
	require.Len(t, *recs, 1)
	var got map[string]any
	require.NoError(t, json.Unmarshal((*recs)[0].body, &got))
	assert.Equal(t, "22222222-2222-2222-2222-222222222222", got["workout_id"])
}

func TestPatchMeal_ForwardsWorkoutIDForSet(t *testing.T) {
	c, recs := newMealRecorder(t, 200, `{"id":"m1"}`)
	wid := "33333333-3333-3333-3333-333333333333"
	_ = handlePatchMeal(context.Background(), c, PatchMealArgs{
		MealID:    "abc",
		WorkoutID: &wid,
	})
	require.Len(t, *recs, 1)
	assert.JSONEq(t, `{"workout_id":"33333333-3333-3333-3333-333333333333"}`, string((*recs)[0].body))
}

func TestPatchMeal_ForwardsEmptyStringForClear(t *testing.T) {
	c, recs := newMealRecorder(t, 200, `{"id":"m1"}`)
	empty := ""
	_ = handlePatchMeal(context.Background(), c, PatchMealArgs{
		MealID:    "abc",
		WorkoutID: &empty,
	})
	require.Len(t, *recs, 1)
	assert.JSONEq(t, `{"workout_id":""}`, string((*recs)[0].body))
}

func TestPatchMeal_OmitsWorkoutIDWhenUnset(t *testing.T) {
	c, recs := newMealRecorder(t, 200, `{"id":"m1"}`)
	qty := 200.0
	_ = handlePatchMeal(context.Background(), c, PatchMealArgs{
		MealID:    "abc",
		QuantityG: &qty,
	})
	require.Len(t, *recs, 1)
	assert.JSONEq(t, `{"quantity_g":200}`, string((*recs)[0].body))
}

// ============================================================================
// log_hydration / patch_hydration forward workout_id
// ============================================================================

func TestLogHydration_ForwardsWorkoutID(t *testing.T) {
	c, recs := newHydrationRecorder(t, 201, `{"id":"h1"}`)
	_ = handleLogHydration(context.Background(), c, LogHydrationArgs{
		QuantityMl: 500, LoggedAt: "2026-06-07T08:30:00Z",
		WorkoutID: "44444444-4444-4444-4444-444444444444",
	})
	require.Len(t, *recs, 1)
	var got map[string]any
	require.NoError(t, json.Unmarshal((*recs)[0].body, &got))
	assert.Equal(t, "44444444-4444-4444-4444-444444444444", got["workout_id"])
}

func TestPatchHydration_ForwardsEmptyStringForClear(t *testing.T) {
	c, recs := newHydrationRecorder(t, 200, `{"id":"h1"}`)
	empty := ""
	_ = handlePatchHydration(context.Background(), c, PatchHydrationArgs{
		ID:        "abc",
		WorkoutID: &empty,
	})
	require.Len(t, *recs, 1)
	assert.JSONEq(t, `{"workout_id":""}`, string((*recs)[0].body))
}

// ============================================================================
// workout_fueling_summary tool
// ============================================================================

func TestWorkoutFuelingSummary_CallsFuelingEndpoint(t *testing.T) {
	c, recs := newWorkoutRecorder(t, 200, `{"workout_id":"abc"}`)
	pre := 180
	post := 90
	r := handleWorkoutFuelingSummary(context.Background(), c, WorkoutFuelingSummaryArgs{
		WorkoutID:     "abc",
		PreWindowMin:  &pre,
		PostWindowMin: &post,
	})
	assert.False(t, r.IsError)
	require.Len(t, *recs, 1)
	rec := (*recs)[0]
	assert.Equal(t, http.MethodGet, rec.method)
	assert.Equal(t, "/workouts/abc/fueling", rec.path)
	q, err := url.ParseQuery(rec.rawQuery)
	require.NoError(t, err)
	assert.Equal(t, "180", q.Get("pre_window_min"))
	assert.Equal(t, "90", q.Get("post_window_min"))
	assert.Empty(t, rec.idemKey, "fueling summary is read-only; no idempotency key")
}

func TestWorkoutFuelingSummary_OmitsUnsetParams(t *testing.T) {
	c, recs := newWorkoutRecorder(t, 200, `{}`)
	_ = handleWorkoutFuelingSummary(context.Background(), c, WorkoutFuelingSummaryArgs{
		WorkoutID: "abc",
	})
	require.Len(t, *recs, 1)
	q, err := url.ParseQuery((*recs)[0].rawQuery)
	require.NoError(t, err)
	assert.Empty(t, q.Get("pre_window_min"), "unset optional params must not appear in query string")
	assert.Empty(t, q.Get("post_window_min"))
}

func TestWorkoutFuelingSummary_404ForwardsIsError(t *testing.T) {
	c, _ := newWorkoutRecorder(t, 404, `{"error":"workout_not_found"}`)
	r := handleWorkoutFuelingSummary(context.Background(), c, WorkoutFuelingSummaryArgs{
		WorkoutID: "unknown",
	})
	assert.True(t, r.IsError)
}
