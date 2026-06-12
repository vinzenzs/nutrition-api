package chatsessions

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
)

// ErrTitleTooLong maps 1:1 to the title_invalid API error code.
var ErrTitleTooLong = errors.New("title_invalid")

// Service orchestrates chat-session CRUD. The agent loop talks to the Repo
// directly for history load/persist; this service backs the REST surface.
type Service struct {
	repo *Repo
}

func NewService(repo *Repo) *Service { return &Service{repo: repo} }

// Create makes a new session. An empty/whitespace title yields an untitled
// session; a too-long title is rejected.
func (s *Service) Create(ctx context.Context, title *string) (*Session, error) {
	norm, err := normalizeTitle(title)
	if err != nil {
		return nil, err
	}
	return s.repo.CreateSession(ctx, norm)
}

// List returns session headers most-recent-first.
func (s *Service) List(ctx context.Context) ([]*Session, error) {
	return s.repo.ListSessions(ctx)
}

// Get returns the session header plus its full ordered transcript. ErrNotFound
// if the session does not exist.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*SessionWithMessages, error) {
	sess, err := s.repo.GetSession(ctx, id)
	if err != nil {
		return nil, err
	}
	msgs, err := s.repo.GetMessages(ctx, id)
	if err != nil {
		return nil, err
	}
	return &SessionWithMessages{Session: *sess, Messages: msgs}, nil
}

// Rename sets or clears the title. A nil or empty/whitespace title clears it
// (untitled); a too-long title is rejected. ErrNotFound if absent.
func (s *Service) Rename(ctx context.Context, id uuid.UUID, title *string) error {
	norm, err := normalizeTitle(title)
	if err != nil {
		return err
	}
	return s.repo.Rename(ctx, id, norm)
}

// Delete removes a session and its turns. ErrNotFound if absent.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// normalizeTitle trims a title; empty/whitespace becomes nil (untitled), and an
// over-length title is rejected.
func normalizeTitle(title *string) (*string, error) {
	if title == nil {
		return nil, nil
	}
	t := strings.TrimSpace(*title)
	if t == "" {
		return nil, nil
	}
	if len(t) > maxTitleLen {
		return nil, ErrTitleTooLong
	}
	return &t, nil
}
