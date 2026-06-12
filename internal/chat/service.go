package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Config carries the chat runtime knobs, sourced from the server config.
type Config struct {
	Model              string
	MaxToolRounds      int
	MaxHistoryMessages int
	RequestTimeout     time.Duration
	DietaryPreferences string
	Timezone           string
	// BaseURL overrides the Anthropic endpoint for fixture tests.
	BaseURL string
}

// StoredTurn is one persisted conversation turn — a role plus the verbatim
// Anthropic content value (a JSON string for plain user text, or a content-block
// array for assistant/tool turns).
type StoredTurn struct {
	Role    string
	Content json.RawMessage
}

// SessionStore is the durable backing the session-backed loop needs: confirm a
// session exists, load its prior turns, append new ones, and name an untitled
// session from its opening message. Implemented by an adapter over
// chatsessions.Repo in production and by a fake in loop tests.
type SessionStore interface {
	SessionExists(ctx context.Context, sessionID uuid.UUID) (bool, error)
	LoadTurns(ctx context.Context, sessionID uuid.UUID, limit int) ([]StoredTurn, error)
	AppendTurns(ctx context.Context, sessionID uuid.UUID, turns []StoredTurn) error
	SetTitleIfEmpty(ctx context.Context, sessionID uuid.UUID, title string) error
}

// Service runs the server-side chat agent loop. It is constructed only when an
// Anthropic API key is present; the handler returns 503 when it is nil.
type Service struct {
	client     *client
	dispatcher *dispatcher
	store      SessionStore
	cfg        Config
}

// New builds the chat Service. Returns ErrAPIKeyMissing when apiKey is empty so
// the caller can leave the Service nil and surface 503 chat_unavailable.
func New(apiKey string, cfg Config) (*Service, error) {
	c, err := newClient(clientConfig{
		APIKey:  apiKey,
		BaseURL: cfg.BaseURL,
		Model:   cfg.Model,
		Timeout: cfg.RequestTimeout,
	})
	if err != nil {
		return nil, err
	}
	if cfg.MaxToolRounds <= 0 {
		cfg.MaxToolRounds = 8
	}
	if cfg.MaxHistoryMessages <= 0 {
		cfg.MaxHistoryMessages = 40
	}
	return &Service{client: c, cfg: cfg}, nil
}

// SetLoopbackHandler wires the in-process HTTP handler (the Gin engine) the tool
// dispatcher calls. Set after the engine is built, since the chat handler is
// itself registered on that engine.
func (s *Service) SetLoopbackHandler(h http.Handler) {
	s.dispatcher = newDispatcher(h)
}

// SetSessionStore wires the durable session store the loop loads history from
// and persists turns into. Required before the loop can run.
func (s *Service) SetSessionStore(store SessionStore) {
	s.store = store
}

// SessionExists reports whether the given session exists, so the handler can
// 404 before starting a stream.
func (s *Service) SessionExists(ctx context.Context, sessionID uuid.UUID) (bool, error) {
	return s.store.SessionExists(ctx, sessionID)
}

// stream runs the agent loop for one request, writing SSE events. sessionID is
// an existing session whose stored turns are loaded as history and into which
// every new turn is persisted; message is the new user message; bearer is the
// caller's token, forwarded to tools.
func (s *Service) stream(ctx context.Context, sse *sseWriter, sessionID uuid.UUID, message, bearer string) {
	specs := registry()
	toolDefs := anthropicToolDefs(specs)
	system := buildSystemPrompt(promptParams{
		DietaryPreferences: s.cfg.DietaryPreferences,
		Timezone:           s.cfg.Timezone,
	})

	// Load prior turns (truncated to the most recent MaxHistoryMessages) as the
	// conversation history, dropping any turn left dangling by truncation.
	prior, err := s.store.LoadTurns(ctx, sessionID, s.cfg.MaxHistoryMessages)
	if err != nil {
		sse.error("persistence_error", "could not load the conversation")
		return
	}
	messages := toAnthropicMessages(sanitizeHistory(prior))

	// Persist the new user message before streaming, and name an untitled
	// session from it. A persist failure here is terminal — nothing has streamed.
	userContent, _ := json.Marshal(message)
	if err := s.store.AppendTurns(ctx, sessionID, []StoredTurn{{Role: "user", Content: userContent}}); err != nil {
		sse.error("persistence_error", "could not save the message")
		return
	}
	_ = s.store.SetTitleIfEmpty(ctx, sessionID, deriveTitle(message))
	messages = append(messages, anthropicMessage{Role: "user", Content: userContent})

	var full strings.Builder
	var usage Usage
	rounds := 0

	for {
		withTools := rounds < s.cfg.MaxToolRounds
		req := messagesRequest{
			Model:     s.cfg.Model,
			MaxTokens: maxTokensPerTurn,
			System:    system,
			Messages:  messages,
		}
		if withTools {
			req.Tools = toolDefs
		}

		turn, err := s.client.stream(ctx, req, func(delta string) {
			full.WriteString(delta)
			sse.text(delta)
		})
		if err != nil {
			s.emitStreamError(sse, ctx, err)
			return
		}
		usage = turn.Usage

		// Terminal: no client tools requested, or tools were withheld this turn.
		if !withTools || len(turn.ClientToolCalls) == 0 {
			stop := turn.StopReason
			if !withTools && rounds >= s.cfg.MaxToolRounds {
				stop = "max_tool_rounds"
			}
			// Persist the final assistant turn (best-effort: the answer has
			// already streamed) before signalling done.
			_ = s.store.AppendTurns(ctx, sessionID, []StoredTurn{
				{Role: "assistant", Content: marshalBlocks(turn.AssistantContent)},
			})
			sse.done(full.String(), stop, usage)
			return
		}

		// Echo the assistant turn (with its tool_use blocks) and dispatch tools.
		assistantContent := marshalBlocks(turn.AssistantContent)
		messages = append(messages, anthropicMessage{
			Role:    "assistant",
			Content: assistantContent,
		})
		resultBlocks := make([]json.RawMessage, 0, len(turn.ClientToolCalls))
		for _, call := range turn.ClientToolCalls {
			sse.tool(call.ID, call.Name, "started", "")
			res := s.dispatcher.execute(ctx, call.Name, call.Input, bearer)
			resultBlocks = append(resultBlocks, toolResultBlock(call.ID, res))
			name, status, summary := toolEventFields(call.Name, res)
			sse.tool(call.ID, name, status, summary)
		}
		toolResultContent := marshalBlocks(resultBlocks)
		messages = append(messages, anthropicMessage{
			Role:    "user",
			Content: toolResultContent,
		})
		// Persist the assistant turn and its tool_result reply together (one
		// atomic append) so a stored session never ends on a dangling tool_use.
		_ = s.store.AppendTurns(ctx, sessionID, []StoredTurn{
			{Role: "assistant", Content: assistantContent},
			{Role: "user", Content: toolResultContent},
		})
		rounds++
	}
}

