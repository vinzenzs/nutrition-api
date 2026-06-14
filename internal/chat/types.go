package chat

import "encoding/json"

// ----- Anthropic Messages API request shapes -----

// messagesRequest is the body POSTed to /v1/messages with stream:true.
type messagesRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	System    string          `json:"system,omitempty"`
	Tools     []json.RawMessage `json:"tools,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
	Stream    bool            `json:"stream"`
}

// anthropicMessage is one turn. Content is either a JSON string (plain user
// text) or a JSON array of content blocks (echoed assistant turns, tool_result
// user turns) — kept raw so both round-trip without a bespoke union type.
type anthropicMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

// ----- parsed-turn result -----

// clientToolCall is one custom (client-executed) tool_use block the model
// emitted. web_search and other server tools are executed by Anthropic inline
// and never surface here.
type clientToolCall struct {
	ID    string
	Name  string
	Input json.RawMessage
}

// turnResult is what one streamed assistant turn yields to the loop.
type turnResult struct {
	// AssistantContent is the finalized content-block array for this turn,
	// echoed back verbatim when the conversation continues after tool use.
	AssistantContent []json.RawMessage
	// Text is the concatenated visible text of the turn (already streamed to
	// the client via the onText callback as it arrived).
	Text string
	// ClientToolCalls are the custom tool_use blocks awaiting local dispatch.
	ClientToolCalls []clientToolCall
	// StopReason is Anthropic's stop_reason for the turn ("tool_use",
	// "end_turn", "max_tokens", ...).
	StopReason string
	Usage      Usage
}

// Usage is the token accounting echoed in the done event.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// ----- public request/response (handler boundary) -----

// ChatRequest is the POST /chat body: the id of an existing session and the
// single new user message. History lives server-side, keyed by SessionID.
type ChatRequest struct {
	SessionID string `json:"session_id"`
	Message   string `json:"message"`
}

// ConfirmDecision is one user verdict on a pending write-confirm call.
type ConfirmDecision struct {
	ToolID  string `json:"tool_id"`
	Approve bool   `json:"approve"`
}

// ConfirmRequest is the POST /chat/sessions/{id}/confirm body: a verdict for
// each pending write-confirm call of the session's paused turn. The decisions
// must cover exactly those calls.
type ConfirmRequest struct {
	Decisions []ConfirmDecision `json:"decisions"`
}
