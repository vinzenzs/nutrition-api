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
)

// Session mirrors a chat_sessions row. Title is NULL/omitted when untitled.
type Session struct {
	ID            uuid.UUID `json:"id"`
	Title         *string   `json:"title,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	LastMessageAt time.Time `json:"last_message_at"`
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
// ordered turns at full fidelity.
type SessionWithMessages struct {
	Session
	Messages []Message `json:"messages"`
}

const maxTitleLen = 200
