package chatsessions

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Handlers wires the /chat/sessions CRUD surface onto a Gin router group.
type Handlers struct {
	svc *Service
}

func NewHandlers(svc *Service) *Handlers { return &Handlers{svc: svc} }

// Register mounts the five session routes onto rg.
func (h *Handlers) Register(rg *gin.RouterGroup) {
	rg.POST("/chat/sessions", h.create)
	rg.GET("/chat/sessions", h.list)
	rg.GET("/chat/sessions/:id", h.get)
	rg.PATCH("/chat/sessions/:id", h.rename)
	rg.DELETE("/chat/sessions/:id", h.delete)
}

type titleRequest struct {
	Title *string `json:"title,omitempty"`
}

// create godoc
// @Summary      Create a chat session
// @Description  Opens a new conversation. `title` is optional — an absent or empty title creates an untitled session (the first /chat turn then names it from the opening message). Honors an Idempotency-Key.
// @Tags         chat-sessions
// @Accept       json
// @Produce      json
// @Param        Idempotency-Key  header  string        false  "Optional client-supplied idempotency key"
// @Param        body             body    titleRequest  false  "Optional title"
// @Success      201  {object}  Session
// @Failure      400  {object}  map[string]string  "invalid_json | title_invalid"
// @Security     BearerAuth
// @Router       /chat/sessions [post]
func (h *Handlers) create(c *gin.Context) {
	var req titleRequest
	// An empty body is allowed (untitled); only malformed JSON is rejected.
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			respondError(c, http.StatusBadRequest, "invalid_json")
			return
		}
	}
	sess, err := h.svc.Create(c.Request.Context(), req.Title)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, sess)
}

// list godoc
// @Summary      List chat sessions
// @Description  Returns session headers (no transcript) most-recent-first by last activity.
// @Tags         chat-sessions
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "{ sessions: [...] }"
// @Security     BearerAuth
// @Router       /chat/sessions [get]
func (h *Handlers) list(c *gin.Context) {
	sessions, err := h.svc.List(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, "list_failed")
		return
	}
	if sessions == nil {
		sessions = []*Session{}
	}
	c.JSON(http.StatusOK, gin.H{"sessions": sessions})
}

// get godoc
// @Summary      Get a chat session with its transcript
// @Description  Returns the session header plus its ordered turns at full fidelity (each turn's role and verbatim content blocks).
// @Tags         chat-sessions
// @Produce      json
// @Param        id  path  string  true  "Session UUID"
// @Success      200  {object}  SessionWithMessages
// @Failure      404  {object}  map[string]string  "session_not_found"
// @Security     BearerAuth
// @Router       /chat/sessions/{id} [get]
func (h *Handlers) get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusNotFound, "session_not_found")
		return
	}
	out, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	if out.Messages == nil {
		out.Messages = []Message{}
	}
	c.JSON(http.StatusOK, out)
}

// rename godoc
// @Summary      Rename a chat session
// @Description  Sets the session title. A title of "" (or null) clears it to untitled.
// @Tags         chat-sessions
// @Accept       json
// @Produce      json
// @Param        id    path  string        true  "Session UUID"
// @Param        body  body  titleRequest  true  "New title (\"\" clears)"
// @Success      200  {object}  Session
// @Failure      400  {object}  map[string]string  "invalid_json | title_invalid"
// @Failure      404  {object}  map[string]string  "session_not_found"
// @Security     BearerAuth
// @Router       /chat/sessions/{id} [patch]
func (h *Handlers) rename(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusNotFound, "session_not_found")
		return
	}
	var req titleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_json")
		return
	}
	if err := h.svc.Rename(c.Request.Context(), id, req.Title); err != nil {
		respondServiceError(c, err)
		return
	}
	sess, err := h.svc.repo.GetSession(c.Request.Context(), id)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, sess)
}

// delete godoc
// @Summary      Delete a chat session
// @Description  Removes the session and cascades its turns.
// @Tags         chat-sessions
// @Param        id  path  string  true  "Session UUID"
// @Success      204  "no content"
// @Failure      404  {object}  map[string]string  "session_not_found"
// @Security     BearerAuth
// @Router       /chat/sessions/{id} [delete]
func (h *Handlers) delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusNotFound, "session_not_found")
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		respondServiceError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ----- helpers -----

func respondError(c *gin.Context, status int, code string) {
	c.JSON(status, gin.H{"error": code})
}

func respondServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		respondError(c, http.StatusNotFound, "session_not_found")
	case errors.Is(err, ErrTitleTooLong):
		respondError(c, http.StatusBadRequest, "title_invalid")
	default:
		respondError(c, http.StatusInternalServerError, "write_failed")
	}
}
