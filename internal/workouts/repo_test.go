package workouts_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/store/storetest"
	"github.com/vinzenzs/kazper/internal/workouts"
)

func ptrStr(s string) *string { return &s }
func ptrF(v float64) *float64 { return &v }
func ptrI(v int) *int         { return &v }

func sample(extID *string, kcal float64) *workouts.Workout {
	return &workouts.Workout{
		ExternalID: extID,
		Source:     workouts.SourceManual,
		Sport:      workouts.SportBike,
		Name:       ptrStr("Morning Z2 ride"),
		StartedAt:  time.Date(2026, 6, 7, 8, 0, 0, 0, time.UTC),
		EndedAt:    time.Date(2026, 6, 7, 9, 30, 0, 0, time.UTC),
		KcalBurned: ptrF(kcal),
		AvgHR:      ptrI(135),
		TSS:        ptrF(78),
		Notes:      ptrStr("felt easy"),
	}
}

func TestUpsert_NewExternalIDInserts(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := workouts.NewRepo(pool)

	w := sample(ptrStr("garmin:1"), 800)
	created, err := repo.Upsert(context.Background(), w)
	require.NoError(t, err)
	assert.True(t, created, "first upsert with unseen external_id should INSERT")
	assert.NotEqual(t, uuid.Nil, w.ID)
}

func TestUpsert_ExistingExternalIDUpdates(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := workouts.NewRepo(pool)
	ctx := context.Background()

	first := sample(ptrStr("garmin:2"), 800)
	created, err := repo.Upsert(ctx, first)
	require.NoError(t, err)
	require.True(t, created)
	originalID := first.ID

	// Re-POST with same external_id but a corrected kcal_burned.
	second := sample(ptrStr("garmin:2"), 850)
	created, err = repo.Upsert(ctx, second)
	require.NoError(t, err)
	assert.False(t, created, "re-upsert with same external_id should UPDATE")
	assert.Equal(t, originalID, second.ID, "id is preserved across UPDATE")

	// Verify the row was actually updated.
	got, err := repo.GetByID(ctx, originalID)
	require.NoError(t, err)
	require.NotNil(t, got.KcalBurned)
	assert.InDelta(t, 850, *got.KcalBurned, 0.001)
}

func TestUpsert_NoExternalIDAlwaysInserts(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := workouts.NewRepo(pool)
	ctx := context.Background()

	a := sample(nil, 300)
	createdA, err := repo.Upsert(ctx, a)
	require.NoError(t, err)
	require.True(t, createdA)

	b := sample(nil, 300) // identical body
	createdB, err := repo.Upsert(ctx, b)
	require.NoError(t, err)
	require.True(t, createdB, "manual workouts always INSERT (NULL external_id never conflicts)")

	assert.NotEqual(t, a.ID, b.ID, "two NULL-external_id rows must have distinct ids")
}

func TestGetByID_NotFound(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := workouts.NewRepo(pool)

	_, err := repo.GetByID(context.Background(), uuid.New())
	assert.ErrorIs(t, err, workouts.ErrNotFound)
}

func TestPatch_MutableFieldsRoundTrip(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := workouts.NewRepo(pool)
	ctx := context.Background()

	w := sample(ptrStr("garmin:patch"), 800)
	_, err := repo.Upsert(ctx, w)
	require.NoError(t, err)

	require.NoError(t, repo.Patch(ctx, w.ID, workouts.PatchParams{
		TSS:   ptrF(85),
		Notes: ptrStr("FTP updated"),
	}))

	got, err := repo.GetByID(ctx, w.ID)
	require.NoError(t, err)
	require.NotNil(t, got.TSS)
	assert.InDelta(t, 85, *got.TSS, 0.001)
	require.NotNil(t, got.Notes)
	assert.Equal(t, "FTP updated", *got.Notes)
}

