// Package workoutfueling composes meals + hydration intake into per-workout
// pre/intra/post fueling windows. Lives in its own package because both
// `meals` and `hydration` already depend on `workouts` (for the workout_id FK
// validation introduced by add-meal-workout-link), so a fueling aggregator
// inside `workouts` would create an import cycle.
package workoutfueling

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/vinzenzs/kazper/internal/hydration"
	"github.com/vinzenzs/kazper/internal/meals"
	"github.com/vinzenzs/kazper/internal/numfmt"
	"github.com/vinzenzs/kazper/internal/summary"
	"github.com/vinzenzs/kazper/internal/workoutfuel"
	"github.com/vinzenzs/kazper/internal/workouts"
)

// Window-length defaults + bounds. Documented in the workouts capability spec.
const (
	DefaultPreWindowMin  = 240
	DefaultPostWindowMin = 60
	MinWindowMin         = 0
	MaxWindowMin         = 720
)

// FuelingNutrition is the per-window nutrition contribution (meals).
type FuelingNutrition struct {
	Totals     summary.Totals `json:"totals"`
	EntryCount int            `json:"entry_count"`
}

// FuelingHydration is the per-window hydration contribution (hydration_entries).
type FuelingHydration struct {
	TotalMl    float64 `json:"total_ml"`
	EntryCount int     `json:"entry_count"`
}

// WorkoutFuelTotals is the unit-isolated summed shape for the workout-fuel
// sub-object. Each field is pointer-nullable so "no contribution" stays
// distinct from "all contributors recorded zero" — important for the
// rehearsal-data rule that omitting a measurement (NULL) differs from
// explicitly logging zero.
type WorkoutFuelTotals struct {
	QuantityMl  *float64 `json:"quantity_ml,omitempty"`
	CarbsG      *float64 `json:"carbs_g,omitempty"`
	SodiumMg    *float64 `json:"sodium_mg,omitempty"`
	PotassiumMg *float64 `json:"potassium_mg,omitempty"`
	CaffeineMg  *float64 `json:"caffeine_mg,omitempty"`
}

// FuelingWorkoutFuel is the per-window workout-fuel contribution
// (workout_fuel_entries). Sibling to the hydration sub-object; carries its
// own field shape (carbs/mg/ml) — no kcal, no per-100g nutriments.
type FuelingWorkoutFuel struct {
	Totals     WorkoutFuelTotals `json:"totals"`
	EntryCount int               `json:"entry_count"`
}

// FuelingWindow is one of pre / intra / post.
type FuelingWindow struct {
	Start       time.Time          `json:"start"`
	End         time.Time          `json:"end"`
	Minutes     int                `json:"minutes"`
	Nutrition   FuelingNutrition   `json:"nutrition"`
	Hydration   FuelingHydration   `json:"hydration"`
	WorkoutFuel FuelingWorkoutFuel `json:"workout_fuel"`
}

// WorkoutFueling is the response shape for GET /workouts/{id}/fueling.
type WorkoutFueling struct {
	WorkoutID uuid.UUID `json:"workout_id"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`

	// Rehearsal-outcome signals echoed from the underlying workout row when
	// set. Lets the agent read "perceived effort + GI + fueling totals" in
	// one call — the natural rehearsal-evaluation shape.
	RPE             *int `json:"rpe,omitempty"`
	GIDistressScore *int `json:"gi_distress_score,omitempty"`

	// Sweat/heat context echoed from the workout row when set — fueling-
	// adequacy inputs the agent compares against the windows' fluid totals.
	// The performance metrics (distance_m, avg_power_w, session_group) are
	// deliberately NOT echoed here: not fueling-judgment inputs.
	SweatLossML  *float64 `json:"sweat_loss_ml,omitempty"`
	TemperatureC *float64 `json:"temperature_c,omitempty"`

	PreWindow   FuelingWindow `json:"pre_window"`
	IntraWindow FuelingWindow `json:"intra_window"`
	PostWindow  FuelingWindow `json:"post_window"`
}

// Service composes meals + hydration + workout-fuel over a workout's
// pre/intra/post windows.
type Service struct {
	workouts    *workouts.Repo
	meals       *meals.Repo
	hydration   *hydration.Repo
	workoutFuel *workoutfuel.Repo
}

func NewService(workoutsRepo *workouts.Repo, mealsRepo *meals.Repo, hydrationRepo *hydration.Repo, workoutFuelRepo *workoutfuel.Repo) *Service {
	return &Service{workouts: workoutsRepo, meals: mealsRepo, hydration: hydrationRepo, workoutFuel: workoutFuelRepo}
}

