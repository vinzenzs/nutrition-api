package chatsessions

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/vinzenzs/nutrition-api/internal/agenttools"
)

// ErrTitleTooLong maps 1:1 to the title_invalid API error code.
var ErrTitleTooLong = errors.New("title_invalid")

// Service orchestrates chat-session CRUD. The agent loop talks to the Repo
// directly for history load/persist; this service backs the REST surface.
type Service struct {
	repo  *Repo
	specs map[string]agenttools.Spec // shared tool surface, for tier lookups (D9)
}

func NewService(repo *Repo) *Service {
	return &Service{repo: repo, specs: agenttools.ByName(agenttools.Registry())}
}

// SetToolSpecs overrides the tool surface used for tier lookups (the awaiting-
// confirmation flag and pending-confirmation previews). Production uses the
// default registry; tests inject a surface containing a write-confirm tool.
func (s *Service) SetToolSpecs(specs []agenttools.Spec) {
	s.specs = agenttools.ByName(specs)
}

// Create makes a new session. An empty/whitespace title yields an untitled
// session; a too-long title is rejected.
func (s *Service) Create(ctx context.Context, title *string) (*Session, error) {
	norm, err := normalizeTitle(title)
	if err != nil {
		return nil, err
	}
	return s.repo.CreateSession(ctx, norm)
}

// List returns session headers most-recent-first, each flagged
// awaiting_confirmation when its trailing turn is paused for a write confirm.
func (s *Service) List(ctx context.Context) ([]*Session, error) {
	sessions, err := s.repo.ListSessions(ctx)
	if err != nil {
		return nil, err
	}
	last, err := s.repo.LastTurns(ctx)
	if err != nil {
		return nil, err
	}
	for _, sess := range sessions {
		if m, ok := last[sess.ID]; ok && m.Role == "assistant" &&
			agenttools.AwaitingConfirmation(m.Content, s.specs) {
			sess.AwaitingConfirmation = true
		}
	}
	return sessions, nil
}

// Get returns the session header plus its full ordered transcript. When the
// trailing turn is paused awaiting a write confirmation, PendingConfirmation is
// populated with server-composed previews (D9). ErrNotFound if absent.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*SessionWithMessages, error) {
	sess, err := s.repo.GetSession(ctx, id)
	if err != nil {
		return nil, err
	}
	msgs, err := s.repo.GetMessages(ctx, id)
	if err != nil {
		return nil, err
	}
	out := &SessionWithMessages{Session: *sess, Messages: msgs}
	if n := len(msgs); n > 0 && msgs[n-1].Role == "assistant" {
		last := msgs[n-1]
		if calls := agenttools.PendingFromContent(last.Content, s.specs); len(calls) > 0 {
			out.AwaitingConfirmation = true
			out.PendingConfirmation = &PendingConfirmation{
				TurnID: agenttools.TurnID(last.Content),
				Calls:  calls,
			}
		}
	}
	return out, nil
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
