// Package workouttemplates is the reusable, structured workout-template library
// — the ~40 swim/bike/run/yoga session definitions that the training plan
// references and the Garmin-scheduling edge compiles into watch workouts. Same
// package shape as every other capability (types / repo / service / handlers /
// tests). No Garmin coupling: the step model is a clean superset of
// garminconnect's step DTOs, but the translation lives in the bridge.
package workouttemplates

import "time"

// Sport reuses the workouts capability's vocabulary so a template's sport and a
// workout's sport share one set of values. Mirrored (not imported) to keep this
// package independent of workouts; the values must stay in sync.
const (
	SportRun      = "run"
	SportBike     = "bike"
	SportSwim     = "swim"
	SportStrength = "strength"
	SportYoga     = "yoga"
	SportMobility = "mobility"
	SportOther    = "other"
)

func validSport(s string) bool {
	switch s {
	case SportRun, SportBike, SportSwim, SportStrength, SportYoga, SportMobility, SportOther:
		return true
	}
	return false
}

// Template mirrors a workout_templates row. Steps is the validated structured
// program (see Step). Nullables carry omitempty so absent stays distinct.
type Template struct {
	ID                   string    `json:"id"`
	Sport                string    `json:"sport"`
	Name                 string    `json:"name"`
	Description          *string   `json:"description,omitempty"`
	EstimatedDurationSec *int      `json:"estimated_duration_sec,omitempty"`
	Steps                []Step    `json:"steps"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// Node kinds for a step entry.
const (
	NodeStep   = "step"
	NodeRepeat = "repeat"
)

// Intent values for a single executable step.
const (
	IntentWarmup   = "warmup"
	IntentActive   = "active"
	IntentInterval = "interval"
	IntentRecovery = "recovery"
	IntentRest     = "rest"
	IntentCooldown = "cooldown"
)

// Duration kinds.
const (
	DurationTime      = "time"
	DurationDistance  = "distance"
	DurationLapButton = "lap_button"
	DurationOpen      = "open"
)

// Target kinds.
const (
	TargetNone      = "none"
	TargetHRZone    = "hr_zone"
	TargetPowerZone = "power_zone"
	TargetPace      = "pace"
	TargetHRBpm     = "hr_bpm"
	TargetPowerW    = "power_w"
	TargetRPE       = "rpe"
)

// Step is one node in a template's ordered program: either a single executable
// step (Type == "step") or a repeat group (Type == "repeat", one level deep).
// The two shapes share this struct; which fields are meaningful depends on Type.
type Step struct {
	Type string `json:"type"`

	// --- single executable step (Type == "step") ---
	Intent   string    `json:"intent,omitempty"`
	Duration *Duration `json:"duration,omitempty"`
	Target   *Target   `json:"target,omitempty"`
	Note     string    `json:"note,omitempty"`

	// --- repeat group (Type == "repeat") ---
	Count int    `json:"count,omitempty"`
	Steps []Step `json:"steps,omitempty"`
}

// Duration is a step's end condition. Exactly one kind; Seconds applies to
// "time", Meters to "distance", neither to "lap_button"/"open".
type Duration struct {
	Kind    string `json:"kind"`
	Seconds *int   `json:"seconds,omitempty"`
	Meters  *int   `json:"meters,omitempty"`
}

// Target is a step's effort target. Zones use Low/High (1..5); pace uses the
// per-km fields; hr_bpm/power_w/rpe use Low/High in their own units.
type Target struct {
	Kind         string `json:"kind"`
	Low          *int   `json:"low,omitempty"`
	High         *int   `json:"high,omitempty"`
	LowSecPerKM  *int   `json:"low_sec_per_km,omitempty"`
	HighSecPerKM *int   `json:"high_sec_per_km,omitempty"`
}
