package trainingphases

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

// ErrPhaseNotFound is returned when no phase row matches a lookup.
var ErrPhaseNotFound = errors.New("phase not found")

// PhasesRepo persists training_phases rows.
type PhasesRepo struct {
	q store.Querier
}

func NewPhasesRepo(q store.Querier) *PhasesRepo {
	return &PhasesRepo{q: q}
}

// Column projection that LEFT JOINs goal_templates so we can populate
// DefaultTemplateName in one round trip. The trailing `name` is from the
// templates table.
const phasesSelectCols = `
    p.id, p.name, p.type, p.start_date, p.end_date,
    p.default_template_id, p.notes,
    p.created_at, p.updated_at,
    t.name
`

const phasesFromJoin = `
    FROM training_phases p
    LEFT JOIN goal_templates t ON t.id = p.default_template_id
`

// Insert creates a phase row. Caller is responsible for validation; the
// repo does no semantic checks beyond what the DB CHECK constraints enforce.
func (r *PhasesRepo) Insert(ctx context.Context, p *Phase) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	now := time.Now().UTC()
	const q = `
        INSERT INTO training_phases
            (id, name, type, start_date, end_date, default_template_id, notes, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8)
    `
	if _, err := r.q.Exec(ctx, q,
		p.ID, p.Name, string(p.Type), p.StartDate, p.EndDate,
		p.DefaultTemplateID, p.Notes, now,
	); err != nil {
		return fmt.Errorf("insert phase: %w", err)
	}
	p.CreatedAt = now
	p.UpdatedAt = now
	return nil
}

// GetByID returns the phase plus its resolved template name (if any) in
// one round trip.
func (r *PhasesRepo) GetByID(ctx context.Context, id uuid.UUID) (*Phase, error) {
	row := r.q.QueryRow(ctx,
		`SELECT `+phasesSelectCols+phasesFromJoin+` WHERE p.id = $1`, id)
	p, err := scanPhase(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPhaseNotFound
		}
		return nil, fmt.Errorf("scan phase: %w", err)
	}
	return p, nil
}

// ListIntersecting returns every phase whose [start_date, end_date]
// intersects [from, to], ordered by start_date ASC then updated_at DESC for
// stable tie-breaking.
func (r *PhasesRepo) ListIntersecting(ctx context.Context, from, to time.Time) ([]*Phase, error) {
	rows, err := r.q.Query(ctx,
		`SELECT `+phasesSelectCols+phasesFromJoin+
			` WHERE p.start_date <= $2 AND p.end_date >= $1`+
			` ORDER BY p.start_date ASC, p.updated_at DESC`,
		from, to)
	if err != nil {
		return nil, fmt.Errorf("list phases intersecting: %w", err)
	}
	defer rows.Close()
	var out []*Phase
	for rows.Next() {
		p, err := scanPhase(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// PhaseFor returns the most-recently-updated phase covering `date`, or
// ErrPhaseNotFound if none. Used by the goals resolver for single-day
// adherence resolution.
func (r *PhasesRepo) PhaseFor(ctx context.Context, date time.Time) (*Phase, error) {
	row := r.q.QueryRow(ctx,
		`SELECT `+phasesSelectCols+phasesFromJoin+
			` WHERE p.start_date <= $1 AND p.end_date >= $1`+
			` ORDER BY p.updated_at DESC LIMIT 1`,
		date)
	p, err := scanPhase(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPhaseNotFound
		}
		return nil, fmt.Errorf("scan phase for date: %w", err)
	}
	return p, nil
}

// PatchParams carries the optional editable fields. A nil pointer leaves
// the field unchanged. Clearing default_template_id uses a sentinel pattern:
// the handler converts a JSON `null` on default_template_id into
// ClearDefaultTemplateID=true; a non-nil DefaultTemplateID sets a new value;
// nil with Clear=false means leave unchanged.
type PatchParams struct {
	Name                     *string
	Type                     *PhaseType
	StartDate                *time.Time
	EndDate                  *time.Time
	DefaultTemplateID        *uuid.UUID
	ClearDefaultTemplateID   bool
	Notes                    *string
}

// HasUpdates reports whether at least one field is set.
func (p PatchParams) HasUpdates() bool {
	return p.Name != nil || p.Type != nil || p.StartDate != nil || p.EndDate != nil ||
		p.DefaultTemplateID != nil || p.ClearDefaultTemplateID || p.Notes != nil
}

// Patch applies a partial update to a phase.
func (r *PhasesRepo) Patch(ctx context.Context, id uuid.UUID, p PatchParams) error {
	sets := []string{"updated_at = now()"}
	args := []any{id}
	next := 2
	if p.Name != nil {
		sets = append(sets, fmt.Sprintf("name = $%d", next))
		args = append(args, *p.Name)
		next++
	}
	if p.Type != nil {
		sets = append(sets, fmt.Sprintf("type = $%d", next))
		args = append(args, string(*p.Type))
		next++
	}
	if p.StartDate != nil {
		sets = append(sets, fmt.Sprintf("start_date = $%d", next))
		args = append(args, *p.StartDate)
		next++
	}
	if p.EndDate != nil {
		sets = append(sets, fmt.Sprintf("end_date = $%d", next))
		args = append(args, *p.EndDate)
		next++
	}
	if p.ClearDefaultTemplateID {
		sets = append(sets, "default_template_id = NULL")
	} else if p.DefaultTemplateID != nil {
		sets = append(sets, fmt.Sprintf("default_template_id = $%d", next))
		args = append(args, *p.DefaultTemplateID)
		next++
	}
	if p.Notes != nil {
		sets = append(sets, fmt.Sprintf("notes = $%d", next))
		args = append(args, *p.Notes)
		next++
	}
	if len(sets) == 1 {
		// No fields to update; confirm the row exists.
		if _, err := r.GetByID(ctx, id); err != nil {
			return err
		}
		return nil
	}
	q := "UPDATE training_phases SET " + strings.Join(sets, ", ") + " WHERE id = $1"
	tag, err := r.q.Exec(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("patch phase: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrPhaseNotFound
	}
	return nil
}

// Delete removes a phase. Returns ErrPhaseNotFound if no row matched.
func (r *PhasesRepo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.q.Exec(ctx, `DELETE FROM training_phases WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete phase: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrPhaseNotFound
	}
	return nil
}

// scanPhase scans one row from the phasesSelectCols+phasesFromJoin
// projection (training_phases columns + the joined template name as the
// last column, NULL when no template is linked).
func scanPhase(s scanner) (*Phase, error) {
	var (
		p            Phase
		typeStr      string
		templateName *string
	)
	if err := s.Scan(
		&p.ID, &p.Name, &typeStr, &p.StartDate, &p.EndDate,
		&p.DefaultTemplateID, &p.Notes,
		&p.CreatedAt, &p.UpdatedAt,
		&templateName,
	); err != nil {
		return nil, err
	}
	p.Type = PhaseType(typeStr)
	p.DefaultTemplateName = templateName
	return &p, nil
}
