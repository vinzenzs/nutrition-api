// Package raceprep computes stateless nutrition primitives for race-week
// planning. The first (and currently only) primitive is the carb-load schedule
// — given a race date and a body weight, return per-day carbohydrate targets
// for the load window and race morning. No DB, no per-user state, no side
// effects: pure math with bounds checking.
package raceprep

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vinzenzs/kazper/internal/bodyweight"
	"github.com/vinzenzs/kazper/internal/workouts"

	"github.com/vinzenzs/kazper/internal/numfmt"
)

// Sentinel validation errors mapped 1:1 to API error codes by the handler.
var (
	ErrRaceDateInPast            = errors.New("race_date_in_past")
	ErrBodyWeightKgInvalid       = errors.New("body_weight_kg_invalid")
	ErrDaysBeforeInvalid         = errors.New("days_before_invalid")
	ErrCarbsPerKgPerDayInvalid   = errors.New("carbs_per_kg_per_day_invalid")
	ErrRaceDayCarbsPerKgInvalid  = errors.New("race_day_carbs_per_kg_invalid")
)

// Bounds for the validation rules. Exported so the handler can echo them back
// in error responses.
const (
	BodyWeightKgMin       = 30.0
	BodyWeightKgMax       = 200.0
	DaysBeforeMin         = 0
	DaysBeforeMax         = 7
	CarbsPerKgPerDayMin   = 1.0
	CarbsPerKgPerDayMax   = 20.0
	RaceDayCarbsPerKgMin  = 0.0
	RaceDayCarbsPerKgMax  = 10.0
)

// Default parameter values applied when the caller omits them.
const (
	DefaultDaysBefore        = 3
	DefaultCarbsPerKgPerDay  = 10.0
	DefaultRaceDayCarbsPerKg = 2.0
)

// CarbLoadParams is the input to PlanCarbLoad.
type CarbLoadParams struct {
	RaceDate          time.Time // local date only; time-of-day ignored
	BodyWeightKg      float64
	DaysBefore        int
	CarbsPerKgPerDay  float64
	RaceDayCarbsPerKg float64
}

// CarbLoadEntry is one row in the schedule.
type CarbLoadEntry struct {
	Date         string  `json:"date"`            // YYYY-MM-DD
	DaysBefore   int     `json:"days_before"`     // 0 for race day, N for N days before
	TargetCarbsG float64 `json:"target_carbs_g"`  // rounded to 1 dp
	Rationale    string  `json:"rationale"`
}

// CarbLoadSchedule is the response shape.
type CarbLoadSchedule struct {
	RaceDate     string          `json:"race_date"`
	BodyWeightKg float64         `json:"body_weight_kg"`
	Params       EchoedParams    `json:"params"`
	Schedule     []CarbLoadEntry `json:"schedule"`
}

// EchoedParams echoes the effective inputs (after defaults applied) so the
// caller can see the protocol that produced the schedule.
type EchoedParams struct {
	DaysBefore        int     `json:"days_before"`
	CarbsPerKgPerDay  float64 `json:"carbs_per_kg_per_day"`
	RaceDayCarbsPerKg float64 `json:"race_day_carbs_per_kg"`
}

// PlanCarbLoad validates p and returns the day-by-day schedule. `today` is
// the current local date (in the user's timezone) — injected for testability;
// race_date must not be strictly before today.
func PlanCarbLoad(p CarbLoadParams, today time.Time) (*CarbLoadSchedule, error) {
	if err := validate(p, today); err != nil {
		return nil, err
	}

	// Truncate both to local date granularity. Time-of-day on race_date
	// should not affect the schedule.
	raceDay := time.Date(p.RaceDate.Year(), p.RaceDate.Month(), p.RaceDate.Day(),
		0, 0, 0, 0, p.RaceDate.Location())

	loadTarget := numfmt.Round1(p.BodyWeightKg * p.CarbsPerKgPerDay)
	raceTarget := numfmt.Round1(p.BodyWeightKg * p.RaceDayCarbsPerKg)

	schedule := make([]CarbLoadEntry, 0, p.DaysBefore+1)
	for i := p.DaysBefore; i > 0; i-- {
		date := raceDay.AddDate(0, 0, -i)
		dayN := p.DaysBefore - i + 1
		schedule = append(schedule, CarbLoadEntry{
			Date:         date.Format("2006-01-02"),
			DaysBefore:   i,
			TargetCarbsG: loadTarget,
			Rationale:    fmt.Sprintf("carb-load day %d of %d", dayN, p.DaysBefore),
		})
	}
	schedule = append(schedule, CarbLoadEntry{
		Date:         raceDay.Format("2006-01-02"),
		DaysBefore:   0,
		TargetCarbsG: raceTarget,
		Rationale:    "race morning, pre-race meal ~3-4h before start",
	})

	return &CarbLoadSchedule{
		RaceDate:     raceDay.Format("2006-01-02"),
		BodyWeightKg: p.BodyWeightKg,
		Params: EchoedParams{
			DaysBefore:        p.DaysBefore,
			CarbsPerKgPerDay:  p.CarbsPerKgPerDay,
			RaceDayCarbsPerKg: p.RaceDayCarbsPerKg,
		},
		Schedule: schedule,
	}, nil
}

