package workoutfuel

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

// ErrNotFound is returned when a workout-fuel entry does not exist.
var ErrNotFound = errors.New("workout-fuel entry not found")

// Repo persists workout-fuel entries.
type Repo struct {
	q store.Querier
}

func NewRepo(q store.Querier) *Repo {
	return &Repo{q: q}
}

const selectCols = `id, logged_at, name, quantity_ml, carbs_g, sodium_mg, potassium_mg, caffeine_mg, note, workout_id, created_at, updated_at`

// Insert creates a workout_fuel_entries row. The supplied Entry's
// ID/CreatedAt/UpdatedAt are set on success.
func (r *Repo) Insert(ctx context.Context, e *Entry) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	now := time.Now().UTC()
	const q = `
        INSERT INTO workout_fuel_entries
            (id, logged_at, name, quantity_ml, carbs_g, sodium_mg, potassium_mg, caffeine_mg, note, workout_id, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $11)
    `
	if _, err := r.q.Exec(ctx, q,
		e.ID, e.LoggedAt, e.Name,
		e.QuantityMl, e.CarbsG, e.SodiumMg, e.PotassiumMg, e.CaffeineMg,
		e.Note, e.WorkoutID, now,
	); err != nil {
		return fmt.Errorf("insert workout-fuel entry: %w", err)
	}
	e.CreatedAt = now
	e.UpdatedAt = now
	return nil
}

// GetByID returns a single workout-fuel entry by id.
func (r *Repo) GetByID(ctx context.Context, id uuid.UUID) (*Entry, error) {
	row := r.q.QueryRow(ctx, `SELECT `+selectCols+` FROM workout_fuel_entries WHERE id = $1`, id)
	return scanEntry(row)
}

// PatchParams holds the editable fields on PATCH /workout-fuel/{id}.
//
// All quantitative fields use *float64 with a companion Clear* flag to model
// the tri-state: nil = no change; non-nil = set to that value; ClearX = set
// to NULL. The handler converts JSON `null` into Clear=true (so the agent
// can explicitly null-out a previously-set field).
//
// WorkoutID follows the meals/hydration empty-string-clear pattern: nil = no
// change; non-nil = set; ClearWorkoutID = true clears.
type PatchParams struct {
	Name             *string
	LoggedAt         *time.Time
	QuantityMl       *float64
	ClearQuantityMl  bool
	CarbsG           *float64
	ClearCarbsG      bool
	SodiumMg         *float64
	ClearSodiumMg    bool
	PotassiumMg      *float64
	ClearPotassiumMg bool
	CaffeineMg       *float64
	ClearCaffeineMg  bool
	Note             *string
	ClearNote        bool
	WorkoutID        *uuid.UUID
	ClearWorkoutID   bool
}

// HasUpdates reports whether at least one field is set for update.
func (p PatchParams) HasUpdates() bool {
	return p.Name != nil || p.LoggedAt != nil ||
		p.QuantityMl != nil || p.ClearQuantityMl ||
		p.CarbsG != nil || p.ClearCarbsG ||
		p.SodiumMg != nil || p.ClearSodiumMg ||
		p.PotassiumMg != nil || p.ClearPotassiumMg ||
		p.CaffeineMg != nil || p.ClearCaffeineMg ||
		p.Note != nil || p.ClearNote ||
		p.WorkoutID != nil || p.ClearWorkoutID
}

// Patch applies a partial update. Returns ErrNotFound if no row matches.
func (r *Repo) Patch(ctx context.Context, id uuid.UUID, p PatchParams) error {
	sets := []string{"updated_at = now()"}
	args := []any{id}
	next := 2
	add := func(col string, val any) {
		sets = append(sets, fmt.Sprintf("%s = $%d", col, next))
		args = append(args, val)
		next++
	}

	if p.Name != nil {
		add("name", *p.Name)
	}
	if p.LoggedAt != nil {
		add("logged_at", *p.LoggedAt)
	}
	switch {
	case p.ClearQuantityMl:
		sets = append(sets, "quantity_ml = NULL")
	case p.QuantityMl != nil:
		add("quantity_ml", *p.QuantityMl)
	}
	switch {
	case p.ClearCarbsG:
		sets = append(sets, "carbs_g = NULL")
	case p.CarbsG != nil:
		add("carbs_g", *p.CarbsG)
	}
	switch {
	case p.ClearSodiumMg:
		sets = append(sets, "sodium_mg = NULL")
	case p.SodiumMg != nil:
		add("sodium_mg", *p.SodiumMg)
	}
	switch {
	case p.ClearPotassiumMg:
		sets = append(sets, "potassium_mg = NULL")
	case p.PotassiumMg != nil:
		add("potassium_mg", *p.PotassiumMg)
	}
	switch {
	case p.ClearCaffeineMg:
		sets = append(sets, "caffeine_mg = NULL")
	case p.CaffeineMg != nil:
		add("caffeine_mg", *p.CaffeineMg)
	}
	switch {
	case p.ClearNote:
		sets = append(sets, "note = NULL")
	case p.Note != nil:
		add("note", *p.Note)
	}
	switch {
	case p.ClearWorkoutID:
		sets = append(sets, "workout_id = NULL")
	case p.WorkoutID != nil:
		add("workout_id", *p.WorkoutID)
	}

	if len(sets) == 1 {
		// No fields to update — just confirm the row exists.
		if _, err := r.GetByID(ctx, id); err != nil {
			return err
		}
		return nil
	}
	q := "UPDATE workout_fuel_entries SET " + strings.Join(sets, ", ") + " WHERE id = $1"
	tag, err := r.q.Exec(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("patch workout-fuel entry: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete removes a workout-fuel entry. Returns ErrNotFound if no row matched.
func (r *Repo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.q.Exec(ctx, `DELETE FROM workout_fuel_entries WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete workout-fuel entry: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// List returns workout-fuel entries with logged_at in [from, to), ordered ASC.
func (r *Repo) List(ctx context.Context, from, to time.Time) ([]*Entry, error) {
	rows, err := r.q.Query(ctx,
		`SELECT `+selectCols+` FROM workout_fuel_entries WHERE logged_at >= $1 AND logged_at < $2 ORDER BY logged_at ASC`,
		from, to,
	)
	if err != nil {
		return nil, fmt.Errorf("list workout-fuel entries: %w", err)
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
	err := s.Scan(
		&e.ID, &e.LoggedAt, &e.Name,
		&e.QuantityMl, &e.CarbsG, &e.SodiumMg, &e.PotassiumMg, &e.CaffeineMg,
		&e.Note, &e.WorkoutID, &e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan workout-fuel entry: %w", err)
	}
	return &e, nil
}
