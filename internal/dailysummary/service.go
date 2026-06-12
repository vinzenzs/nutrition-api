package dailysummary

import (
	"context"
	"errors"
	"math"
	"time"
)

// Validation errors map 1:1 to API error codes.
var (
	ErrDateInvalid                     = errors.New("date_invalid")
	ErrActiveKcalInvalid               = errors.New("active_kcal_invalid")
	ErrRestingKcalInvalid              = errors.New("resting_kcal_invalid")
	ErrTotalKcalInvalid                = errors.New("total_kcal_invalid")
	ErrStepsInvalid                    = errors.New("steps_invalid")
	ErrFloorsInvalid                   = errors.New("floors_invalid")
	ErrModerateIntensityMinutesInvalid = errors.New("moderate_intensity_minutes_invalid")
	ErrVigorousIntensityMinutesInvalid = errors.New("vigorous_intensity_minutes_invalid")
	ErrDistanceMInvalid                = errors.New("distance_m_invalid")
)

// dateLayout is the canonical YYYY-MM-DD form for the snapshot identity.
const dateLayout = "2006-01-02"

// Service orchestrates daily-summary CRUD over the repo.
type Service struct {
	repo *Repo
}

func NewService(repo *Repo) *Service {
	return &Service{repo: repo}
}

// Upsert validates and date-keyed-upserts a snapshot. created=true on INSERT.
func (s *Service) Upsert(ctx context.Context, in *Snapshot) (*Snapshot, bool, error) {
	if !validDate(in.Date) {
		return nil, false, ErrDateInvalid
	}
	if err := validate(in); err != nil {
		return nil, false, err
	}
	created, err := s.repo.Upsert(ctx, in)
	if err != nil {
		return nil, false, err
	}
	out, err := s.repo.GetByDate(ctx, in.Date)
	if err != nil {
		return nil, false, err
	}
	return out, created, nil
}

// Get returns the snapshot for a date.
func (s *Service) Get(ctx context.Context, date string) (*Snapshot, error) {
	if !validDate(date) {
		return nil, ErrDateInvalid
	}
	return s.repo.GetByDate(ctx, date)
}

// ListWindow returns snapshots in [from, to].
func (s *Service) ListWindow(ctx context.Context, from, to string) ([]*Snapshot, error) {
	return s.repo.List(ctx, from, to)
}

// Delete removes the snapshot for a date.
func (s *Service) Delete(ctx context.Context, date string) error {
	if !validDate(date) {
		return ErrDateInvalid
	}
	return s.repo.DeleteByDate(ctx, date)
}

// ----- validators -----

func validDate(d string) bool {
	if d == "" {
		return false
	}
	_, err := time.Parse(dateLayout, d)
	return err == nil
}

// validate rejects any negative metric. Zero is a valid count (no floors today),
// so the rule is >= 0, distinct from the recovery snapshot's > 0 convention.
func validate(s *Snapshot) error {
	if err := nonNegInt(s.ActiveKcal, ErrActiveKcalInvalid); err != nil {
		return err
	}
	if err := nonNegInt(s.RestingKcal, ErrRestingKcalInvalid); err != nil {
		return err
	}
	if err := nonNegInt(s.TotalKcal, ErrTotalKcalInvalid); err != nil {
		return err
	}
	if err := nonNegInt(s.Steps, ErrStepsInvalid); err != nil {
		return err
	}
	if err := nonNegInt(s.Floors, ErrFloorsInvalid); err != nil {
		return err
	}
	if err := nonNegInt(s.ModerateIntensityMinutes, ErrModerateIntensityMinutesInvalid); err != nil {
		return err
	}
	if err := nonNegInt(s.VigorousIntensityMinutes, ErrVigorousIntensityMinutesInvalid); err != nil {
		return err
	}
	if err := nonNegFloat(s.DistanceM, ErrDistanceMInvalid); err != nil {
		return err
	}
	return nil
}

func nonNegInt(v *int, e error) error {
	if v != nil && *v < 0 {
		return e
	}
	return nil
}

func nonNegFloat(v *float64, e error) error {
	if v != nil && (math.IsNaN(*v) || math.IsInf(*v, 0) || *v < 0) {
		return e
	}
	return nil
}
