package athleteconfig

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/vinzenzs/nutrition-api/internal/store"
)

// singletonID is the fixed primary key of the one allowed athlete_config row.
const singletonID = "00000000-0000-0000-0000-000000000001"

// Repo persists the athlete_config singleton row.
type Repo struct {
	q store.Querier
}

func NewRepo(q store.Querier) *Repo {
	return &Repo{q: q}
}

const selectCols = `
    ftp_watts, threshold_hr, lactate_threshold_hr, max_hr,
    threshold_pace_sec_per_km, threshold_swim_pace_sec_per_100m,
    hr_zone_1_max, hr_zone_2_max, hr_zone_3_max, hr_zone_4_max, hr_zone_5_max,
    power_zone_1_max, power_zone_2_max, power_zone_3_max, power_zone_4_max, power_zone_5_max,
    created_at, updated_at
`

// Get returns the config row, or (nil, nil) if no row exists yet. The nil-row
// signal is distinct from any DB error so the handler can return
// {"athlete_config": null} straightforwardly.
func (r *Repo) Get(ctx context.Context) (*AthleteConfig, error) {
	row := r.q.QueryRow(ctx, `SELECT `+selectCols+` FROM athlete_config WHERE id = $1`, singletonID)
	cfg, err := scanConfig(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("scan athlete config: %w", err)
	}
	return cfg, nil
}

// Upsert writes the singleton row, replacing all field values with what's on
// cfg. Absent fields (nil pointers) overwrite to NULL — full-replace PUT
// semantics, matching PUT /goals.
func (r *Repo) Upsert(ctx context.Context, cfg *AthleteConfig) error {
	const q = `
        INSERT INTO athlete_config (
            id,
            ftp_watts, threshold_hr, lactate_threshold_hr, max_hr,
            threshold_pace_sec_per_km, threshold_swim_pace_sec_per_100m,
            hr_zone_1_max, hr_zone_2_max, hr_zone_3_max, hr_zone_4_max, hr_zone_5_max,
            power_zone_1_max, power_zone_2_max, power_zone_3_max, power_zone_4_max, power_zone_5_max,
            updated_at
        ) VALUES (
            $1,
            $2, $3, $4, $5,
            $6, $7,
            $8, $9, $10, $11, $12,
            $13, $14, $15, $16, $17,
            now()
        )
        ON CONFLICT (id) DO UPDATE SET
            ftp_watts                        = EXCLUDED.ftp_watts,
            threshold_hr                     = EXCLUDED.threshold_hr,
            lactate_threshold_hr             = EXCLUDED.lactate_threshold_hr,
            max_hr                           = EXCLUDED.max_hr,
            threshold_pace_sec_per_km        = EXCLUDED.threshold_pace_sec_per_km,
            threshold_swim_pace_sec_per_100m = EXCLUDED.threshold_swim_pace_sec_per_100m,
            hr_zone_1_max                    = EXCLUDED.hr_zone_1_max,
            hr_zone_2_max                    = EXCLUDED.hr_zone_2_max,
            hr_zone_3_max                    = EXCLUDED.hr_zone_3_max,
            hr_zone_4_max                    = EXCLUDED.hr_zone_4_max,
            hr_zone_5_max                    = EXCLUDED.hr_zone_5_max,
            power_zone_1_max                 = EXCLUDED.power_zone_1_max,
            power_zone_2_max                 = EXCLUDED.power_zone_2_max,
            power_zone_3_max                 = EXCLUDED.power_zone_3_max,
            power_zone_4_max                 = EXCLUDED.power_zone_4_max,
            power_zone_5_max                 = EXCLUDED.power_zone_5_max,
            updated_at                       = now()
    `
	_, err := r.q.Exec(ctx, q,
		singletonID,
		cfg.FtpWatts, cfg.ThresholdHR, cfg.LactateThresholdHR, cfg.MaxHR,
		cfg.ThresholdPaceSecPerKm, cfg.ThresholdSwimPaceSecPer100m,
		cfg.HRZone1Max, cfg.HRZone2Max, cfg.HRZone3Max, cfg.HRZone4Max, cfg.HRZone5Max,
		cfg.PowerZone1Max, cfg.PowerZone2Max, cfg.PowerZone3Max, cfg.PowerZone4Max, cfg.PowerZone5Max,
	)
	if err != nil {
		return fmt.Errorf("upsert athlete config: %w", err)
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanConfig(s scanner) (*AthleteConfig, error) {
	var cfg AthleteConfig
	err := s.Scan(
		&cfg.FtpWatts, &cfg.ThresholdHR, &cfg.LactateThresholdHR, &cfg.MaxHR,
		&cfg.ThresholdPaceSecPerKm, &cfg.ThresholdSwimPaceSecPer100m,
		&cfg.HRZone1Max, &cfg.HRZone2Max, &cfg.HRZone3Max, &cfg.HRZone4Max, &cfg.HRZone5Max,
		&cfg.PowerZone1Max, &cfg.PowerZone2Max, &cfg.PowerZone3Max, &cfg.PowerZone4Max, &cfg.PowerZone5Max,
		&cfg.CreatedAt, &cfg.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}
