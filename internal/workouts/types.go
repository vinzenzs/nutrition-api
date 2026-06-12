package workouts

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Source records workout provenance: which writer pushed the row.
type Source string

const (
	SourceGarmin Source = "garmin"
	SourceManual Source = "manual"
	SourceOther  Source = "other"
)

func ValidSource(s string) bool {
	switch Source(s) {
	case SourceGarmin, SourceManual, SourceOther:
		return true
	}
	return false
}

func ParseSource(s string) (Source, error) {
	if !ValidSource(s) {
		return "", fmt.Errorf("invalid source %q", s)
	}
	return Source(s), nil
}

// Sport is the activity category. Kept deliberately small in v1; extend the
// enum when a specific sport's fueling tools earn the surface.
type Sport string

const (
	SportRun      Sport = "run"
	SportBike     Sport = "bike"
	SportSwim     Sport = "swim"
	SportStrength Sport = "strength"
	SportOther    Sport = "other"
)

func ValidSport(s string) bool {
	switch Sport(s) {
	case SportRun, SportBike, SportSwim, SportStrength, SportOther:
		return true
	}
	return false
}

func ParseSport(s string) (Sport, error) {
	if !ValidSport(s) {
		return "", fmt.Errorf("invalid sport %q", s)
	}
	return Sport(s), nil
}

// Status is the workout lifecycle: a completed activity (the default, what every
// Garmin-synced activity is) or a planned/scheduled session that has not happened
// yet. Planned workouts may be future-dated; completed ones keep the 24h guard.
type Status string

const (
	StatusPlanned   Status = "planned"
	StatusCompleted Status = "completed"
)

func ValidStatus(s string) bool {
	switch Status(s) {
	case StatusPlanned, StatusCompleted:
		return true
	}
	return false
}

