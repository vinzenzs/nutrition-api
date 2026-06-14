package energy

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/vinzenzs/kazper/internal/bodyweight"
	"github.com/vinzenzs/kazper/internal/meals"
	"github.com/vinzenzs/kazper/internal/workouts"
)

// Validation errors map 1:1 to API error codes.
var (
	ErrLeanMassInvalid   = errors.New("lean_mass_kg_invalid")
	ErrBodyFatInvalid    = errors.New("body_fat_pct_invalid")
	ErrWeightDataMissing = errors.New("weight_data_missing")
	ErrWindowInvalid     = errors.New("window_invalid")
	ErrRangeTooLarge     = errors.New("range_too_large")
)

const maxWindowDays = 92

// Service composes meals + workouts + body-weight into per-day EA values.
type Service struct {
	meals      *meals.Repo
	workouts   *workouts.Repo
	bodyWeight *bodyweight.Repo
}

func NewService(mealsRepo *meals.Repo, workoutsRepo *workouts.Repo, bwRepo *bodyweight.Repo) *Service {
	return &Service{meals: mealsRepo, workouts: workoutsRepo, bodyWeight: bwRepo}
}

// Compute returns the per-day EA breakdown + window aggregate. Validates the
// input first; resolves composition (FFM + body weight) per the documented
// rules; then aggregates meals + workouts into calendar-day buckets in
// params.TZ.
func (s *Service) Compute(ctx context.Context, params AvailabilityParams) (*Availability, error) {
	if err := validateParams(params); err != nil {
		return nil, err
	}

	comp, err := s.resolveComposition(ctx, params)
	if err != nil {
		return nil, err
	}

	// Pull every meal + workout in [from, to) once, bucket in-memory. Only
	// COMPLETED workouts count toward energy expenditure — a planned session
	// has no kcal_burned and would otherwise be flagged missing-burn and
	// exclude its day from the window aggregate (add-garmin-daily-metrics).
	mealsAll, err := s.meals.List(ctx, meals.ListParams{From: params.From, To: params.To})
	if err != nil {
		return nil, err
	}
	completed := string(workouts.StatusCompleted)
	workoutsAll, err := s.workouts.List(ctx, params.From, params.To, nil, &completed)
	if err != nil {
		return nil, err
	}

	days := buildDays(params.From, params.To, params.TZ, mealsAll, workoutsAll, comp.FFMKg)
	window := buildWindow(days)

	return &Availability{
		From:        params.From,
		To:          params.To,
		TZ:          params.TZ.String(),
		Days:        days,
		Window:      window,
		Composition: *comp,
	}, nil
}

func validateParams(p AvailabilityParams) error {
	if p.From.IsZero() || p.To.IsZero() || !p.From.Before(p.To) {
		return ErrWindowInvalid
	}
	if p.To.Sub(p.From) > time.Duration(maxWindowDays)*24*time.Hour {
		return ErrRangeTooLarge
	}
	if p.LeanMassKg != nil {
		v := *p.LeanMassKg
		if math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 {
			return ErrLeanMassInvalid
		}
	}
	if p.BodyFatPct != nil {
		v := *p.BodyFatPct
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v >= 100 {
			return ErrBodyFatInvalid
		}
	}
	if p.TZ == nil {
		// Defensive — the handler should never pass a nil location, but if
		// it did the bucketing would panic. Treat as window_invalid.
		return ErrWindowInvalid
	}
	return nil
}
