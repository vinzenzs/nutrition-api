package bodyweight

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/vinzenzs/kazper/internal/store"
)

// ErrNotFound is returned when a body-weight entry does not exist.
var ErrNotFound = errors.New("body weight entry not found")

// Repo persists body-weight entries.
type Repo struct {
	q store.Querier
}

func NewRepo(q store.Querier) *Repo {
	return &Repo{q: q}
}

const selectCols = `id, logged_at, weight_kg, body_fat_pct, muscle_mass_kg, body_water_pct, bone_mass_kg, bmi, note, created_at, updated_at`

// Insert creates a body_weight_entries row.
func (r *Repo) Insert(ctx context.Context, e *Entry) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	now := time.Now().UTC()
	const q = `
        INSERT INTO body_weight_entries
            (id, logged_at, weight_kg, body_fat_pct, muscle_mass_kg, body_water_pct, bone_mass_kg, bmi, note, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $10)
    `
	if _, err := r.q.Exec(ctx, q,
		e.ID, e.LoggedAt, e.WeightKg, e.BodyFatPct,
		e.MuscleMassKg, e.BodyWaterPct, e.BoneMassKg, e.BMI,
		e.Note, now,
	); err != nil {
		return fmt.Errorf("insert body weight entry: %w", err)
	}
	e.CreatedAt = now
	e.UpdatedAt = now
	return nil
}

// GetByID returns a single body-weight entry.
func (r *Repo) GetByID(ctx context.Context, id uuid.UUID) (*Entry, error) {
	row := r.q.QueryRow(ctx,
		`SELECT `+selectCols+` FROM body_weight_entries WHERE id = $1`, id)
	return scanEntry(row)
}

// PatchParams holds the optional editable fields on PATCH /weight/{id}.
type PatchParams struct {
	WeightKg     *float64
	BodyFatPct   *float64
	MuscleMassKg *float64
	BodyWaterPct *float64
	BoneMassKg   *float64
	BMI          *float64
	LoggedAt     *time.Time
	Note         *string
}

// HasUpdates reports whether at least one field is set.
func (p PatchParams) HasUpdates() bool {
	return p.WeightKg != nil || p.BodyFatPct != nil || p.MuscleMassKg != nil ||
		p.BodyWaterPct != nil || p.BoneMassKg != nil || p.BMI != nil ||
		p.LoggedAt != nil || p.Note != nil
}

// Patch applies a partial update.
func (r *Repo) Patch(ctx context.Context, id uuid.UUID, p PatchParams) error {
	sets := []string{"updated_at = now()"}
	args := []any{id}
	next := 2
	if p.WeightKg != nil {
		sets = append(sets, fmt.Sprintf("weight_kg = $%d", next))
		args = append(args, *p.WeightKg)
		next++
	}
	if p.BodyFatPct != nil {
		sets = append(sets, fmt.Sprintf("body_fat_pct = $%d", next))
		args = append(args, *p.BodyFatPct)
		next++
	}
	if p.MuscleMassKg != nil {
		sets = append(sets, fmt.Sprintf("muscle_mass_kg = $%d", next))
		args = append(args, *p.MuscleMassKg)
		next++
	}
	if p.BodyWaterPct != nil {
		sets = append(sets, fmt.Sprintf("body_water_pct = $%d", next))
		args = append(args, *p.BodyWaterPct)
		next++
	}
	if p.BoneMassKg != nil {
		sets = append(sets, fmt.Sprintf("bone_mass_kg = $%d", next))
		args = append(args, *p.BoneMassKg)
		next++
	}
	if p.BMI != nil {
		sets = append(sets, fmt.Sprintf("bmi = $%d", next))
		args = append(args, *p.BMI)
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
	if len(sets) == 1 {
		// Nothing to update; just confirm the row exists.
		if _, err := r.GetByID(ctx, id); err != nil {
			return err
		}
		return nil
	}
	q := "UPDATE body_weight_entries SET " + strings.Join(sets, ", ") + " WHERE id = $1"
	tag, err := r.q.Exec(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("patch body weight entry: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete removes an entry.
func (r *Repo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.q.Exec(ctx, `DELETE FROM body_weight_entries WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete body weight entry: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// List returns entries with logged_at in [from, to), ordered ASC.
func (r *Repo) List(ctx context.Context, from, to time.Time) ([]*Entry, error) {
	rows, err := r.q.Query(ctx,
		`SELECT `+selectCols+
			` FROM body_weight_entries WHERE logged_at >= $1 AND logged_at < $2 ORDER BY logged_at ASC`,
		from, to,
	)
	if err != nil {
		return nil, fmt.Errorf("list body weight entries: %w", err)
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

// ListInRange is an alias for List used by the trend computation; kept distinct
// for callers to make intent explicit.
func (r *Repo) ListInRange(ctx context.Context, from, to time.Time) ([]*Entry, error) {
	return r.List(ctx, from, to)
}

// LatestBefore returns the most-recent entry whose logged_at is strictly before
// `before`. Returns ErrNotFound when no such entry exists. Used by the energy
// availability composition resolver as the last-before-window fallback.
func (r *Repo) LatestBefore(ctx context.Context, before time.Time) (*Entry, error) {
	row := r.q.QueryRow(ctx,
		`SELECT `+selectCols+
			` FROM body_weight_entries WHERE logged_at < $1 ORDER BY logged_at DESC LIMIT 1`,
		before,
	)
	return scanEntry(row)
}

type scanner interface {
	Scan(dest ...any) error
}

func scanEntry(s scanner) (*Entry, error) {
	var e Entry
	err := s.Scan(&e.ID, &e.LoggedAt, &e.WeightKg, &e.BodyFatPct,
		&e.MuscleMassKg, &e.BodyWaterPct, &e.BoneMassKg, &e.BMI,
		&e.Note, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan body weight entry: %w", err)
	}
	return &e, nil
}
