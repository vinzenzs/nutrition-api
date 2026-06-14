package hydration

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/vinzenzs/kazper/internal/numfmt"
)

const maxWindowDays = 92

// Handlers wires hydration endpoints onto a Gin router group.
type Handlers struct {
	svc *Service
}

func NewHandlers(svc *Service) *Handlers {
	return &Handlers{svc: svc}
}

func (h *Handlers) Register(rg *gin.RouterGroup) {
	rg.POST("/hydration", h.create)
	rg.GET("/hydration", h.list)
	rg.PATCH("/hydration/:id", h.patch)
	rg.DELETE("/hydration/:id", h.delete)
}

type createRequest struct {
	QuantityMl *float64 `json:"quantity_ml"`
	LoggedAt   string   `json:"logged_at"`
	Note       *string  `json:"note,omitempty"`
	WorkoutID  *string  `json:"workout_id,omitempty"`
}

// create godoc
// @Summary      Log a hydration entry
// @Description  Records a volume of fluid drunk at a moment in time. The optional `note` carries beverage context (e.g. "water", "coffee", "electrolytes").
// @Tags         hydration
// @Accept       json
// @Produce      json
// @Param        Idempotency-Key  header  string         false  "Optional client-supplied idempotency key"
// @Param        body             body    createRequest  true   "Hydration entry"
// @Success      201  {object}  Entry
// @Failure      400  {object}  map[string]string  "quantity_ml_invalid | logged_at_invalid | logged_at_too_far_future | note_too_long"
// @Security     BearerAuth
// @Router       /hydration [post]
func (h *Handlers) create(c *gin.Context) {
	var req createRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_json")
		return
	}
	if req.QuantityMl == nil {
		respondError(c, http.StatusBadRequest, "quantity_ml_invalid")
		return
	}
	ts, err := parseLoggedAt(req.LoggedAt)
	if err != nil {
		respondError(c, http.StatusBadRequest, "logged_at_invalid")
		return
	}
	in := CreateInput{
		QuantityMl: *req.QuantityMl,
		LoggedAt:   ts,
		Note:       req.Note,
	}
	if req.WorkoutID != nil && *req.WorkoutID != "" {
		wid, err := uuid.Parse(*req.WorkoutID)
		if err != nil {
			respondError(c, http.StatusBadRequest, "workout_id_invalid")
			return
		}
		in.WorkoutID = &wid
	}
	e, err := h.svc.Create(c.Request.Context(), in)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, roundEntry(e))
}

// list godoc
// @Summary      List hydration entries in a window
// @Tags         hydration
// @Produce      json
// @Param        from  query  string  true   "Inclusive RFC3339 lower bound"
// @Param        to    query  string  true   "Exclusive RFC3339 upper bound"
// @Success      200  {object}  map[string]interface{}  "{ entries: [...] }"
// @Failure      400  {object}  map[string]interface{}  "window_required | window_invalid | range_too_large"
// @Security     BearerAuth
// @Router       /hydration [get]
func (h *Handlers) list(c *gin.Context) {
	fromStr := c.Query("from")
	toStr := c.Query("to")
	if fromStr == "" || toStr == "" {
		respondError(c, http.StatusBadRequest, "window_required")
		return
	}
	from, err := time.Parse(time.RFC3339, fromStr)
	if err != nil {
		respondError(c, http.StatusBadRequest, "window_invalid")
		return
	}
	to, err := time.Parse(time.RFC3339, toStr)
	if err != nil {
		respondError(c, http.StatusBadRequest, "window_invalid")
		return
	}
	if !from.Before(to) {
		respondError(c, http.StatusBadRequest, "window_invalid")
		return
	}
	if to.Sub(from) > time.Duration(maxWindowDays)*24*time.Hour {
		c.JSON(http.StatusBadRequest, gin.H{"error": "range_too_large", "max_days": maxWindowDays})
		return
	}
	entries, err := h.svc.List(c.Request.Context(), from, to)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "list_failed")
		return
	}
	out := make([]*Entry, 0, len(entries))
	for _, e := range entries {
		out = append(out, roundEntry(e))
	}
	c.JSON(http.StatusOK, gin.H{"entries": out})
}