func validate(p CarbLoadParams, today time.Time) error {
	if math.IsNaN(p.BodyWeightKg) || math.IsInf(p.BodyWeightKg, 0) ||
		p.BodyWeightKg < BodyWeightKgMin || p.BodyWeightKg > BodyWeightKgMax {
		return ErrBodyWeightKgInvalid
	}
	if p.DaysBefore < DaysBeforeMin || p.DaysBefore > DaysBeforeMax {
		return ErrDaysBeforeInvalid
	}
	if math.IsNaN(p.CarbsPerKgPerDay) || math.IsInf(p.CarbsPerKgPerDay, 0) ||
		p.CarbsPerKgPerDay < CarbsPerKgPerDayMin || p.CarbsPerKgPerDay > CarbsPerKgPerDayMax {
		return ErrCarbsPerKgPerDayInvalid
	}
	if math.IsNaN(p.RaceDayCarbsPerKg) || math.IsInf(p.RaceDayCarbsPerKg, 0) ||
		p.RaceDayCarbsPerKg < RaceDayCarbsPerKgMin || p.RaceDayCarbsPerKg > RaceDayCarbsPerKgMax {
		return ErrRaceDayCarbsPerKgInvalid
	}

	// race_date strictly before today (in the same local TZ) is rejected;
	// today is acceptable.
	raceDay := time.Date(p.RaceDate.Year(), p.RaceDate.Month(), p.RaceDate.Day(),
		0, 0, 0, 0, p.RaceDate.Location())
	todayDay := time.Date(today.Year(), today.Month(), today.Day(),
		0, 0, 0, 0, today.Location())
	if raceDay.Before(todayDay) {
		return ErrRaceDateInPast
	}
	return nil
}

// Service is a thin wrapper around PlanCarbLoad that carries a clock and a
// user timezone. Tests inject their own clock / TZ; production wires
// time.Now and the configured DEFAULT_USER_TZ. The pool is required only by
// ApplyCarbLoad; the read-only Plan path works without one (so tests for the
// pure-compute paths can pass `nil`).
type Service struct {
	now  func() time.Time
	tz   *time.Location
	pool *pgxpool.Pool

	// workoutsRepo + bodyWeightRepo are wired via SetWorkoutsRepo /
	// SetBodyWeightRepo for the recommend-workout-fuel endpoint
	// (add-recommend-workout-fuel). Optional setters to keep the
	// existing constructor signature stable.
	workoutsRepo   *workouts.Repo
	bodyWeightRepo *bodyweight.Repo
}

func NewService(now func() time.Time, tz *time.Location, pool *pgxpool.Pool) *Service {
	if now == nil {
		now = time.Now
	}
	if tz == nil {
		tz = time.UTC
	}
	return &Service{now: now, tz: tz, pool: pool}
}

// Plan calls PlanCarbLoad with "today" resolved from the service's clock and TZ.
func (s *Service) Plan(p CarbLoadParams) (*CarbLoadSchedule, error) {
	return PlanCarbLoad(p, s.now().In(s.tz))
}

// TZ returns the configured user timezone. Exposed so the handler can parse
// the inbound race_date in the same TZ.
func (s *Service) TZ() *time.Location { return s.tz }