// Workout mirrors a workouts row.
type Workout struct {
	ID         uuid.UUID `json:"id"`
	ExternalID *string   `json:"external_id,omitempty"`
	Source     Source    `json:"source"`
	Sport      Sport     `json:"sport"`
	Status     Status    `json:"status"`
	Name       *string   `json:"name,omitempty"`

	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`

	KcalBurned *float64 `json:"kcal_burned,omitempty"`
	AvgHR      *int     `json:"avg_hr,omitempty"`
	TSS        *float64 `json:"tss,omitempty"`

	// Per-session rehearsal-outcome signals — both nullable, set by the user
	// after a fueling-rehearsal workout. Validated 1..10 (Borg CR-10) and 1..5
	// (1=no GI distress, 5=severe) at handler + DB CHECK layers.
	RPE             *int `json:"rpe,omitempty"`
	GIDistressScore *int `json:"gi_distress_score,omitempty"`

	// Source-agnostic ingestion metrics — all nullable, populated by whatever
	// writer measured them (Garmin today). distance in metres, average power in
	// watts, ambient temperature in °C, estimated sweat loss in ml. SessionGroup
	// is a free-text key linking the legs of a brick/multisport session (e.g. the
	// Garmin parent activity id set on every leg).
	DistanceM    *float64 `json:"distance_m,omitempty"`
	AvgPowerW    *int     `json:"avg_power_w,omitempty"`
	TemperatureC *float64 `json:"temperature_c,omitempty"`
	SweatLossML  *float64 `json:"sweat_loss_ml,omitempty"`
	SessionGroup *string  `json:"session_group,omitempty"`

	// Per-activity detail (per add-garmin-workout-detail), all nullable, mapped
	// from the Garmin activity summary + weather. Elevation in metres, normalized
	// power in watts, IF a ratio, cadence in spm/rpm, stride in metres, training
	// effect on Garmin's 0–5 scale. Weather: humidity %, wind in m/s (temperature
	// already lives above). "Not measured" stays a meaningful NULL.
	ElevationGainM   *float64 `json:"elevation_gain_m,omitempty"`
	ElevationLossM   *float64 `json:"elevation_loss_m,omitempty"`
	NormalizedPowerW *int     `json:"normalized_power_w,omitempty"`
	IntensityFactor  *float64 `json:"intensity_factor,omitempty"`
	AvgCadence       *int     `json:"avg_cadence,omitempty"`
	AvgStrideM       *float64 `json:"avg_stride_m,omitempty"`
	MaxHR            *int     `json:"max_hr,omitempty"`
	AerobicTE        *float64 `json:"aerobic_te,omitempty"`
	AnaerobicTE      *float64 `json:"anaerobic_te,omitempty"`
	HumidityPct      *float64 `json:"humidity_pct,omitempty"`
	WindSpeedMPS     *float64 `json:"wind_speed_mps,omitempty"`

	// HR-zone time, seconds in each of the fixed five Garmin zones. Fixed
	// cardinality → columns (not a child table), so the most-queried fueling
	// signal needs no join and rides along on list responses.
	SecsInZone1 *int `json:"secs_in_zone_1,omitempty"`
	SecsInZone2 *int `json:"secs_in_zone_2,omitempty"`
	SecsInZone3 *int `json:"secs_in_zone_3,omitempty"`
	SecsInZone4 *int `json:"secs_in_zone_4,omitempty"`
	SecsInZone5 *int `json:"secs_in_zone_5,omitempty"`

	// Nested detail — populated on single-get only (the list query omits them to
	// keep payloads bounded). Empty slices serialize as absent via omitempty.
	Splits []Split `json:"splits,omitempty"`
	Sets   []Set   `json:"sets,omitempty"`

	// Plan links (per add-training-plan), both nullable. TemplateID is the
	// workout-template a planned workout was compiled from; PlanSlotID is the
	// training-plan slot it materializes (the materialize upsert key). Imported
	// activities carry neither.
	TemplateID *uuid.UUID `json:"template_id,omitempty"`
	PlanSlotID *uuid.UUID `json:"plan_slot_id,omitempty"`

	// Garmin scheduling ids (per add-garmin-scheduling), both nullable opaque
	// Garmin identifiers: the structured workout created in the Garmin library
	// and the calendar entry scheduling it. Set on push, cleared on unschedule.
	GarminWorkoutID  *string `json:"garmin_workout_id,omitempty"`
	GarminScheduleID *string `json:"garmin_schedule_id,omitempty"`

	// NeedsLink (per add-workout-reconciliation) marks a completed import that
	// matched more than one open planned workout, so it was inserted standalone
	// rather than auto-merged — the app/agent should offer to link it. Cleared
	// on fulfill.
	NeedsLink bool `json:"needs_link,omitempty"`

	Notes *string `json:"notes,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Split mirrors a workout_splits row — one per lap of an endurance activity.
// All metrics nullable; SplitIndex is the 0-based lap ordinal. AvgSpeedMPS is
// stored sport-agnostically (pace is derivable) at its native precision.
type Split struct {
	SplitIndex     int      `json:"split_index"`
	DistanceM      *float64 `json:"distance_m,omitempty"`
	DurationS      *float64 `json:"duration_s,omitempty"`
	AvgHR          *int     `json:"avg_hr,omitempty"`
	AvgPowerW      *int     `json:"avg_power_w,omitempty"`
	AvgSpeedMPS    *float64 `json:"avg_speed_mps,omitempty"`
	ElevationGainM *float64 `json:"elevation_gain_m,omitempty"`
}

// Set mirrors a workout_sets row — one per strength set. SetIndex is the
// 0-based ordinal; all exercise fields nullable.
type Set struct {
	SetIndex         int      `json:"set_index"`
	ExerciseName     *string  `json:"exercise_name,omitempty"`
	ExerciseCategory *string  `json:"exercise_category,omitempty"`
	Reps             *int     `json:"reps,omitempty"`
	WeightKg         *float64 `json:"weight_kg,omitempty"`
	DurationS        *float64 `json:"duration_s,omitempty"`
}
