// Package recoverymetrics stores one daily recovery snapshot per calendar date
// (sleep, HRV, resting HR, stress, body battery, training readiness). Sister to
// fitnessmetrics; unit-isolated — sleep is seconds, HRV is ms, the rest are
// unitless 0–100 scores, never mixed into a shared Totals struct. Written by an
// external wellness source (Garmin today) via date-keyed upsert.
package recoverymetrics

import "time"

// Snapshot mirrors a recovery_metrics row. Date is the identity (one row per
// calendar day), carried as a YYYY-MM-DD string. Every metric is a nullable
// pointer so absent stays distinct from a real zero.
type Snapshot struct {
	Date               string    `json:"date"`
	SleepSeconds       *int      `json:"sleep_seconds,omitempty"`
	SleepScore         *int      `json:"sleep_score,omitempty"`
	HRVMs              *float64  `json:"hrv_ms,omitempty"`
	RestingHR          *int      `json:"resting_hr,omitempty"`
	StressAvg          *int      `json:"stress_avg,omitempty"`
	BodyBatteryCharged *int      `json:"body_battery_charged,omitempty"`
	BodyBatteryDrained *int      `json:"body_battery_drained,omitempty"`
	TrainingReadiness  *int      `json:"training_readiness,omitempty"`
	Spo2Avg            *int      `json:"spo2_avg,omitempty"`
	Spo2Lowest         *int      `json:"spo2_lowest,omitempty"`
	RespirationAvg     *float64  `json:"respiration_avg,omitempty"`
	RespirationLowest  *float64  `json:"respiration_lowest,omitempty"`
	DeepSleepSeconds   *int      `json:"deep_sleep_seconds,omitempty"`
	LightSleepSeconds  *int      `json:"light_sleep_seconds,omitempty"`
	RemSleepSeconds    *int      `json:"rem_sleep_seconds,omitempty"`
	AwakeSeconds       *int      `json:"awake_seconds,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}
