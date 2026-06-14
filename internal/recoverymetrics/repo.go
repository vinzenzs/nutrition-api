package recoverymetrics

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/vinzenzs/kazper/internal/store"
)

// ErrNotFound is returned when no snapshot exists for a date.
var ErrNotFound = errors.New("recovery metrics not found")

// Repo persists recovery snapshots.
type Repo struct {
	q store.Querier
}

func NewRepo(q store.Querier) *Repo {
	return &Repo{q: q}
}

// selectCols renders date as text so it round-trips as YYYY-MM-DD.
const selectCols = `to_char(date, 'YYYY-MM-DD') AS date, sleep_seconds, sleep_score, hrv_ms, resting_hr, stress_avg, body_battery_charged, body_battery_drained, training_readiness, spo2_avg, spo2_lowest, respiration_avg, respiration_lowest, deep_sleep_seconds, light_sleep_seconds, rem_sleep_seconds, awake_seconds, created_at, updated_at`

// Upsert inserts a snapshot, or full-replaces the metric columns when a row for
// the date already exists. Returns created=true on INSERT (HTTP 201), false on
// UPDATE (HTTP 200).
func (r *Repo) Upsert(ctx context.Context, s *Snapshot) (created bool, err error) {
	const q = `
        INSERT INTO recovery_metrics (
            date, sleep_seconds, sleep_score, hrv_ms, resting_hr, stress_avg,
            body_battery_charged, body_battery_drained, training_readiness,
            spo2_avg, spo2_lowest, respiration_avg, respiration_lowest,
            deep_sleep_seconds, light_sleep_seconds, rem_sleep_seconds, awake_seconds,
            created_at, updated_at
        ) VALUES (
            $1::date, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, now(), now()
        )
        ON CONFLICT (date) DO UPDATE SET
            sleep_seconds        = EXCLUDED.sleep_seconds,
            sleep_score          = EXCLUDED.sleep_score,
            hrv_ms               = EXCLUDED.hrv_ms,
            resting_hr           = EXCLUDED.resting_hr,
            stress_avg           = EXCLUDED.stress_avg,
            body_battery_charged = EXCLUDED.body_battery_charged,
            body_battery_drained = EXCLUDED.body_battery_drained,
            training_readiness   = EXCLUDED.training_readiness,
            spo2_avg             = EXCLUDED.spo2_avg,
            spo2_lowest          = EXCLUDED.spo2_lowest,
            respiration_avg      = EXCLUDED.respiration_avg,
            respiration_lowest   = EXCLUDED.respiration_lowest,
            deep_sleep_seconds   = EXCLUDED.deep_sleep_seconds,
            light_sleep_seconds  = EXCLUDED.light_sleep_seconds,
            rem_sleep_seconds    = EXCLUDED.rem_sleep_seconds,
            awake_seconds        = EXCLUDED.awake_seconds,
            updated_at           = now()
        RETURNING (xmax = 0) AS inserted
    `
	row := r.q.QueryRow(ctx, q,
		s.Date, s.SleepSeconds, s.SleepScore, s.HRVMs, s.RestingHR, s.StressAvg,
		s.BodyBatteryCharged, s.BodyBatteryDrained, s.TrainingReadiness,
		s.Spo2Avg, s.Spo2Lowest, s.RespirationAvg, s.RespirationLowest,
		s.DeepSleepSeconds, s.LightSleepSeconds, s.RemSleepSeconds, s.AwakeSeconds,
	)
	if err := row.Scan(&created); err != nil {
		return false, fmt.Errorf("upsert recovery metrics: %w", err)
	}
	return created, nil
}

// GetByDate returns the snapshot for a YYYY-MM-DD date, or ErrNotFound.
func (r *Repo) GetByDate(ctx context.Context, date string) (*Snapshot, error) {
	row := r.q.QueryRow(ctx,
		`SELECT `+selectCols+` FROM recovery_metrics WHERE date = $1::date`, date)
	return scanSnapshot(row)
}

// List returns snapshots with date in [from, to] inclusive, ordered ASC.
func (r *Repo) List(ctx context.Context, from, to string) ([]*Snapshot, error) {
	rows, err := r.q.Query(ctx,
		`SELECT `+selectCols+` FROM recovery_metrics WHERE date >= $1::date AND date <= $2::date ORDER BY date ASC`,
		from, to,
	)
	if err != nil {
		return nil, fmt.Errorf("list recovery metrics: %w", err)
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
	tag, err := r.q.Exec(ctx, `DELETE FROM recovery_metrics WHERE date = $1::date`, date)
	if err != nil {
		return fmt.Errorf("delete recovery metrics: %w", err)
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
		&snap.Date, &snap.SleepSeconds, &snap.SleepScore, &snap.HRVMs, &snap.RestingHR,
		&snap.StressAvg, &snap.BodyBatteryCharged, &snap.BodyBatteryDrained, &snap.TrainingReadiness,
		&snap.Spo2Avg, &snap.Spo2Lowest, &snap.RespirationAvg, &snap.RespirationLowest,
		&snap.DeepSleepSeconds, &snap.LightSleepSeconds, &snap.RemSleepSeconds, &snap.AwakeSeconds,
		&snap.CreatedAt, &snap.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan recovery metrics: %w", err)
	}
	return &snap, nil
}
