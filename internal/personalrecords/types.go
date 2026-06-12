// Package personalrecords mirrors Garmin's personal records (fastest 5k/10k,
// longest ride, …) as slowly-changing, upsert-by-external-id rows — not
// date-keyed snapshots. It is coaching context for the chat agent ("you're
// PR-fit right now") and is unit-isolated: PR values never feed any
// nutrition/hydration/energy total.
package personalrecords

import (
	"time"

	"github.com/google/uuid"
)

// PersonalRecord mirrors a personal_records row. Identity is the backend `id`;
// `external_id` is the stable Garmin PR id the upsert dedups on. `value` carries
// an accompanying `unit` note (e.g. `s` for a time, `m` for a distance) so the
// agent can render it without a hard-coded PR-type lookup.
type PersonalRecord struct {
	ID         uuid.UUID `json:"id"`
	ExternalID string    `json:"external_id"`
	PRType     string    `json:"pr_type"`
	Value      *float64  `json:"value"`
	Unit       string    `json:"unit"`
	ActivityID *string   `json:"activity_id,omitempty"`
	AchievedAt time.Time `json:"achieved_at"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
