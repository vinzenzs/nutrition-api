package workouts_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/nutrition-api/internal/workouts"
)

// A fixed past day so completed activities clear the 24h-future guard.
var reconDay = time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC)

func at(hour int) time.Time { return reconDay.Add(time.Duration(hour) * time.Hour) }

// seedPlannedFromSlot builds a minimal plan→week→slot→template chain and
// materializes one planned workout from it (status=planned, plan_slot_id +
// template_id set, no external_id) — an open reconciliation candidate.
func seedPlannedFromSlot(t *testing.T, f *fixture, sport string, start time.Time) (*workouts.Workout, uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	var templateID, planID, weekID, planSlotID uuid.UUID
	require.NoError(t, f.pool.QueryRow(ctx,
		`INSERT INTO workout_templates (sport, name, steps) VALUES ($1, 'T', '[{"kind":"work"}]'::jsonb) RETURNING id`,
		sport).Scan(&templateID))
	require.NoError(t, f.pool.QueryRow(ctx,
		`INSERT INTO training_plans (name, start_date) VALUES ('P', '2026-06-01') RETURNING id`).Scan(&planID))
	require.NoError(t, f.pool.QueryRow(ctx,
		`INSERT INTO plan_weeks (plan_id, ordinal) VALUES ($1, 1) RETURNING id`, planID).Scan(&weekID))
	require.NoError(t, f.pool.QueryRow(ctx,
		`INSERT INTO plan_slots (plan_week_id, weekday, template_id) VALUES ($1, 0, $2) RETURNING id`,
		weekID, templateID).Scan(&planSlotID))

	w, err := f.repo.UpsertPlannedFromSlot(ctx, f.pool, workouts.PlannedSlotInput{
		PlanSlotID: planSlotID,
		TemplateID: templateID,
		Sport:      sport,
		StartedAt:  start,
		EndedAt:    start.Add(time.Hour),
	})
	require.NoError(t, err)
	return w, planSlotID, templateID
}

