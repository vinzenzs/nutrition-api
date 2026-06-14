// Package trainingplan makes the backend the system of record for the 18-week
// training plan: a plan owns ordered weeks, each week owns ordered day-slots,
// each slot points at a workout-template for a weekday. A materialize operation
// expands the plan into dated, planned `workouts` rows (idempotent, keyed by
// slot). Anchored optionally to a race and per-week training-phases.
package trainingplan

import (
	"time"

	"github.com/google/uuid"

	"github.com/vinzenzs/kazper/internal/workouttemplates"
)

// Plan mirrors a training_plans row. Weeks is populated only by the nested
// Load (GET /{id}); flat list/create responses leave it nil.
type Plan struct {
	ID        uuid.UUID  `json:"id"`
	Name      string     `json:"name"`
	RaceID    *uuid.UUID `json:"race_id,omitempty"`
	StartDate string     `json:"start_date"` // YYYY-MM-DD (Monday of week 1)
	Notes     *string    `json:"notes,omitempty"`
	// Methodology is curated, cited Markdown prose (Key Principles, Rowing
	// Strategy — cross-cutting, non-phase-specific reference) the coach reads,
	// distinct from the operational Notes. Stored verbatim; pushed from the vault.
	Methodology *string    `json:"methodology,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	Weeks       []PlanWeek `json:"weeks,omitempty"`
}

// PlanWeek mirrors a plan_weeks row. Slots is populated by the nested Load.
type PlanWeek struct {
	ID        uuid.UUID  `json:"id"`
	PlanID    uuid.UUID  `json:"plan_id"`
	Ordinal   int        `json:"ordinal"`
	PhaseID   *uuid.UUID `json:"phase_id,omitempty"`
	Notes     *string    `json:"notes,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	Slots     []PlanSlot `json:"slots,omitempty"`
}

// PlanSlot mirrors a plan_slots row. TimeOfDay is HH:MM:SS or nil.
type PlanSlot struct {
	ID         uuid.UUID `json:"id"`
	PlanWeekID uuid.UUID `json:"plan_week_id"`
	Weekday    int       `json:"weekday"` // 0=Mon … 6=Sun
	Ordinal    int       `json:"ordinal"`
	TemplateID uuid.UUID `json:"template_id"`
	TimeOfDay  *string   `json:"time_of_day,omitempty"`
	// TargetOverrides supersede the referenced template's step targets, matched
	// by intent, when the planned workout's effective program is resolved. At
	// most one entry per intent; nil/empty means no overrides.
	TargetOverrides []SlotTargetOverride `json:"target_overrides,omitempty"`
	// DurationOverrides supersede the referenced template's step durations,
	// matched by intent, when the planned workout's effective program is
	// resolved (and drive the materialized session length). At most one entry
	// per intent; nil/empty means no overrides.
	DurationOverrides []SlotDurationOverride `json:"duration_overrides,omitempty"`
	CreatedAt         time.Time              `json:"created_at"`
	UpdatedAt         time.Time              `json:"updated_at"`
}

// SlotTargetOverride replaces the effort target of every template step whose
// intent matches, when a planned workout's effective program is resolved. The
// Target reuses the workout-templates Target shape and validator verbatim.
type SlotTargetOverride struct {
	Intent string                  `json:"intent"`
	Target workouttemplates.Target `json:"target"`
}

// SlotDurationOverride replaces the duration of every template step whose intent
// matches, when a planned workout's effective program is resolved. The Duration
// reuses the workout-templates Duration shape and validator verbatim, restricted
// to the bounded kinds (time / distance) — see validateDurationOverrides.
type SlotDurationOverride struct {
	Intent   string                    `json:"intent"`
	Duration workouttemplates.Duration `json:"duration"`
}

// Program is a planned workout's effective program: its template steps with the
// slot's target overrides applied (per-intent). Steps is empty for a workout
// with no template.
type Program struct {
	WorkoutID uuid.UUID               `json:"workout_id"`
	Sport     string                  `json:"sport"`
	Name      *string                 `json:"name,omitempty"`
	Steps     []workouttemplates.Step `json:"steps"`
}
