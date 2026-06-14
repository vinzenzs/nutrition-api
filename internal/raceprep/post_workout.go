package raceprep

import "github.com/vinzenzs/kazper/internal/numfmt"

// postWorkoutFor returns the recovery-window recommendation. The 0.3 g/kg
// protein factor is the same MPS threshold add-protein-distribution uses to
// flag `mps_effective: true` on per-meal rows — a single literature constant
// across both endpoints so a user who hits this recommendation automatically
// hits the per-meal MPS bar in the daily distribution view.
//
// Sport/zone-agnostic: the glycogen-replenishment window (0–60 min, 1 g/kg)
// + the MPS-trigger protein dose (0.3 g/kg) don't materially differentiate by
// modality at the response granularity the API offers.
func postWorkoutFor(bodyWeightKg float64) PostWorkout {
	return PostWorkout{
		WindowMinutesAfter: [2]int{0, 60},
		CarbsG:             numfmt.Round1(1.0 * bodyWeightKg),
		ProteinG:           numfmt.Round1(0.3 * bodyWeightKg),
		Rationale:          "Recovery window: 1 g/kg CHO inside 60 min maximises glycogen replenishment when another hard session is < 24h away. 0.3 g/kg protein hits the per-meal MPS threshold (same factor protein_distribution uses for `mps_effective`).",
	}
}
