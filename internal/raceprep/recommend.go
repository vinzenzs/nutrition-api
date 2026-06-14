package raceprep

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"

	"github.com/vinzenzs/kazper/internal/bodyweight"
	"github.com/vinzenzs/kazper/internal/numfmt"
	"github.com/vinzenzs/kazper/internal/workouts"
)

// RecommendInputs echoes the resolved inputs back so the agent can see what
// the recommendation was computed against — particularly important for
// workout-mode where the agent didn't supply sport/duration/intensity
// directly and might want to confirm what was derived from the row.
type RecommendInputs struct {
	Sport            string     `json:"sport"`
	DurationMin      int        `json:"duration_min"`
	IntensityZone    int        `json:"intensity_zone"`
	BodyWeightKg     float64    `json:"body_weight_kg"`
	BodyWeightSource string     `json:"body_weight_source"`
	WorkoutID        *uuid.UUID `json:"workout_id,omitempty"`
}

// PreWorkout is the pre-session fueling recommendation. The window is a
// [lo, hi] minute range *before* the start; carbs is the total grams.
type PreWorkout struct {
	WindowMinutesBefore [2]int  `json:"window_minutes_before"`
	CarbsG              float64 `json:"carbs_g"`
	CarbsGPerKg         float64 `json:"carbs_g_per_kg"`
	Rationale           string  `json:"rationale"`
}

// IntraWorkout is the in-session fueling recommendation. When Applicable is
// false (sub-45-min sessions, strength, short swims) every numeric field is
// nil — distinct from "zero" which would imply "literature says zero," which
// isn't the same as "not applicable here."
type IntraWorkout struct {
	Applicable      bool     `json:"applicable"`
	CarbsGPerHour   *float64 `json:"carbs_g_per_hour"`
	CarbsGTotal     *float64 `json:"carbs_g_total"`
	FluidMlPerHour  *float64 `json:"fluid_ml_per_hour"`
	SodiumMgPerHour *float64 `json:"sodium_mg_per_hour"`
	Rationale       string   `json:"rationale"`
}

// PostWorkout is the recovery-window recommendation. Reuses the 0.3 g/kg MPS
// threshold from add-protein-distribution so the post-workout protein target
// automatically hits mps_effective: true in the daily distribution view.
type PostWorkout struct {
	WindowMinutesAfter [2]int  `json:"window_minutes_after"`
	CarbsG             float64 `json:"carbs_g"`
	ProteinG           float64 `json:"protein_g"`
	Rationale          string  `json:"rationale"`
}

// FuelRecommendation is the response shape for GET /race-prep/recommend-workout-fuel.
type FuelRecommendation struct {
	Inputs       RecommendInputs `json:"inputs"`
	PreWorkout   PreWorkout      `json:"pre_workout"`
	IntraWorkout IntraWorkout    `json:"intra_workout"`
	PostWorkout  PostWorkout     `json:"post_workout"`
	Notes        []string        `json:"notes"`
}

// RecommendParams scopes a recommendation request. Exactly one of
// (WorkoutID) or (Sport + DurationMin + IntensityZone) must be present —
// validated by the handler. Today + Loc anchor the body-weight resolver.
type RecommendParams struct {
	WorkoutID            *uuid.UUID
	Sport                *string
	DurationMin          *int
	IntensityZone        *int
	BodyWeightKgOverride *float64
	Today                time.Time
	Loc                  *time.Location
}

// Validation errors for the recommend path. Mapped to per-endpoint error
// codes by the handler.
var (
	ErrInputRequired         = errors.New("input_required")
	ErrInputConflict         = errors.New("input_conflict")
	ErrSportRequired         = errors.New("sport_required")
	ErrDurationMinRequired   = errors.New("duration_min_required")
	ErrIntensityZoneRequired = errors.New("intensity_zone_required")
	ErrSportInvalid          = errors.New("sport_invalid")
	ErrDurationMinInvalid    = errors.New("duration_min_invalid")
	ErrIntensityZoneInvalid  = errors.New("intensity_zone_invalid")
	ErrBodyWeightInvalid     = errors.New("body_weight_kg_invalid")
)

