package gear

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/vinzenzs/nutrition-api/internal/store"
)

// ErrNotFound is returned when no gear row exists for an id.
var ErrNotFound = errors.New("gear not found")

// Repo persists gear rows.
type Repo struct {
	q store.Querier
}

func NewRepo(q store.Querier) *Repo {
	return &Repo{q: q}
}

// selectCols renders the two DATE columns as text so they round-trip as
// YYYY-MM-DD.
const selectCols = `id, external_id, gear_type, display_name, total_distance_m, total_activities, retired, to_char(date_begin, 'YYYY-MM-DD') AS date_begin, to_char(date_end, 'YYYY-MM-DD') AS date_end, created_at, updated_at`

// Upsert inserts a gear row, or full-replaces its fields when a row with the
// same external_id already exists. Returns created=true on INSERT (HTTP 201),
// false on UPDATE (HTTP 200). The backend id is generated on insert and
// preserved on update; the returned id is reflected onto g.
func (r *Repo) Upsert(ctx context.Context, g *Gear) (created bool, err error) {
	const q = `
        INSERT INTO gear (
            id, external_id, gear_type, display_name,
            total_distance_m, total_activities, retired, date_begin, date_end,
            created_at, updated_at
        ) VALUES (
            gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7::date, $8::date, now(), now()
        )
        ON CONFLICT (external_id) DO UPDATE SET
            gear_type        = EXCLUDED.gear_type,
            display_name     = EXCLUDED.display_name,
            total_distance_m = EXCLUDED.total_distance_m,
            total_activities = EXCLUDED.total_activities,
            retired          = EXCLUDED.retired,
            date_begin       = EXCLUDED.date_begin,
            date_end         = EXCLUDED.date_end,
            updated_at       = now()
        RETURNING id, (xmax = 0) AS inserted
    `
	row := r.q.QueryRow(ctx, q,
		g.ExternalID, string(g.GearType), g.DisplayName,
		g.TotalDistanceM, g.TotalActivities, g.Retired, g.DateBegin, g.DateEnd,
	)
	if err := row.Scan(&g.ID, &created); err != nil {
		return false, fmt.Errorf("upsert gear: %w", err)
	}
	return created, nil
}

// GetByID returns the gear row with the given backend id, or ErrNotFound.
func (r *Repo) GetByID(ctx context.Context, id uuid.UUID) (*Gear, error) {
	row := r.q.QueryRow(ctx, `SELECT `+selectCols+` FROM gear WHERE id = $1`, id)
	return scanGear(row)
}

// List returns gear rows ordered by display_name ascending. When retired is
// non-nil it narrows to rows with that retirement state.
func (r *Repo) List(ctx context.Context, retired *bool) ([]*Gear, error) {
	q := `SELECT ` + selectCols + ` FROM gear`
	args := []any{}
	if retired != nil {
		q += ` WHERE retired = $1`
		args = append(args, *retired)
	}
	q += ` ORDER BY display_name ASC`
	rows, err := r.q.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list gear: %w", err)
	}
	defer rows.Close()
	var out []*Gear
	for rows.Next() {
		g, err := scanGear(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanGear(s scanner) (*Gear, error) {
	var (
		g       Gear
		typeStr string
	)
	err := s.Scan(
		&g.ID, &g.ExternalID, &typeStr, &g.DisplayName,
		&g.TotalDistanceM, &g.TotalActivities, &g.Retired, &g.DateBegin, &g.DateEnd,
		&g.CreatedAt, &g.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan gear: %w", err)
	}
	g.GearType = Type(typeStr)
	return &g, nil
}
