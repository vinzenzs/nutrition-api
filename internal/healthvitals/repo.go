package healthvitals

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/vinzenzs/kazper/internal/store"
)

// ErrNotFound is returned when no snapshot exists for a date.
var ErrNotFound = errors.New("health vitals not found")

// Repo persists health-vitals snapshots.
type Repo struct {
	q store.Querier
}

func NewRepo(q store.Querier) *Repo {
	return &Repo{q: q}
}

// selectCols renders date as text so it round-trips as YYYY-MM-DD.
const selectCols = `to_char(date, 'YYYY-MM-DD') AS date, bp_systolic, bp_diastolic, bp_pulse, resting_hr, min_hr, max_hr, stress_avg, stress_max, created_at, updated_at`

// Upsert inserts a snapshot, or full-replaces the metric columns when a row for
// the date exists. Returns created=true on INSERT (HTTP 201).
func (r *Repo) Upsert(ctx context.Context, s *Snapshot) (created bool, err error) {
	const q = `
        INSERT INTO health_vitals (
            date, bp_systolic, bp_diastolic, bp_pulse, resting_hr, min_hr, max_hr, stress_avg, stress_max,
            created_at, updated_at
        ) VALUES (
            $1::date, $2, $3, $4, $5, $6, $7, $8, $9, now(), now()
        )
        ON CONFLICT (date) DO UPDATE SET
            bp_systolic  = EXCLUDED.bp_systolic,
            bp_diastolic = EXCLUDED.bp_diastolic,
            bp_pulse     = EXCLUDED.bp_pulse,
            resting_hr   = EXCLUDED.resting_hr,
            min_hr       = EXCLUDED.min_hr,
            max_hr       = EXCLUDED.max_hr,
            stress_avg   = EXCLUDED.stress_avg,
            stress_max   = EXCLUDED.stress_max,
            updated_at   = now()
        RETURNING (xmax = 0) AS inserted
    `
	row := r.q.QueryRow(ctx, q,
		s.Date, s.BPSystolic, s.BPDiastolic, s.BPPulse, s.RestingHR, s.MinHR, s.MaxHR, s.StressAvg, s.StressMax,
	)
	if err := row.Scan(&created); err != nil {
		return false, fmt.Errorf("upsert health vitals: %w", err)
	}
	return created, nil
}

// GetByDate returns the snapshot for a YYYY-MM-DD date, or ErrNotFound.
func (r *Repo) GetByDate(ctx context.Context, date string) (*Snapshot, error) {
	row := r.q.QueryRow(ctx, `SELECT `+selectCols+` FROM health_vitals WHERE date = $1::date`, date)
	return scanSnapshot(row)
}

// List returns snapshots with date in [from, to] inclusive, ordered ASC.
func (r *Repo) List(ctx context.Context, from, to string) ([]*Snapshot, error) {
	rows, err := r.q.Query(ctx,
		`SELECT `+selectCols+` FROM health_vitals WHERE date >= $1::date AND date <= $2::date ORDER BY date ASC`,
		from, to,
	)
	if err != nil {
		return nil, fmt.Errorf("list health vitals: %w", err)
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

type scanner interface {
	Scan(dest ...any) error
}

func scanSnapshot(s scanner) (*Snapshot, error) {
	var snap Snapshot
	err := s.Scan(
		&snap.Date, &snap.BPSystolic, &snap.BPDiastolic, &snap.BPPulse, &snap.RestingHR,
		&snap.MinHR, &snap.MaxHR, &snap.StressAvg, &snap.StressMax, &snap.CreatedAt, &snap.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan health vitals: %w", err)
	}
	return &snap, nil
}
