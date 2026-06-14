package goals_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/goals"
	"github.com/vinzenzs/kazper/internal/store/storetest"
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

func TestOverridesRepo_UpsertPatch_CreatesNewRowWhenNoneExists(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := goals.NewOverridesRepo(pool)
	ctx := context.Background()

	date := time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)
	patch := &goals.Goals{
		CarbsG: &goals.Range{Min: ptr(700.0)},
	}
	created, err := repo.UpsertPatch(ctx, date, patch)
	require.NoError(t, err)
	assert.True(t, created)

	got, err := repo.GetOverride(ctx, date)
	require.NoError(t, err)
	require.NotNil(t, got.CarbsG)
	assert.InDelta(t, 700.0, *got.CarbsG.Min, 0.001)
	assert.Nil(t, got.CarbsG.Max)
	// Every other field stays nil.
	assert.Nil(t, got.Kcal)
	assert.Nil(t, got.ProteinG)
	assert.Nil(t, got.FatG)
	assert.Nil(t, got.FiberG)
	assert.Nil(t, got.SugarG)
	assert.Nil(t, got.SaltG)
	assert.Nil(t, got.IronMg)
	assert.Nil(t, got.CalciumMg)
	assert.Nil(t, got.VitaminDMcg)
	assert.Nil(t, got.VitaminB12Mcg)
	assert.Nil(t, got.VitaminCMg)
	assert.Nil(t, got.MagnesiumMg)
	assert.Nil(t, got.PotassiumMg)
	assert.Nil(t, got.ZincMg)
}

func TestOverridesRepo_UpsertPatch_MergesIntoExistingPreservingOthers(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := goals.NewOverridesRepo(pool)
	ctx := context.Background()

	date := time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)
	// Pre-seed a training-day style override with kcal + protein.
	require.NoError(t, repo.Upsert(ctx, date, &goals.Goals{
		Kcal:     &goals.Range{Min: ptr(2090.0), Max: ptr(2310.0)},
		ProteinG: &goals.Range{Min: ptr(150.0), Max: ptr(190.0)},
	}))

	// Patch only carbs.
	created, err := repo.UpsertPatch(ctx, date, &goals.Goals{
		CarbsG: &goals.Range{Min: ptr(700.0)},
	})
	require.NoError(t, err)
	assert.False(t, created, "row already existed")

	got, err := repo.GetOverride(ctx, date)
	require.NoError(t, err)
	require.NotNil(t, got.Kcal)
	assert.InDelta(t, 2090.0, *got.Kcal.Min, 0.001)
	assert.InDelta(t, 2310.0, *got.Kcal.Max, 0.001)
	require.NotNil(t, got.ProteinG)
	assert.InDelta(t, 150.0, *got.ProteinG.Min, 0.001)
	assert.InDelta(t, 190.0, *got.ProteinG.Max, 0.001)
	require.NotNil(t, got.CarbsG)
	assert.InDelta(t, 700.0, *got.CarbsG.Min, 0.001)
	assert.Nil(t, got.CarbsG.Max)
}

func TestOverridesRepo_UpsertPatch_ReplacesPatchedField(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := goals.NewOverridesRepo(pool)
	ctx := context.Background()

	date := time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)
	require.NoError(t, repo.Upsert(ctx, date, &goals.Goals{
		CarbsG: &goals.Range{Min: ptr(500.0), Max: ptr(600.0)},
		Kcal:   &goals.Range{Min: ptr(2200.0)},
	}))

	// Patch overrides carbs (min-only — max becomes nil since patch.CarbsG
	// replaces the whole Range pointer, not field-by-field).
	created, err := repo.UpsertPatch(ctx, date, &goals.Goals{
		CarbsG: &goals.Range{Min: ptr(700.0)},
	})
	require.NoError(t, err)
	assert.False(t, created)

	got, err := repo.GetOverride(ctx, date)
	require.NoError(t, err)
	require.NotNil(t, got.CarbsG)
	assert.InDelta(t, 700.0, *got.CarbsG.Min, 0.001)
	assert.Nil(t, got.CarbsG.Max, "patch's Range replaces wholesale; the previous max is gone")
	require.NotNil(t, got.Kcal)
	assert.InDelta(t, 2200.0, *got.Kcal.Min, 0.001)
}