func TestDelete_AndSecondDeleteReturnsNotFound(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := workouts.NewRepo(pool)
	ctx := context.Background()

	w := sample(nil, 200)
	_, err := repo.Upsert(ctx, w)
	require.NoError(t, err)

	require.NoError(t, repo.Delete(ctx, w.ID))
	err = repo.Delete(ctx, w.ID)
	assert.ErrorIs(t, err, workouts.ErrNotFound)
}

func TestList_WindowFilterAndOrdering(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := workouts.NewRepo(pool)
	ctx := context.Background()

	makeAt := func(extID string, start time.Time) {
		t.Helper()
		w := &workouts.Workout{
			ExternalID: ptrStr(extID),
			Source:     workouts.SourceGarmin,
			Sport:      workouts.SportRun,
			StartedAt:  start,
			EndedAt:    start.Add(45 * time.Minute),
		}
		_, err := repo.Upsert(ctx, w)
		require.NoError(t, err)
	}

	// Three workouts: two in window, one before.
	makeAt("g:earlier", time.Date(2026, 5, 30, 8, 0, 0, 0, time.UTC))
	makeAt("g:later", time.Date(2026, 6, 7, 18, 0, 0, 0, time.UTC))
	makeAt("g:middle", time.Date(2026, 6, 7, 8, 0, 0, 0, time.UTC))

	rows, err := repo.List(ctx,
		time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC),
		nil, nil,
	)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	// Ordered by started_at ASC: middle (8am) before later (6pm).
	assert.Equal(t, "g:middle", *rows[0].ExternalID)
	assert.Equal(t, "g:later", *rows[1].ExternalID)
}

// ----- RPE + GI distress score (rehearsal-outcome fields) -----

func TestUpsert_StoresRPEAndGIDistressScore(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := workouts.NewRepo(pool)
	w := sample(nil, 600)
	w.RPE = ptrI(7)
	w.GIDistressScore = ptrI(2)
	_, err := repo.Upsert(context.Background(), w)
	require.NoError(t, err)

	got, err := repo.GetByID(context.Background(), w.ID)
	require.NoError(t, err)
	require.NotNil(t, got.RPE)
	assert.Equal(t, 7, *got.RPE)
	require.NotNil(t, got.GIDistressScore)
	assert.Equal(t, 2, *got.GIDistressScore)
}

func TestUpsert_OmittedRPEAndGIRemainNull(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := workouts.NewRepo(pool)
	w := sample(nil, 600)
	// Don't set RPE/GI — they stay nil.
	_, err := repo.Upsert(context.Background(), w)
	require.NoError(t, err)
	got, err := repo.GetByID(context.Background(), w.ID)
	require.NoError(t, err)
	assert.Nil(t, got.RPE)
	assert.Nil(t, got.GIDistressScore)
}

func TestUpsert_DBChecksRejectOutOfRange(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := workouts.NewRepo(pool)
	w := sample(nil, 600)
	w.RPE = ptrI(11) // out of 1..10
	_, err := repo.Upsert(context.Background(), w)
	require.Error(t, err, "DB CHECK constraint must reject rpe=11")
}

func TestUpsert_DBChecksRejectOutOfRangeGI(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := workouts.NewRepo(pool)
	w := sample(nil, 600)
	w.GIDistressScore = ptrI(6) // out of 1..5
	_, err := repo.Upsert(context.Background(), w)
	require.Error(t, err, "DB CHECK constraint must reject gi_distress_score=6")
}

