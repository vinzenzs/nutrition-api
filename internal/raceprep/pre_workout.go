package raceprep

import (
	"fmt"

	"github.com/vinzenzs/kazper/internal/numfmt"
	"github.com/vinzenzs/kazper/internal/workouts"
)

// preWorkoutFor returns the pre-session fueling recommendation per the
// add-recommend-workout-fuel design § Pre-workout table. Values are scaled by
// body weight; the rationale string is agent-readable and names the bucket so
// the agent can surface "why this number" to the user verbatim.
//
// Lookup order (strength wins over zone-based rules):
//
//	strength    → 0.5 g/kg, window [30, 90] min
//	zone 5      → 1.0 g/kg, window [60, 90]  min
//	zone 4      → 2.0 g/kg, window [60, 180] min
//	zone 3      → 1.5 g/kg, window [60, 120] min
//	zone 1-2    → 1.0 g/kg, window [60, 120] min
func preWorkoutFor(sport string, zone int, bodyWeightKg float64) PreWorkout {
	if sport == string(workouts.SportStrength) {
		return PreWorkout{
			WindowMinutesBefore: [2]int{30, 90},
			CarbsG:              numfmt.Round1(0.5 * bodyWeightKg),
			CarbsGPerKg:         0.5,
			Rationale:           "Strength session — a small carb dose (0.5 g/kg) in the 30–90 min pre-window primes blood glucose without overloading the gut. Protein in the prior meal matters more than pre-session carbs for hypertrophy work.",
		}
	}

	switch zone {
	case 5:
		return PreWorkout{
			WindowMinutesBefore: [2]int{60, 90},
			CarbsG:              numfmt.Round1(1.0 * bodyWeightKg),
			CarbsGPerKg:         1.0,
			Rationale:           "Zone 5 (VO2max / sprints) — keep the pre-session dose small and close (1 g/kg, 60–90 min before) so glycogen is topped off but the gut is empty enough to handle the intensity.",
		}
	case 4:
		return PreWorkout{
			WindowMinutesBefore: [2]int{60, 180},
			CarbsG:              numfmt.Round1(2.0 * bodyWeightKg),
			CarbsGPerKg:         2.0,
			Rationale:           "Zone 4 (threshold) — sustained near-LT work depletes glycogen fast. 2 g/kg in the 1–3h pre-window fills the tank for a hard effort.",
		}
	case 3:
		return PreWorkout{
			WindowMinutesBefore: [2]int{60, 120},
			CarbsG:              numfmt.Round1(1.5 * bodyWeightKg),
			CarbsGPerKg:         1.5,
			Rationale:           fmt.Sprintf("Zone 3 (tempo) on the %s — 1.5 g/kg in the 1–2h pre-window tops off glycogen for a sustained sub-threshold effort.", sport),
		}
	default:
		// Zones 1-2 (and any unexpected fall-through).
		return PreWorkout{
			WindowMinutesBefore: [2]int{60, 120},
			CarbsG:              numfmt.Round1(1.0 * bodyWeightKg),
			CarbsGPerKg:         1.0,
			Rationale:           fmt.Sprintf("Zone 1–2 (aerobic / endurance) on the %s — fat oxidation provides most of the energy; 1 g/kg in the 1–2h pre-window is sufficient.", sport),
		}
	}
}
