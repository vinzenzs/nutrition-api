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

func setupResolver(t *testing.T) (*goals.Resolver, *goals.Repo, *goals.OverridesRepo) {
	t.Helper()
	pool := storetest.NewPool(t)
	defaults := goals.NewRepo(pool)
	overrides := goals.NewOverridesRepo(pool)
	return goals.NewResolver(defaults, overrides), defaults, overrides
}

func TestEffectiveFor_OverrideWins(t *testing.T) {
	res, defaults, overrides := setupResolver(t)
	ctx := context.Background()

	require.NoError(t, defaults.Upsert(ctx, &goals.Goals{
		Kcal: &goals.Range{Min: ptr(2090.0), Max: ptr(2310.0)},
	}))
	date := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	require.NoError(t, overrides.Upsert(ctx, date, &goals.Goals{
		Kcal: &goals.Range{Min: ptr(2280.0), Max: ptr(2520.0)},
	}))

	g, source, err := res.EffectiveFor(ctx, date)
	require.NoError(t, err)
	assert.Equal(t, goals.GoalSourceOverride, source)
	require.NotNil(t, g.Kcal)
	assert.InDelta(t, 2280, *g.Kcal.Min, 0.001)
}

func TestEffectiveFor_DefaultUsedWhenNoOverride(t *testing.T) {
	res, defaults, _ := setupResolver(t)
	ctx := context.Background()
	require.NoError(t, defaults.Upsert(ctx, &goals.Goals{
		Kcal: &goals.Range{Min: ptr(2090.0), Max: ptr(2310.0)},
	}))

	g, source, err := res.EffectiveFor(ctx, time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	assert.Equal(t, goals.GoalSourceDefault, source)
	require.NotNil(t, g.Kcal)
	assert.InDelta(t, 2090, *g.Kcal.Min, 0.001)
}

func TestEffectiveFor_NeitherReturnsNone(t *testing.T) {
	res, _, _ := setupResolver(t)
	g, source, err := res.EffectiveFor(context.Background(),
		time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	assert.Nil(t, g)
	assert.Equal(t, goals.GoalSourceNone, source)
}

func TestEffectiveForRange_MixesAllSources(t *testing.T) {
	res, defaults, overrides := setupResolver(t)
	ctx := context.Background()
	require.NoError(t, defaults.Upsert(ctx, &goals.Goals{
		Kcal: &goals.Range{Min: ptr(2090.0), Max: ptr(2310.0)},
	}))
	// Override only on June 16.
	require.NoError(t, overrides.Upsert(ctx,
		time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC),
		&goals.Goals{Kcal: &goals.Range{Min: ptr(2400.0)}}))

	effective, sources, err := res.EffectiveForRange(ctx,
		time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 6, 17, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.Len(t, sources, 3)
	assert.Equal(t, goals.GoalSourceDefault, sources["2026-06-15"])
	assert.Equal(t, goals.GoalSourceOverride, sources["2026-06-16"])
	assert.Equal(t, goals.GoalSourceDefault, sources["2026-06-17"])
	// June 16 uses the override's kcal.
	require.NotNil(t, effective["2026-06-16"].Kcal)
	assert.InDelta(t, 2400, *effective["2026-06-16"].Kcal.Min, 0.001)
}

func TestEffectiveForRange_NoDefaultAndNoOverrideReturnsNone(t *testing.T) {
	res, _, _ := setupResolver(t)
	effective, sources, err := res.EffectiveForRange(context.Background(),
		time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.Len(t, sources, 2)
	assert.Equal(t, goals.GoalSourceNone, sources["2026-06-15"])
	assert.Equal(t, goals.GoalSourceNone, sources["2026-06-16"])
	assert.Nil(t, effective["2026-06-15"])
	assert.Nil(t, effective["2026-06-16"])
}