type patchRequest struct {
	QuantityMl *float64 `json:"quantity_ml,omitempty"`
	LoggedAt   *string  `json:"logged_at,omitempty"`
	Note       *string  `json:"note,omitempty"`
	// WorkoutID supports the empty-string clear sentinel:
	//   omitted   → leave unchanged
	//   "<uuid>"  → set the link
	//   ""        → clear the link
	WorkoutID *string `json:"workout_id,omitempty"`
}

// patch godoc
// @Summary      Partially update a hydration entry
// @Tags         hydration
// @Accept       json
// @Produce      json
// @Param        id    path  string        true   "Hydration entry UUID"
// @Param        body  body  patchRequest  true   "Fields to update"
// @Success      200  {object}  Entry
// @Failure      400  {object}  map[string]string  "quantity_ml_invalid | logged_at_invalid | note_too_long"
// @Failure      404  {object}  map[string]string  "hydration_not_found"
// @Security     BearerAuth
// @Router       /hydration/{id} [patch]
func (h *Handlers) patch(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusNotFound, "hydration_not_found")
		return
	}
	raw, err := c.GetRawData()
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid_json")
		return
	}
	var req patchRequest
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &req); err != nil {
			respondError(c, http.StatusBadRequest, "invalid_json")
			return
		}
	}
	in := PatchInput{
		QuantityMl: req.QuantityMl,
		Note:       req.Note,
	}
	if req.LoggedAt != nil {
		ts, err := time.Parse(time.RFC3339, *req.LoggedAt)
		if err != nil {
			respondError(c, http.StatusBadRequest, "logged_at_invalid")
			return
		}
		in.LoggedAt = &ts
	}
	if req.WorkoutID != nil {
		if *req.WorkoutID == "" {
			in.ClearWorkoutID = true
		} else {
			wid, err := uuid.Parse(*req.WorkoutID)
			if err != nil {
				respondError(c, http.StatusBadRequest, "workout_id_invalid")
				return
			}
			in.WorkoutID = &wid
		}
	}
	e, err := h.svc.Patch(c.Request.Context(), id, in)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respondError(c, http.StatusNotFound, "hydration_not_found")
			return
		}
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, roundEntry(e))
}

// delete godoc
// @Summary      Delete a hydration entry
// @Tags         hydration
// @Param        id   path  string  true  "Hydration entry UUID"
// @Success      204  "no content"
// @Failure      404  {object}  map[string]string  "hydration_not_found"
// @Security     BearerAuth
// @Router       /hydration/{id} [delete]
func (h *Handlers) delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusNotFound, "hydration_not_found")
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		if errors.Is(err, ErrNotFound) {
			respondError(c, http.StatusNotFound, "hydration_not_found")
			return
		}
		respondError(c, http.StatusInternalServerError, "delete_failed")
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
	case errors.Is(err, ErrWorkoutNotFound):
		respondError(c, http.StatusBadRequest, "workout_not_found")
	case errors.Is(err, ErrQuantityInvalid):
		respondError(c, http.StatusBadRequest, "quantity_ml_invalid")
	case errors.Is(err, ErrLoggedAtFuture):
		respondError(c, http.StatusBadRequest, "logged_at_too_far_future")
	case errors.Is(err, ErrNoteTooLong):
		respondError(c, http.StatusBadRequest, "note_too_long")
	default:
		respondError(c, http.StatusInternalServerError, "write_failed")
	}
}

func parseLoggedAt(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, errors.New("empty")
	}
	return time.Parse(time.RFC3339, s)
}

// roundEntry returns a copy with quantity_ml rounded to 1dp at response time.
func roundEntry(e *Entry) *Entry {
	if e == nil {
		return nil
	}
	out := *e
	out.QuantityMl = numfmt.Round1(e.QuantityMl)
	return &out
}
