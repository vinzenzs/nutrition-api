package goals_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/goals"
	"github.com/vinzenzs/kazper/internal/store/storetest"
)

func ptrF(v float64) *float64 { return &v }

func TestGoalsGet_EmptyReturnsNil(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := goals.NewRepo(pool)

	got, err := repo.Get(context.Background())
	require.NoError(t, err)
	assert.Nil(t, got, "Get on empty table must return (nil, nil)")
}

func TestGoalsUpsertCreatesAndReplaces(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := goals.NewRepo(pool)
	ctx := context.Background()

	first := &goals.Goals{
		Kcal:     &goals.Range{Min: ptrF(2090), Max: ptrF(2310)},
		ProteinG: &goals.Range{Min: ptrF(150), Max: ptrF(190)},
		FiberG:   &goals.Range{Min: ptrF(30)},
		SugarG:   &goals.Range{Max: ptrF(50)},
		IronMg:   &goals.Range{Min: ptrF(14)},
	}
	require.NoError(t, repo.Upsert(ctx, first))

	got, err := repo.Get(ctx)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.NotNil(t, got.Kcal)
	require.NotNil(t, got.Kcal.Min)
	require.NotNil(t, got.Kcal.Max)
	assert.InDelta(t, 2090, *got.Kcal.Min, 0.001)
	assert.InDelta(t, 2310, *got.Kcal.Max, 0.001)

	require.NotNil(t, got.ProteinG)
	require.NotNil(t, got.ProteinG.Min)
	assert.InDelta(t, 150, *got.ProteinG.Min, 0.001)
	require.NotNil(t, got.ProteinG.Max)
	assert.InDelta(t, 190, *got.ProteinG.Max, 0.001)

	require.NotNil(t, got.FiberG)
	require.NotNil(t, got.FiberG.Min)
	assert.InDelta(t, 30, *got.FiberG.Min, 0.001)
	assert.Nil(t, got.FiberG.Max, "min-only fiber stays single-bound on read")

	require.NotNil(t, got.SugarG)
	assert.Nil(t, got.SugarG.Min, "max-only sugar stays single-bound on read")
	require.NotNil(t, got.SugarG.Max)
	assert.InDelta(t, 50, *got.SugarG.Max, 0.001)

	require.NotNil(t, got.IronMg)
	require.NotNil(t, got.IronMg.Min)
	assert.InDelta(t, 14, *got.IronMg.Min, 0.001)

	// Replace-all semantics: a subsequent PUT without a previously-set field
	// must clear it.
	second := &goals.Goals{
		Kcal: &goals.Range{Min: ptrF(2280), Max: ptrF(2520)},
		// no ProteinG, no FiberG, no SugarG, no IronMg
	}
	require.NoError(t, repo.Upsert(ctx, second))

	got, err = repo.Get(ctx)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.NotNil(t, got.Kcal)
	require.NotNil(t, got.Kcal.Min)
	assert.InDelta(t, 2280, *got.Kcal.Min, 0.001)
	assert.Nil(t, got.ProteinG, "absent field on PUT should clear stored value")
	assert.Nil(t, got.FiberG)
	assert.Nil(t, got.SugarG)
	assert.Nil(t, got.IronMg)
}

func TestGoalsUpsertEnforcesSingleton(t *testing.T) {
	// We never expose a way to insert with a non-singleton id, so this verifies
	// that two Upsert calls produce exactly one row (no duplicates introduced
	// by the ON CONFLICT path).
	pool := storetest.NewPool(t)
	repo := goals.NewRepo(pool)
	ctx := context.Background()

	require.NoError(t, repo.Upsert(ctx, &goals.Goals{Kcal: &goals.Range{Min: ptrF(1900), Max: ptrF(2100)}}))
	require.NoError(t, repo.Upsert(ctx, &goals.Goals{Kcal: &goals.Range{Min: ptrF(2200), Max: ptrF(2400)}}))

	var count int
	err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM nutrition_goals`).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}
