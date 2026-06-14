package bodyweight_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/bodyweight"
	"github.com/vinzenzs/kazper/internal/store/storetest"
)

func setupSvc(t *testing.T) *bodyweight.Service {
	t.Helper()
	pool := storetest.NewPool(t)
	repo := bodyweight.NewRepo(pool)
	return bodyweight.NewService(repo)
}

// seed inserts entries via the service so validation paths are exercised.
func seed(t *testing.T, svc *bodyweight.Service, samples ...sample) {
	t.Helper()
	for _, s := range samples {
		_, err := svc.Create(context.Background(), bodyweight.CreateInput{
			WeightKg: s.kg,
			LoggedAt: s.at,
		})
		require.NoError(t, err)
	}
}

type sample struct {
	at time.Time
	kg float64
}

func TestTrendFor_ThreeConsecutiveDaysWindow3(t *testing.T) {
	svc := setupSvc(t)
	loc := time.UTC

	// Three samples on consecutive days, morning UTC.
	seed(t, svc,
		sample{time.Date(2026, 6, 5, 7, 0, 0, 0, loc), 73.1},
		sample{time.Date(2026, 6, 6, 7, 0, 0, 0, loc), 72.4},
		sample{time.Date(2026, 6, 7, 7, 0, 0, 0, loc), 73.6},
	)

	tr, err := svc.TrendFor(context.Background(), bodyweight.TrendParams{
		From:       time.Date(2026, 6, 7, 0, 0, 0, 0, loc),
		To:         time.Date(2026, 6, 7, 0, 0, 0, 0, loc),
		Loc:        loc,
		WindowDays: 3,
	})
	require.NoError(t, err)
	require.Len(t, tr.Points, 1)
	p := tr.Points[0]
	assert.Equal(t, "2026-06-07", p.Date)
	assert.Equal(t, 3, p.SampleCount)
	require.NotNil(t, p.RollingAvgKg)
	// mean(73.1, 72.4, 73.6) = 73.0333… → 73.0 rounded to 1dp
	assert.InDelta(t, 73.0, *p.RollingAvgKg, 0.001)
}

func TestTrendFor_SparseWindowReportsSampleCount1(t *testing.T) {
	svc := setupSvc(t)
	loc := time.UTC
	seed(t, svc, sample{time.Date(2026, 6, 7, 7, 0, 0, 0, loc), 72.5})

	tr, err := svc.TrendFor(context.Background(), bodyweight.TrendParams{
		From:       time.Date(2026, 6, 7, 0, 0, 0, 0, loc),
		To:         time.Date(2026, 6, 7, 0, 0, 0, 0, loc),
		Loc:        loc,
		WindowDays: 7,
	})
	require.NoError(t, err)
	require.Len(t, tr.Points, 1)
	assert.Equal(t, 1, tr.Points[0].SampleCount)
	require.NotNil(t, tr.Points[0].RollingAvgKg)
	assert.Equal(t, 72.5, *tr.Points[0].RollingAvgKg)
}

func TestTrendFor_EmptyWindowNullAvgZeroCount(t *testing.T) {
	svc := setupSvc(t)
	loc := time.UTC

	tr, err := svc.TrendFor(context.Background(), bodyweight.TrendParams{
		From:       time.Date(2026, 6, 7, 0, 0, 0, 0, loc),
		To:         time.Date(2026, 6, 9, 0, 0, 0, 0, loc),
		Loc:        loc,
		WindowDays: 7,
	})
	require.NoError(t, err)
	require.Len(t, tr.Points, 3, "three points for three days even with no data")
	for _, p := range tr.Points {
		assert.Equal(t, 0, p.SampleCount)
		assert.Nil(t, p.RollingAvgKg)
	}
}

func TestTrendFor_TwoSamplesSameDayWindow1(t *testing.T) {
	svc := setupSvc(t)
	loc := time.UTC

	// Morning and evening on the same date.
	seed(t, svc,
		sample{time.Date(2026, 6, 7, 7, 0, 0, 0, loc), 72.0},
		sample{time.Date(2026, 6, 7, 20, 0, 0, 0, loc), 72.6},
	)

	tr, err := svc.TrendFor(context.Background(), bodyweight.TrendParams{
		From:       time.Date(2026, 6, 7, 0, 0, 0, 0, loc),
		To:         time.Date(2026, 6, 7, 0, 0, 0, 0, loc),
		Loc:        loc,
		WindowDays: 1,
	})
	require.NoError(t, err)
	require.Len(t, tr.Points, 1)
	assert.Equal(t, 2, tr.Points[0].SampleCount)
	require.NotNil(t, tr.Points[0].RollingAvgKg)
	// mean(72.0, 72.6) = 72.3
	assert.Equal(t, 72.3, *tr.Points[0].RollingAvgKg)
}

func TestTrendFor_Rounding(t *testing.T) {
	svc := setupSvc(t)
	loc := time.UTC

	// Three samples whose mean is 73.4666… → expect 73.5
	seed(t, svc,
		sample{time.Date(2026, 6, 5, 7, 0, 0, 0, loc), 73.40},
		sample{time.Date(2026, 6, 6, 7, 0, 0, 0, loc), 73.40},
		sample{time.Date(2026, 6, 7, 7, 0, 0, 0, loc), 73.60},
	)

	tr, err := svc.TrendFor(context.Background(), bodyweight.TrendParams{
		From:       time.Date(2026, 6, 7, 0, 0, 0, 0, loc),
		To:         time.Date(2026, 6, 7, 0, 0, 0, 0, loc),
		Loc:        loc,
		WindowDays: 3,
	})
	require.NoError(t, err)
	require.Len(t, tr.Points, 1)
	require.NotNil(t, tr.Points[0].RollingAvgKg)
	// (73.40 + 73.40 + 73.60) / 3 = 73.4666... → 73.5 rounded
	assert.Equal(t, 73.5, *tr.Points[0].RollingAvgKg)
}
