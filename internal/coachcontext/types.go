// Package coachcontext exposes aggregate, composition-only read bundles the
// in-app coach grounds on before giving training or recovery advice — the
// training and recovery siblings of internal/dailycontext's nutrition bundle.
// Each endpoint fans out across existing read repos in parallel and returns one
// shape; nothing is stored and there is no migration.
package coachcontext

import (
	"time"

	"github.com/google/uuid"

	"github.com/vinzenzs/kazper/internal/athleteconfig"
	"github.com/vinzenzs/kazper/internal/fitnessmetrics"
	"github.com/vinzenzs/kazper/internal/recoverymetrics"
	"github.com/vinzenzs/kazper/internal/trainingphases"
)

// PhaseLite is the training-phase slice of the training context.
type PhaseLite struct {
	ID        uuid.UUID                `json:"id"`
	Name      string                   `json:"name"`
	Type      trainingphases.PhaseType `json:"type"`
	StartDate time.Time                `json:"start_date"`
	EndDate   time.Time                `json:"end_date"`
	// Methodology is the covering phase's curated "why" prose (Markdown, null
	// when unset), so the coach has the current phase's reasoning in the same
	// grounding call.
	Methodology *string `json:"methodology"`
}

// WorkoutLite is a compact workout for the recent/upcoming lists — enough to
// reason about load and schedule without the full splits/sets detail (use
// get_workout / list_workouts for that).
type WorkoutLite struct {
	ID          uuid.UUID `json:"id"`
	Sport       string    `json:"sport"`
	Status      string    `json:"status"`
	Name        *string   `json:"name,omitempty"`
	StartedAt   time.Time `json:"started_at"`
	EndedAt     time.Time `json:"ended_at"`
	DurationMin float64   `json:"duration_min"`
	KcalBurned  *float64  `json:"kcal_burned,omitempty"`
	TSS         *float64  `json:"tss,omitempty"`
}

// LoadSummary aggregates the recent completed workouts in the lookback window.
type LoadSummary struct {
	Count            int            `json:"count"`
	TotalDurationMin float64        `json:"total_duration_min"`
	TotalKcal        float64        `json:"total_kcal"`
	BySport          map[string]int `json:"by_sport"`
}

// TrainingContext is the GET /context/training bundle.
type TrainingContext struct {
	Date          string                   `json:"date"`
	TZ            string                   `json:"tz"`
	LookbackDays  int                      `json:"lookback_days"`
	LookaheadDays int                      `json:"lookahead_days"`
	Phase         *PhaseLite               `json:"phase"`
	Fitness       *fitnessmetrics.Snapshot `json:"fitness"`
	// ACWR is the acute:chronic load ratio, derived (acute ÷ chronic) only when
	// both loads are present; null otherwise. Never stored.
	ACWR *float64 `json:"acwr"`
	// AthleteConfig is the singleton physiology config (FTP, thresholds, HR/power
	// zones) so the coach grounds intensity advice on the athlete's zones in the
	// same call; null when no config row has been set.
	AthleteConfig *athleteconfig.AthleteConfig `json:"athlete_config"`
	// WattsPerKg is power-to-weight, derived (ftp_watts ÷ latest bodyweight kg)
	// only when both are present and bodyweight is non-zero; null otherwise.
	// Never stored.
	WattsPerKg       *float64       `json:"watts_per_kg"`
	RecentLoad       LoadSummary    `json:"recent_load"`
	RecentWorkouts   []*WorkoutLite `json:"recent_workouts"`
	UpcomingWorkouts []*WorkoutLite `json:"upcoming_workouts"`
}

// RecoveryContext is the GET /context/recovery bundle.
type RecoveryContext struct {
	Date   string                      `json:"date"`
	Days   int                         `json:"days"`
	Latest *recoverymetrics.Snapshot   `json:"latest"`
	Recent []*recoverymetrics.Snapshot `json:"recent"`
}
