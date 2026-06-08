package idempotency

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/vinzenzs/nutrition-api/internal/store"
)

// Record is one stored response keyed on (client_id, method, path, key).
type Record struct {
	Status          int
	ResponseBody    []byte
	RequestBodyHash string
	CreatedAt       time.Time
}

// Key identifies a stored idempotency record.
type Key struct {
	ClientID string
	Method   string
	Path     string
	Key      string
}

// ErrNotFound is returned by Get when no record exists for the given key.
var ErrNotFound = errors.New("idempotency record not found")

// Repo persists idempotency records in Postgres.
type Repo struct {
	q store.Querier
}

func NewRepo(q store.Querier) *Repo {
	return &Repo{q: q}
}

// Get returns the stored record for k, or ErrNotFound if none exists or it
// has expired (older than ttl). Expired records are treated as not-found but
// not deleted here — that is the cleanup loop's job.
func (r *Repo) Get(ctx context.Context, k Key, ttl time.Duration) (*Record, error) {
	const q = `
        SELECT status, response_body, request_body_hash, created_at
        FROM idempotency_records
        WHERE client_id = $1 AND method = $2 AND path = $3 AND idempotency_key = $4
    `
	row := r.q.QueryRow(ctx, q, k.ClientID, k.Method, k.Path, k.Key)

	var rec Record
	if err := row.Scan(&rec.Status, &rec.ResponseBody, &rec.RequestBodyHash, &rec.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get idempotency record: %w", err)
	}
	if time.Since(rec.CreatedAt) > ttl {
		return nil, ErrNotFound
	}
	return &rec, nil
}

// Insert stores a new record. Returns an error if the key already exists.
func (r *Repo) Insert(ctx context.Context, k Key, rec Record) error {
	const q = `
        INSERT INTO idempotency_records
            (client_id, method, path, idempotency_key, status, response_body, request_body_hash, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
    `
	_, err := r.q.Exec(ctx, q, k.ClientID, k.Method, k.Path, k.Key, rec.Status, rec.ResponseBody, rec.RequestBodyHash, rec.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert idempotency record: %w", err)
	}
	return nil
}

// DeleteExpired removes all records older than ttl. Returns the number of rows removed.
func (r *Repo) DeleteExpired(ctx context.Context, ttl time.Duration) (int64, error) {
	const q = `DELETE FROM idempotency_records WHERE created_at < $1`
	tag, err := r.q.Exec(ctx, q, time.Now().Add(-ttl))
	if err != nil {
		return 0, fmt.Errorf("delete expired idempotency records: %w", err)
	}
	return tag.RowsAffected(), nil
}
