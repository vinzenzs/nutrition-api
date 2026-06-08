package hydration

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/vinzenzs/nutrition-api/internal/store"
)

// ErrNotFound is returned when a hydration entry does not exist.
var ErrNotFound = errors.New("hydration entry not found")

// Repo persists hydration entries.
type Repo struct {
	q store.Querier
}

func NewRepo(q store.Querier) *Repo {
	return &Repo{q: q}
}

const selectCols = `id, logged_at, quantity_ml, note, workout_id, created_at, updated_at`

// Insert creates a hydration_entries row. The supplied Entry's ID/CreatedAt/
// UpdatedAt are set on success.
func (r *Repo) Insert(ctx context.Context, e *Entry) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	now := time.Now().UTC()
	const q = `
        INSERT INTO hydration_entries (id, logged_at, quantity_ml, note, workout_id, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, $6)
    `
	if _, err := r.q.Exec(ctx, q, e.ID, e.LoggedAt, e.QuantityMl, e.Note, e.WorkoutID, now); err != nil {
		return fmt.Errorf("insert hydration entry: %w", err)
	}
	e.CreatedAt = now
	e.UpdatedAt = now
	return nil
}

// GetByID returns a single hydration entry by id.
func (r *Repo) GetByID(ctx context.Context, id uuid.UUID) (*Entry, error) {
	row := r.q.QueryRow(ctx, `SELECT `+selectCols+` FROM hydration_entries WHERE id = $1`, id)
	return scanEntry(row)
}

// PatchParams holds the optional fields editable via PATCH /hydration/{id}.
// nil pointers mean "do not update".
//
// WorkoutID tri-state: nil = no change; non-nil = update with the value.
// ClearWorkoutID = true clears the link (carries the empty-string sentinel
// from the handler).
type PatchParams struct {
	QuantityMl     *float64
	LoggedAt       *time.Time
	Note           *string
	WorkoutID      *uuid.UUID
	ClearWorkoutID bool
}

// HasUpdates reports whether at least one field is set for update.
func (p PatchParams) HasUpdates() bool {
	return p.QuantityMl != nil || p.LoggedAt != nil || p.Note != nil ||
		p.WorkoutID != nil || p.ClearWorkoutID
}

// Patch applies a partial update. Returns ErrNotFound if no row matches.
func (r *Repo) Patch(ctx context.Context, id uuid.UUID, p PatchParams) error {
	sets := []string{"updated_at = now()"}
	args := []any{id}
	next := 2
	if p.QuantityMl != nil {
		sets = append(sets, fmt.Sprintf("quantity_ml = $%d", next))
		args = append(args, *p.QuantityMl)
		next++
	}
	if p.LoggedAt != nil {
		sets = append(sets, fmt.Sprintf("logged_at = $%d", next))
		args = append(args, *p.LoggedAt)
		next++
	}
	if p.Note != nil {
		sets = append(sets, fmt.Sprintf("note = $%d", next))
		args = append(args, *p.Note)
		next++
	}
	if p.ClearWorkoutID {
		sets = append(sets, "workout_id = NULL")
	} else if p.WorkoutID != nil {
		sets = append(sets, fmt.Sprintf("workout_id = $%d", next))
		args = append(args, *p.WorkoutID)
		next++
	}
	if len(sets) == 1 {
		// No fields to update — just confirm the row exists.
		if _, err := r.GetByID(ctx, id); err != nil {
			return err
		}
		return nil
	}
	q := "UPDATE hydration_entries SET " + strings.Join(sets, ", ") + " WHERE id = $1"
	tag, err := r.q.Exec(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("patch hydration entry: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete removes a hydration entry. Returns ErrNotFound if no row matched.
func (r *Repo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.q.Exec(ctx, `DELETE FROM hydration_entries WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete hydration entry: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// List returns hydration entries with logged_at in [from, to), ordered ASC.
func (r *Repo) List(ctx context.Context, from, to time.Time) ([]*Entry, error) {
	rows, err := r.q.Query(ctx,
		`SELECT `+selectCols+` FROM hydration_entries WHERE logged_at >= $1 AND logged_at < $2 ORDER BY logged_at ASC`,
		from, to,
	)
	if err != nil {
		return nil, fmt.Errorf("list hydration entries: %w", err)
	}
	defer rows.Close()
	var out []*Entry
	for rows.Next() {
		e, err := scanEntry(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanEntry(s scanner) (*Entry, error) {
	var e Entry
	err := s.Scan(&e.ID, &e.LoggedAt, &e.QuantityMl, &e.Note, &e.WorkoutID, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan hydration entry: %w", err)
	}
	return &e, nil
}