// FueledFor returns the workout's pre/intra/post fueling aggregation. preMin
// and postMin are validated by the handler; this layer assumes them in range.
func (s *Service) FueledFor(ctx context.Context, id uuid.UUID, preMin, postMin int) (*WorkoutFueling, error) {
	w, err := s.workouts.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	preStart := w.StartedAt.Add(-time.Duration(preMin) * time.Minute)
	postEnd := w.EndedAt.Add(time.Duration(postMin) * time.Minute)

	// Pull every meal + hydration in the unioned [preStart, postEnd) window
	// once, then bucket in-memory. Two queries total instead of six.
	mealsAll, err := s.meals.List(ctx, meals.ListParams{From: preStart, To: postEnd})
	if err != nil {
		return nil, fmt.Errorf("list meals for fueling: %w", err)
	}
	hydAll, err := s.hydration.List(ctx, preStart, postEnd)
	if err != nil {
		return nil, fmt.Errorf("list hydration for fueling: %w", err)
	}
	fuelAll, err := s.workoutFuel.List(ctx, preStart, postEnd)
	if err != nil {
		return nil, fmt.Errorf("list workout-fuel for fueling: %w", err)
	}

	out := &WorkoutFueling{
		WorkoutID:       w.ID,
		StartedAt:       w.StartedAt,
		EndedAt:         w.EndedAt,
		RPE:             w.RPE,
		GIDistressScore: w.GIDistressScore,
		SweatLossML:     numfmt.Round1Ptr(w.SweatLossML),
		TemperatureC:    numfmt.Round1Ptr(w.TemperatureC),
		PreWindow:       buildWindow(preStart, w.StartedAt, preMin, mealsAll, hydAll, fuelAll),
		IntraWindow:     buildWindow(w.StartedAt, w.EndedAt, intraMinutes(w.StartedAt, w.EndedAt), mealsAll, hydAll, fuelAll),
		PostWindow:      buildWindow(w.EndedAt, postEnd, postMin, mealsAll, hydAll, fuelAll),
	}
	return out, nil
}

// intraMinutes computes the workout's actual duration in whole minutes
// (rounded to nearest integer; sufficient for the `minutes` echo field).
func intraMinutes(start, end time.Time) int {
	return int(end.Sub(start).Minutes())
}

// buildWindow filters the supplied entries to those whose logged_at falls in
// [start, end), sums their nutriments, and rounds at the response boundary.
// Half-open: end-boundary entries belong to the NEXT window, not this one.
func buildWindow(start, end time.Time, minutes int, mealsAll []*meals.MealEntry, hydAll []*hydration.Entry, fuelAll []*workoutfuel.Entry) FuelingWindow {
	var mealsIn []*meals.MealEntry
	for _, m := range mealsAll {
		if !m.LoggedAt.Before(start) && m.LoggedAt.Before(end) {
			mealsIn = append(mealsIn, m)
		}
	}
	var hydMl float64
	var hydCount int
	for _, h := range hydAll {
		if !h.LoggedAt.Before(start) && h.LoggedAt.Before(end) {
			hydMl += h.QuantityMl
			hydCount++
		}
	}
	var fuelIn []*workoutfuel.Entry
	for _, f := range fuelAll {
		if !f.LoggedAt.Before(start) && f.LoggedAt.Before(end) {
			fuelIn = append(fuelIn, f)
		}
	}

	totals := summary.SumEntries(mealsIn)
	return FuelingWindow{
		Start:   start,
		End:     end,
		Minutes: minutes,
		Nutrition: FuelingNutrition{
			Totals:     roundTotals(totals),
			EntryCount: len(mealsIn),
		},
		Hydration: FuelingHydration{
			TotalMl:    numfmt.Round1(hydMl),
			EntryCount: hydCount,
		},
		WorkoutFuel: FuelingWorkoutFuel{
			Totals:     sumWorkoutFuel(fuelIn),
			EntryCount: len(fuelIn),
		},
	}
}

// sumWorkoutFuel sums each nullable field across contributing entries.
// A field with no non-null contributor stays nil in the output; if any
// contributor supplied a value, the sum is non-nil even when zero —
// preserving the "I measured zero" signal across the aggregation. Rounds
// each non-nil result at the response boundary.
func sumWorkoutFuel(in []*workoutfuel.Entry) WorkoutFuelTotals {
	var qml, carbs, sodium, potass, caff *float64
	addFloat := func(acc, v *float64) *float64 {
		if v == nil {
			return acc
		}
		if acc == nil {
			s := *v
			return &s
		}
		s := *acc + *v
		return &s
	}
	for _, e := range in {
		qml = addFloat(qml, e.QuantityMl)
		carbs = addFloat(carbs, e.CarbsG)
		sodium = addFloat(sodium, e.SodiumMg)
		potass = addFloat(potass, e.PotassiumMg)
		caff = addFloat(caff, e.CaffeineMg)
	}
	return WorkoutFuelTotals{
		QuantityMl:  numfmt.Round1Ptr(qml),
		CarbsG:      numfmt.Round1Ptr(carbs),
		SodiumMg:    numfmt.Round1Ptr(sodium),
		PotassiumMg: numfmt.Round1Ptr(potass),
		CaffeineMg:  numfmt.Round1Ptr(caff),
	}
}

// roundTotals applies 1dp rounding to every nutrient field at the response
// boundary (consistent with summary's `rounded()` helper).
func roundTotals(t summary.Totals) summary.Totals {
	return summary.Totals{
		Kcal:          numfmt.Round1(t.Kcal),
		ProteinG:      numfmt.Round1(t.ProteinG),
		CarbsG:        numfmt.Round1(t.CarbsG),
		FatG:          numfmt.Round1(t.FatG),
		FiberG:        numfmt.Round1(t.FiberG),
		SugarG:        numfmt.Round1(t.SugarG),
		SaltG:         numfmt.Round1(t.SaltG),
		IronMg:        numfmt.Round1Ptr(t.IronMg),
		CalciumMg:     numfmt.Round1Ptr(t.CalciumMg),
		VitaminDMcg:   numfmt.Round1Ptr(t.VitaminDMcg),
		VitaminB12Mcg: numfmt.Round1Ptr(t.VitaminB12Mcg),
		VitaminCMg:    numfmt.Round1Ptr(t.VitaminCMg),
		MagnesiumMg:   numfmt.Round1Ptr(t.MagnesiumMg),
		PotassiumMg:   numfmt.Round1Ptr(t.PotassiumMg),
		ZincMg:        numfmt.Round1Ptr(t.ZincMg),
	}
}