// toAnthropicMessages maps stored turns to the upstream message shape; each
// turn's content is already the verbatim Anthropic content value.
func toAnthropicMessages(turns []StoredTurn) []anthropicMessage {
	out := make([]anthropicMessage, 0, len(turns))
	for _, t := range turns {
		out = append(out, anthropicMessage{Role: t.Role, Content: t.Content})
	}
	return out
}

// sanitizeHistory makes a (possibly truncated) turn window safe to send
// upstream: it drops leading turns that cannot legally open a request — an
// assistant turn, or a tool_result user turn orphaned from its tool_use — and a
// trailing assistant turn left ending on an unanswered tool_use.
func sanitizeHistory(turns []StoredTurn) []StoredTurn {
	for len(turns) > 0 && !opensCleanly(turns[0]) {
		turns = turns[1:]
	}
	if n := len(turns); n > 0 && turns[n-1].Role == "assistant" && hasBlockType(turns[n-1].Content, "tool_use") {
		turns = turns[:n-1]
	}
	return turns
}

// opensCleanly reports whether a turn can be the first message of a request: a
// user turn that is not a tool_result reply.
func opensCleanly(t StoredTurn) bool {
	return t.Role == "user" && !hasBlockType(t.Content, "tool_result")
}

// hasBlockType reports whether content is a content-block array containing a
// block of the given type. A plain-string content (user text) has none.
func hasBlockType(content json.RawMessage, blockType string) bool {
	var blocks []struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(content, &blocks); err != nil {
		return false
	}
	for _, b := range blocks {
		if b.Type == blockType {
			return true
		}
	}
	return false
}

// deriveTitle builds a short session title from the opening user message.
func deriveTitle(message string) string {
	t := strings.TrimSpace(strings.Join(strings.Fields(message), " "))
	const maxRunes = 60
	if utf8.RuneCountInString(t) <= maxRunes {
		return t
	}
	r := []rune(t)
	return strings.TrimSpace(string(r[:maxRunes])) + "…"
}

func (s *Service) emitStreamError(sse *sseWriter, ctx context.Context, err error) {
	if ctx.Err() == context.DeadlineExceeded {
		sse.error("timeout", "the request timed out")
		return
	}
	var up *ErrUpstreamUnavailable
	var proto *ErrUpstreamProtocol
	if errors.As(err, &up) || errors.As(err, &proto) {
		sse.error("upstream_unavailable", "the language model is temporarily unavailable")
		return
	}
	sse.error("upstream_unavailable", err.Error())
}

// marshalBlocks renders a slice of content-block JSON values as a JSON array.
func marshalBlocks(blocks []json.RawMessage) json.RawMessage {
	if len(blocks) == 0 {
		return json.RawMessage("[]")
	}
	raw, _ := json.Marshal(blocks)
	return raw
}

// toolResultBlock builds the tool_result content block fed back to the model.
// The REST response body becomes the tool result content; non-2xx and
// build-failures are marked is_error so the model can react.
func toolResultBlock(toolUseID string, res toolResult) json.RawMessage {
	var content string
	isError := false
	switch {
	case res.err != nil:
		content = "tool input error: " + res.err.Error()
		isError = true
	case !res.ok:
		content = string(res.body)
		isError = true
	default:
		content = string(res.body)
	}
	block := map[string]any{
		"type":        "tool_result",
		"tool_use_id": toolUseID,
		"content":     content,
	}
	if isError {
		block["is_error"] = true
	}
	raw, _ := json.Marshal(block)
	return raw
}

// toolEventFields derives the SSE tool event fields from a result — name, a
// status of ok|error, and a short summary that never leaks the response body.
func toolEventFields(name string, res toolResult) (string, string, string) {
	switch {
	case res.err != nil:
		return name, "error", "invalid input"
	case !res.ok:
		return name, "error", fmt.Sprintf("failed (status %d)", res.status)
	default:
		// Success carries no summary; the client labels the chip by name and
		// shows status via the icon.
		return name, "ok", ""
	}
}
