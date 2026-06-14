package dailycontext

import (
	"context"
	"errors"
	"fmt"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/vinzenzs/kazper/internal/bodyweight"
	"github.com/vinzenzs/kazper/internal/fitnessmetrics"
	"github.com/vinzenzs/kazper/internal/goals"
	"github.com/vinzenzs/kazper/internal/hydration"
	"github.com/vinzenzs/kazper/internal/hydrationbalance"
	"github.com/vinzenzs/kazper/internal/numfmt"
	"github.com/vinzenzs/kazper/internal/recoverymetrics"
	"github.com/vinzenzs/kazper/internal/summary"
	"github.com/vinzenzs/kazper/internal/trainingphases"
	"github.com/vinzenzs/kazper/internal/workoutfuel"
	"github.com/vinzenzs/kazper/internal/workouts"
)

// Service composes a DailyContext bundle from existing read-side primitives.
// The wide constructor is intentional — every dep is a load-bearing consumer,
// and a service-of-services indirection would obscure the shape (design §1).
type Service struct {
	hydrationRepo     *hydration.Repo
	workoutsRepo      *workouts.Repo
	workoutFuelRepo   *workoutfuel.Repo
	bodyWeightRepo    *bodyweight.Repo
	goalOverridesRepo *goals.OverridesRepo
	phasesRepo        *trainingphases.PhasesRepo
	recoveryRepo      *recoverymetrics.Repo
	fitnessRepo       *fitnessmetrics.Repo
	hydrationBalRepo  *hydrationbalance.Repo
	summarySvc        *summary.Service
}

// NewService wires all dependencies. Order matches the BuildFor fetch order
// for ease of mental mapping.
func NewService(
	summarySvc *summary.Service,
	hydrationRepo *hydration.Repo,
	workoutsRepo *workouts.Repo,
	workoutFuelRepo *workoutfuel.Repo,
	bodyWeightRepo *bodyweight.Repo,
	goalOverridesRepo *goals.OverridesRepo,
	phasesRepo *trainingphases.PhasesRepo,
	recoveryRepo *recoverymetrics.Repo,
	fitnessRepo *fitnessmetrics.Repo,
	hydrationBalRepo *hydrationbalance.Repo,
) *Service {
	return &Service{
		summarySvc:        summarySvc,
		hydrationRepo:     hydrationRepo,
		workoutsRepo:      workoutsRepo,
		workoutFuelRepo:   workoutFuelRepo,
		bodyWeightRepo:    bodyWeightRepo,
		goalOverridesRepo: goalOverridesRepo,
		phasesRepo:        phasesRepo,
		recoveryRepo:      recoveryRepo,
		fitnessRepo:       fitnessRepo,
		hydrationBalRepo:  hydrationBalRepo,
	}
}

