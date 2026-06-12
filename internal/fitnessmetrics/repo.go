package fitnessmetrics

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/vinzenzs/nutrition-api/internal/store"
)

// ErrNotFound is returned when no snapshot exists for a date.
var ErrNotFound = errors.New("fitness metrics not found")

// Repo persists fitness snapshots.
type Repo struct {
	q store.Querier
}

func NewRepo(q store.Querier) *Repo {
	return &Repo{q: q}
}

const selectCols = `to_char(date, 'YYYY-MM-DD') AS date, vo2max_running, vo2max_cycling, race_predictor_5k_seconds, race_predictor_10k_seconds, race_predictor_half_seconds, race_predictor_full_seconds, acute_load, chronic_load, endurance_score, hill_score, fitness_age, training_status, created_at, updated_at`

// Upsert inserts a snapshot, or full-replaces the metric columns when a row for
// the date exists. created=true on INSERT (HTTP 201), false on UPDATE (HTTP 200).
func (r *Repo) Upsert(ctx context.Context, s *Snapshot) (created bool, err error) {
	const q = `
        INSERT INTO fitness_metrics (
            date, vo2max_running, vo2max_cycling,
            race_predictor_5k_seconds, race_predictor_10k_seconds,
            race_predictor_half_seconds, race_predictor_full_seconds,
            acute_load, chronic_load,
            endurance_score, hill_score, fitness_age, training_status,
            created_at, updated_at
        ) VALUES (
            $1::date, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, now(), now()
        )
        ON CONFLICT (date) DO UPDATE SET
            vo2max_running              = EXCLUDED.vo2max_running,
            vo2max_cycling              = EXCLUDED.vo2max_cycling,
            race_predictor_5k_seconds   = EXCLUDED.race_predictor_5k_seconds,
            race_predictor_10k_seconds  = EXCLUDED.race_predictor_10k_seconds,
            race_predictor_half_seconds = EXCLUDED.race_predictor_half_seconds,
            race_predictor_full_seconds = EXCLUDED.race_predictor_full_seconds,
            acute_load                  = EXCLUDED.acute_load,
            chronic_load                = EXCLUDED.chronic_load,
            endurance_score             = EXCLUDED.endurance_score,
            hill_score                  = EXCLUDED.hill_score,
            fitness_age                 = EXCLUDED.fitness_age,
            training_status             = EXCLUDED.training_status,
            updated_at                  = now()
        RETURNING (xmax = 0) AS inserted
    `
	row := r.q.QueryRow(ctx, q,
		s.Date, s.VO2MaxRunning, s.VO2MaxCycling,
		s.RacePredictor5kSeconds, s.RacePredictor10kSeconds,
		s.RacePredictorHalfSeconds, s.RacePredictorFullSeconds,
		s.AcuteLoad, s.ChronicLoad,
		s.EnduranceScore, s.HillScore, s.FitnessAge, s.TrainingStatus,
	)
	if err := row.Scan(&created); err != nil {
		return false, fmt.Errorf("upsert fitness metrics: %w", err)
	}
	return created, nil
}

// GetByDate returns the snapshot for a YYYY-MM-DD date, or ErrNotFound.
func (r *Repo) GetByDate(ctx context.Context, date string) (*Snapshot, error) {
	row := r.q.QueryRow(ctx,
		`SELECT `+selectCols+` FROM fitness_metrics WHERE date = $1::date`, date)
	return scanSnapshot(row)
}

// List returns snapshots with date in [from, to] inclusive, ordered ASC.
func (r *Repo) List(ctx context.Context, from, to string) ([]*Snapshot, error) {
	rows, err := r.q.Query(ctx,
		`SELECT `+selectCols+` FROM fitness_metrics WHERE date >= $1::date AND date <= $2::date ORDER BY date ASC`,
		from, to,
	)
	if err != nil {
		return nil, fmt.Errorf("list fitness metrics: %w", err)
	}
	defer rows.Close()
	var out []*Snapshot
	for rows.Next() {
		s, err := scanSnapshot(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// DeleteByDate removes the snapshot for a date. Returns ErrNotFound if none.
func (r *Repo) DeleteByDate(ctx context.Context, date string) error {
	tag, err := r.q.Exec(ctx, `DELETE FROM fitness_metrics WHERE date = $1::date`, date)
	if err != nil {
		return fmt.Errorf("delete fitness metrics: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanSnapshot(s scanner) (*Snapshot, error) {
	var snap Snapshot
	err := s.Scan(
		&snap.Date, &snap.VO2MaxRunning, &snap.VO2MaxCycling,
		&snap.RacePredictor5kSeconds, &snap.RacePredictor10kSeconds,
		&snap.RacePredictorHalfSeconds, &snap.RacePredictorFullSeconds,
		&snap.AcuteLoad, &snap.ChronicLoad,
		&snap.EnduranceScore, &snap.HillScore, &snap.FitnessAge, &snap.TrainingStatus,
		&snap.CreatedAt, &snap.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan fitness metrics: %w", err)
	}
	return &snap, nil
}
