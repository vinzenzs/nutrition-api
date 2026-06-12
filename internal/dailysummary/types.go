// Package dailysummary stores one whole-day energy/activity snapshot per
// calendar date (active/resting/total kcal, steps, floors, intensity minutes,
// distance) imported from a wellness source (Garmin today) via date-keyed
// upsert. Sister to recoverymetrics/fitnessmetrics; unit-isolated — these
// expenditure/activity totals are never mixed into summary's nutrition Totals
// struct nor into the Energy Availability denominator.
package dailysummary

import "time"

// Snapshot mirrors a daily_summary row. Date is the identity (one row per
// calendar day), carried as a YYYY-MM-DD string. Every metric is a nullable
// pointer so absent stays distinct from a real zero.
type Snapshot struct {
	Date                     string    `json:"date"`
	ActiveKcal               *int      `json:"active_kcal,omitempty"`
	RestingKcal              *int      `json:"resting_kcal,omitempty"`
	TotalKcal                *int      `json:"total_kcal,omitempty"`
	Steps                    *int      `json:"steps,omitempty"`
	Floors                   *int      `json:"floors,omitempty"`
	ModerateIntensityMinutes *int      `json:"moderate_intensity_minutes,omitempty"`
	VigorousIntensityMinutes *int      `json:"vigorous_intensity_minutes,omitempty"`
	DistanceM                *float64  `json:"distance_m,omitempty"`
	CreatedAt                time.Time `json:"created_at"`
	UpdatedAt                time.Time `json:"updated_at"`
}
