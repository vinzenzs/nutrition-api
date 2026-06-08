package workouts_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/nutrition-api/internal/store/storetest"
	"github.com/vinzenzs/nutrition-api/internal/workouts"
)

func ptrStr(s string) *string  { return &s }
func ptrF(v float64) *float64  { return &v }
func ptrI(v int) *int          { return &v }

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
	)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	// Ordered by started_at ASC: middle (8am) before later (6pm).
	assert.Equal(t, "g:middle", *rows[0].ExternalID)
	assert.Equal(t, "g:later", *rows[1].ExternalID)
}

func TestList_EmptyWindowReturnsEmpty(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := workouts.NewRepo(pool)

	rows, err := repo.List(context.Background(),
		time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2030, 1, 2, 0, 0, 0, 0, time.UTC),
	)
	require.NoError(t, err)
	assert.Empty(t, rows)
}
