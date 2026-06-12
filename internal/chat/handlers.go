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

// Register mounts POST /chat onto rg.
func (h *Handlers) Register(rg *gin.RouterGroup) {
	rg.POST("/chat", h.chat)
}

// chat godoc
// @Summary      Stream a nutrition-planning chat turn
// @Description  Runs a server-side Anthropic agent loop scoped to meal planning and streams the result as Server-Sent Events. The request body is `{session_id, message}` — an existing chat session and the single new user message; the server loads the session's prior turns, persists the new ones, and holds the conversation as the source of truth. The response is text/event-stream with four event types — `text` (assistant delta), `tool` (name+status+summary), `done` (final message, stop_reason, usage), and `error` (typed code). Tools are dispatched as loopback REST calls under the caller's bearer token. Returns 503 chat_unavailable when ANTHROPIC_API_KEY is unset, 404 session_not_found for an unknown session, and 400 for an empty message.
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

func extractBearer(authHeader string) string {
	const prefix = "Bearer "
	if strings.HasPrefix(authHeader, prefix) {
		return strings.TrimSpace(authHeader[len(prefix):])
	}
	return strings.TrimSpace(authHeader)
}