// postPlanned inserts an open planned workout via the API (no plan links).
func postPlanned(t *testing.T, f *fixture, sport string, start time.Time) workouts.Workout {
	t.Helper()
	body := fmt.Sprintf(`{"source":"manual","sport":%q,"status":"planned","started_at":%q,"ended_at":%q}`,
		sport, start.Format(time.RFC3339), start.Add(time.Hour).Format(time.RFC3339))
	rec := doReq(t, f.r, http.MethodPost, "/workouts", body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	return decodeWorkout(t, rec.Body.Bytes())
}

// ingestGarmin posts a completed garmin activity and asserts the HTTP code.
func ingestGarmin(t *testing.T, f *fixture, extID, sport string, start time.Time, wantCode int) workouts.Workout {
	t.Helper()
	body := fmt.Sprintf(`{"external_id":%q,"source":"garmin","sport":%q,"started_at":%q,"ended_at":%q,"kcal_burned":600,"avg_hr":140}`,
		extID, sport, start.Format(time.RFC3339), start.Add(time.Hour).Format(time.RFC3339))
	rec := doReq(t, f.r, http.MethodPost, "/workouts", body)
	require.Equal(t, wantCode, rec.Code, rec.Body.String())
	return decodeWorkout(t, rec.Body.Bytes())
}

func decodeWorkout(t *testing.T, b []byte) workouts.Workout {
	t.Helper()
	var w workouts.Workout
	require.NoError(t, json.Unmarshal(b, &w))
	return w
}

func getWorkout(t *testing.T, f *fixture, id uuid.UUID) (workouts.Workout, int) {
	t.Helper()
	rec := doReq(t, f.r, http.MethodGet, "/workouts/"+id.String(), "")
	if rec.Code != http.StatusOK {
		return workouts.Workout{}, rec.Code
	}
	return decodeWorkout(t, rec.Body.Bytes()), rec.Code
}

// countDay returns how many workouts of a status fall on reconDay.
func countDay(t *testing.T, f *fixture, status string) int {
	t.Helper()
	from := reconDay.Add(-24 * time.Hour).Format(time.RFC3339)
	to := reconDay.Add(36 * time.Hour).Format(time.RFC3339)
	url := "/workouts?from=" + from + "&to=" + to
	if status != "" {
		url += "&status=" + status
	}
	rec := doReq(t, f.r, http.MethodGet, url, "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out struct {
		Workouts []workouts.Workout `json:"workouts"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	return len(out.Workouts)
}

// ----- 5.1 ingestion reconciliation -----

func TestIngest_SingleMatchFulfillsAndKeepsPlanLinks(t *testing.T) {
	f := setup(t)
	planned, planSlotID, templateID := seedPlannedFromSlot(t, f, "run", at(18))

	// A garmin run on the same local day → merges into the planned row.
	merged := ingestGarmin(t, f, "garmin:r1", "run", at(7), http.StatusOK)
	assert.Equal(t, planned.ID, merged.ID, "merged into the planned row, no new row")
	assert.Equal(t, workouts.StatusCompleted, merged.Status)
	require.NotNil(t, merged.ExternalID)
	assert.Equal(t, "garmin:r1", *merged.ExternalID)
	require.NotNil(t, merged.PlanSlotID)
	assert.Equal(t, planSlotID, *merged.PlanSlotID, "plan_slot_id retained")
	require.NotNil(t, merged.TemplateID)
	assert.Equal(t, templateID, *merged.TemplateID, "template_id retained")

	assert.Equal(t, 1, countDay(t, f, ""), "exactly one row for the day — no sibling")
	assert.Equal(t, 0, countDay(t, f, "planned"), "the planned row was fulfilled")

	// Re-sync the same activity → idempotent UPSERT, no re-match, links kept.
	resync := ingestGarmin(t, f, "garmin:r1", "run", at(7), http.StatusOK)
	assert.Equal(t, planned.ID, resync.ID)
	require.NotNil(t, resync.PlanSlotID)
	assert.Equal(t, planSlotID, *resync.PlanSlotID)
	assert.Equal(t, 1, countDay(t, f, ""), "re-sync did not create a row")
}

func TestIngest_NoMatchCreatesStandalone(t *testing.T) {
	f := setup(t)
	w := ingestGarmin(t, f, "garmin:solo", "run", at(7), http.StatusCreated)
	assert.Equal(t, workouts.StatusCompleted, w.Status)
	assert.False(t, w.NeedsLink)
	assert.Nil(t, w.PlanSlotID)
	assert.Equal(t, 1, countDay(t, f, "completed"))
}

func TestIngest_AmbiguousFlagsNeedsLink(t *testing.T) {
	f := setup(t)
	// Two open planned runs the same day → auto-merge unsafe.
	postPlanned(t, f, "run", at(7))
	postPlanned(t, f, "run", at(18))

	w := ingestGarmin(t, f, "garmin:amb", "run", at(8), http.StatusCreated)
	assert.True(t, w.NeedsLink, "ambiguous import is flagged")
	assert.Equal(t, workouts.StatusCompleted, w.Status)
	assert.Equal(t, 2, countDay(t, f, "planned"), "neither planned row was auto-fulfilled")
}

func TestIngest_DifferentSportDoesNotMatch(t *testing.T) {
	f := setup(t)
	postPlanned(t, f, "bike", at(18)) // a planned BIKE
	w := ingestGarmin(t, f, "garmin:run", "run", at(7), http.StatusCreated)
	assert.Nil(t, w.PlanSlotID, "a run does not fulfill a planned bike")
	assert.Equal(t, 1, countDay(t, f, "planned"))
}

// ----- 5.2 materialize guard cross-check -----

func TestMaterializeGuard_DoesNotRevertFulfilled(t *testing.T) {
	f := setup(t)
	planned, planSlotID, templateID := seedPlannedFromSlot(t, f, "run", at(18))
	ingestGarmin(t, f, "garmin:r1", "run", at(7), http.StatusOK) // fulfills the planned row

	// Re-materialize the same slot (exactly what the plan materializer does):
	// the WHERE status='planned' guard must skip the now-completed row.
	got, err := f.repo.UpsertPlannedFromSlot(context.Background(), f.pool, workouts.PlannedSlotInput{
		PlanSlotID: planSlotID,
		TemplateID: templateID,
		Sport:      "run",
		StartedAt:  at(18),
		EndedAt:    at(19),
	})
	require.NoError(t, err)
	assert.Equal(t, planned.ID, got.ID)
	assert.Equal(t, workouts.StatusCompleted, got.Status, "fulfilled row not reverted to planned")
	require.NotNil(t, got.ExternalID)
	assert.Equal(t, "garmin:r1", *got.ExternalID, "actuals preserved through re-materialize")
}

// ----- 3.3 explicit fulfill / unfulfill -----

func TestFulfill_MergesTwoExistingRowsAndKeepsSlot(t *testing.T) {
	f := setup(t)
	// A standalone completed activity (ingested before any plan existed).
	standalone := ingestGarmin(t, f, "garmin:x", "run", at(7), http.StatusCreated)
	// Then the plan materializes a planned run for the same session.
	planned, planSlotID, _ := seedPlannedFromSlot(t, f, "run", at(18))

	rec := doReq(t, f.r, http.MethodPost, "/workouts/"+planned.ID.String()+"/fulfill",
		fmt.Sprintf(`{"completed_id":%q}`, standalone.ID.String()))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	merged := decodeWorkout(t, rec.Body.Bytes())
	assert.Equal(t, planned.ID, merged.ID, "planned row survives")
	assert.Equal(t, workouts.StatusCompleted, merged.Status)
	require.NotNil(t, merged.ExternalID)
	assert.Equal(t, "garmin:x", *merged.ExternalID)
	require.NotNil(t, merged.PlanSlotID)
	assert.Equal(t, planSlotID, *merged.PlanSlotID)

	// The standalone row is gone.
	_, code := getWorkout(t, f, standalone.ID)
	assert.Equal(t, http.StatusNotFound, code)
}

func TestFulfill_RejectsNonPlannedTarget(t *testing.T) {
	f := setup(t)
	completedA := ingestGarmin(t, f, "garmin:a", "run", at(7), http.StatusCreated)
	completedB := ingestGarmin(t, f, "garmin:b", "run", at(9), http.StatusCreated)
	rec := doReq(t, f.r, http.MethodPost, "/workouts/"+completedA.ID.String()+"/fulfill",
		fmt.Sprintf(`{"completed_id":%q}`, completedB.ID.String()))
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"planned_workout_required"}`, rec.Body.String())
}

func TestUnfulfill_RestoresPlanned(t *testing.T) {
	f := setup(t)
	planned, planSlotID, templateID := seedPlannedFromSlot(t, f, "run", at(18))
	ingestGarmin(t, f, "garmin:r1", "run", at(7), http.StatusOK) // fulfills it

	rec := doReq(t, f.r, http.MethodPost, "/workouts/"+planned.ID.String()+"/unfulfill", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	restored := decodeWorkout(t, rec.Body.Bytes())
	assert.Equal(t, workouts.StatusPlanned, restored.Status)
	assert.Nil(t, restored.ExternalID, "external_id cleared")
	assert.Nil(t, restored.KcalBurned, "actuals cleared")
	require.NotNil(t, restored.PlanSlotID)
	assert.Equal(t, planSlotID, *restored.PlanSlotID, "plan_slot_id kept")
	require.NotNil(t, restored.TemplateID)
	assert.Equal(t, templateID, *restored.TemplateID, "template_id kept")
}

func TestUnfulfill_RejectsNonFulfilledRow(t *testing.T) {
	f := setup(t)
	standalone := ingestGarmin(t, f, "garmin:plain", "run", at(7), http.StatusCreated)
	rec := doReq(t, f.r, http.MethodPost, "/workouts/"+standalone.ID.String()+"/unfulfill", "")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"workout_not_fulfilled"}`, rec.Body.String())
}
