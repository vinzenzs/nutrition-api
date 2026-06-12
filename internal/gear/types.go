// Package gear mirrors Garmin's gear inventory (shoes, bikes, other equipment)
// as slowly-changing, upsert-by-external-id rows — not date-keyed snapshots. It
// is coaching context for the chat agent (gear-retirement reminders) and is
// unit-isolated: gear distance never feeds any nutrition/hydration/energy total.
package gear

import (
	"time"

	"github.com/google/uuid"
)

// Type is the gear category, collapsed from Garmin's gearTypeName. Anything
// unmapped becomes "other" so an unknown type never drops the gear.
type Type string

const (
	TypeShoes Type = "shoes"
	TypeBike  Type = "bike"
	TypeOther Type = "other"
)

func ValidType(s string) bool {
	switch Type(s) {
	case TypeShoes, TypeBike, TypeOther:
		return true
	}
	return false
}

// Gear mirrors a gear row. Identity is the backend `id`; `external_id` is the
// stable Garmin gear uuid the upsert dedups on. Nullable fields are pointers
// with omitempty so absent stays distinct from a real zero.
type Gear struct {
	ID              uuid.UUID `json:"id"`
	ExternalID      string    `json:"external_id"`
	GearType        Type      `json:"gear_type"`
	DisplayName     string    `json:"display_name"`
	TotalDistanceM  *float64  `json:"total_distance_m,omitempty"`
	TotalActivities *int      `json:"total_activities,omitempty"`
	Retired         bool      `json:"retired"`
	DateBegin       *string   `json:"date_begin,omitempty"`
	DateEnd         *string   `json:"date_end,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}
