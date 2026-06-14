package energy_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/bodyweight"
	"github.com/vinzenzs/kazper/internal/energy"
	"github.com/vinzenzs/kazper/internal/meals"
	"github.com/vinzenzs/kazper/internal/store/storetest"
	"github.com/vinzenzs/kazper/internal/workouts"
)

// ptr makes test bodies readable when constructing AvailabilityParams.
func ptr[T any](v T) *T { return &v }

func newSvc(t *testing.T) (*energy.Service, *bodyweight.Repo) {
	t.Helper()
	pool := storetest.NewPool(t)
	mRepo := meals.NewRepo(pool)
	wRepo := workouts.NewRepo(pool)
	bwRepo := bodyweight.NewRepo(pool)
	svc := energy.NewService(mRepo, wRepo, bwRepo)
	return svc, bwRepo
}

func insertWeight(t *testing.T, repo *bodyweight.Repo, at time.Time, kg float64, bf *float64) {
	t.Helper()
	require.NoError(t, repo.Insert(context.Background(), &bodyweight.Entry{
		LoggedAt:   at,
		WeightKg:   kg,
		BodyFatPct: bf,
	}))
}

func newParams(from, to time.Time) energy.AvailabilityParams {
	return energy.AvailabilityParams{From: from, To: to, TZ: time.UTC}
}

func TestComposition_ExplicitLeanMassWins(t *testing.T) {
	svc, bw := newSvc(t)
	from := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC)
	// Stored body-fat % that would otherwise be picked.
	insertWeight(t, bw, time.Date(2026, 6, 4, 7, 0, 0, 0, time.UTC), 72.0, ptr(18.0))

	p := newParams(from, to)
	p.LeanMassKg = ptr(58.0)

	out, err := svc.Compute(context.Background(), p)
	require.NoError(t, err)
	assert.Equal(t, 58.0, out.Composition.FFMKg)
	assert.Equal(t, energy.SourceExplicitLeanMass, out.Composition.Source)
	require.NotNil(t, out.Composition.BodyWeightKg)
	assert.Equal(t, 72.0, *out.Composition.BodyWeightKg)
}

func TestComposition_ExplicitLeanMassWorksWithNoWeightEntries(t *testing.T) {
	svc, _ := newSvc(t)
	from := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC)

	p := newParams(from, to)
	p.LeanMassKg = ptr(58.0)

	out, err := svc.Compute(context.Background(), p)
	require.NoError(t, err)
	assert.Equal(t, 58.0, out.Composition.FFMKg)
	assert.Equal(t, energy.SourceExplicitLeanMass, out.Composition.Source)
	assert.Nil(t, out.Composition.BodyWeightKg, "no weight data → body_weight_kg is null")
	assert.Nil(t, out.Composition.BodyWeightSource)
}

func TestComposition_ExplicitBodyFatOverridesStored(t *testing.T) {
	svc, bw := newSvc(t)
	from := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC)
	// Stored body-fat 18% — should be IGNORED in favour of explicit 15%.
	insertWeight(t, bw, time.Date(2026, 6, 4, 7, 0, 0, 0, time.UTC), 80.0, ptr(18.0))

	p := newParams(from, to)
	p.BodyFatPct = ptr(15.0)

	out, err := svc.Compute(context.Background(), p)
	require.NoError(t, err)
	assert.Equal(t, energy.SourceExplicitBodyFat, out.Composition.Source)
	assert.InDelta(t, 80*(1-0.15), out.Composition.FFMKg, 1e-9)
}

func TestComposition_StoredBodyFatFromMostRecentInWindow(t *testing.T) {
	svc, bw := newSvc(t)
	from := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC)
	// Older entry with 18%, newer with 16% — most recent wins.
	insertWeight(t, bw, time.Date(2026, 6, 2, 7, 0, 0, 0, time.UTC), 72.0, ptr(18.0))
	insertWeight(t, bw, time.Date(2026, 6, 6, 7, 0, 0, 0, time.UTC), 72.0, ptr(16.0))

	out, err := svc.Compute(context.Background(), newParams(from, to))
	require.NoError(t, err)
	assert.Equal(t, energy.SourceStoredBodyFat, out.Composition.Source)
	assert.InDelta(t, 72*(1-0.16), out.Composition.FFMKg, 1e-9)
}

