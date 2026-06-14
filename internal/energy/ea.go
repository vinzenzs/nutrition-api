package energy

import "github.com/vinzenzs/kazper/internal/numfmt"

// computeEA returns the Loucks EA value in kcal / kg FFM / day.
//
// Defensive: ffmKg == 0 should never happen (validateParams + resolveFFM
// guarantee it), but if it did dividing would yield ±Inf or NaN — return 0
// instead so the output stays JSON-serialisable.
func computeEA(intakeKcal, burnedKcal, ffmKg float64) float64 {
	if ffmKg == 0 {
		return 0
	}
	return (intakeKcal - burnedKcal) / ffmKg
}

// classifyBand maps an EA value to a Loucks band. Closed-low / open-high at
// both boundaries: 30.0 is sub_optimal (above the danger zone), 45.0 is
// adequate (cleared the threshold). Encourages the athlete to clear, not sit
// on, the boundary.
func classifyBand(ea float64) string {
	switch {
	case ea < LowThreshold:
		return BandLow
	case ea < AdequateThreshold:
		return BandSubOptimal
	default:
		return BandAdequate
	}
}

// buildWindow folds the per-day series into the window-level aggregate.
// avg_ea is computed across days with complete_data only — including
// missing-burn days would dilute the signal with optimistic values.
//
// Returns nil AvgEA and nil Band when zero days qualify (incomplete-data
// across the entire window is not an error; it's just an "I can't honestly
// summarise" signal).
func buildWindow(days []Day) Window {
	var sum float64
	var n int
	for _, d := range days {
		if d.CompleteData {
			sum += d.EA
			n++
		}
	}
	w := Window{
		TotalDays:            len(days),
		DaysWithCompleteData: n,
	}
	if n == 0 {
		return w
	}
	avg := numfmt.Round1(sum / float64(n))
	band := classifyBand(avg)
	w.AvgEA = &avg
	w.Band = &band
	return w
}
