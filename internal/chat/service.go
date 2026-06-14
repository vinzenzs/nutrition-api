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
	"github.com/vinzenzs/nutrition-api/internal/agenttools"
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
	client      *client
	dispatcher  *dispatcher
	store       SessionStore
	cfg         Config
	specs       []agenttools.Spec
	specsByName map[string]agenttools.Spec
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
	specs := agenttools.Registry()
	return &Service{client: c, cfg: cfg, specs: specs, specsByName: agenttools.ByName(specs)}, nil
}

// SetToolSpecs overrides the tool surface the loop exposes and dispatches
// against. Production uses the default agenttools.Registry(); tests inject a
// surface containing a write-confirm tool to exercise the confirmation
// protocol. Call before SetLoopbackHandler (or it re-syncs the dispatcher).
func (s *Service) SetToolSpecs(specs []agenttools.Spec) {
	s.specs = specs
	s.specsByName = agenttools.ByName(specs)
	if s.dispatcher != nil {
		s.dispatcher.specs = s.specsByName
	}
}

// SetLoopbackHandler wires the in-process HTTP handler (the Gin engine) the tool
// dispatcher calls. Set after the engine is built, since the chat handler is
// itself registered on that engine.
func (s *Service) SetLoopbackHandler(h http.Handler) {
	s.dispatcher = newDispatcher(h)
	s.dispatcher.specs = s.specsByName
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
	// Load prior turns (truncated to the most recent MaxHistoryMessages) as the
	// conversation history, dropping any turn left dangling by truncation.
	prior, err := s.store.LoadTurns(ctx, sessionID, s.cfg.MaxHistoryMessages)
	if err != nil {
		sse.error("persistence_error", "could not load the conversation")
		return
	}

	// If the session is paused awaiting a write confirmation, a new free-text
	// message implicitly rejects the pending writes: append a declined
	// tool_result for each so the history is well-formed, then proceed (2.6).
	prior = s.implicitlyRejectPending(ctx, sessionID, prior)

	messages := toAnthropicMessages(sanitizeHistory(prior, s.specsByName))

	// Persist the new user message before streaming, and name an untitled
	// session from it. A persist failure here is terminal — nothing has streamed.
	userContent, _ := json.Marshal(message)
	if err := s.store.AppendTurns(ctx, sessionID, []StoredTurn{{Role: "user", Content: userContent}}); err != nil {
		sse.error("persistence_error", "could not save the message")
		return
	}
	_ = s.store.SetTitleIfEmpty(ctx, sessionID, deriveTitle(message))
	messages = append(messages, anthropicMessage{Role: "user", Content: userContent})

	s.runLoop(ctx, sse, sessionID, messages, bearer)
}