// SetWorkoutsRepo wires the workouts repo so workout-mode can pull the
// underlying row. SetBodyWeightRepo wires the resolver source. Both are
// optional setters following the same pattern meals/hydration use for
// SetWorkoutsRepo from add-meal-workout-link.
func (s *Service) SetWorkoutsRepo(r *workouts.Repo)    { s.workoutsRepo = r }
func (s *Service) SetBodyWeightRepo(r *bodyweight.Repo) { s.bodyWeightRepo = r }

// RecommendFor returns a pre/intra/post fueling recommendation for one
// session. See openspec/changes/add-recommend-workout-fuel/ for the full
// algorithm and the literature provenance.
func (s *Service) RecommendFor(ctx context.Context, p RecommendParams) (*FuelRecommendation, error) {
	if p.BodyWeightKgOverride != nil {
		v := *p.BodyWeightKgOverride
		if math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 {
			return nil, ErrBodyWeightInvalid
		}
	}

	sport, durationMin, zone, workoutID, defaultedIntensity, err := s.resolveMode(ctx, p)
	if err != nil {
		return nil, err
	}

	bwKg, bwSource, err := bodyweight.ResolveAtDate(ctx, s.bodyWeightRepo, p.Today, p.Loc, p.BodyWeightKgOverride)
	if err != nil {
		return nil, err
	}

	pre := preWorkoutFor(sport, zone, bwKg)
	intra := intraWorkoutFor(sport, durationMin, zone)
	post := postWorkoutFor(bwKg)
	notes := buildNotes(sport, durationMin, zone, defaultedIntensity)

	out := &FuelRecommendation{
		Inputs: RecommendInputs{
			Sport:            sport,
			DurationMin:      durationMin,
			IntensityZone:    zone,
			BodyWeightKg:     numfmt.Round1(bwKg),
			BodyWeightSource: bwSource,
			WorkoutID:        workoutID,
		},
		PreWorkout:   pre,
		IntraWorkout: intra,
		PostWorkout:  post,
		Notes:        notes,
	}
	return out, nil
}

// resolveMode runs the mode-exclusivity validation and, in workout-mode,
// pulls sport/duration/intensity from the row. Returns
// (sport, durationMin, zone, *workoutID, defaultedIntensity, err).
//
// `defaultedIntensity` is true only in workout-mode when the workout had no
// TSS and the helper defaulted to Z2 — the notes builder uses it to decide
// whether to surface the disclosure note.
func (s *Service) resolveMode(ctx context.Context, p RecommendParams) (string, int, int, *uuid.UUID, bool, error) {
	explicitPresent := p.Sport != nil || p.DurationMin != nil || p.IntensityZone != nil

	switch {
	case p.WorkoutID != nil && explicitPresent:
		return "", 0, 0, nil, false, ErrInputConflict
	case p.WorkoutID == nil && !explicitPresent:
		return "", 0, 0, nil, false, ErrInputRequired
	}

	if p.WorkoutID != nil {
		if s.workoutsRepo == nil {
			// Defensive — production wires this in server.go. Tests that
			// want workout-mode wire it via SetWorkoutsRepo. Without a repo
			// the row can't be fetched.
			return "", 0, 0, nil, false, fmt.Errorf("workouts repo not wired")
		}
		w, err := s.workoutsRepo.GetByID(ctx, *p.WorkoutID)
		if err != nil {
			return "", 0, 0, nil, false, err
		}
		duration := int(w.EndedAt.Sub(w.StartedAt).Minutes())
		zone, defaulted := IntensityFromTSS(w.TSS, duration)
		return string(w.Sport), duration, zone, &w.ID, defaulted, nil
	}

	// Explicit mode — validate each field in first-missing-wins order so
	// the agent has a single field to fix at a time.
	if p.Sport == nil {
		return "", 0, 0, nil, false, ErrSportRequired
	}
	if p.DurationMin == nil {
		return "", 0, 0, nil, false, ErrDurationMinRequired
	}
	if p.IntensityZone == nil {
		return "", 0, 0, nil, false, ErrIntensityZoneRequired
	}
	if !workouts.ValidSport(*p.Sport) {
		return "", 0, 0, nil, false, ErrSportInvalid
	}
	if *p.DurationMin <= 0 {
		return "", 0, 0, nil, false, ErrDurationMinInvalid
	}
	if *p.IntensityZone < 1 || *p.IntensityZone > 5 {
		return "", 0, 0, nil, false, ErrIntensityZoneInvalid
	}
	return *p.Sport, *p.DurationMin, *p.IntensityZone, nil, false, nil
}
