package raceprep_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/vinzenzs/kazper/internal/raceprep"
)

// TestIntensityFromTSS covers the closed-low / open-high band boundaries
// listed in the design table. The IF formula is
//
//   IF = sqrt(tss / (durationHr * 100))
//
// so to land on IF=0.65 with durationHr=1.0 we need tss = 0.65² × 100 = 42.25.
// Cleaner choice: durationMin=60, picking tss values that hit each boundary.
func TestIntensityFromTSS_BandBoundaries(t *testing.T) {
	cases := []struct {
		name        string
		tss         float64
		durationMin int
		wantZone    int
	}{
		// IF = 0.6 → Zone 1 (below the 0.65 boundary)
		{"IF 0.6 → Z1", 36, 60, 1},
		// IF = exactly 0.65 → Z2 (closed-low)
		{"IF 0.65 → Z2", 42.25, 60, 2},
		// IF = 0.74 → Z2 (just below 0.75)
		{"IF 0.74 → Z2", 54.76, 60, 2},
		// IF = 0.75 → Z3 (closed-low)
		{"IF 0.75 → Z3", 56.25, 60, 3},
		// IF = 0.84 → Z3 (just below 0.85)
		{"IF 0.84 → Z3", 70.56, 60, 3},
		// IF = 0.85 → Z4 (closed-low)
		{"IF 0.85 → Z4", 72.25, 60, 4},
		// IF = 0.91 → Z4 (just below 0.92)
		{"IF 0.91 → Z4", 82.81, 60, 4},
		// IF = 0.92 → Z5 (closed-low)
		{"IF 0.92 → Z5", 84.64, 60, 5},
		// IF = 1.05 (sprint) → Z5
		{"IF 1.05 → Z5", 110.25, 60, 5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tss := tc.tss
			z, defaulted := raceprep.IntensityFromTSS(&tss, tc.durationMin)
			assert.Equal(t, tc.wantZone, z)
			assert.False(t, defaulted, "non-nil TSS shouldn't default")
		})
	}
}

func TestIntensityFromTSS_NilTSSDefaultsToZ2(t *testing.T) {
	z, defaulted := raceprep.IntensityFromTSS(nil, 60)
	assert.Equal(t, 2, z)
	assert.True(t, defaulted, "nil TSS triggers the disclosure note")
}

func TestIntensityFromTSS_ZeroDurationDefaultsToZ2(t *testing.T) {
	tss := 50.0
	z, defaulted := raceprep.IntensityFromTSS(&tss, 0)
	assert.Equal(t, 2, z)
	assert.True(t, defaulted)
}