func TestComposition_FallbackTo85PctWhenNoStoredBodyFat(t *testing.T) {
	svc, bw := newSvc(t)
	from := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC)
	insertWeight(t, bw, time.Date(2026, 6, 4, 7, 0, 0, 0, time.UTC), 80.0, nil)

	out, err := svc.Compute(context.Background(), newParams(from, to))
	require.NoError(t, err)
	assert.Equal(t, energy.SourceEstimated85pct, out.Composition.Source)
	assert.InDelta(t, 80*0.85, out.Composition.FFMKg, 1e-9)
	assert.True(t, out.Composition.CompositionEstimated, "loud flag must be set")
}

func TestComposition_WeightDataMissingReturnsError(t *testing.T) {
	svc, _ := newSvc(t)
	from := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC)
	// No weight entries; no lean_mass_kg override.

	_, err := svc.Compute(context.Background(), newParams(from, to))
	require.ErrorIs(t, err, energy.ErrWeightDataMissing)
}

func TestComposition_BodyWeightRolling7dAvg(t *testing.T) {
	svc, bw := newSvc(t)
	from := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC)
	// Three in-window entries: 73, 72, 71 — rolling 7d avg ending at `to` includes all.
	insertWeight(t, bw, time.Date(2026, 6, 2, 7, 0, 0, 0, time.UTC), 73.0, ptr(15.0))
	insertWeight(t, bw, time.Date(2026, 6, 4, 7, 0, 0, 0, time.UTC), 72.0, ptr(15.0))
	insertWeight(t, bw, time.Date(2026, 6, 6, 7, 0, 0, 0, time.UTC), 71.0, ptr(15.0))

	out, err := svc.Compute(context.Background(), newParams(from, to))
	require.NoError(t, err)
	require.NotNil(t, out.Composition.BodyWeightKg)
	assert.InDelta(t, 72.0, *out.Composition.BodyWeightKg, 1e-9, "mean of 73+72+71")
	require.NotNil(t, out.Composition.BodyWeightSource)
	assert.Equal(t, energy.BodyWeightSourceRolling7d, *out.Composition.BodyWeightSource)
}

func TestComposition_BodyWeightLastBeforeWindow(t *testing.T) {
	svc, bw := newSvc(t)
	from := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC)
	// Only entry is before the window (and outside the 7d rolling lookback before `to`).
	insertWeight(t, bw, time.Date(2026, 5, 20, 7, 0, 0, 0, time.UTC), 75.0, ptr(15.0))

	out, err := svc.Compute(context.Background(), newParams(from, to))
	require.NoError(t, err)
	require.NotNil(t, out.Composition.BodyWeightKg)
	assert.Equal(t, 75.0, *out.Composition.BodyWeightKg)
	require.NotNil(t, out.Composition.BodyWeightSource)
	assert.Equal(t, energy.BodyWeightSourceLastBefore, *out.Composition.BodyWeightSource)
}

func TestValidate_InvalidLeanMass(t *testing.T) {
	svc, _ := newSvc(t)
	from := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC)
	cases := []float64{0, -1, -100}
	for _, v := range cases {
		p := newParams(from, to)
		p.LeanMassKg = ptr(v)
		_, err := svc.Compute(context.Background(), p)
		require.ErrorIs(t, err, energy.ErrLeanMassInvalid)
	}
}

func TestValidate_InvalidBodyFat(t *testing.T) {
	svc, _ := newSvc(t)
	from := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC)
	cases := []float64{-1, 100, 101}
	for _, v := range cases {
		p := newParams(from, to)
		p.BodyFatPct = ptr(v)
		_, err := svc.Compute(context.Background(), p)
		require.ErrorIs(t, err, energy.ErrBodyFatInvalid)
	}
}

func TestValidate_InvertedWindow(t *testing.T) {
	svc, _ := newSvc(t)
	from := time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	_, err := svc.Compute(context.Background(), newParams(from, to))
	require.ErrorIs(t, err, energy.ErrWindowInvalid)
}

func TestValidate_RangeTooLarge(t *testing.T) {
	svc, _ := newSvc(t)
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC) // ~ 364 days
	_, err := svc.Compute(context.Background(), newParams(from, to))
	require.ErrorIs(t, err, energy.ErrRangeTooLarge)
}