// runLoop drives the Anthropic agent loop from a prepared message window: it
// streams each turn, dispatches read/write-auto tool calls inline, pauses when a
// turn contains a write-confirm call (emitting a proposal + awaiting_
// confirmation), and persists every turn. Shared by the /chat path and the
// confirm-resume path.
func (s *Service) runLoop(ctx context.Context, sse *sseWriter, sessionID uuid.UUID, messages []anthropicMessage, bearer string) {
	toolDefs := anthropicToolDefs(s.specs)
	system := buildSystemPrompt(promptParams{
		DietaryPreferences: s.cfg.DietaryPreferences,
		Timezone:           s.cfg.Timezone,
	})

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

		assistantContent := marshalBlocks(turn.AssistantContent)

		// Pause-and-resume gate (2.2): if any call in this turn is write-confirm,
		// dispatch NOTHING — persist the assistant turn (its tool_use blocks, no
		// tool_result yet), surface a proposal describing the pending writes, and
		// end the stream awaiting confirmation. The confirm endpoint resumes it.
		if pending := agenttools.PendingFromContent(assistantContent, s.specsByName); len(pending) > 0 {
			_ = s.store.AppendTurns(ctx, sessionID, []StoredTurn{
				{Role: "assistant", Content: assistantContent},
			})
			sse.proposal(agenttools.TurnID(assistantContent), pending)
			sse.done(full.String(), "awaiting_confirmation", usage)
			return
		}

		// Echo the assistant turn (with its tool_use blocks) and dispatch tools.
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

// implicitlyRejectPending answers a trailing awaiting-confirmation turn by
// declining every one of its tool_use blocks, persisting one tool_result user
// turn, and returning the extended history. A no-op when the session is not
// paused. Every tool_use block must be answered before another user message, so
// even read/write-auto calls in the (rare) mixed paused turn are declined.
func (s *Service) implicitlyRejectPending(ctx context.Context, sessionID uuid.UUID, prior []StoredTurn) []StoredTurn {
	if len(prior) == 0 {
		return prior
	}
	last := prior[len(prior)-1]
	if last.Role != "assistant" || !agenttools.AwaitingConfirmation(last.Content, s.specsByName) {
		return prior
	}
	blocks := agenttools.ParseToolUseBlocks(last.Content)
	resultBlocks := make([]json.RawMessage, 0, len(blocks))
	for _, b := range blocks {
		resultBlocks = append(resultBlocks, declinedResultBlock(b.ID))
	}
	declined := StoredTurn{Role: "user", Content: marshalBlocks(resultBlocks)}
	_ = s.store.AppendTurns(ctx, sessionID, []StoredTurn{declined})
	return append(prior, declined)
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
// trailing assistant turn left ending on a truncation-dangling tool_use. A
// trailing turn that is *intentionally* paused awaiting a write confirmation
// (it carries a write-confirm tool_use) is PRESERVED — it is the resume anchor
// the confirm endpoint answers (2.5 / D7).
func sanitizeHistory(turns []StoredTurn, specs map[string]agenttools.Spec) []StoredTurn {
	for len(turns) > 0 && !opensCleanly(turns[0]) {
		turns = turns[1:]
	}
	if n := len(turns); n > 0 && turns[n-1].Role == "assistant" && hasBlockType(turns[n-1].Content, "tool_use") {
		if !agenttools.AwaitingConfirmation(turns[n-1].Content, specs) {
			turns = turns[:n-1]
		}
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

// declinedResultBlock builds the synthetic tool_result for a call the user
// declined (explicitly via confirm-reject, or implicitly by sending a new
// message). It is a normal result, not is_error — the model adapts its reply.
func declinedResultBlock(toolUseID string) json.RawMessage {
	block := map[string]any{
		"type":        "tool_result",
		"tool_use_id": toolUseID,
		"content":     "The user declined this action.",
	}
	raw, _ := json.Marshal(block)
	return raw
}

// confirmMode selects how a confirm request resumes a session.
type confirmMode int

const (
	// confirmExecute: the trailing turn is an awaiting-confirmation assistant
	// tool_use turn; consume decisions, dispatch, append results, continue.
	confirmExecute confirmMode = iota
	// confirmContinue: the trailing turn is a tool_result with no following
	// assistant (a continuation owed after a prior resume's stream died); ignore
	// decisions and just continue the loop — re-running a dropped confirm
	// recovers the answer without re-executing the writes (D5 case b).
	confirmContinue
)

// confirmPlan is the validated outcome of prepareConfirm, carrying everything
// streamConfirm needs without reloading the session.
type confirmPlan struct {
	mode     confirmMode
	turns    []StoredTurn    // the loaded history (trailing turn is the anchor)
	approved map[string]bool // execute mode: tool_id → approved
}

// prepareConfirm validates a confirm request against the session's trailing-turn
// shape and returns a plan, or a non-empty error code the handler maps to a
// status (nothing_to_confirm → 409, invalid_confirmation → 400, persistence_
// error → 500). It performs no streaming and no writes.
func (s *Service) prepareConfirm(ctx context.Context, sessionID uuid.UUID, decisions []ConfirmDecision) (*confirmPlan, string) {
	turns, err := s.store.LoadTurns(ctx, sessionID, s.cfg.MaxHistoryMessages)
	if err != nil {
		return nil, "persistence_error"
	}
	if len(turns) == 0 {
		return nil, "nothing_to_confirm"
	}
	last := turns[len(turns)-1]
	switch {
	case last.Role == "assistant" && hasBlockType(last.Content, "tool_use"):
		pending := agenttools.PendingFromContent(last.Content, s.specsByName)
		if len(pending) == 0 {
			// A tool_use turn with no write-confirm call is not a paused turn.
			return nil, "nothing_to_confirm"
		}
		approved, ok := matchDecisions(pending, decisions)
		if !ok {
			return nil, "invalid_confirmation"
		}
		return &confirmPlan{mode: confirmExecute, turns: turns, approved: approved}, ""
	case last.Role == "user" && hasBlockType(last.Content, "tool_result"):
		return &confirmPlan{mode: confirmContinue, turns: turns}, ""
	default:
		return nil, "nothing_to_confirm"
	}
}

// matchDecisions checks that decisions cover exactly the pending write-confirm
// calls (no missing, no extra, no duplicate tool_ids) and returns the approval
// map. ok is false on any mismatch (→ 400 invalid_confirmation).
func matchDecisions(pending []agenttools.ProposalCall, decisions []ConfirmDecision) (map[string]bool, bool) {
	want := make(map[string]bool, len(pending))
	for _, p := range pending {
		want[p.ToolID] = true
	}
	got := make(map[string]bool, len(decisions))
	for _, d := range decisions {
		if !want[d.ToolID] {
			return nil, false // decision for a non-pending (or duplicate) call
		}
		if _, dup := got[d.ToolID]; dup {
			return nil, false
		}
		got[d.ToolID] = d.Approve
	}
	if len(got) != len(want) {
		return nil, false // some pending call has no decision
	}
	return got, true
}

// streamConfirm resumes a paused session per the prepared plan and streams the
// continuation with the same SSE contract as /chat. In execute mode it
// dispatches the trailing turn's calls in original order (approved write-confirm
// + every read/write-auto), synthesizes a declined result for rejected calls,
// appends one tool_result user turn, then continues the loop. In continue mode
// it just continues the loop.
func (s *Service) streamConfirm(ctx context.Context, sse *sseWriter, sessionID uuid.UUID, plan *confirmPlan, bearer string) {
	messages := toAnthropicMessages(sanitizeHistory(plan.turns, s.specsByName))

	if plan.mode == confirmContinue {
		s.runLoop(ctx, sse, sessionID, messages, bearer)
		return
	}

	// execute: the trailing (preserved) assistant turn holds the tool_use blocks.
	last := plan.turns[len(plan.turns)-1]
	blocks := agenttools.ParseToolUseBlocks(last.Content)
	resultBlocks := make([]json.RawMessage, 0, len(blocks))
	for _, b := range blocks {
		spec, known := s.specsByName[b.Name]
		if known && spec.Tier == agenttools.TierWriteConfirm && !plan.approved[b.ID] {
			resultBlocks = append(resultBlocks, declinedResultBlock(b.ID))
			continue
		}
		// Approved write-confirm, or a read/write-auto call sharing the turn.
		sse.tool(b.ID, b.Name, "started", "")
		res := s.dispatcher.execute(ctx, b.Name, b.Input, bearer)
		resultBlocks = append(resultBlocks, toolResultBlock(b.ID, res))
		name, status, summary := toolEventFields(b.Name, res)
		sse.tool(b.ID, name, status, summary)
	}
	toolResultContent := marshalBlocks(resultBlocks)
	_ = s.store.AppendTurns(ctx, sessionID, []StoredTurn{{Role: "user", Content: toolResultContent}})
	messages = append(messages, anthropicMessage{Role: "user", Content: toolResultContent})

	s.runLoop(ctx, sse, sessionID, messages, bearer)
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
