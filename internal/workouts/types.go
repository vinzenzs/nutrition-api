package workouts

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Source records workout provenance: which writer pushed the row.
type Source string

const (
	SourceGarmin Source = "garmin"
	SourceManual Source = "manual"
	SourceOther  Source = "other"
)

func ValidSource(s string) bool {
	switch Source(s) {
	case SourceGarmin, SourceManual, SourceOther:
		return true
	}
	return false
}

func ParseSource(s string) (Source, error) {
	if !ValidSource(s) {
		return "", fmt.Errorf("invalid source %q", s)
	}
	return Source(s), nil
}

// Sport is the activity category. Kept deliberately small in v1; extend the
// enum when a specific sport's fueling tools earn the surface.
type Sport string

const (
	SportRun      Sport = "run"
	SportBike     Sport = "bike"
	SportSwim     Sport = "swim"
	SportStrength Sport = "strength"
	SportOther    Sport = "other"
)

func ValidSport(s string) bool {
	switch Sport(s) {
	case SportRun, SportBike, SportSwim, SportStrength, SportOther:
		return true
	}
	return false
}

func ParseSport(s string) (Sport, error) {
	if !ValidSport(s) {
		return "", fmt.Errorf("invalid sport %q", s)
	}
	return Sport(s), nil
}

// Workout mirrors a workouts row.
type Workout struct {
	ID         uuid.UUID `json:"id"`
	ExternalID *string   `json:"external_id,omitempty"`
	Source     Source    `json:"source"`
	Sport      Sport     `json:"sport"`
	Name       *string   `json:"name,omitempty"`

	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`

	KcalBurned *float64 `json:"kcal_burned,omitempty"`
	AvgHR      *int     `json:"avg_hr,omitempty"`
	TSS        *float64 `json:"tss,omitempty"`
	Notes      *string  `json:"notes,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
