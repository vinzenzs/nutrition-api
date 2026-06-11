// Package mealplan persists planned meals — a selection for a date+slot that
// is not yet an eaten event. It sits between "recommended" (agent reasoning)
// and "logged" (a meal_entries row): picking tonight's dinner at 9am is a
// plan, and logging it future-dated would corrupt meal history and adherence.
// A planned meal becomes a real meal entry only at eating time, via the
// dedicated `eaten` transition.
package mealplan

import (
	"time"

	"github.com/google/uuid"
)

// Slots and statuses mirror the DB CHECK constraints.
const (
	SlotBreakfast = "breakfast"
	SlotLunch     = "lunch"
	SlotDinner    = "dinner"
	SlotSnack     = "snack"

	StatusPlanned = "planned"
	StatusEaten   = "eaten"
	StatusSkipped = "skipped"
)

// slotOrder ranks slots for stable range-list ordering within a date.
var slotOrder = map[string]int{
	SlotBreakfast: 0,
	SlotLunch:     1,
	SlotDinner:    2,
	SlotSnack:     3,
}

func validSlot(s string) bool { _, ok := slotOrder[s]; return ok }
func validStatus(s string) bool {
	return s == StatusPlanned || s == StatusEaten || s == StatusSkipped
}

// PlannedMeal mirrors a planned_meals row. ProductName is joined in for
// display and is not a stored column. MealEntryID is set once the entry is
// marked eaten (informational; not FK-enforced).
type PlannedMeal struct {
	ID          uuid.UUID  `json:"id"`
	PlanDate    string     `json:"plan_date"` // YYYY-MM-DD
	Slot        string     `json:"slot"`
	ProductID   uuid.UUID  `json:"product_id"`
	ProductName string     `json:"product_name,omitempty"`
	QuantityG   *float64   `json:"quantity_g,omitempty"`
	Status      string     `json:"status"`
	MealEntryID *uuid.UUID `json:"meal_entry_id,omitempty"`
	Notes       *string    `json:"notes,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

const dateLayout = "2006-01-02"