// BuildFor returns the full daily context bundle for `date` (interpreted as
// a calendar day in `loc`). All 7 slice reads run in parallel via errgroup;
// any one failure cancels the rest and returns an error (no partial bundle —
// design decision #2).
func (s *Service) BuildFor(ctx context.Context, date time.Time, loc *time.Location) (*DailyContext, error) {
	dayStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, loc)
	dayEnd := dayStart.Add(24 * time.Hour)

	out := &DailyContext{
		Date:        date.Format("2006-01-02"),
		TZ:          loc.String(),
		Workouts:    []*WorkoutLite{},     // never nil — empty array on quiet days
		WorkoutFuel: []*WorkoutFuelLite{}, // ditto
	}

	g, gctx := errgroup.WithContext(ctx)

	// Adherence + nutrition totals via summary.DailyFor — re-uses the
	// existing resolver chain (override > phase template > default > none).
	g.Go(func() error {
		daily, err := s.summarySvc.DailyFor(gctx, summary.DailyParams{Date: date, Loc: loc})
		if err != nil {
			return fmt.Errorf("daily summary: %w", err)
		}
		out.Adherence = AdherenceBlock{
			GoalSource: daily.GoalSource,
			PhaseName:  daily.PhaseName,
			Adherence:  daily.Adherence,
		}
		out.Nutrition = NutritionBlock{
			Totals:       daily.Totals,
			EntriesCount: len(daily.Entries),
		}
		return nil
	})

	// Hydration: sum ml directly, no double-rounding through the summary
	// handler shape (design decision #4).
	g.Go(func() error {
		entries, err := s.hydrationRepo.List(gctx, dayStart.UTC(), dayEnd.UTC())
		if err != nil {
			return fmt.Errorf("hydration list: %w", err)
		}
		var total float64
		for _, e := range entries {
			total += e.QuantityMl
		}
		out.Hydration = HydrationBlock{
			TotalMl:      numfmt.Round1(total),
			EntriesCount: len(entries),
		}
		return nil
	})

	// Workouts whose started_at falls in the day's window.
	g.Go(func() error {
		ws, err := s.workoutsRepo.List(gctx, dayStart.UTC(), dayEnd.UTC(), nil, nil)
		if err != nil {
			return fmt.Errorf("workouts list: %w", err)
		}
		out.Workouts = make([]*WorkoutLite, 0, len(ws))
		for _, w := range ws {
			out.Workouts = append(out.Workouts, &WorkoutLite{
				ID:          w.ID,
				Sport:       string(w.Sport),
				StartedAt:   w.StartedAt,
				EndedAt:     w.EndedAt,
				DurationMin: numfmt.Round1(w.EndedAt.Sub(w.StartedAt).Minutes()),
				KcalBurned:  w.KcalBurned,
				Notes:       w.Notes,
			})
		}
		return nil
	})

	// Workout-fuel entries logged on the day.
	g.Go(func() error {
		fs, err := s.workoutFuelRepo.List(gctx, dayStart.UTC(), dayEnd.UTC())
		if err != nil {
			return fmt.Errorf("workout-fuel list: %w", err)
		}
		out.WorkoutFuel = make([]*WorkoutFuelLite, 0, len(fs))
		for _, f := range fs {
			out.WorkoutFuel = append(out.WorkoutFuel, &WorkoutFuelLite{
				ID:          f.ID,
				LoggedAt:    f.LoggedAt,
				Name:        f.Name,
				QuantityMl:  f.QuantityMl,
				CarbsG:      f.CarbsG,
				SodiumMg:    f.SodiumMg,
				PotassiumMg: f.PotassiumMg,
				CaffeineMg:  f.CaffeineMg,
				WorkoutID:   f.WorkoutID,
			})
		}
		return nil
	})

	// Weight: fresh entry on the day if any, else the most recent prior
	// (with is_carryover=true), else nil.
	g.Go(func() error {
		todays, err := s.bodyWeightRepo.List(gctx, dayStart.UTC(), dayEnd.UTC())
		if err != nil {
			return fmt.Errorf("bodyweight list: %w", err)
		}
		if len(todays) > 0 {
			// Most-recent same-day entry (List returns ASC, so last = latest).
			out.Weight = weightBlock(todays[len(todays)-1], false)
			return nil
		}
		prior, err := s.bodyWeightRepo.LatestBefore(gctx, dayStart.UTC())
		if err != nil {
			if errors.Is(err, bodyweight.ErrNotFound) {
				return nil // out.Weight stays nil
			}
			return fmt.Errorf("bodyweight latest-before: %w", err)
		}
		out.Weight = weightBlock(prior, true)
		return nil
	})

	// Phase covering the date (resolver picks most-recently-updated on overlap).
	g.Go(func() error {
		p, err := s.phasesRepo.PhaseFor(gctx, date)
		if err != nil {
			if errors.Is(err, trainingphases.ErrPhaseNotFound) {
				return nil
			}
			return fmt.Errorf("phase for date: %w", err)
		}
		out.Phase = &PhaseBlock{
			ID:                  p.ID,
			Name:                p.Name,
			Type:                p.Type,
			StartDate:           p.StartDate,
			EndDate:             p.EndDate,
			DefaultTemplateID:   p.DefaultTemplateID,
			DefaultTemplateName: p.DefaultTemplateName,
			Notes:               p.Notes,
		}
		return nil
	})

	// Recovery snapshot for the date (same-day-or-null, no carryover).
	dateStr := date.Format("2006-01-02")
	g.Go(func() error {
		snap, err := s.recoveryRepo.GetByDate(gctx, dateStr)
		if err != nil {
			if errors.Is(err, recoverymetrics.ErrNotFound) {
				return nil // out.Recovery stays nil
			}
			return fmt.Errorf("recovery metrics: %w", err)
		}
		out.Recovery = snap
		return nil
	})

	// Fitness snapshot for the date (same-day-or-null, no carryover).
	g.Go(func() error {
		snap, err := s.fitnessRepo.GetByDate(gctx, dateStr)
		if err != nil {
			if errors.Is(err, fitnessmetrics.ErrNotFound) {
				return nil // out.Fitness stays nil
			}
			return fmt.Errorf("fitness metrics: %w", err)
		}
		out.Fitness = snap
		return nil
	})

	// Hydration-balance snapshot for the date (same-day-or-null, no carryover).
	g.Go(func() error {
		snap, err := s.hydrationBalRepo.GetByDate(gctx, dateStr)
		if err != nil {
			if errors.Is(err, hydrationbalance.ErrNotFound) {
				return nil // out.HydrationBalance stays nil
			}
			return fmt.Errorf("hydration balance: %w", err)
		}
		out.HydrationBalance = snap
		return nil
	})

	// Goal override on the date.
	g.Go(func() error {
		ov, err := s.goalOverridesRepo.GetOverride(gctx, date)
		if err != nil {
			if errors.Is(err, goals.ErrOverrideNotFound) {
				out.GoalOverride = GoalOverrideBlock{Present: false, Goals: nil}
				return nil
			}
			return fmt.Errorf("goal override: %w", err)
		}
		out.GoalOverride = GoalOverrideBlock{Present: true, Goals: roundGoals(ov)}
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}
	return out, nil
}

