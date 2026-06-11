package races

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func iptr(v int) *int         { return &v }
func fptr(v float64) *float64 { return &v }

func raceWith(legs ...*RaceLeg) *Race {
	return &Race{ID: uuid.New(), Name: "Test Tri", RaceDate: "2026-07-24", Legs: legs}
}

func leg(ordinal int, d Discipline, durMin *int) *RaceLeg {
	return &RaceLeg{Ordinal: ordinal, Discipline: d, ExpectedDurationMin: durMin}
}

func TestBaseCarbsBands(t *testing.T) {
	cases := []struct {
		min  int
		want float64
	}{
		{74, 0}, {75, 60}, {149, 60}, {150, 90}, {600, 90},
	}
	for _, c := range cases {
		if got := baseCarbsGPerHr(c.min); got != c.want {
			t.Errorf("baseCarbsGPerHr(%d) = %v, want %v", c.min, got, c.want)
		}
	}
}

func TestComputeFueling_DisciplineFactors(t *testing.T) {
	// Total 150 min → 90 g/hr baseline.
	r := raceWith(
		leg(1, DisciplineBike, iptr(90)),
		leg(2, DisciplineRun, iptr(60)),
	)
	plan := ComputeFueling(r, FuelingParams{BodyWeightKg: 70})
	if plan.TotalDurationMin != 150 {
		t.Fatalf("total duration = %d, want 150", plan.TotalDurationMin)
	}
	if plan.Legs[0].CarbsGPerHr != 90 {
		t.Errorf("bike carbs/hr = %v, want 90", plan.Legs[0].CarbsGPerHr)
	}
	if plan.Legs[1].CarbsGPerHr != 63 {
		t.Errorf("run carbs/hr = %v, want 63 (0.7×90)", plan.Legs[1].CarbsGPerHr)
	}
	// bike: 90 g/hr × 1.5h = 135; run: 63 × 1h = 63 → total 198.
	if plan.Total.CarbsGTotal != 198 {
		t.Errorf("total carbs = %v, want 198", plan.Total.CarbsGTotal)
	}
}

func TestComputeFueling_SwimAndTransitionZero(t *testing.T) {
	r := raceWith(
		leg(1, DisciplineSwim, iptr(30)),
		leg(2, DisciplineTransition, iptr(5)),
		leg(3, DisciplineBike, iptr(120)),
	)
	plan := ComputeFueling(r, FuelingParams{BodyWeightKg: 70})
	for _, i := range []int{0, 1} {
		l := plan.Legs[i]
		if l.CarbsGPerHr != 0 || l.SodiumMgPerHr != 0 || l.FluidMlPerHr != 0 {
			t.Errorf("%s leg should be zero, got %+v", l.Discipline, l)
		}
		if l.CarbsGTotal != 0 || l.SodiumMgTotal != 0 || l.FluidMlTotal != 0 {
			t.Errorf("%s leg totals should be zero, got %+v", l.Discipline, l)
		}
	}
}

func TestComputeFueling_SweatRateSupplied(t *testing.T) {
	r := raceWith(leg(1, DisciplineBike, iptr(180)))
	plan := ComputeFueling(r, FuelingParams{BodyWeightKg: 70, SweatRateMlPerHr: fptr(1000)})
	l := plan.Legs[0]
	if l.FluidMlPerHr != 1000 {
		t.Errorf("fluid/hr = %v, want 1000", l.FluidMlPerHr)
	}
	if l.SodiumMgPerHr != 800 {
		t.Errorf("sodium/hr = %v, want 800 (1000ml × 0.8mg/ml)", l.SodiumMgPerHr)
	}
}

func TestComputeFueling_SweatRateCapped(t *testing.T) {
	r := raceWith(leg(1, DisciplineBike, iptr(180)))
	plan := ComputeFueling(r, FuelingParams{BodyWeightKg: 70, SweatRateMlPerHr: fptr(1500)})
	if plan.Legs[0].FluidMlPerHr != 1000 {
		t.Errorf("fluid/hr = %v, want 1000 (capped)", plan.Legs[0].FluidMlPerHr)
	}
	if plan.Legs[0].SodiumMgPerHr != 1200 {
		t.Errorf("sodium/hr = %v, want 1200 (1500×0.8, sodium uncapped)", plan.Legs[0].SodiumMgPerHr)
	}
}

func TestComputeFueling_DefaultSweatRateFlagged(t *testing.T) {
	r := raceWith(leg(1, DisciplineBike, iptr(180)))
	plan := ComputeFueling(r, FuelingParams{BodyWeightKg: 70})
	l := plan.Legs[0]
	if l.FluidMlPerHr != 600 || l.SodiumMgPerHr != 600 {
		t.Errorf("default fluid/sodium = %v/%v, want 600/600", l.FluidMlPerHr, l.SodiumMgPerHr)
	}
	if !strings.Contains(l.Rationale, "default sweat rate") {
		t.Errorf("rationale should flag default sweat rate, got %q", l.Rationale)
	}
}

func TestComputeFueling_UnknownDurationLeg(t *testing.T) {
	r := raceWith(
		leg(1, DisciplineBike, iptr(180)),
		leg(2, DisciplineRun, nil),
	)
	plan := ComputeFueling(r, FuelingParams{BodyWeightKg: 70})
	if plan.TotalDurationMin != 180 {
		t.Errorf("total duration = %d, want 180 (unknown leg excluded)", plan.TotalDurationMin)
	}
	l := plan.Legs[1]
	if l.CarbsGPerHr != 0 || l.FluidMlPerHr != 0 || l.SodiumMgPerHr != 0 {
		t.Errorf("unknown-duration leg should be zero, got %+v", l)
	}
	if !strings.Contains(l.Rationale, "duration unknown") {
		t.Errorf("rationale = %q, want 'duration unknown'", l.Rationale)
	}
}

func TestFuelingParamsValidate(t *testing.T) {
	cases := []struct {
		name string
		p    FuelingParams
		want error
	}{
		{"missing weight", FuelingParams{}, ErrBodyWeightRequired},
		{"weight too low", FuelingParams{BodyWeightKg: 15}, ErrBodyWeightRange},
		{"weight too high", FuelingParams{BodyWeightKg: 250}, ErrBodyWeightRange},
		{"ok weight", FuelingParams{BodyWeightKg: 70}, nil},
		{"sweat too high", FuelingParams{BodyWeightKg: 70, SweatRateMlPerHr: fptr(9000)}, ErrSweatRateRange},
		{"sweat negative", FuelingParams{BodyWeightKg: 70, SweatRateMlPerHr: fptr(-1)}, ErrSweatRateRange},
		{"ok sweat", FuelingParams{BodyWeightKg: 70, SweatRateMlPerHr: fptr(900)}, nil},
	}
	for _, c := range cases {
		if got := c.p.validate(); got != c.want {
			t.Errorf("%s: validate() = %v, want %v", c.name, got, c.want)
		}
	}
}
