// Package energy computes Energy Availability (EA) over a date window from
// existing primitives — meals (intake), workouts (exercise burn), and
// body-weight entries (composition). Pure read; no schema; no migration.
//
// Loucks bands (kcal / kg FFM / day): < 30 low, 30-45 sub_optimal, >= 45 adequate.
// Closes T1 #4 in openspec/priorities.md.
package energy

import (
	"time"

	"github.com/google/uuid"
)

// Band constants (Loucks 2007 / 2018 IOC consensus).
const (
	BandLow        = "low"
	BandSubOptimal = "sub_optimal"
	BandAdequate   = "adequate"

	LowThreshold      = 30.0 // ea < 30 → low
	AdequateThreshold = 45.0 // ea >= 45 → adequate
)

// AvailabilityParams is the validated, parsed input to Service.Compute.
//
// Window is half-open [From, To). LeanMassKg and BodyFatPct are explicit
// composition overrides; both nil → the service falls back to stored data
// then to the 85 % FFM heuristic.
type AvailabilityParams struct {
	From       time.Time
	To         time.Time
	TZ         *time.Location
	LeanMassKg *float64
	BodyFatPct *float64
}

// Composition reports the resolved FFM and how it was determined, so the agent
// can frame "low-confidence EA" appropriately when only an estimate was used.
type Composition struct {
	FFMKg                float64  `json:"ffm_kg"`
	Source               string   `json:"source"`
	BodyWeightKg         *float64 `json:"body_weight_kg"`
	BodyWeightSource     *string  `json:"body_weight_source"`
	CompositionEstimated bool     `json:"composition_estimated,omitempty"`
}

// Composition.Source constants.
const (
	SourceExplicitLeanMass = "explicit_lean_mass"
	SourceExplicitBodyFat  = "explicit_body_fat"
	SourceStoredBodyFat    = "stored_body_fat"
	SourceEstimated85pct   = "estimated_85pct"

	BodyWeightSourceRolling7d   = "rolling_7d_avg"
	BodyWeightSourceInWindow    = "in_window_mean"
	BodyWeightSourceLastBefore  = "last_before_window"
)

// Day is one calendar-day row.
//
// `ExerciseEnergyKcal` is the formal name (matches the Loucks formula); the
// JSON tag matches what the spec example shows. Missing-burn workouts go
// into MissingBurnWorkoutIDs and contribute zero to the day's burn total.
type Day struct {
	Date                  string      `json:"date"`
	IntakeKcal            float64     `json:"intake_kcal"`
	ExerciseEnergyKcal    float64     `json:"exercise_energy_kcal"`
	EA                    float64     `json:"ea"`
	Band                  string      `json:"band"`
	MissingBurnWorkoutIDs []uuid.UUID `json:"missing_burn_workout_ids"`
	CompleteData          bool        `json:"complete_data"`
}

// Window is the aggregate across days with complete data only.
//
// AvgEA / Band are nullable: when every day in the window has missing burn
// data, the headline number is omitted rather than computed from a biased
// sample.
type Window struct {
	AvgEA                *float64 `json:"avg_ea"`
	Band                 *string  `json:"band"`
	DaysWithCompleteData int      `json:"days_with_complete_data"`
	TotalDays            int      `json:"total_days"`
}

// Availability is the response shape for GET /energy/availability.
type Availability struct {
	From        time.Time   `json:"from"`
	To          time.Time   `json:"to"`
	TZ          string      `json:"tz"`
	Days        []Day       `json:"days"`
	Window      Window      `json:"window"`
	Composition Composition `json:"composition"`
}