func TestPatch_SetClearAndLeaveUnchanged(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := workouts.NewRepo(pool)
	ctx := context.Background()

	w := sample(nil, 600)
	w.RPE = ptrI(7)
	w.GIDistressScore = ptrI(2)
	_, err := repo.Upsert(ctx, w)
	require.NoError(t, err)

	// Set RPE to 8; leave GI unchanged.
	require.NoError(t, repo.Patch(ctx, w.ID, workouts.PatchParams{
		RPE: ptrI(8),
	}))
	got, err := repo.GetByID(ctx, w.ID)
	require.NoError(t, err)
	assert.Equal(t, 8, *got.RPE)
	assert.Equal(t, 2, *got.GIDistressScore, "untouched field stays at previous value")

	// Clear RPE via the ClearRPE flag.
	require.NoError(t, repo.Patch(ctx, w.ID, workouts.PatchParams{
		ClearRPE: true,
	}))
	got, err = repo.GetByID(ctx, w.ID)
	require.NoError(t, err)
	assert.Nil(t, got.RPE)
	assert.Equal(t, 2, *got.GIDistressScore, "GI still unchanged")

	// Clear GI too.
	require.NoError(t, repo.Patch(ctx, w.ID, workouts.PatchParams{
		ClearGIDistressScore: true,
	}))
	got, err = repo.GetByID(ctx, w.ID)
	require.NoError(t, err)
	assert.Nil(t, got.RPE)
	assert.Nil(t, got.GIDistressScore)
}

func TestList_EmptyWindowReturnsEmpty(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := workouts.NewRepo(pool)

	rows, err := repo.List(context.Background(),
		time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2030, 1, 2, 0, 0, 0, 0, time.UTC),
		nil, nil,
	)
	require.NoError(t, err)
	assert.Empty(t, rows)
}

// ----- Ingestion metrics (distance / power / temperature / sweat / group) -----

func TestUpsert_StoresIngestionMetrics(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := workouts.NewRepo(pool)
	w := sample(ptrStr("garmin:metrics"), 1800)
	w.DistanceM = ptrF(80500)
	w.AvgPowerW = ptrI(182)
	w.TemperatureC = ptrF(27.5)
	w.SweatLossML = ptrF(2400)
	w.SessionGroup = ptrStr("garmin:554")
	_, err := repo.Upsert(context.Background(), w)
	require.NoError(t, err)

	got, err := repo.GetByID(context.Background(), w.ID)
	require.NoError(t, err)
	require.NotNil(t, got.DistanceM)
	assert.InDelta(t, 80500, *got.DistanceM, 0.05)
	require.NotNil(t, got.AvgPowerW)
	assert.Equal(t, 182, *got.AvgPowerW)
	require.NotNil(t, got.TemperatureC)
	assert.InDelta(t, 27.5, *got.TemperatureC, 0.05)
	require.NotNil(t, got.SweatLossML)
	assert.InDelta(t, 2400, *got.SweatLossML, 0.05)
	require.NotNil(t, got.SessionGroup)
	assert.Equal(t, "garmin:554", *got.SessionGroup)
}

func TestUpsert_OmittedIngestionMetricsRemainNull(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := workouts.NewRepo(pool)
	w := sample(nil, 600) // none of the five fields set
	_, err := repo.Upsert(context.Background(), w)
	require.NoError(t, err)
	got, err := repo.GetByID(context.Background(), w.ID)
	require.NoError(t, err)
	assert.Nil(t, got.DistanceM)
	assert.Nil(t, got.AvgPowerW)
	assert.Nil(t, got.TemperatureC)
	assert.Nil(t, got.SweatLossML)
	assert.Nil(t, got.SessionGroup)
}

func TestUpsert_FullReplaceNullsOmittedMetric(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := workouts.NewRepo(pool)
	ctx := context.Background()

	first := sample(ptrStr("garmin:replace"), 1800)
	first.SweatLossML = ptrF(2400)
	_, err := repo.Upsert(ctx, first)
	require.NoError(t, err)

	// Re-POST the same external_id WITHOUT sweat_loss_ml — full-replace nulls it.
	second := sample(ptrStr("garmin:replace"), 1800)
	_, err = repo.Upsert(ctx, second)
	require.NoError(t, err)

	got, err := repo.GetByID(ctx, second.ID)
	require.NoError(t, err)
	assert.Nil(t, got.SweatLossML, "UPSERT full-replace nulls an omitted mutable field")
}

