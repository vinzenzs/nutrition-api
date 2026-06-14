package bodyweight_test

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/bodyweight"
	"github.com/vinzenzs/kazper/internal/store/storetest"
)

// Shared fixture for the resolver tests. Each test gets its own pool/repo
// because storetest.NewPool already isolates per-test schemas.
func newResolver(t *testing.T) *bodyweight.Repo {
	t.Helper()
	pool := storetest.NewPool(t)
	return bodyweight.NewRepo(pool)
}

func insertEntry(t *testing.T, repo *bodyweight.Repo, at time.Time, kg float64) {
	t.Helper()
	require.NoError(t, repo.Insert(context.Background(), &bodyweight.Entry{
		LoggedAt: at,
		WeightKg: kg,
	}))
}

func ptr(v float64) *float64 { return &v }

// ============================================================================
// Tier 1: explicit override
// ============================================================================

func TestResolveAtDate_ExplicitOverride_WinsOverEverything(t *testing.T) {
	repo := newResolver(t)
	// Stored entry that would otherwise be used.
	insertEntry(t, repo, time.Date(2026, 6, 4, 7, 0, 0, 0, time.UTC), 72.5)

	kg, source, err := bodyweight.ResolveAtDate(
		context.Background(), repo,
		time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC),
		time.UTC,
		ptr(80.0),
	)
	require.NoError(t, err)
	assert.Equal(t, 80.0, kg)
	assert.Equal(t, bodyweight.SourceExplicit, source)
}

func TestResolveAtDate_ExplicitOverride_NoRepoStillWorks(t *testing.T) {
	// Passing nil repo with an explicit override is allowed — the explicit
	// path never touches the repo.
	kg, source, err := bodyweight.ResolveAtDate(
		context.Background(), nil,
		time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC),
		time.UTC,
		ptr(80.0),
	)
	require.NoError(t, err)
	assert.Equal(t, 80.0, kg)
	assert.Equal(t, bodyweight.SourceExplicit, source)
}

func TestResolveAtDate_ExplicitOverride_InvalidReturnsErrWeightDataMissing(t *testing.T) {
	// Defensive: invalid override → ErrWeightDataMissing. Per-endpoint
	// handlers should validate before calling so they can emit a more
	// specific error code (e.g. `body_weight_kg_invalid`), but the resolver
	// itself is honest about what it can do.
	repo := newResolver(t)
	for _, v := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		_, _, err := bodyweight.ResolveAtDate(
			context.Background(), repo,
			time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC),
			time.UTC,
			&v,
		)
		assert.ErrorIs(t, err, bodyweight.ErrWeightDataMissing, "value=%v", v)
	}
}

// ============================================================================
// Tier 2: rolling 7-day mean (entries in [date-6d, date+1d) local midnights)
// ============================================================================

func TestResolveAtDate_Rolling7dMean(t *testing.T) {
	repo := newResolver(t)
	// 7-day window ending 2026-06-09 (inclusive) covers June 3-9.
	// Three entries (73, 72, 71) on June 4, 6, 8 → mean 72.
	insertEntry(t, repo, time.Date(2026, 6, 4, 7, 0, 0, 0, time.UTC), 73)
	insertEntry(t, repo, time.Date(2026, 6, 6, 7, 0, 0, 0, time.UTC), 72)
	insertEntry(t, repo, time.Date(2026, 6, 8, 7, 0, 0, 0, time.UTC), 71)

	kg, source, err := bodyweight.ResolveAtDate(
		context.Background(), repo,
		time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC),
		time.UTC,
		nil,
	)
	require.NoError(t, err)
	assert.InDelta(t, 72.0, kg, 1e-9)
	assert.Equal(t, bodyweight.SourceRolling7dAvg, source)
}

// ============================================================================
// Tier 3: last-before-date when in-window has nothing
// ============================================================================

func TestResolveAtDate_LastBeforeDate(t *testing.T) {
	repo := newResolver(t)
	// Only entry is 14 days before the query date → outside the 7-day window.
	insertEntry(t, repo, time.Date(2026, 5, 26, 7, 0, 0, 0, time.UTC), 75.0)

	kg, source, err := bodyweight.ResolveAtDate(
		context.Background(), repo,
		time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC),
		time.UTC,
		nil,
	)
	require.NoError(t, err)
	assert.Equal(t, 75.0, kg)
	assert.Equal(t, bodyweight.SourceLastBeforeDate, source)
}

// ============================================================================
// Tier 4: no data at all
// ============================================================================

func TestResolveAtDate_NoDataReturnsErrWeightDataMissing(t *testing.T) {
	repo := newResolver(t)
	_, _, err := bodyweight.ResolveAtDate(
		context.Background(), repo,
		time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC),
		time.UTC,
		nil,
	)
	assert.ErrorIs(t, err, bodyweight.ErrWeightDataMissing)
}

func TestResolveAtDate_NilRepoNoOverrideReturnsErrWeightDataMissing(t *testing.T) {
	_, _, err := bodyweight.ResolveAtDate(
		context.Background(), nil,
		time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC),
		time.UTC,
		nil,
	)
	assert.ErrorIs(t, err, bodyweight.ErrWeightDataMissing)
}

// ============================================================================
// TZ boundary — rolling window uses LOCAL midnights, not UTC.
// ============================================================================

func TestResolveAtDate_LocalMidnightBoundary(t *testing.T) {
	repo := newResolver(t)
	berlin, err := time.LoadLocation("Europe/Berlin")
	require.NoError(t, err)

	// Query date June 9 in Berlin (UTC+2 in summer):
	// 7-day window = [June 3 00:00 Berlin, June 10 00:00 Berlin)
	//              = [June 2 22:00 UTC, June 9 22:00 UTC) in UTC.
	//
	// Insert one entry at 21:00 UTC on June 9 — that's June 9 23:00 LOCAL,
	// inside the window. Another at 22:30 UTC on June 9 — that's June 10
	// 00:30 LOCAL, OUTSIDE the window.
	insertEntry(t, repo, time.Date(2026, 6, 9, 21, 0, 0, 0, time.UTC), 70)
	insertEntry(t, repo, time.Date(2026, 6, 9, 22, 30, 0, 0, time.UTC), 80)

	kg, source, err := bodyweight.ResolveAtDate(
		context.Background(), repo,
		time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC), // calendar date June 9
		berlin,
		nil,
	)
	require.NoError(t, err)
	assert.Equal(t, 70.0, kg, "only the local-window entry counts; the across-midnight one belongs to June 10 local")
	assert.Equal(t, bodyweight.SourceRolling7dAvg, source)
}
