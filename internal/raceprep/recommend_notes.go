package raceprep

import "github.com/vinzenzs/kazper/internal/workouts"

// buildNotes assembles the literature-context notes that don't fit as
// single numbers in the per-section rationale. Always returns the three
// always-present notes; appends conditional ones for TSS-defaulted intensity
// and run-cap behaviour.
func buildNotes(sport string, durationMin, zone int, defaultedIntensity bool) []string {
	notes := []string{
		"Intra-session sodium target is a midpoint; the validated range is 300–800 mg/hr. Heavy sweaters and hot conditions push toward the upper end. A sweat-rate test session calibrates the personalized number — currently not tracked by this API (see priorities T3 #6).",
		"CHO/hr buckets: < 45 min none required; 45–90 min 30 g/hr; 90–180 min 60 g/hr (single transportable, e.g. glucose); > 180 min 90 g/hr (multiple transportable — glucose+fructose 2:1).",
		"For races > 90 min, also run `plan_carb_load` for the 24–72h pre-loading schedule. This tool answers per-session; pre-loading is a multi-day protocol.",
	}

	if defaultedIntensity {
		notes = append(notes,
			"Intensity defaulted to Z2 because the workout has no TSS. Pass an explicit `intensity_zone` (1–5) if you have RPE/HR context to set it.",
		)
	}

	// Run cap disclosure: when the duration bucket would have produced > 60 g/hr
	// but the run cap kicked in. The intra rationale already mentions it
	// inline, so this is a duplicate signal for agents that summarise
	// notes[] separately.
	if sport == string(workouts.SportRun) && durationMin >= 90 {
		// Only surface the cap note when the duration bucket would otherwise
		// have produced > 60 g/hr — i.e. only the > 180 min bucket.
		if durationMin > 180 {
			notes = append(notes,
				"Run-specific cap: intra carbs cap at 60 g/hr even on long runs (over 3h) where bike or row recommendations would suggest 90 g/hr. Running's impact loading limits GI tolerance.",
			)
		}
	}

	return notes
}