func TestUpsert_DBChecksRejectNegativeDistance(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := workouts.NewRepo(pool)
	w := sample(nil, 600)
	w.DistanceM = ptrF(-100)
	_, err := repo.Upsert(context.Background(), w)
	require.Error(t, err, "DB CHECK must reject distance_m <= 0")
}

func TestUpsert_DBChecksRejectTemperatureOutOfRange(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := workouts.NewRepo(pool)
	w := sample(nil, 600)
	w.TemperatureC = ptrF(100) // outside [-40, 60]
	_, err := repo.Upsert(context.Background(), w)
	require.Error(t, err, "DB CHECK must reject temperature_c=100")
}

func TestPatch_IngestionSetClearAndLeaveUnchanged(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := workouts.NewRepo(pool)
	ctx := context.Background()

	w := sample(nil, 1800)
	w.SweatLossML = ptrF(2400)
	w.SessionGroup = ptrStr("garmin:554")
	_, err := repo.Upsert(ctx, w)
	require.NoError(t, err)

	// Set temperature; leave sweat + group unchanged.
	require.NoError(t, repo.Patch(ctx, w.ID, workouts.PatchParams{
		TemperatureC: ptrF(31),
	}))
	got, err := repo.GetByID(ctx, w.ID)
	require.NoError(t, err)
	require.NotNil(t, got.TemperatureC)
	assert.InDelta(t, 31, *got.TemperatureC, 0.05)
	assert.InDelta(t, 2400, *got.SweatLossML, 0.05, "untouched field stays")
	assert.Equal(t, "garmin:554", *got.SessionGroup, "untouched field stays")

	// Clear the session group (un-group a mis-linked leg).
	require.NoError(t, repo.Patch(ctx, w.ID, workouts.PatchParams{
		ClearSessionGroup: true,
	}))
	got, err = repo.GetByID(ctx, w.ID)
	require.NoError(t, err)
	assert.Nil(t, got.SessionGroup)
	assert.InDelta(t, 2400, *got.SweatLossML, 0.05, "sweat loss still unchanged")
}

func TestList_FilteredBySessionGroup(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := workouts.NewRepo(pool)
	ctx := context.Background()

	mk := func(extID string, start time.Time, sport workouts.Sport, group *string) {
		t.Helper()
		w := &workouts.Workout{
			ExternalID:   ptrStr(extID),
			Source:       workouts.SourceGarmin,
			Sport:        sport,
			StartedAt:    start,
			EndedAt:      start.Add(30 * time.Minute),
			SessionGroup: group,
		}
		_, err := repo.Upsert(ctx, w)
		require.NoError(t, err)
	}

	base := time.Date(2026, 6, 13, 8, 0, 0, 0, time.UTC)
	// A brick: bike leg then run leg, sharing a group key; plus an unrelated swim.
	mk("g:bike", base, workouts.SportBike, ptrStr("garmin:9876543"))
	mk("g:run", base.Add(90*time.Minute), workouts.SportRun, ptrStr("garmin:9876543"))
	mk("g:swim", base.Add(3*time.Hour), workouts.SportSwim, nil)

	from := time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC)

	// Filtered: exactly the two legs, in started_at (leg) order.
	group := "garmin:9876543"
	legs, err := repo.List(ctx, from, to, &group, nil)
	require.NoError(t, err)
	require.Len(t, legs, 2)
	assert.Equal(t, "g:bike", *legs[0].ExternalID)
	assert.Equal(t, "g:run", *legs[1].ExternalID)

	// Unmatched group → empty.
	none := "garmin:nope"
	empty, err := repo.List(ctx, from, to, &none, nil)
	require.NoError(t, err)
	assert.Empty(t, empty)

	// No filter → all three.
	all, err := repo.List(ctx, from, to, nil, nil)
	require.NoError(t, err)
	assert.Len(t, all, 3)
}