func TestOverridesRepo_UpsertPatch_NilPatchFieldLeavesExistingUnchanged(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := goals.NewOverridesRepo(pool)
	ctx := context.Background()

	date := time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)
	require.NoError(t, repo.Upsert(ctx, date, &goals.Goals{
		Kcal:   &goals.Range{Min: ptr(2200.0)},
		CarbsG: &goals.Range{Min: ptr(500.0)},
	}))

	// Patch contains only carbs; kcal stays untouched.
	_, err := repo.UpsertPatch(ctx, date, &goals.Goals{
		CarbsG: &goals.Range{Min: ptr(700.0)},
	})
	require.NoError(t, err)

	got, err := repo.GetOverride(ctx, date)
	require.NoError(t, err)
	require.NotNil(t, got.Kcal, "patch did not touch kcal — must remain")
	assert.InDelta(t, 2200.0, *got.Kcal.Min, 0.001)
}

func TestOverridesRepo_UpsertPatch_CoversEveryNullableField(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := goals.NewOverridesRepo(pool)
	ctx := context.Background()

	date := time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)
	// Insert via patch on an empty date — exercises every field path through
	// mergeGoalsPatch with a non-nil patch field on the patch side.
	full := &goals.Goals{
		Kcal:          &goals.Range{Min: ptr(2000.0), Max: ptr(2200.0)},
		ProteinG:      &goals.Range{Min: ptr(150.0)},
		CarbsG:        &goals.Range{Min: ptr(700.0)},
		FatG:          &goals.Range{Max: ptr(80.0)},
		FiberG:        &goals.Range{Min: ptr(30.0)},
		SugarG:        &goals.Range{Max: ptr(50.0)},
		SaltG:         &goals.Range{Max: ptr(6.0)},
		IronMg:        &goals.Range{Min: ptr(10.0)},
		CalciumMg:     &goals.Range{Min: ptr(1000.0)},
		VitaminDMcg:   &goals.Range{Min: ptr(20.0)},
		VitaminB12Mcg: &goals.Range{Min: ptr(2.4)},
		VitaminCMg:    &goals.Range{Min: ptr(90.0)},
		MagnesiumMg:   &goals.Range{Min: ptr(400.0)},
		PotassiumMg:   &goals.Range{Min: ptr(3500.0)},
		ZincMg:        &goals.Range{Min: ptr(11.0)},
	}
	_, err := repo.UpsertPatch(ctx, date, full)
	require.NoError(t, err)

	got, err := repo.GetOverride(ctx, date)
	require.NoError(t, err)
	assert.NotNil(t, got.Kcal)
	assert.NotNil(t, got.ProteinG)
	assert.NotNil(t, got.CarbsG)
	assert.NotNil(t, got.FatG)
	assert.NotNil(t, got.FiberG)
	assert.NotNil(t, got.SugarG)
	assert.NotNil(t, got.SaltG)
	assert.NotNil(t, got.IronMg)
	assert.NotNil(t, got.CalciumMg)
	assert.NotNil(t, got.VitaminDMcg)
	assert.NotNil(t, got.VitaminB12Mcg)
	assert.NotNil(t, got.VitaminCMg)
	assert.NotNil(t, got.MagnesiumMg)
	assert.NotNil(t, got.PotassiumMg)
	assert.NotNil(t, got.ZincMg)

	// Now patch carbs again on the same row — every other field must survive.
	_, err = repo.UpsertPatch(ctx, date, &goals.Goals{
		CarbsG: &goals.Range{Min: ptr(800.0)},
	})
	require.NoError(t, err)
	after, err := repo.GetOverride(ctx, date)
	require.NoError(t, err)
	assert.NotNil(t, after.Kcal)
	assert.NotNil(t, after.ProteinG)
	require.NotNil(t, after.CarbsG)
	assert.InDelta(t, 800.0, *after.CarbsG.Min, 0.001)
	assert.NotNil(t, after.FatG)
	assert.NotNil(t, after.FiberG)
	assert.NotNil(t, after.SugarG)
	assert.NotNil(t, after.SaltG)
	assert.NotNil(t, after.IronMg)
	assert.NotNil(t, after.CalciumMg)
	assert.NotNil(t, after.VitaminDMcg)
	assert.NotNil(t, after.VitaminB12Mcg)
	assert.NotNil(t, after.VitaminCMg)
	assert.NotNil(t, after.MagnesiumMg)
	assert.NotNil(t, after.PotassiumMg)
	assert.NotNil(t, after.ZincMg)
}
