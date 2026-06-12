package fitnessmetrics

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"
)

// Validation errors map 1:1 to API error codes.
var (
	ErrDateInvalid              = errors.New("date_invalid")
	ErrVO2MaxRunningInvalid     = errors.New("vo2max_running_invalid")
	ErrVO2MaxCyclingInvalid     = errors.New("vo2max_cycling_invalid")
	ErrRacePredictor5kInvalid   = errors.New("race_predictor_5k_seconds_invalid")
	ErrRacePredictor10kInvalid  = errors.New("race_predictor_10k_seconds_invalid")
	ErrRacePredictorHalfInvalid = errors.New("race_predictor_half_seconds_invalid")
	ErrRacePredictorFullInvalid = errors.New("race_predictor_full_seconds_invalid")
	ErrAcuteLoadInvalid         = errors.New("acute_load_invalid")
	ErrChronicLoadInvalid       = errors.New("chronic_load_invalid")
	ErrEnduranceScoreInvalid    = errors.New("endurance_score_invalid")
	ErrHillScoreInvalid         = errors.New("hill_score_invalid")
	ErrFitnessAgeInvalid        = errors.New("fitness_age_invalid")
	ErrTrainingStatusInvalid    = errors.New("training_status_invalid")
)

// maxTrainingStatusLen mirrors the column CHECK (length BETWEEN 1 AND 64): the
// label is stored verbatim, NOT gated against a fixed enum, so a future Garmin
// vocabulary is preserved rather than dropped.
const maxTrainingStatusLen = 64

const dateLayout = "2006-01-02"

// Service orchestrates fitness-metrics CRUD over the repo.
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

func validate(s *Snapshot) error {
	if err := posFloat(s.VO2MaxRunning, ErrVO2MaxRunningInvalid); err != nil {
		return err
	}
	if err := posFloat(s.VO2MaxCycling, ErrVO2MaxCyclingInvalid); err != nil {
		return err
	}
	if err := posInt(s.RacePredictor5kSeconds, ErrRacePredictor5kInvalid); err != nil {
		return err
	}
	if err := posInt(s.RacePredictor10kSeconds, ErrRacePredictor10kInvalid); err != nil {
		return err
	}
	if err := posInt(s.RacePredictorHalfSeconds, ErrRacePredictorHalfInvalid); err != nil {
		return err
	}
	if err := posInt(s.RacePredictorFullSeconds, ErrRacePredictorFullInvalid); err != nil {
		return err
	}
	if err := nonNegFloat(s.AcuteLoad, ErrAcuteLoadInvalid); err != nil {
		return err
	}
	if err := nonNegFloat(s.ChronicLoad, ErrChronicLoadInvalid); err != nil {
		return err
	}
	if err := posInt(s.EnduranceScore, ErrEnduranceScoreInvalid); err != nil {
		return err
	}
	if err := posInt(s.HillScore, ErrHillScoreInvalid); err != nil {
		return err
	}
	if err := posFloat(s.FitnessAge, ErrFitnessAgeInvalid); err != nil {
		return err
	}
	if err := validTrainingStatus(s); err != nil {
		return err
	}
	return nil
}

// validTrainingStatus trims the label in place and rejects only empty/oversized
// strings — it is stored verbatim, never gated against a fixed enum (design D4).
func validTrainingStatus(s *Snapshot) error {
	if s.TrainingStatus == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*s.TrainingStatus)
	if trimmed == "" || len(trimmed) > maxTrainingStatusLen {
		return ErrTrainingStatusInvalid
	}
	s.TrainingStatus = &trimmed
	return nil
}

func posInt(v *int, e error) error {
	if v != nil && *v <= 0 {
		return e
	}
	return nil
}

func posFloat(v *float64, e error) error {
	if v != nil && (math.IsNaN(*v) || math.IsInf(*v, 0) || *v <= 0) {
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
