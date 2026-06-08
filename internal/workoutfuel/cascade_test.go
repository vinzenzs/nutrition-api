package workoutfuel_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/nutrition-api/internal/workoutfuel"
)

// TestDeleteWorkoutCascadesNullOnFuelEntry asserts the FK's
// `ON DELETE SET NULL` behaviour: deleting a workout MUST clear `workout_id`
// on every workout-fuel row that referenced it, without removing the row.
func TestDeleteWorkoutCascadesNullOnFuelEntry(t *testing.T) {
	f := setup(t)
	wid := seedWorkout(t, f.workoutsRepo)

	body := fmt.Sprintf(
		`{"name":"Gel","logged_at":"2026-06-07T08:30:00Z","carbs_g":25,"workout_id":%q}`, wid)
	created := mustCreate(t, f.r, body)
	require.NotNil(t, created.WorkoutID, "fixture must start tagged")

	require.NoError(t, f.workoutsRepo.Delete(context.Background(), wid))

	rec := doRequest(t, f.r, http.MethodGet,
		"/workout-fuel?from=2026-06-07T00:00:00Z&to=2026-06-08T00:00:00Z", "", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body2 struct {
		Entries []workoutfuel.Entry `json:"entries"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body2))
	require.Len(t, body2.Entries, 1, "row must be preserved by SET NULL cascade")
	assert.Nil(t, body2.Entries[0].WorkoutID,
		"workout deletion should cascade SET NULL on the workout-fuel entry")
	require.NotNil(t, body2.Entries[0].CarbsG, "non-FK columns must be unchanged")
	assert.Equal(t, 25.0, *body2.Entries[0].CarbsG)
}