// weightBlock builds a WeightBlock from a body-weight entry, rounding every
// numeric at the response boundary and echoing the smart-scale biometrics when
// present (omitempty drops the absent ones).
func weightBlock(e *bodyweight.Entry, carryover bool) *WeightBlock {
	return &WeightBlock{
		LoggedAt:     e.LoggedAt,
		WeightKg:     numfmt.Round1(e.WeightKg),
		BodyFatPct:   numfmt.Round1Ptr(e.BodyFatPct),
		MuscleMassKg: numfmt.Round1Ptr(e.MuscleMassKg),
		BodyWaterPct: numfmt.Round1Ptr(e.BodyWaterPct),
		BoneMassKg:   numfmt.Round1Ptr(e.BoneMassKg),
		BMI:          numfmt.Round1Ptr(e.BMI),
		IsCarryover:  carryover,
	}
}

// roundGoals duplicates internal/goals.roundGoals so we don't have to export
// it across packages. Cheap — 15 fields, all the same shape. If a third
// consumer surfaces, promote the helper to goals exports then.
func roundGoals(g *goals.Goals) *goals.Goals {
	if g == nil {
		return nil
	}
	out := *g
	round := func(r *goals.Range) *goals.Range {
		if r == nil {
			return nil
		}
		return &goals.Range{Min: numfmt.Round1Ptr(r.Min), Max: numfmt.Round1Ptr(r.Max)}
	}
	out.Kcal = round(g.Kcal)
	out.ProteinG = round(g.ProteinG)
	out.CarbsG = round(g.CarbsG)
	out.FatG = round(g.FatG)
	out.FiberG = round(g.FiberG)
	out.SugarG = round(g.SugarG)
	out.SaltG = round(g.SaltG)
	out.IronMg = round(g.IronMg)
	out.CalciumMg = round(g.CalciumMg)
	out.VitaminDMcg = round(g.VitaminDMcg)
	out.VitaminB12Mcg = round(g.VitaminB12Mcg)
	out.VitaminCMg = round(g.VitaminCMg)
	out.MagnesiumMg = round(g.MagnesiumMg)
	out.PotassiumMg = round(g.PotassiumMg)
	out.ZincMg = round(g.ZincMg)
	return &out
}