// seedSlot inserts the template→plan→week→slot chain needed for a plan_slot FK
// and returns the new slot id. Uses raw SQL so the workouts package stays
// independent of the trainingplan package in tests.
func seedSlot(t *testing.T, pool interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}) (slotID, templateID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	var planID, weekID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO workout_templates (sport, name, steps) VALUES ('run','Easy run','[{"type":"step","intent":"active","duration":{"kind":"open"},"target":{"kind":"none"}}]'::jsonb) RETURNING id`).Scan(&templateID))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO training_plans (name, start_date) VALUES ('Plan','2026-06-01') RETURNING id`).Scan(&planID))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO plan_weeks (plan_id, ordinal) VALUES ($1, 1) RETURNING id`, planID).Scan(&weekID))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO plan_slots (plan_week_id, weekday, ordinal, template_id) VALUES ($1, 0, 0, $2) RETURNING id`, weekID, templateID).Scan(&slotID))
	return slotID, templateID
}

func plannedInput(slotID, templateID uuid.UUID) workouts.PlannedSlotInput {
	return workouts.PlannedSlotInput{
		PlanSlotID: slotID,
		TemplateID: templateID,
		Sport:      "run",
		Name:       ptrStr("Easy run"),
		StartedAt:  time.Date(2026, 6, 1, 6, 0, 0, 0, time.UTC),
		EndedAt:    time.Date(2026, 6, 1, 7, 0, 0, 0, time.UTC),
	}
}

func TestUpsertPlannedFromSlot_Idempotent(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := workouts.NewRepo(pool)
	ctx := context.Background()
	slotID, templateID := seedSlot(t, pool)

	in := plannedInput(slotID, templateID)
	w1, err := repo.UpsertPlannedFromSlot(ctx, pool, in)
	require.NoError(t, err)
	require.NotNil(t, w1)
	assert.Equal(t, "planned", string(w1.Status))
	require.NotNil(t, w1.PlanSlotID)
	assert.Equal(t, slotID, *w1.PlanSlotID)

	// Re-upsert the same slot → same row (no duplicate).
	w2, err := repo.UpsertPlannedFromSlot(ctx, pool, in)
	require.NoError(t, err)
	assert.Equal(t, w1.ID, w2.ID, "re-materialize updates the same row")

	all, err := repo.List(ctx, time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), nil, nil)
	require.NoError(t, err)
	assert.Len(t, all, 1)
}

func TestUpsertPlannedFromSlot_CoexistsWithExternalID(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := workouts.NewRepo(pool)
	ctx := context.Background()
	slotID, templateID := seedSlot(t, pool)

	// An imported Garmin activity (external_id, no plan_slot_id).
	_, err := repo.Upsert(ctx, sample(ptrStr("garmin:99"), 800))
	require.NoError(t, err)
	// A planned-from-slot workout.
	_, err = repo.UpsertPlannedFromSlot(ctx, pool, plannedInput(slotID, templateID))
	require.NoError(t, err)

	all, err := repo.List(ctx, time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), nil, nil)
	require.NoError(t, err)
	assert.Len(t, all, 2, "external_id and plan_slot_id paths are disjoint")
}

func TestUpsertPlannedFromSlot_CompletedNotReverted(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := workouts.NewRepo(pool)
	ctx := context.Background()
	slotID, templateID := seedSlot(t, pool)

	w, err := repo.UpsertPlannedFromSlot(ctx, pool, plannedInput(slotID, templateID))
	require.NoError(t, err)

	// Simulate reconciliation flipping it to completed (keeping plan_slot_id).
	completed := "completed"
	require.NoError(t, repo.Patch(ctx, w.ID, workouts.PatchParams{Status: &completed}))

	// Re-materialize: the status='planned' guard must NOT revert it.
	again, err := repo.UpsertPlannedFromSlot(ctx, pool, plannedInput(slotID, templateID))
	require.NoError(t, err)
	assert.Equal(t, w.ID, again.ID)
	assert.Equal(t, "completed", string(again.Status), "guard prevents reverting a fulfilled session")
}
