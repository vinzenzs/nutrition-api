package chat

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Handlers wires the chat service to the POST /chat route. svc may be nil when
// the server starts without an Anthropic API key — the handler then returns 503
// chat_unavailable, mirroring how meals/from_photo handles a missing vision key.
type Handlers struct {
	svc *Service
}

func NewHandlers(svc *Service) *Handlers {
	return &Handlers{svc: svc}
}

// Register mounts the chat routes onto rg: the streaming turn endpoint and the
// confirmation-resume endpoint.
func (h *Handlers) Register(rg *gin.RouterGroup) {
	rg.POST("/chat", h.chat)
	rg.POST("/chat/sessions/:id/confirm", h.confirm)
}

// chat godoc
// @Summary      Stream a nutrition-planning chat turn
// @Description  Runs a server-side Anthropic agent loop scoped to meal planning and streams the result as Server-Sent Events. The request body is `{session_id, message}` — an existing chat session and the single new user message; the server loads the session's prior turns, persists the new ones, and holds the conversation as the source of truth. The response is text/event-stream with four event types — `text` (assistant delta), `tool` (id+name+status+summary; a call's started and ok/error events share one id so clients coalesce them), `done` (final message, stop_reason, usage), and `error` (typed code). Tools are dispatched as loopback REST calls under the caller's bearer token. Returns 503 chat_unavailable when ANTHROPIC_API_KEY is unset, 404 session_not_found for an unknown session, and 400 for an empty message.
// @Tags         chat
// @Accept       json
// @Produce      text/event-stream
// @Param        body  body  ChatRequest  true  "Session id + new message"
// @Success      200   {string}  string  "SSE stream"
// @Failure      400   {object}  map[string]string  "invalid_json | empty_message"
// @Failure      404   {object}  map[string]string  "session_not_found"
// @Failure      503   {object}  map[string]string  "chat_unavailable"
// @Security     BearerAuth
// @Router       /chat [post]
func (h *Handlers) chat(c *gin.Context) {
	// 503 before any stream when the key is unset.
	if h.svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "chat_unavailable"})
		return
	}

	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
		return
	}
	sessionID, err := uuid.Parse(req.SessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "session_not_found"})
		return
	}
	if strings.TrimSpace(req.Message) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "empty_message"})
		return
	}

	// Resolve the session before opening the stream so an unknown id is a clean
	// 404 rather than an error event.
	exists, err := h.svc.SessionExists(c.Request.Context(), sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "persistence_error"})
		return
	}
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "session_not_found"})
		return
	}

	bearer := extractBearer(c.GetHeader("Authorization"))

	// Per-request timeout independent of the client connection's context.
	ctx, cancel := context.WithTimeout(c.Request.Context(), h.svc.cfg.RequestTimeout)
	defer cancel()

	sse, ok := newSSEWriter(c.Writer)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming_unsupported"})
		return
	}
	h.svc.stream(ctx, sse, sessionID, req.Message, bearer)
}

// confirm godoc
// @Summary      Resume a chat session paused awaiting a write confirmation
// @Description  Resolves a session whose trailing turn proposed one or more `write-confirm` actions (the `/chat` stream ended with `done.stop_reason = "awaiting_confirmation"` after a `proposal` event). The body is `{decisions: [{tool_id, approve}]}` and MUST cover exactly the pending write-confirm calls. Approved calls (plus any read/write-auto calls in the same turn) are dispatched in order; rejected calls get a synthetic declined `tool_result`; then the agent loop resumes and streams the continuation as Server-Sent Events using the same event contract as `/chat`. Idempotency keys are content-derived, so re-sending the same confirmation replays rather than double-writes; if a prior resume's stream died after the writes committed (trailing turn is a `tool_result`), re-posting simply continues the loop. Returns 404 session_not_found, 409 nothing_to_confirm (the session is not paused), or 400 invalid_confirmation (decisions do not match the pending calls) — all before any stream is started.
// @Tags         chat
// @Accept       json
// @Produce      text/event-stream
// @Param        id    path  string          true  "Session UUID"
// @Param        body  body  ConfirmRequest  true  "Per-call approve/reject decisions"
// @Success      200   {string}  string  "SSE stream"
// @Failure      400   {object}  map[string]string  "invalid_json | invalid_confirmation"
// @Failure      404   {object}  map[string]string  "session_not_found"
// @Failure      409   {object}  map[string]string  "nothing_to_confirm"
// @Failure      503   {object}  map[string]string  "chat_unavailable"
// @Security     BearerAuth
// @Router       /chat/sessions/{id}/confirm [post]
func (h *Handlers) confirm(c *gin.Context) {
	if h.svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "chat_unavailable"})
		return
	}

	sessionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "session_not_found"})
		return
	}
	var req ConfirmRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
		return
	}

	exists, err := h.svc.SessionExists(c.Request.Context(), sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "persistence_error"})
		return
	}
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "session_not_found"})
		return
	}

	bearer := extractBearer(c.GetHeader("Authorization"))
	ctx, cancel := context.WithTimeout(c.Request.Context(), h.svc.cfg.RequestTimeout)
	defer cancel()

	// Validate the trailing-turn shape before opening the stream, so a bad
	// request is a clean JSON error rather than an SSE error event.
	plan, code := h.svc.prepareConfirm(ctx, sessionID, req.Decisions)
	if code != "" {
		c.JSON(confirmErrorStatus(code), gin.H{"error": code})
		return
	}

	sse, ok := newSSEWriter(c.Writer)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming_unsupported"})
		return
	}
	h.svc.streamConfirm(ctx, sse, sessionID, plan, bearer)
}

// confirmErrorStatus maps a prepareConfirm error code to its HTTP status.
func confirmErrorStatus(code string) int {
	switch code {
	case "nothing_to_confirm":
		return http.StatusConflict
	case "invalid_confirmation":
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

func extractBearer(authHeader string) string {
	const prefix = "Bearer "
	if strings.HasPrefix(authHeader, prefix) {
		return strings.TrimSpace(authHeader[len(prefix):])
	}
	return strings.TrimSpace(authHeader)
}
