package recoverymetrics

import (
	"context"
	"errors"
	"math"
	"time"
)

// Validation errors map 1:1 to API error codes.
var (
	ErrDateInvalid               = errors.New("date_invalid")
	ErrSleepSecondsInvalid       = errors.New("sleep_seconds_invalid")
	ErrSleepScoreInvalid         = errors.New("sleep_score_invalid")
	ErrHRVInvalid                = errors.New("hrv_ms_invalid")
	ErrRestingHRInvalid          = errors.New("resting_hr_invalid")
	ErrStressAvgInvalid          = errors.New("stress_avg_invalid")
	ErrBodyBatteryChargedInvalid = errors.New("body_battery_charged_invalid")
	ErrBodyBatteryDrainedInvalid = errors.New("body_battery_drained_invalid")
	ErrTrainingReadinessInvalid  = errors.New("training_readiness_invalid")
	ErrSpo2AvgInvalid            = errors.New("spo2_avg_invalid")
	ErrSpo2LowestInvalid         = errors.New("spo2_lowest_invalid")
	ErrRespirationAvgInvalid     = errors.New("respiration_avg_invalid")
	ErrRespirationLowestInvalid  = errors.New("respiration_lowest_invalid")
	ErrDeepSleepSecondsInvalid   = errors.New("deep_sleep_seconds_invalid")
	ErrLightSleepSecondsInvalid  = errors.New("light_sleep_seconds_invalid")
	ErrRemSleepSecondsInvalid    = errors.New("rem_sleep_seconds_invalid")
	ErrAwakeSecondsInvalid       = errors.New("awake_seconds_invalid")
)

// dateLayout is the canonical YYYY-MM-DD form for the snapshot identity.
const dateLayout = "2006-01-02"

// Service orchestrates recovery-metrics CRUD over the repo.
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
	if err := posInt(s.SleepSeconds, ErrSleepSecondsInvalid); err != nil {
		return err
	}
	if err := rangeInt(s.SleepScore, 0, 100, ErrSleepScoreInvalid); err != nil {
		return err
	}
	if err := posFloat(s.HRVMs, ErrHRVInvalid); err != nil {
		return err
	}
	if err := posInt(s.RestingHR, ErrRestingHRInvalid); err != nil {
		return err
	}
	if err := rangeInt(s.StressAvg, 0, 100, ErrStressAvgInvalid); err != nil {
		return err
	}
	if err := rangeInt(s.BodyBatteryCharged, 0, 100, ErrBodyBatteryChargedInvalid); err != nil {
		return err
	}
	if err := rangeInt(s.BodyBatteryDrained, 0, 100, ErrBodyBatteryDrainedInvalid); err != nil {
		return err
	}
	if err := rangeInt(s.TrainingReadiness, 0, 100, ErrTrainingReadinessInvalid); err != nil {
		return err
	}
	if err := rangeInt(s.Spo2Avg, 0, 100, ErrSpo2AvgInvalid); err != nil {
		return err
	}
	if err := rangeInt(s.Spo2Lowest, 0, 100, ErrSpo2LowestInvalid); err != nil {
		return err
	}
	if err := posFloat(s.RespirationAvg, ErrRespirationAvgInvalid); err != nil {
		return err
	}
	if err := posFloat(s.RespirationLowest, ErrRespirationLowestInvalid); err != nil {
		return err
	}
	if err := nonNegInt(s.DeepSleepSeconds, ErrDeepSleepSecondsInvalid); err != nil {
		return err
	}
	if err := nonNegInt(s.LightSleepSeconds, ErrLightSleepSecondsInvalid); err != nil {
		return err
	}
	if err := nonNegInt(s.RemSleepSeconds, ErrRemSleepSecondsInvalid); err != nil {
		return err
	}
	if err := nonNegInt(s.AwakeSeconds, ErrAwakeSecondsInvalid); err != nil {
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

func posInt(v *int, e error) error {
	if v != nil && *v <= 0 {
		return e
	}
	return nil
}

func rangeInt(v *int, lo, hi int, e error) error {
	if v != nil && (*v < lo || *v > hi) {
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
