package garminauth

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/vinzenzs/nutrition-api/internal/store"
)

// ErrNotFound is returned when no garmin token blob has been stored.
var ErrNotFound = errors.New("garmin token not found")

// Repo persists the single garmin token row against store.Querier.
type Repo struct {
	q store.Querier
}

func NewRepo(q store.Querier) *Repo {
	return &Repo{q: q}
}

// Upsert writes the single row (id = 1), replacing any prior value and
// stamping updated_at.
func (r *Repo) Upsert(ctx context.Context, rec record) error {
	const q = `
        INSERT INTO garmin_tokens (id, ciphertext, nonce, updated_at)
        VALUES (1, $1, $2, now())
        ON CONFLICT (id) DO UPDATE
            SET ciphertext = EXCLUDED.ciphertext,
                nonce = EXCLUDED.nonce,
                updated_at = EXCLUDED.updated_at
    `
	if _, err := r.q.Exec(ctx, q, rec.Ciphertext, rec.Nonce); err != nil {
		return fmt.Errorf("upsert garmin token: %w", err)
	}
	return nil
}

// Get returns the stored row, or ErrNotFound when none exists.
func (r *Repo) Get(ctx context.Context) (record, error) {
	var rec record
	err := r.q.QueryRow(ctx,
		`SELECT ciphertext, nonce, updated_at FROM garmin_tokens WHERE id = 1`,
	).Scan(&rec.Ciphertext, &rec.Nonce, &rec.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return record{}, ErrNotFound
		}
		return record{}, fmt.Errorf("get garmin token: %w", err)
	}
	return rec, nil
}

// Delete removes the stored row. Returns ErrNotFound when nothing was stored.
func (r *Repo) Delete(ctx context.Context) error {
	tag, err := r.q.Exec(ctx, `DELETE FROM garmin_tokens WHERE id = 1`)
	if err != nil {
		return fmt.Errorf("delete garmin token: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
