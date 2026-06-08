package goals_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/nutrition-api/internal/goals"
	"github.com/vinzenzs/nutrition-api/internal/store/storetest"
)

func ptr[T any](v T) *T { return &v }

func TestOverridesRepo_UpsertAndGet(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := goals.NewOverridesRepo(pool)
	ctx := context.Background()

	date := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	g := &goals.Goals{
		Kcal:     &goals.Range{Min: ptr(2280.0), Max: ptr(2520.0)},
		ProteinG: &goals.Range{Min: ptr(160.0), Max: ptr(200.0)},
		FiberG:   &goals.Range{Min: ptr(30.0)}, // min-only
	}
	require.NoError(t, repo.Upsert(ctx, date, g))

	got, err := repo.GetOverride(ctx, date)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.NotNil(t, got.Kcal)
	assert.InDelta(t, 2280, *got.Kcal.Min, 0.001)
	assert.InDelta(t, 2520, *got.Kcal.Max, 0.001)
	require.NotNil(t, got.FiberG)
	assert.Nil(t, got.FiberG.Max)
}

func TestOverridesRepo_UpsertReplacesPreviousValues(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := goals.NewOverridesRepo(pool)
	ctx := context.Background()

	date := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	first := &goals.Goals{
		Kcal:     &goals.Range{Min: ptr(2000.0), Max: ptr(2200.0)},
		ProteinG: &goals.Range{Min: ptr(150.0)},
	}
	require.NoError(t, repo.Upsert(ctx, date, first))

	// Second upsert omits protein — full-replace semantics should clear it.
	second := &goals.Goals{
		Kcal: &goals.Range{Min: ptr(2400.0), Max: ptr(2600.0)},
	}
	require.NoError(t, repo.Upsert(ctx, date, second))

	got, err := repo.GetOverride(ctx, date)
	require.NoError(t, err)
	require.NotNil(t, got.Kcal)
	assert.InDelta(t, 2400, *got.Kcal.Min, 0.001)
	assert.Nil(t, got.ProteinG, "previously-set field should be cleared")
}

func TestOverridesRepo_GetUnknownReturnsErrNotFound(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := goals.NewOverridesRepo(pool)
	_, err := repo.GetOverride(context.Background(), time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC))
	assert.ErrorIs(t, err, goals.ErrOverrideNotFound)
}

func TestOverridesRepo_DeleteRemovesRow(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := goals.NewOverridesRepo(pool)
	ctx := context.Background()
	date := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	require.NoError(t, repo.Upsert(ctx, date, &goals.Goals{
		Kcal: &goals.Range{Min: ptr(2000.0)},
	}))

	require.NoError(t, repo.Delete(ctx, date))
	_, err := repo.GetOverride(ctx, date)
	assert.ErrorIs(t, err, goals.ErrOverrideNotFound)
}

func TestOverridesRepo_DeleteUnknownReturnsErrNotFound(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := goals.NewOverridesRepo(pool)
	err := repo.Delete(context.Background(), time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC))
	assert.ErrorIs(t, err, goals.ErrOverrideNotFound)
}

func TestOverridesRepo_ListOrdersByDateAscending(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := goals.NewOverridesRepo(pool)
	ctx := context.Background()

	// Insert in a non-sorted order.
	for _, d := range []time.Time{
		time.Date(2026, 6, 17, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC),
	} {
		require.NoError(t, repo.Upsert(ctx, d, &goals.Goals{
			Kcal: &goals.Range{Min: ptr(2000.0)},
		}))
	}

	out, err := repo.List(ctx,
		time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC),
	)
	require.NoError(t, err)
	require.Len(t, out, 3)
	assert.Equal(t, "2026-06-15", out[0].Date.Format("2006-01-02"))
	assert.Equal(t, "2026-06-16", out[1].Date.Format("2006-01-02"))
	assert.Equal(t, "2026-06-17", out[2].Date.Format("2006-01-02"))
}

func TestOverridesRepo_ListEmptyWindow(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := goals.NewOverridesRepo(pool)
	out, err := repo.List(context.Background(),
		time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC),
	)
	require.NoError(t, err)
	assert.Empty(t, out)
}
