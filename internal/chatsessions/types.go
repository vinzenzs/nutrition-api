// Package chatsessions persists chat conversations server-side: a session
// header (chat_sessions) plus its ordered turns (chat_messages) stored at full
// Anthropic content-block fidelity. It is the durable backing store the
// nutrition-chat loop reads history from and writes new turns into, and it
// exposes a small REST CRUD surface (/chat/sessions) for listing, fetching,
// renaming and deleting conversations.
package chatsessions

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/vinzenzs/nutrition-api/internal/agenttools"
)

// Session mirrors a chat_sessions row. Title is NULL/omitted when untitled.
// AwaitingConfirmation is a derived flag (not a column): true when the session's
// most recent turn is paused awaiting a write confirmation, so the history view
// can badge it (D9). Omitted when false.
type Session struct {
	ID                   uuid.UUID `json:"id"`
	Title                *string   `json:"title,omitempty"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
	LastMessageAt        time.Time `json:"last_message_at"`
	AwaitingConfirmation bool      `json:"awaiting_confirmation,omitempty"`
}

// PendingConfirmation mirrors the chat `proposal` SSE event for cold-open: the
// pending write-confirm calls of a session's paused trailing turn, with
// server-composed previews, so the client renders the same approve/reject card
// whether it arrived live or on reopening the session (D9).
type PendingConfirmation struct {
	TurnID string                    `json:"turn_id"`
	Calls  []agenttools.ProposalCall `json:"calls"`
}

// Message is one persisted turn. Content is the verbatim Anthropic content
// value — a JSON string for plain user text, or a content-block array for
// assistant/tool turns — handed back to the model unchanged on resume.
type Message struct {
	Role string `json:"role"`
	// Content is the verbatim Anthropic content value — a JSON string (plain
	// user text) or a content-block array — so its OpenAPI type is left open.
	Content json.RawMessage `json:"content" swaggertype:"object"`
}

// SessionWithMessages is the GET /chat/sessions/{id} body: the header plus its
// ordered turns at full fidelity. PendingConfirmation is non-null only when the
// session is paused awaiting a write confirmation (D9).
type SessionWithMessages struct {
	Session
	Messages            []Message            `json:"messages"`
	PendingConfirmation *PendingConfirmation `json:"pending_confirmation"`
}

const maxTitleLen = 200
