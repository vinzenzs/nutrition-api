package personalrecords

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/vinzenzs/kazper/internal/store"
)

// ErrNotFound is returned when no record exists for an id.
var ErrNotFound = errors.New("personal record not found")

// Repo persists personal-record rows.
type Repo struct {
	q store.Querier
}

func NewRepo(q store.Querier) *Repo {
	return &Repo{q: q}
}

const selectCols = `id, external_id, pr_type, value, unit, activity_id, achieved_at, created_at, updated_at`

// Upsert inserts a record, or full-replaces its fields when a row with the same
// external_id already exists (a beaten PR overwrites the prior value). Returns
// created=true on INSERT (HTTP 201), false on UPDATE (HTTP 200). The backend id
// is generated on insert and preserved on update; it is reflected onto pr.
func (r *Repo) Upsert(ctx context.Context, pr *PersonalRecord) (created bool, err error) {
	const q = `
        INSERT INTO personal_records (
            id, external_id, pr_type, value, unit, activity_id, achieved_at,
            created_at, updated_at
        ) VALUES (
            gen_random_uuid(), $1, $2, $3, $4, $5, $6, now(), now()
        )
        ON CONFLICT (external_id) DO UPDATE SET
            pr_type     = EXCLUDED.pr_type,
            value       = EXCLUDED.value,
            unit        = EXCLUDED.unit,
            activity_id = EXCLUDED.activity_id,
            achieved_at = EXCLUDED.achieved_at,
            updated_at  = now()
        RETURNING id, (xmax = 0) AS inserted
    `
	row := r.q.QueryRow(ctx, q,
		pr.ExternalID, pr.PRType, pr.Value, pr.Unit, pr.ActivityID, pr.AchievedAt,
	)
	if err := row.Scan(&pr.ID, &created); err != nil {
		return false, fmt.Errorf("upsert personal record: %w", err)
	}
	return created, nil
}

// GetByID returns the record with the given backend id, or ErrNotFound.
func (r *Repo) GetByID(ctx context.Context, id any) (*PersonalRecord, error) {
	row := r.q.QueryRow(ctx, `SELECT `+selectCols+` FROM personal_records WHERE id = $1`, id)
	return scanRecord(row)
}

// List returns records ordered by achieved_at descending (most recent first).
// When prType is non-nil it narrows to that PR type.
func (r *Repo) List(ctx context.Context, prType *string) ([]*PersonalRecord, error) {
	q := `SELECT ` + selectCols + ` FROM personal_records`
	args := []any{}
	if prType != nil {
		q += ` WHERE pr_type = $1`
		args = append(args, *prType)
	}
	q += ` ORDER BY achieved_at DESC`
	rows, err := r.q.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list personal records: %w", err)
	}
	defer rows.Close()
	var out []*PersonalRecord
	for rows.Next() {
		pr, err := scanRecord(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, pr)
	}
	return out, rows.Err()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanRecord(s scanner) (*PersonalRecord, error) {
	var pr PersonalRecord
	err := s.Scan(
		&pr.ID, &pr.ExternalID, &pr.PRType, &pr.Value, &pr.Unit, &pr.ActivityID,
		&pr.AchievedAt, &pr.CreatedAt, &pr.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan personal record: %w", err)
	}
	return &pr, nil
}
