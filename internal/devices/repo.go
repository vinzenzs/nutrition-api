package devices

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/vinzenzs/kazper/internal/store"
)

// ErrNotFound is returned when no device row exists for an id.
var ErrNotFound = errors.New("device not found")

// Repo persists device rows.
type Repo struct {
	q store.Querier
}

func NewRepo(q store.Querier) *Repo {
	return &Repo{q: q}
}

const selectCols = `id, external_id, display_name, model, last_sync_at, battery_pct, firmware_version, created_at, updated_at`

// Upsert inserts a device row, or full-replaces its fields when a row with the
// same external_id exists. Returns created=true on INSERT (HTTP 201).
func (r *Repo) Upsert(ctx context.Context, d *Device) (created bool, err error) {
	const q = `
        INSERT INTO devices (
            id, external_id, display_name, model, last_sync_at, battery_pct, firmware_version,
            created_at, updated_at
        ) VALUES (
            gen_random_uuid(), $1, $2, $3, $4, $5, $6, now(), now()
        )
        ON CONFLICT (external_id) DO UPDATE SET
            display_name     = EXCLUDED.display_name,
            model            = EXCLUDED.model,
            last_sync_at     = EXCLUDED.last_sync_at,
            battery_pct      = EXCLUDED.battery_pct,
            firmware_version = EXCLUDED.firmware_version,
            updated_at       = now()
        RETURNING id, (xmax = 0) AS inserted
    `
	row := r.q.QueryRow(ctx, q,
		d.ExternalID, d.DisplayName, d.Model, d.LastSyncAt, d.BatteryPct, d.FirmwareVersion,
	)
	if err := row.Scan(&d.ID, &created); err != nil {
		return false, fmt.Errorf("upsert device: %w", err)
	}
	return created, nil
}

// GetByID returns the device row with the given backend id, or ErrNotFound.
func (r *Repo) GetByID(ctx context.Context, id uuid.UUID) (*Device, error) {
	row := r.q.QueryRow(ctx, `SELECT `+selectCols+` FROM devices WHERE id = $1`, id)
	return scanDevice(row)
}

// List returns device rows ordered by display_name ascending.
func (r *Repo) List(ctx context.Context) ([]*Device, error) {
	rows, err := r.q.Query(ctx, `SELECT `+selectCols+` FROM devices ORDER BY display_name ASC`)
	if err != nil {
		return nil, fmt.Errorf("list devices: %w", err)
	}
	defer rows.Close()
	var out []*Device
	for rows.Next() {
		d, err := scanDevice(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanDevice(s scanner) (*Device, error) {
	var d Device
	err := s.Scan(
		&d.ID, &d.ExternalID, &d.DisplayName, &d.Model, &d.LastSyncAt, &d.BatteryPct,
		&d.FirmwareVersion, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan device: %w", err)
	}
	return &d, nil
}
