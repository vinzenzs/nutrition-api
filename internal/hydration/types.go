package hydration

import (
	"time"

	"github.com/google/uuid"
)

// Entry mirrors a hydration_entries row.
type Entry struct {
	ID         uuid.UUID  `json:"id"`
	LoggedAt   time.Time  `json:"logged_at"`
	QuantityMl float64    `json:"quantity_ml"`
	Note       *string    `json:"note,omitempty"`
	WorkoutID  *uuid.UUID `json:"workout_id,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}
