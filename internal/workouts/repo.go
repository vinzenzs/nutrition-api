package workouts

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

// ErrNotFound is returned when a workout does not exist.
var ErrNotFound = errors.New("workout not found")

// Repo persists workouts.
type Repo struct {
	q store.Querier
}

func NewRepo(q store.Querier) *Repo {
	return &Repo{q: q}
}

const selectCols = `id, external_id, source, sport, name, started_at, ended_at, kcal_burned, avg_hr, tss, notes, created_at, updated_at`

// Upsert inserts a new workout row, or updates an existing row when
// external_id collides with the partial unique index. Returns `created=true`
// when the row was inserted (the caller maps this to HTTP 201) and `false`
// when an existing row was updated (HTTP 200).
//
// The partial unique index `workouts_external_id_uidx ON (external_id) WHERE
// external_id IS NOT NULL` is critical: rows with NULL external_id never
// conflict, so manual workouts always INSERT.
func (r *Repo) Upsert(ctx context.Context, w *Workout) (created bool, err error) {
	if w.ID == uuid.Nil {
		w.ID = uuid.New()
	}
	now := time.Now().UTC()
	if w.CreatedAt.IsZero() {
		w.CreatedAt = now
	}
	w.UpdatedAt = now

	const q = `
        INSERT INTO workouts (
            id, external_id, source, sport, name,
            started_at, ended_at,
            kcal_burned, avg_hr, tss, notes,
            created_at, updated_at
        ) VALUES (
            $1, $2, $3, $4, $5,
            $6, $7,
            $8, $9, $10, $11,
            $12, $13
        )
        ON CONFLICT (external_id) WHERE external_id IS NOT NULL DO UPDATE SET
            source       = EXCLUDED.source,
            sport        = EXCLUDED.sport,
            name         = EXCLUDED.name,
            started_at   = EXCLUDED.started_at,
            ended_at     = EXCLUDED.ended_at,
            kcal_burned  = EXCLUDED.kcal_burned,
            avg_hr       = EXCLUDED.avg_hr,
            tss          = EXCLUDED.tss,
            notes        = EXCLUDED.notes,
            updated_at   = EXCLUDED.updated_at
        RETURNING id, created_at = $12 AS inserted
    `
	row := r.q.QueryRow(ctx, q,
		w.ID, w.ExternalID, string(w.Source), string(w.Sport), w.Name,
		w.StartedAt, w.EndedAt,
		w.KcalBurned, w.AvgHR, w.TSS, w.Notes,
		w.CreatedAt, w.UpdatedAt,
	)
	var (
		returnedID uuid.UUID
		inserted   bool
	)
	if err := row.Scan(&returnedID, &inserted); err != nil {
		return false, fmt.Errorf("upsert workout: %w", err)
	}
	// On UPDATE the existing row's id wins; reflect it back onto the caller's
	// pointer so the response carries the persisted id.
	w.ID = returnedID
	if !inserted {
		// Re-read so the caller sees authoritative timestamps + any fields the
		// caller didn't supply but the existing row had.
		fresh, err := r.GetByID(ctx, returnedID)
		if err != nil {
			return false, fmt.Errorf("re-read after update: %w", err)
		}
		*w = *fresh
	}
	return inserted, nil
}

// GetByID returns a single workout row.
func (r *Repo) GetByID(ctx context.Context, id uuid.UUID) (*Workout, error) {
	row := r.q.QueryRow(ctx, `SELECT `+selectCols+` FROM workouts WHERE id = $1`, id)
	return scanWorkout(row)
}

// PatchParams carries the optional mutable fields. nil pointers mean "leave
// the existing value unchanged".
type PatchParams struct {
	Name       *string
	Notes      *string
	KcalBurned *float64
	AvgHR      *int
	TSS        *float64
}

// HasUpdates reports whether at least one mutable field is being changed.
func (p PatchParams) HasUpdates() bool {
	return p.Name != nil || p.Notes != nil || p.KcalBurned != nil || p.AvgHR != nil || p.TSS != nil
}

// Patch applies a partial update over the mutable subset. Returns ErrNotFound
// if no row matches.
func (r *Repo) Patch(ctx context.Context, id uuid.UUID, p PatchParams) error {
	sets := []string{"updated_at = now()"}
	args := []any{id}
	next := 2
	if p.Name != nil {
		sets = append(sets, fmt.Sprintf("name = $%d", next))
		args = append(args, *p.Name)
		next++
	}
	if p.Notes != nil {
		sets = append(sets, fmt.Sprintf("notes = $%d", next))
		args = append(args, *p.Notes)
		next++
	}
	if p.KcalBurned != nil {
		sets = append(sets, fmt.Sprintf("kcal_burned = $%d", next))
		args = append(args, *p.KcalBurned)
		next++
	}
	if p.AvgHR != nil {
		sets = append(sets, fmt.Sprintf("avg_hr = $%d", next))
		args = append(args, *p.AvgHR)
		next++
	}
	if p.TSS != nil {
		sets = append(sets, fmt.Sprintf("tss = $%d", next))
		args = append(args, *p.TSS)
		next++
	}
	if len(sets) == 1 {
		// Nothing to update — just confirm the row exists so the caller can
		// distinguish "noop on existing" from "404 on missing".
		if _, err := r.GetByID(ctx, id); err != nil {
			return err
		}
		return nil
	}
	q := "UPDATE workouts SET " + strings.Join(sets, ", ") + " WHERE id = $1"
	tag, err := r.q.Exec(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("patch workout: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete removes a workout. Returns ErrNotFound if no row matched.
func (r *Repo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.q.Exec(ctx, `DELETE FROM workouts WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete workout: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// List returns workouts whose started_at falls within [from, to] inclusive,
// ordered by started_at ascending.
func (r *Repo) List(ctx context.Context, from, to time.Time) ([]*Workout, error) {
	const q = `SELECT ` + selectCols + ` FROM workouts WHERE started_at >= $1 AND started_at <= $2 ORDER BY started_at ASC`
	rows, err := r.q.Query(ctx, q, from, to)
	if err != nil {
		return nil, fmt.Errorf("list workouts: %w", err)
	}
	defer rows.Close()
	var out []*Workout
	for rows.Next() {
		w, err := scanWorkout(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanWorkout(s scanner) (*Workout, error) {
	var (
		w           Workout
		sourceStr   string
		sportStr    string
	)
	err := s.Scan(
		&w.ID, &w.ExternalID, &sourceStr, &sportStr, &w.Name,
		&w.StartedAt, &w.EndedAt,
		&w.KcalBurned, &w.AvgHR, &w.TSS, &w.Notes,
		&w.CreatedAt, &w.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan workout: %w", err)
	}
	w.Source = Source(sourceStr)
	w.Sport = Sport(sportStr)
	return &w, nil
}
