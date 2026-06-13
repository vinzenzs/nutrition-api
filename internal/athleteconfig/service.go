package athleteconfig

import (
	"context"
	"fmt"
	"math"
)

// ValidationError carries the spec-defined error code plus the offending field
// name. All athlete-config validation failures use the single code
// `athlete_config_value_invalid` with a `field` hint.
type ValidationError struct {
	Field string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("athlete_config_value_invalid: %s", e.Field)
}

// Service orchestrates the athlete-config singleton over the repo.
type Service struct {
	repo *Repo
}

func NewService(repo *Repo) *Service {
	return &Service{repo: repo}
}

// Get returns the singleton config, or (nil, nil) when none has been written.
func (s *Service) Get(ctx context.Context) (*AthleteConfig, error) {
	return s.repo.Get(ctx)
}

// Put validates and full-replaces the singleton config, then reads it back.
func (s *Service) Put(ctx context.Context, cfg *AthleteConfig) (*AthleteConfig, error) {
	if err := validate(cfg); err != nil {
		return nil, err
	}
	if err := s.repo.Upsert(ctx, cfg); err != nil {
		return nil, err
	}
	return s.repo.Get(ctx)
}

// validate rejects any present field that is not strictly positive (matching
// the column CHECKs) or, for floats, not finite. Each field is independent.
func validate(cfg *AthleteConfig) error {
	ints := []struct {
		name string
		v    *int
	}{
		{"ftp_watts", cfg.FtpWatts},
		{"threshold_hr", cfg.ThresholdHR},
		{"lactate_threshold_hr", cfg.LactateThresholdHR},
		{"max_hr", cfg.MaxHR},
		{"hr_zone_1_max", cfg.HRZone1Max},
		{"hr_zone_2_max", cfg.HRZone2Max},
		{"hr_zone_3_max", cfg.HRZone3Max},
		{"hr_zone_4_max", cfg.HRZone4Max},
		{"hr_zone_5_max", cfg.HRZone5Max},
		{"power_zone_1_max", cfg.PowerZone1Max},
		{"power_zone_2_max", cfg.PowerZone2Max},
		{"power_zone_3_max", cfg.PowerZone3Max},
		{"power_zone_4_max", cfg.PowerZone4Max},
		{"power_zone_5_max", cfg.PowerZone5Max},
	}
	for _, f := range ints {
		if f.v != nil && *f.v <= 0 {
			return &ValidationError{Field: f.name}
		}
	}
	floats := []struct {
		name string
		v    *float64
	}{
		{"threshold_pace_sec_per_km", cfg.ThresholdPaceSecPerKm},
		{"threshold_swim_pace_sec_per_100m", cfg.ThresholdSwimPaceSecPer100m},
	}
	for _, f := range floats {
		if f.v != nil && (math.IsNaN(*f.v) || math.IsInf(*f.v, 0) || *f.v <= 0) {
			return &ValidationError{Field: f.name}
		}
	}
	return nil
}
