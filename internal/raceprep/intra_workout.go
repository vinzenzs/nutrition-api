package raceprep

import (
	"fmt"

	"github.com/vinzenzs/kazper/internal/numfmt"
	"github.com/vinzenzs/kazper/internal/workouts"
)

// intraWorkoutFor returns the in-session fueling recommendation per the
// add-recommend-workout-fuel design § Intra-workout. The cap for sport=run
// reflects GI tolerance during impact-loaded effort. Sub-45-min sessions,
// strength sessions, and swims ≤ 120 min return Applicable: false.
func intraWorkoutFor(sport string, durationMin int, zone int) IntraWorkout {
	// Sport-level not-applicable rules.
	if sport == string(workouts.SportStrength) {
		return IntraWorkout{
			Applicable: false,
			Rationale:  "Strength sessions get fueled before and after, not during. No intra-session intake is needed even for sets that run an hour.",
		}
	}
	if sport == string(workouts.SportSwim) && durationMin <= 120 {
		return IntraWorkout{
			Applicable: false,
			Rationale:  "Swim segments under 2h rarely allow practical in-session intake. Treat the pre and post phases as the actionable surface and use pool-side fluid between sets.",
		}
	}

	// Duration-level not-applicable rule.
	if durationMin < 45 {
		return IntraWorkout{
			Applicable: false,
			Rationale:  "Sessions under 45 min don't need intra-session carbs — endogenous glycogen handles the work. Water suffices.",
		}
	}

	// Pick the per-hour rate from the duration × zone matrix.
	var perHour, fluidPerHour, sodiumPerHour float64
	var rationale string

	switch {
	case durationMin > 180:
		// Long endurance: multi-transportable territory.
		perHour = 90
		fluidPerHour = 700
		sodiumPerHour = 700
		rationale = fmt.Sprintf("Over 3h on the %s — multi-transportable CHO (glucose+fructose 2:1) supports up to 90 g/hr without GI overload. Fluid and sodium scale with duration; hot conditions push both higher.", sport)
	case durationMin >= 90:
		// 90-180 min, single-transportable CHO at 60 g/hr regardless of zone.
		perHour = 60
		if zone >= 3 {
			fluidPerHour = 700
			sodiumPerHour = 600
			rationale = fmt.Sprintf("90–180 min, Zone 3–4 on the %s — 60 g/hr single-transportable CHO is the literature-validated ceiling for sustained sub-threshold work. Fluid/sodium higher than aerobic Z1–2 because sweat rate climbs with intensity.", sport)
		} else {
			fluidPerHour = 600
			sodiumPerHour = 450
			rationale = fmt.Sprintf("90–180 min, Zone 1–2 on the %s — 60 g/hr CHO is generous for aerobic intensity but tops off glycogen so the second half doesn't fade. Single-transportable (glucose) is fine at this rate.", sport)
		}
	default:
		// 45-90 min — zone-driven split.
		if zone >= 3 {
			perHour = 60
			fluidPerHour = 600
			sodiumPerHour = 500
			rationale = fmt.Sprintf("45–90 min at Zone 3–4 on the %s — 60 g/hr CHO is the standard for tempo/threshold work. Carry fluid at ~600 ml/hr with ~500 mg sodium/hr.", sport)
		} else {
			perHour = 30
			fluidPerHour = 500
			sodiumPerHour = 300
			rationale = fmt.Sprintf("45–90 min at Zone 1–2 on the %s — 30 g/hr CHO is the low-end of the literature band; aerobic work doesn't draw down glycogen aggressively. Single-transportable CHO is sufficient.", sport)
		}
	}

	// Run cap: GI tolerance during impact-loaded effort tops out around 60 g/hr.
	if sport == string(workouts.SportRun) && perHour > 60 {
		perHour = 60
		rationale += " Note: the run-specific cap of 60 g/hr applies — running's impact loading limits GI tolerance below the multi-transportable ceiling that bike/row allow."
	}

	total := perHour * float64(durationMin) / 60.0

	return IntraWorkout{
		Applicable:      true,
		CarbsGPerHour:   ptrRound1(perHour),
		CarbsGTotal:     ptrRound1(total),
		FluidMlPerHour:  ptrRound1(fluidPerHour),
		SodiumMgPerHour: ptrRound1(sodiumPerHour),
		Rationale:       rationale,
	}
}

func ptrRound1(v float64) *float64 {
	r := numfmt.Round1(v)
	return &r
}
