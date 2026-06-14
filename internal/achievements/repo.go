package achievements

import (
	"context"
	"fmt"

	"github.com/vinzenzs/kazper/internal/store"
)

// Repo persists achievement rows.
type Repo struct {
	q store.Querier
}

func NewRepo(q store.Querier) *Repo {
	return &Repo{q: q}
}

const selectCols = `id, external_id, kind, name, earned_at, progress_pct, created_at, updated_at`

// Upsert inserts a row, or full-replaces its fields when a row with the same
// external_id exists (a challenge's progress advances, a badge's earned_at is
// set when completed). Returns created=true on INSERT (HTTP 201).
func (r *Repo) Upsert(ctx context.Context, a *Achievement) (created bool, err error) {
	const q = `
        INSERT INTO achievements (
            id, external_id, kind, name, earned_at, progress_pct, created_at, updated_at
        ) VALUES (
            gen_random_uuid(), $1, $2, $3, $4, $5, now(), now()
        )
        ON CONFLICT (external_id) DO UPDATE SET
            kind         = EXCLUDED.kind,
            name         = EXCLUDED.name,
            earned_at    = EXCLUDED.earned_at,
            progress_pct = EXCLUDED.progress_pct,
            updated_at   = now()
        RETURNING id, (xmax = 0) AS inserted
    `
	row := r.q.QueryRow(ctx, q, a.ExternalID, string(a.Kind), a.Name, a.EarnedAt, a.ProgressPct)
	if err := row.Scan(&a.ID, &created); err != nil {
		return false, fmt.Errorf("upsert achievement: %w", err)
	}
	return created, nil
}

// List returns achievements ordered by earned_at DESC with NULLs (in-progress
// challenges) last. When kind is non-nil it narrows to that kind.
func (r *Repo) List(ctx context.Context, kind *string) ([]*Achievement, error) {
	q := `SELECT ` + selectCols + ` FROM achievements`
	args := []any{}
	if kind != nil {
		q += ` WHERE kind = $1`
		args = append(args, *kind)
	}
	q += ` ORDER BY earned_at DESC NULLS LAST, created_at DESC`
	rows, err := r.q.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list achievements: %w", err)
	}
	defer rows.Close()
	var out []*Achievement
	for rows.Next() {
		a, err := scanAchievement(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanAchievement(s scanner) (*Achievement, error) {
	var (
		a       Achievement
		kindStr string
	)
	if err := s.Scan(&a.ID, &a.ExternalID, &kindStr, &a.Name, &a.EarnedAt, &a.ProgressPct, &a.CreatedAt, &a.UpdatedAt); err != nil {
		return nil, fmt.Errorf("scan achievement: %w", err)
	}
	a.Kind = Kind(kindStr)
	return &a, nil
}
