// Package fitnessmetrics stores one daily fitness snapshot per calendar date
// (VO2max run/bike, race predictions, acute/chronic training load). Sister to
// recoverymetrics; unit-isolated and written by an external source (Garmin
// today) via date-keyed upsert. Race predictions are whole seconds; ACWR is
// derived (acute/chronic), never stored.
package fitnessmetrics

import "time"

// Snapshot mirrors a fitness_metrics row. Date is the identity, carried as a
// YYYY-MM-DD string. Every metric is a nullable pointer.
type Snapshot struct {
	Date                     string    `json:"date"`
	VO2MaxRunning            *float64  `json:"vo2max_running,omitempty"`
	VO2MaxCycling            *float64  `json:"vo2max_cycling,omitempty"`
	RacePredictor5kSeconds   *int      `json:"race_predictor_5k_seconds,omitempty"`
	RacePredictor10kSeconds  *int      `json:"race_predictor_10k_seconds,omitempty"`
	RacePredictorHalfSeconds *int      `json:"race_predictor_half_seconds,omitempty"`
	RacePredictorFullSeconds *int      `json:"race_predictor_full_seconds,omitempty"`
	AcuteLoad                *float64  `json:"acute_load,omitempty"`
	ChronicLoad              *float64  `json:"chronic_load,omitempty"`
	EnduranceScore           *int      `json:"endurance_score,omitempty"`
	HillScore                *int      `json:"hill_score,omitempty"`
	FitnessAge               *float64  `json:"fitness_age,omitempty"`
	TrainingStatus           *string   `json:"training_status,omitempty"`
	CreatedAt                time.Time `json:"created_at"`
	UpdatedAt                time.Time `json:"updated_at"`
}
