package raceprep

import (
	"context"
	"fmt"
	"time"

	"github.com/vinzenzs/kazper/internal/goals"
)

// ApplyRequest carries the same parameters as a carb-load Plan, with the
// difference that the apply step persists the schedule into per-date goal
// overrides. The request shape mirrors CarbLoadParams so the handler can
// build it from the same query/body params it uses for the GET endpoint.
type ApplyRequest struct {
	RaceDate          time.Time
	BodyWeightKg      float64
	DaysBefore        int
	CarbsPerKgPerDay  float64
	RaceDayCarbsPerKg float64
}

// ApplyResponse combines the carb-load schedule with a per-date outcome
// reporting whether each target row was newly created or merged into an
// existing override. The Schedule and Applied arrays share the same order
// (ascending by date) and the same length.
type ApplyResponse struct {
	RaceDate     string          `json:"race_date"`
	BodyWeightKg float64         `json:"body_weight_kg"`
	Params       EchoedParams    `json:"params"`
	Schedule     []CarbLoadEntry `json:"schedule"`
	Applied      []AppliedEntry  `json:"applied"`
}

// AppliedEntry is one row in the apply outcome. `Created` is true when the
// apply step inserted a brand-new override row on that date, false when an
// existing override was merged into.
type AppliedEntry struct {
	Date       string  `json:"date"`
	CarbsGMin  float64 `json:"carbs_g_min"`
	Created    bool    `json:"created"`
}

// ApplyCarbLoad computes the carb-load schedule and writes the per-day
// carbohydrate target into the per-date goal overrides inside a single
// transaction. The apply step writes ONLY the carbs_g bound (min-only).
// Existing kcal/protein/other bounds on the target dates are preserved.
//
// Atomicity: if any UpsertPatch fails the whole transaction is rolled back
// and zero overrides are persisted. The caller surfaces a 500 in that case.
func (s *Service) ApplyCarbLoad(ctx context.Context, req ApplyRequest) (*ApplyResponse, error) {
	if s.pool == nil {
		return nil, fmt.Errorf("apply requires a database pool — Service was constructed without one")
	}

	plan, err := s.Plan(CarbLoadParams{
		RaceDate:          req.RaceDate,
		BodyWeightKg:      req.BodyWeightKg,
		DaysBefore:        req.DaysBefore,
		CarbsPerKgPerDay:  req.CarbsPerKgPerDay,
		RaceDayCarbsPerKg: req.RaceDayCarbsPerKg,
	})
	if err != nil {
		return nil, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("apply begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	repo := goals.NewOverridesRepo(tx)
	applied := make([]AppliedEntry, 0, len(plan.Schedule))
	for _, entry := range plan.Schedule {
		date, err := time.ParseInLocation("2006-01-02", entry.Date, s.tz)
		if err != nil {
			return nil, fmt.Errorf("apply parse schedule date %q: %w", entry.Date, err)
		}
		carbs := entry.TargetCarbsG
		patch := &goals.Goals{
			CarbsG: &goals.Range{Min: &carbs},
		}
		created, err := repo.UpsertPatch(ctx, date, patch)
		if err != nil {
			return nil, fmt.Errorf("apply upsert %s: %w", entry.Date, err)
		}
		applied = append(applied, AppliedEntry{
			Date:      entry.Date,
			CarbsGMin: carbs,
			Created:   created,
		})
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("apply commit: %w", err)
	}

	return &ApplyResponse{
		RaceDate:     plan.RaceDate,
		BodyWeightKg: plan.BodyWeightKg,
		Params:       plan.Params,
		Schedule:     plan.Schedule,
		Applied:      applied,
	}, nil
}

