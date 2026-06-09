// Package dailycontext exposes GET /context/daily — a single read that
// returns the day's adherence, totals, hydration, workouts, fueling, weight,
// training-phase context, and goal-override presence in one bundle.
// Composition-only over existing primitives: no schema, no writes.
package dailycontext

import (
	"time"

	"github.com/google/uuid"

	"github.com/vinzenzs/nutrition-api/internal/goals"
	"github.com/vinzenzs/nutrition-api/internal/summary"
	"github.com/vinzenzs/nutrition-api/internal/trainingphases"
)

// DailyContext is the top-level response shape. Each sub-block re-uses the
// existing capability shapes verbatim where possible (Adherence, Totals,
// Goals) so the agent's mental model — already trained on those shapes —
// doesn't have to relearn anything.
type DailyContext struct {
	Date         string             `json:"date"`
	TZ           string             `json:"tz"`
	Adherence    AdherenceBlock     `json:"adherence"`
	Nutrition    NutritionBlock     `json:"nutrition"`
	Hydration    HydrationBlock     `json:"hydration"`
	Workouts     []*WorkoutLite     `json:"workouts"`     // never nil; empty array on quiet days
	WorkoutFuel  []*WorkoutFuelLite `json:"workout_fuel"` // never nil; empty array on quiet days
	Weight       *WeightBlock       `json:"weight"`       // nil when no entry ever logged
	Phase        *PhaseBlock        `json:"phase"`        // nil when no phase covers the date
	GoalOverride GoalOverrideBlock  `json:"goal_override"`
}

// AdherenceBlock mirrors the summary.Daily adherence + source fields.
type AdherenceBlock struct {
	GoalSource string            `json:"goal_source"`
	PhaseName  string            `json:"phase_name,omitempty"`
	Adherence  summary.Adherence `json:"adherence,omitempty"`
}

// NutritionBlock carries the day's totals plus a count. Full meal entries
// are intentionally omitted — call daily_summary for the per-entry view.
type NutritionBlock struct {
	Totals       summary.Totals `json:"totals"`
	EntriesCount int            `json:"entries_count"`
}

// HydrationBlock carries the day's total ml + entry count. Total is a
// scalar (not nullable) — zero is the meaningful empty state.
type HydrationBlock struct {
	TotalMl      float64 `json:"total_ml"`
	EntriesCount int     `json:"entries_count"`
}

// WorkoutLite is a compact projection of a workouts row. Drops fields the
// aggregator doesn't surface (avg_hr, tss, external_id) to keep the bundle
// agent-readable; the agent calls get_workout if it needs the full row.
type WorkoutLite struct {
	ID          uuid.UUID `json:"id"`
	Sport       string    `json:"sport"`
	StartedAt   time.Time `json:"started_at"`
	EndedAt     time.Time `json:"ended_at"`
	DurationMin float64   `json:"duration_min"`
	KcalBurned  *float64  `json:"kcal_burned,omitempty"`
	Notes       *string   `json:"notes,omitempty"`
}

// WorkoutFuelLite mirrors workoutfuel.Entry minus updated_at / created_at.
type WorkoutFuelLite struct {
	ID          uuid.UUID  `json:"id"`
	LoggedAt    time.Time  `json:"logged_at"`
	Name        string     `json:"name"`
	QuantityMl  *float64   `json:"quantity_ml,omitempty"`
	CarbsG      *float64   `json:"carbs_g,omitempty"`
	SodiumMg    *float64   `json:"sodium_mg,omitempty"`
	PotassiumMg *float64   `json:"potassium_mg,omitempty"`
	CaffeineMg  *float64   `json:"caffeine_mg,omitempty"`
	WorkoutID   *uuid.UUID `json:"workout_id,omitempty"`
}

// WeightBlock carries the latest body-weight reading relevant to the
// requested day. IsCarryover discriminates "fresh entry today" from
// "last seen N days ago" — the agent uses it to decide whether to nudge.
type WeightBlock struct {
	LoggedAt    time.Time `json:"logged_at"`
	WeightKg    float64   `json:"weight_kg"`
	BodyFatPct  *float64  `json:"body_fat_pct,omitempty"`
	IsCarryover bool      `json:"is_carryover"`
}

// PhaseBlock is the full phase row covering the date (resolver-picked when
// overlapping). Carries default_template_name so the agent can describe the
// period without a follow-up call.
type PhaseBlock struct {
	ID                  uuid.UUID                `json:"id"`
	Name                string                   `json:"name"`
	Type                trainingphases.PhaseType `json:"type"`
	StartDate           time.Time                `json:"start_date"`
	EndDate             time.Time                `json:"end_date"`
	DefaultTemplateID   *uuid.UUID               `json:"default_template_id,omitempty"`
	DefaultTemplateName *string                  `json:"default_template_name,omitempty"`
	Notes               *string                  `json:"notes,omitempty"`
}

// GoalOverrideBlock uses a two-field shape so the agent's check is
// `if context.goal_override.present { ... }` — stable regardless of
// whether the goals object is null or non-null.
type GoalOverrideBlock struct {
	Present bool         `json:"present"`
	Goals   *goals.Goals `json:"goals"`
}
