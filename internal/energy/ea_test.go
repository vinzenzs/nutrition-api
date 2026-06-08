package energy

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestComputeEA(t *testing.T) {
	cases := []struct {
		name   string
		intake float64
		burn   float64
		ffm    float64
		want   float64
	}{
		{"adequate", 2400, 600, 60, 30},
		{"high adequate", 3000, 500, 60, (3000 - 500) / 60.0},
		{"low", 1800, 600, 60, 20},
		{"zero ffm (defensive)", 2000, 0, 0, 0},
		{"all zeros", 0, 0, 60, 0},
		{"negative ea (rest day, high burn)", 0, 1800, 60, -30},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := computeEA(tc.intake, tc.burn, tc.ffm)
			if math.IsNaN(got) || math.IsInf(got, 0) {
				t.Fatalf("expected finite EA, got %v", got)
			}
			assert.InDelta(t, tc.want, got, 1e-9)
		})
	}
}

func TestClassifyBand(t *testing.T) {
	cases := []struct {
		ea   float64
		want string
		desc string
	}{
		{0.0, BandLow, "zero falls in low"},
		{29.9, BandLow, "just below 30 is low"},
		{29.999, BandLow, "epsilon below 30 is low"},
		{30.0, BandSubOptimal, "exactly 30 is sub_optimal (closed-low boundary)"},
		{37.5, BandSubOptimal, "middle of sub_optimal band"},
		{44.9, BandSubOptimal, "just below 45 is sub_optimal"},
		{45.0, BandAdequate, "exactly 45 is adequate (open-high boundary on sub_optimal)"},
		{60.0, BandAdequate, "well into adequate"},
		{-10.0, BandLow, "negative EA is low (rest day with high burn)"},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			assert.Equal(t, tc.want, classifyBand(tc.ea))
		})
	}
}

func TestBuildWindow_OnlyCompleteDaysCounted(t *testing.T) {
	days := []Day{
		{EA: 25, CompleteData: true},  // low
		{EA: 40, CompleteData: true},  // sub_optimal
		{EA: 50, CompleteData: false}, // incomplete — excluded
		{EA: 35, CompleteData: true},  // sub_optimal
	}
	w := buildWindow(days)
	assert.Equal(t, 4, w.TotalDays)
	assert.Equal(t, 3, w.DaysWithCompleteData)
	if assert.NotNil(t, w.AvgEA) {
		// (25 + 40 + 35) / 3 = 33.333... → round to 33.3
		assert.InDelta(t, 33.3, *w.AvgEA, 1e-9)
	}
	if assert.NotNil(t, w.Band) {
		assert.Equal(t, BandSubOptimal, *w.Band)
	}
}

func TestBuildWindow_NoCompleteDaysReturnsNilAggregate(t *testing.T) {
	days := []Day{
		{EA: 25, CompleteData: false},
		{EA: 40, CompleteData: false},
	}
	w := buildWindow(days)
	assert.Equal(t, 2, w.TotalDays)
	assert.Equal(t, 0, w.DaysWithCompleteData)
	assert.Nil(t, w.AvgEA, "headline number omitted when no day qualifies")
	assert.Nil(t, w.Band)
}

func TestBuildWindow_EmptyDaysSlice(t *testing.T) {
	w := buildWindow(nil)
	assert.Equal(t, 0, w.TotalDays)
	assert.Equal(t, 0, w.DaysWithCompleteData)
	assert.Nil(t, w.AvgEA)
	assert.Nil(t, w.Band)
}
