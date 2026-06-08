// Package workoutfuel persists in-session fueling events — gels, electrolyte
// drinks, salt tabs, caffeine pills. Sibling to hydration: kept separate so
// the ml-only hydration totals don't have to carry mg/g fields, and the
// "did I drink water" question stays distinct from "did I take the right
// gels during the ride."
package workoutfuel

import (
	"time"

	"github.com/google/uuid"
)

// Entry mirrors a workout_fuel_entries row.
//
// All nutriment fields and quantity_ml are pointer-nullable so the
// "explicitly zero" (e.g. "this gel has no caffeine") vs "not measured" (NULL)
// distinction survives the round-trip — agents downstream need it for
// rehearsal-data quality.
type Entry struct {
	ID          uuid.UUID  `json:"id"`
	LoggedAt    time.Time  `json:"logged_at"`
	Name        string     `json:"name"`
	QuantityMl  *float64   `json:"quantity_ml,omitempty"`
	CarbsG      *float64   `json:"carbs_g,omitempty"`
	SodiumMg    *float64   `json:"sodium_mg,omitempty"`
	PotassiumMg *float64   `json:"potassium_mg,omitempty"`
	CaffeineMg  *float64   `json:"caffeine_mg,omitempty"`
	Note        *string    `json:"note,omitempty"`
	WorkoutID   *uuid.UUID `json:"workout_id,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}
