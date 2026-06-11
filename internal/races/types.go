// Package races persists a race calendar (a race plus its ordered legs) and
// computes a deterministic per-leg in-event fuelling plan over it. It is the
// stateful counterpart to the stateless race-prep carb-load math: race-prep
// answers "how do I carb-load the days before", races answers "what do I take
// per leg on race day", anchored on a durable race the agent reuses.
package races

import (
	"time"

	"github.com/google/uuid"
)

// Discipline is the leg type. Intake feasibility differs per discipline — you
// cannot eat while swimming or in transition — so the fuelling math keys off it.
type Discipline string

const (
	DisciplineSwim       Discipline = "swim"
	DisciplineBike       Discipline = "bike"
	DisciplineRun        Discipline = "run"
	DisciplineTransition Discipline = "transition"
	DisciplineOther      Discipline = "other"
)

func (d Discipline) valid() bool {
	switch d {
	case DisciplineSwim, DisciplineBike, DisciplineRun, DisciplineTransition, DisciplineOther:
		return true
	default:
		return false
	}
}

// Race mirrors a races row plus its ordered legs.
type Race struct {
	ID        uuid.UUID  `json:"id"`
	Name      string     `json:"name"`
	RaceDate  string     `json:"race_date"` // YYYY-MM-DD
	RaceType  *string    `json:"race_type,omitempty"`
	Location  *string    `json:"location,omitempty"`
	Notes     *string    `json:"notes,omitempty"`
	Legs      []*RaceLeg `json:"legs"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// RaceLeg mirrors a race_legs row. DistanceM, ExpectedDurationMin and Intensity
// are nullable; legs without a duration contribute nothing to the fuelling plan.
type RaceLeg struct {
	ID                  uuid.UUID  `json:"id"`
	Ordinal             int        `json:"ordinal"`
	Discipline          Discipline `json:"discipline"`
	DistanceM           *float64   `json:"distance_m,omitempty"`
	ExpectedDurationMin *int       `json:"expected_duration_min,omitempty"`
	Intensity           *string    `json:"intensity,omitempty"`
}

const dateLayout = "2006-01-02"
