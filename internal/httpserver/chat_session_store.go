package httpserver

import (
	"context"

	"github.com/google/uuid"

	"github.com/vinzenzs/kazper/internal/chat"
	"github.com/vinzenzs/kazper/internal/chatsessions"
)

// chatSessionStore adapts *chatsessions.Repo to the chat.SessionStore interface
// the agent loop depends on, converting between the two packages' turn shapes so
// neither needs to import the other.
type chatSessionStore struct {
	repo *chatsessions.Repo
}

func (a chatSessionStore) SessionExists(ctx context.Context, id uuid.UUID) (bool, error) {
	return a.repo.SessionExists(ctx, id)
}

func (a chatSessionStore) LoadTurns(ctx context.Context, id uuid.UUID, limit int) ([]chat.StoredTurn, error) {
	msgs, err := a.repo.LoadTurns(ctx, id, limit)
	if err != nil {
		return nil, err
	}
	out := make([]chat.StoredTurn, len(msgs))
	for i, m := range msgs {
		out[i] = chat.StoredTurn{Role: m.Role, Content: m.Content}
	}
	return out, nil
}

func (a chatSessionStore) AppendTurns(ctx context.Context, id uuid.UUID, turns []chat.StoredTurn) error {
	msgs := make([]chatsessions.Message, len(turns))
	for i, t := range turns {
		msgs[i] = chatsessions.Message{Role: t.Role, Content: t.Content}
	}
	return a.repo.AppendMessages(ctx, id, msgs)
}

func (a chatSessionStore) SetTitleIfEmpty(ctx context.Context, id uuid.UUID, title string) error {
	return a.repo.SetTitleIfEmpty(ctx, id, title)
}
