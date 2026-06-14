package chatsessions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/vinzenzs/nutrition-api/internal/store"
)

// ErrNotFound is returned when a session does not exist.
var ErrNotFound = errors.New("chat session not found")

// Repo persists chat sessions and their turns against a store.Querier (pool or tx).
type Repo struct {
	q store.Querier
}

func NewRepo(q store.Querier) *Repo { return &Repo{q: q} }

const sessionCols = `id, title, created_at, updated_at, last_message_at`

// CreateSession inserts a new (optionally titled) session.
func (r *Repo) CreateSession(ctx context.Context, title *string) (*Session, error) {
	id := uuid.New()
	row := r.q.QueryRow(ctx,
		`INSERT INTO chat_sessions (id, title) VALUES ($1, $2) RETURNING `+sessionCols,
		id, title)
	return scanSession(row)
}

// ListSessions returns session headers (no turns), most-recent activity first.
func (r *Repo) ListSessions(ctx context.Context) ([]*Session, error) {
	rows, err := r.q.Query(ctx,
		`SELECT `+sessionCols+` FROM chat_sessions ORDER BY last_message_at DESC, created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list chat sessions: %w", err)
	}
	defer rows.Close()
	out := []*Session{}
	for rows.Next() {
		s, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// LastTurns returns the most recent turn of every session, keyed by session id,
// in a single query. Used to derive the awaiting-confirmation flag for the
// session list without an N+1 fan-out (D9).
func (r *Repo) LastTurns(ctx context.Context) (map[uuid.UUID]Message, error) {
	rows, err := r.q.Query(ctx,
		`SELECT DISTINCT ON (session_id) session_id, role, content
		 FROM chat_messages ORDER BY session_id, seq DESC`)
	if err != nil {
		return nil, fmt.Errorf("last chat turns: %w", err)
	}
	defer rows.Close()
	out := map[uuid.UUID]Message{}
	for rows.Next() {
		var id uuid.UUID
		var m Message
		var raw []byte
		if err := rows.Scan(&id, &m.Role, &raw); err != nil {
			return nil, fmt.Errorf("scan last chat turn: %w", err)
		}
		m.Content = json.RawMessage(raw)
		out[id] = m
	}
	return out, rows.Err()
}

// GetSession returns one session header. ErrNotFound if absent.
func (r *Repo) GetSession(ctx context.Context, id uuid.UUID) (*Session, error) {
	return scanSession(r.q.QueryRow(ctx, `SELECT `+sessionCols+` FROM chat_sessions WHERE id = $1`, id))
}

// SessionExists reports whether a session with id exists.
func (r *Repo) SessionExists(ctx context.Context, id uuid.UUID) (bool, error) {
	var exists bool
	err := r.q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM chat_sessions WHERE id = $1)`, id).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("chat session exists: %w", err)
	}
	return exists, nil
}

// GetMessages returns all turns for a session in chronological order.
func (r *Repo) GetMessages(ctx context.Context, id uuid.UUID) ([]Message, error) {
	rows, err := r.q.Query(ctx,
		`SELECT role, content FROM chat_messages WHERE session_id = $1 ORDER BY seq ASC`, id)
	if err != nil {
		return nil, fmt.Errorf("get chat messages: %w", err)
	}
	return scanMessages(rows)
}

// LoadTurns returns the most recent `limit` turns for a session in chronological
// order (limit <= 0 means all).
func (r *Repo) LoadTurns(ctx context.Context, id uuid.UUID, limit int) ([]Message, error) {
	if limit <= 0 {
		return r.GetMessages(ctx, id)
	}
	rows, err := r.q.Query(ctx,
		`SELECT role, content FROM (
			SELECT role, content, seq FROM chat_messages
			WHERE session_id = $1 ORDER BY seq DESC LIMIT $2
		) t ORDER BY seq ASC`, id, limit)
	if err != nil {
		return nil, fmt.Errorf("load chat turns: %w", err)
	}
	return scanMessages(rows)
}

// AppendMessages inserts the given turns (atomically, in order) and bumps the
// session's last_message_at / updated_at. A no-op on an empty slice.
func (r *Repo) AppendMessages(ctx context.Context, id uuid.UUID, msgs []Message) error {
	if len(msgs) == 0 {
		return nil
	}
	var sb strings.Builder
	args := []any{id}
	sb.WriteString(`INSERT INTO chat_messages (id, session_id, role, content) VALUES `)
	for i, m := range msgs {
		if i > 0 {
			sb.WriteString(", ")
		}
		base := len(args)
		fmt.Fprintf(&sb, "($%d, $1, $%d, $%d::jsonb)", base+1, base+2, base+3)
		args = append(args, uuid.New(), m.Role, string(m.Content))
	}
	if _, err := r.q.Exec(ctx, sb.String(), args...); err != nil {
		return fmt.Errorf("append chat messages: %w", err)
	}
	if _, err := r.q.Exec(ctx,
		`UPDATE chat_sessions SET last_message_at = now(), updated_at = now() WHERE id = $1`, id); err != nil {
		return fmt.Errorf("bump chat session: %w", err)
	}
	return nil
}

// SetTitleIfEmpty sets the title only when the session is currently untitled.
func (r *Repo) SetTitleIfEmpty(ctx context.Context, id uuid.UUID, title string) error {
	if title == "" {
		return nil
	}
	if _, err := r.q.Exec(ctx,
		`UPDATE chat_sessions SET title = $2, updated_at = now() WHERE id = $1 AND title IS NULL`,
		id, title); err != nil {
		return fmt.Errorf("set chat session title: %w", err)
	}
	return nil
}

// Rename sets (title non-nil) or clears (title nil) the session title.
// ErrNotFound if no row matches.
func (r *Repo) Rename(ctx context.Context, id uuid.UUID, title *string) error {
	tag, err := r.q.Exec(ctx,
		`UPDATE chat_sessions SET title = $2, updated_at = now() WHERE id = $1`, id, title)
	if err != nil {
		return fmt.Errorf("rename chat session: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete removes a session and (by cascade) its turns. ErrNotFound if absent.
func (r *Repo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.q.Exec(ctx, `DELETE FROM chat_sessions WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete chat session: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanSession(s interface{ Scan(...any) error }) (*Session, error) {
	var sess Session
	if err := s.Scan(&sess.ID, &sess.Title, &sess.CreatedAt, &sess.UpdatedAt, &sess.LastMessageAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan chat session: %w", err)
	}
	return &sess, nil
}

func scanMessages(rows pgx.Rows) ([]Message, error) {
	defer rows.Close()
	out := []Message{}
	for rows.Next() {
		var m Message
		var raw []byte
		if err := rows.Scan(&m.Role, &raw); err != nil {
			return nil, fmt.Errorf("scan chat message: %w", err)
		}
		m.Content = json.RawMessage(raw)
		out = append(out, m)
	}
	return out, rows.Err()
}
