package races

import (
	"fmt"
	"math"

	"github.com/google/uuid"
)

// Fuelling model constants. See design.md for the evidence anchoring; these
// are deterministic baselines the agent dials against (weather, gut tolerance).
const (
	carbsShortGPerHr  = 0.0  // total race effort < 75 min: glycogen suffices
	carbsMidGPerHr    = 60.0 // [75, 150) min
	carbsLongGPerHr   = 90.0 // >= 150 min
	carbsMidMinutes   = 75
	carbsLongMinutes  = 150
	defaultFluidMlHr  = 600.0  // when no sweat rate supplied
	defaultSodiumMgHr = 600.0  // when no sweat rate supplied
	fluidCapMlHr      = 1000.0 // practical absorption ceiling
	sweatSodiumMgPerL = 800.0  // mid of 0.5–1.2 g/L literature
)

// FuelingParams are the athlete inputs supplied at read time.
type FuelingParams struct {
	BodyWeightKg     float64
	SweatRateMlPerHr *float64
}

func (p FuelingParams) validate() error {
	if p.BodyWeightKg <= 0 {
		return ErrBodyWeightRequired
	}
	if math.IsNaN(p.BodyWeightKg) || math.IsInf(p.BodyWeightKg, 0) ||
		p.BodyWeightKg < bodyWeightKgMin || p.BodyWeightKg > bodyWeightKgMax {
		return ErrBodyWeightRange
	}
	if p.SweatRateMlPerHr != nil {
		v := *p.SweatRateMlPerHr
		if math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 || v > sweatRateMlMax {
			return ErrSweatRateRange
		}
	}
	return nil
}

// LegFuelingPlan is the computed plan for one leg. Carbs/sodium/fluid are kept
// in distinct unit families (_g / _mg / _ml) — never merged (unit isolation).
type LegFuelingPlan struct {
	Ordinal             int        `json:"ordinal"`
	Discipline          Discipline `json:"discipline"`
	ExpectedDurationMin *int       `json:"expected_duration_min,omitempty"`
	CarbsGPerHr         float64    `json:"carbs_g_per_hr"`
	CarbsGTotal         float64    `json:"carbs_g_total"`
	SodiumMgPerHr       float64    `json:"sodium_mg_per_hr"`
	SodiumMgTotal       float64    `json:"sodium_mg_total"`
	FluidMlPerHr        float64    `json:"fluid_ml_per_hr"`
	FluidMlTotal        float64    `json:"fluid_ml_total"`
	Rationale           string     `json:"rationale"`
}

// FuelTotals are the race-level sums, kept unit-isolated.
type FuelTotals struct {
	CarbsGTotal   float64 `json:"carbs_g_total"`
	SodiumMgTotal float64 `json:"sodium_mg_total"`
	FluidMlTotal  float64 `json:"fluid_ml_total"`
}

// FuelingPlan is the response of GET /races/{id}/fueling-plan.
type FuelingPlan struct {
	RaceID           uuid.UUID         `json:"race_id"`
	RaceName         string            `json:"race_name"`
	RaceDate         string            `json:"race_date"`
	BodyWeightKg     float64           `json:"body_weight_kg"`
	SweatRateMlPerHr *float64          `json:"sweat_rate_ml_per_hr,omitempty"`
	TotalDurationMin int               `json:"total_duration_min"`
	Legs             []*LegFuelingPlan `json:"legs"`
	Total            FuelTotals        `json:"total"`
}

func baseCarbsGPerHr(totalMin int) float64 {
	switch {
	case totalMin >= carbsLongMinutes:
		return carbsLongGPerHr
	case totalMin >= carbsMidMinutes:
		return carbsMidGPerHr
	default:
		return carbsShortGPerHr
	}
}

// disciplineCarbFactor scales the carb baseline by intake capacity.
func disciplineCarbFactor(d Discipline) float64 {
	switch d {
	case DisciplineBike:
		return 1.0
	case DisciplineRun:
		return 0.7
	case DisciplineOther:
		return 0.8
	default: // swim, transition
		return 0.0
	}
}

// canIngest reports whether fluid/sodium intake is feasible during the leg.
func canIngest(d Discipline) bool {
	return d != DisciplineSwim && d != DisciplineTransition
}

// ComputeFueling builds the per-leg fuelling plan for a race. Pure — no I/O.
func ComputeFueling(race *Race, p FuelingParams) *FuelingPlan {
	totalMin := 0
	for _, leg := range race.Legs {
		if leg.ExpectedDurationMin != nil {
			totalMin += *leg.ExpectedDurationMin
		}
	}
	baseCarbs := baseCarbsGPerHr(totalMin)

	fluidBase, fluidDefaulted := defaultFluidMlHr, true
	sodiumBase, sodiumDefaulted := defaultSodiumMgHr, true
	if p.SweatRateMlPerHr != nil {
		fluidBase = math.Min(*p.SweatRateMlPerHr, fluidCapMlHr)
		fluidDefaulted = false
		sodiumBase = math.Round(*p.SweatRateMlPerHr / 1000.0 * sweatSodiumMgPerL)
		sodiumDefaulted = false
	}

	plan := &FuelingPlan{
		RaceID:           race.ID,
		RaceName:         race.Name,
		RaceDate:         race.RaceDate,
		BodyWeightKg:     p.BodyWeightKg,
		SweatRateMlPerHr: p.SweatRateMlPerHr,
		TotalDurationMin: totalMin,
		Legs:             make([]*LegFuelingPlan, 0, len(race.Legs)),
	}

	for _, leg := range race.Legs {
		lp := &LegFuelingPlan{
			Ordinal:             leg.Ordinal,
			Discipline:          leg.Discipline,
			ExpectedDurationMin: leg.ExpectedDurationMin,
		}
		if leg.ExpectedDurationMin == nil {
			lp.Rationale = "duration unknown — no fuelling computed"
			plan.Legs = append(plan.Legs, lp)
			continue
		}
		durHr := float64(*leg.ExpectedDurationMin) / 60.0

		lp.CarbsGPerHr = math.Round(baseCarbs * disciplineCarbFactor(leg.Discipline))
		if canIngest(leg.Discipline) {
			lp.FluidMlPerHr = math.Round(fluidBase)
			lp.SodiumMgPerHr = math.Round(sodiumBase)
		}
		lp.CarbsGTotal = math.Round(lp.CarbsGPerHr * durHr)
		lp.FluidMlTotal = math.Round(lp.FluidMlPerHr * durHr)
		lp.SodiumMgTotal = math.Round(lp.SodiumMgPerHr * durHr)
		lp.Rationale = legRationale(leg.Discipline, baseCarbs, fluidDefaulted, sodiumDefaulted)

		plan.Legs = append(plan.Legs, lp)
		plan.Total.CarbsGTotal += lp.CarbsGTotal
		plan.Total.SodiumMgTotal += lp.SodiumMgTotal
		plan.Total.FluidMlTotal += lp.FluidMlTotal
	}
	return plan
}

func legRationale(d Discipline, baseCarbs float64, fluidDefaulted, sodiumDefaulted bool) string {
	if !canIngest(d) {
		return fmt.Sprintf("no intake feasible during %s", d)
	}
	if baseCarbs == 0 {
		return "race under 75 min — carbohydrate fuelling not required; hydrate to thirst"
	}
	msg := fmt.Sprintf("baseline %.0f g/hr carbs scaled to %s intake capacity", baseCarbs, d)
	if fluidDefaulted || sodiumDefaulted {
		msg += "; default sweat rate assumed (supply sweat_rate_ml_per_hr to personalise fluid and sodium)"
	}
	return msg
}
