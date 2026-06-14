package workouttemplates

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/vinzenzs/kazper/internal/store"
)

// ErrNotFound is returned when no template exists for an id.
var ErrNotFound = errors.New("workout template not found")

// ErrInUse is returned when a delete is refused because a training-plan slot
// still references the template (FK ON DELETE RESTRICT, added by
// add-training-plan).
var ErrInUse = errors.New("template_in_use")

// Repo persists workout templates.
type Repo struct {
	q store.Querier
}

func NewRepo(q store.Querier) *Repo {
	return &Repo{q: q}
}

const selectCols = `id, sport, name, description, estimated_duration_sec, steps, created_at, updated_at`

// Create inserts a template and returns the persisted row (id + timestamps).
func (r *Repo) Create(ctx context.Context, t *Template) (*Template, error) {
	stepsJSON, err := json.Marshal(t.Steps)
	if err != nil {
		return nil, fmt.Errorf("marshal steps: %w", err)
	}
	row := r.q.QueryRow(ctx, `
        INSERT INTO workout_templates (sport, name, description, estimated_duration_sec, steps, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, now(), now())
        RETURNING `+selectCols,
		t.Sport, t.Name, t.Description, t.EstimatedDurationSec, stepsJSON,
	)
	return scanTemplate(row)
}

// GetByID returns the template for an id, or ErrNotFound.
func (r *Repo) GetByID(ctx context.Context, id string) (*Template, error) {
	row := r.q.QueryRow(ctx, `SELECT `+selectCols+` FROM workout_templates WHERE id = $1`, id)
	return scanTemplate(row)
}

// List returns templates, optionally filtered by sport (empty = all), newest first.
func (r *Repo) List(ctx context.Context, sport string) ([]*Template, error) {
	var (
		rows pgx.Rows
		err  error
	)
	if sport == "" {
		rows, err = r.q.Query(ctx, `SELECT `+selectCols+` FROM workout_templates ORDER BY created_at DESC`)
	} else {
		rows, err = r.q.Query(ctx, `SELECT `+selectCols+` FROM workout_templates WHERE sport = $1 ORDER BY created_at DESC`, sport)
	}
	if err != nil {
		return nil, fmt.Errorf("list workout templates: %w", err)
	}
	defer rows.Close()
	var out []*Template
	for rows.Next() {
		t, err := scanTemplate(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// Update full-replaces the mutable columns of an existing template and returns
// the updated row. Returns ErrNotFound when the id does not exist.
func (r *Repo) Update(ctx context.Context, t *Template) (*Template, error) {
	stepsJSON, err := json.Marshal(t.Steps)
	if err != nil {
		return nil, fmt.Errorf("marshal steps: %w", err)
	}
	row := r.q.QueryRow(ctx, `
        UPDATE workout_templates
        SET sport = $2, name = $3, description = $4, estimated_duration_sec = $5, steps = $6, updated_at = now()
        WHERE id = $1
        RETURNING `+selectCols,
		t.ID, t.Sport, t.Name, t.Description, t.EstimatedDurationSec, stepsJSON,
	)
	return scanTemplate(row)
}

// Delete removes a template. Returns ErrNotFound if none existed.
func (r *Repo) Delete(ctx context.Context, id string) error {
	tag, err := r.q.Exec(ctx, `DELETE FROM workout_templates WHERE id = $1`, id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			// A plan_slot still references this template (ON DELETE RESTRICT).
			return ErrInUse
		}
		return fmt.Errorf("delete workout template: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanTemplate(s scanner) (*Template, error) {
	var (
		t        Template
		stepsRaw []byte
	)
	err := s.Scan(&t.ID, &t.Sport, &t.Name, &t.Description, &t.EstimatedDurationSec, &stepsRaw, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan workout template: %w", err)
	}
	if len(stepsRaw) > 0 {
		if err := json.Unmarshal(stepsRaw, &t.Steps); err != nil {
			return nil, fmt.Errorf("scan workout template steps: %w", err)
		}
	}
	return &t, nil
}
