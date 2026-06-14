package hydrationbalance

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/vinzenzs/kazper/internal/store"
)

// ErrNotFound is returned when no snapshot exists for a date.
var ErrNotFound = errors.New("hydration balance not found")

// Repo persists hydration-balance snapshots.
type Repo struct {
	q store.Querier
}

func NewRepo(q store.Querier) *Repo {
	return &Repo{q: q}
}

// selectCols renders date as text so it round-trips as YYYY-MM-DD.
const selectCols = `to_char(date, 'YYYY-MM-DD') AS date, sweat_loss_ml, activity_intake_ml, goal_ml, created_at, updated_at`

// Upsert inserts a snapshot, or full-replaces the metric columns when a row for
// the date already exists. created=true on INSERT (HTTP 201), false on UPDATE
// (HTTP 200).
func (r *Repo) Upsert(ctx context.Context, s *Snapshot) (created bool, err error) {
	const q = `
        INSERT INTO hydration_balance_metrics (
            date, sweat_loss_ml, activity_intake_ml, goal_ml, created_at, updated_at
        ) VALUES (
            $1::date, $2, $3, $4, now(), now()
        )
        ON CONFLICT (date) DO UPDATE SET
            sweat_loss_ml      = EXCLUDED.sweat_loss_ml,
            activity_intake_ml = EXCLUDED.activity_intake_ml,
            goal_ml            = EXCLUDED.goal_ml,
            updated_at         = now()
        RETURNING (xmax = 0) AS inserted
    `
	row := r.q.QueryRow(ctx, q, s.Date, s.SweatLossML, s.ActivityIntakeML, s.GoalML)
	if err := row.Scan(&created); err != nil {
		return false, fmt.Errorf("upsert hydration balance: %w", err)
	}
	return created, nil
}

// GetByDate returns the snapshot for a YYYY-MM-DD date, or ErrNotFound.
func (r *Repo) GetByDate(ctx context.Context, date string) (*Snapshot, error) {
	row := r.q.QueryRow(ctx,
		`SELECT `+selectCols+` FROM hydration_balance_metrics WHERE date = $1::date`, date)
	return scanSnapshot(row)
}

// List returns snapshots with date in [from, to] inclusive, ordered ASC.
func (r *Repo) List(ctx context.Context, from, to string) ([]*Snapshot, error) {
	rows, err := r.q.Query(ctx,
		`SELECT `+selectCols+` FROM hydration_balance_metrics WHERE date >= $1::date AND date <= $2::date ORDER BY date ASC`,
		from, to,
	)
	if err != nil {
		return nil, fmt.Errorf("list hydration balance: %w", err)
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
	tag, err := r.q.Exec(ctx, `DELETE FROM hydration_balance_metrics WHERE date = $1::date`, date)
	if err != nil {
		return fmt.Errorf("delete hydration balance: %w", err)
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
		&snap.Date, &snap.SweatLossML, &snap.ActivityIntakeML, &snap.GoalML,
		&snap.CreatedAt, &snap.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan hydration balance: %w", err)
	}
	return &snap, nil
}
