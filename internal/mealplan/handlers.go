package mealplan

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Handlers wires planned-meal endpoints onto a Gin router group.
type Handlers struct {
	svc *Service
}

func NewHandlers(svc *Service) *Handlers {
	return &Handlers{svc: svc}
}

func (h *Handlers) Register(rg *gin.RouterGroup) {
	rg.POST("/plan", h.create)
	rg.GET("/plan", h.list)
	rg.GET("/plan/:id", h.get)
	rg.PATCH("/plan/:id", h.patch)
	rg.DELETE("/plan/:id", h.delete)
	rg.POST("/plan/:id/eaten", h.eaten)
}

type createRequest struct {
	PlanDate  string   `json:"plan_date"`
	Slot      string   `json:"slot"`
	ProductID string   `json:"product_id"`
	QuantityG *float64 `json:"quantity_g,omitempty"`
	Notes     *string  `json:"notes,omitempty"`
}

// create godoc
// @Summary      Create a planned meal
// @Description  Persists a meal *selection* for a date+slot. `slot` is one of breakfast|lunch|dinner|snack; `product_id` is required and must reference an existing product. The plan is NOT a logged meal — it enters meal history only via POST /plan/{id}/eaten. Two planned meals may share a date+slot.
// @Tags         meal-plan
// @Accept       json
// @Produce      json
// @Param        Idempotency-Key  header  string         false  "Optional client-supplied idempotency key"
// @Param        body             body    createRequest  true   "Planned meal"
// @Success      201  {object}  PlannedMeal
// @Failure      400  {object}  map[string]string  "slot_invalid | plan_date_invalid | product_id_required | product_id_invalid | quantity_g_invalid"
// @Failure      404  {object}  map[string]string  "product_not_found"
// @Security     BearerAuth
// @Router       /plan [post]
func (h *Handlers) create(c *gin.Context) {
	var req createRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_json")
		return
	}
	if req.ProductID == "" {
		respondError(c, http.StatusBadRequest, "product_id_required")
		return
	}
	pid, err := uuid.Parse(req.ProductID)
	if err != nil {
		respondError(c, http.StatusBadRequest, "product_id_invalid")
		return
	}
	pm, err := h.svc.Create(c.Request.Context(), CreateInput{
		PlanDate:  req.PlanDate,
		Slot:      req.Slot,
		ProductID: pid,
		QuantityG: req.QuantityG,
		Notes:     req.Notes,
	})
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, pm)
}

// list godoc
// @Summary      List planned meals in a date range
// @Description  Returns planned meals with plan_date in [from, to] inclusive, ordered by date then slot (breakfast→snack), each carrying its product name.
// @Tags         meal-plan
// @Produce      json
// @Param        from  query  string  true  "Inclusive lower bound YYYY-MM-DD"
// @Param        to    query  string  true  "Inclusive upper bound YYYY-MM-DD"
// @Success      200  {object}  map[string]interface{}  "{ planned_meals: [...] }"
// @Failure      400  {object}  map[string]string  "range_required | plan_date_invalid"
// @Security     BearerAuth
// @Router       /plan [get]
func (h *Handlers) list(c *gin.Context) {
	from := c.Query("from")
	to := c.Query("to")
	if from == "" || to == "" {
		respondError(c, http.StatusBadRequest, "range_required")
		return
	}
	out, err := h.svc.ListRange(c.Request.Context(), from, to)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	if out == nil {
		out = []*PlannedMeal{}
	}
	c.JSON(http.StatusOK, gin.H{"planned_meals": out})
}

// get godoc
// @Summary      Get a planned meal
// @Tags         meal-plan
// @Produce      json
// @Param        id  path  string  true  "Planned meal UUID"
// @Success      200  {object}  PlannedMeal
// @Failure      404  {object}  map[string]string  "planned_meal_not_found"
// @Security     BearerAuth
// @Router       /plan/{id} [get]
func (h *Handlers) get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusNotFound, "planned_meal_not_found")
		return
	}
	pm, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, pm)
}

// patch godoc
// @Summary      Update a planned meal
// @Description  Partial update. `status` may move planned↔skipped only; eaten is terminal here (use POST /plan/{id}/eaten to log). `quantity_g` and `notes` accept JSON null to clear.
// @Tags         meal-plan
// @Accept       json
// @Produce      json
// @Param        id    path  string                 true  "Planned meal UUID"
// @Param        body  body  map[string]interface{}  true  "Fields to update"
// @Success      200  {object}  PlannedMeal
// @Failure      400  {object}  map[string]string  "slot_invalid | status_invalid | plan_date_invalid | quantity_g_invalid | product_id_invalid | plan_entry_eaten_via_endpoint_only"
// @Failure      404  {object}  map[string]string  "planned_meal_not_found | product_not_found"
// @Failure      409  {object}  map[string]string  "plan_entry_eaten_terminal"
// @Security     BearerAuth
// @Router       /plan/{id} [patch]
func (h *Handlers) patch(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusNotFound, "planned_meal_not_found")
		return
	}
	var fields map[string]json.RawMessage
	if err := c.ShouldBindJSON(&fields); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_json")
		return
	}
	in, code := buildUpdateInput(fields)
	if code != "" {
		respondError(c, http.StatusBadRequest, code)
		return
	}
	pm, err := h.svc.Update(c.Request.Context(), id, in)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, pm)
}

// delete godoc
// @Summary      Delete a planned meal
// @Tags         meal-plan
// @Param        id  path  string  true  "Planned meal UUID"
// @Success      204  "no content"
// @Failure      404  {object}  map[string]string  "planned_meal_not_found"
// @Security     BearerAuth
// @Router       /plan/{id} [delete]
func (h *Handlers) delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusNotFound, "planned_meal_not_found")
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		respondServiceError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

type eatenRequest struct {
	QuantityG *float64 `json:"quantity_g,omitempty"`
	LoggedAt  *string  `json:"logged_at,omitempty"`
}

// eaten godoc
// @Summary      Mark a planned meal eaten (logs a real meal entry)
// @Description  Atomically creates a meal entry through the meals capability (product, effective quantity = body override → plan quantity_g → product serving → 100 g, logged_at = now unless overridden and never in the future, meal_type = slot) and flips the plan entry to `eaten`, storing the new meal entry id. This is the ONLY path that turns a plan into meal history. If the meal create fails the plan stays planned.
// @Tags         meal-plan
// @Accept       json
// @Produce      json
// @Param        Idempotency-Key  header  string        false  "Optional client-supplied idempotency key"
// @Param        id               path    string        true   "Planned meal UUID"
// @Param        body             body    eatenRequest  false  "Optional corrections"
// @Success      200  {object}  map[string]interface{}  "{ plan: PlannedMeal, meal: MealEntry }"
// @Failure      400  {object}  map[string]string  "logged_at_invalid | logged_at_too_far_future | quantity_g_invalid"
// @Failure      404  {object}  map[string]string  "planned_meal_not_found"
// @Failure      409  {object}  map[string]string  "plan_entry_already_eaten | plan_entry_not_planned"
// @Security     BearerAuth
// @Router       /plan/{id}/eaten [post]
func (h *Handlers) eaten(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusNotFound, "planned_meal_not_found")
		return
	}
	var req eatenRequest
	// Body is optional; tolerate an empty body.
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			respondError(c, http.StatusBadRequest, "invalid_json")
			return
		}
	}
	in := EatenInput{QuantityG: req.QuantityG}
	if req.LoggedAt != nil {
		ts, err := time.Parse(time.RFC3339, *req.LoggedAt)
		if err != nil {
			respondError(c, http.StatusBadRequest, "logged_at_invalid")
			return
		}
		in.LoggedAt = &ts
	}
	res, err := h.svc.MarkEaten(c.Request.Context(), id, in)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"plan": res.Plan, "meal": res.Meal})
}

// ----- helpers -----

func buildUpdateInput(fields map[string]json.RawMessage) (UpdateInput, string) {
	var in UpdateInput
	isNull := func(raw json.RawMessage) bool {
		return string(bytes.TrimSpace(raw)) == "null"
	}
	str := func(raw json.RawMessage) (string, bool) {
		var s string
		return s, json.Unmarshal(raw, &s) == nil
	}
	if v, ok := fields["plan_date"]; ok && !isNull(v) {
		s, ok := str(v)
		if !ok {
			return in, "plan_date_invalid"
		}
		in.PlanDate = &s
	}
	if v, ok := fields["slot"]; ok && !isNull(v) {
		s, ok := str(v)
		if !ok {
			return in, "slot_invalid"
		}
		in.Slot = &s
	}
	if v, ok := fields["product_id"]; ok && !isNull(v) {
		s, ok := str(v)
		if !ok {
			return in, "product_id_invalid"
		}
		pid, err := uuid.Parse(s)
		if err != nil {
			return in, "product_id_invalid"
		}
		in.ProductID = &pid
	}
	if v, ok := fields["status"]; ok && !isNull(v) {
		s, ok := str(v)
		if !ok {
			return in, "status_invalid"
		}
		in.Status = &s
	}
	if v, ok := fields["quantity_g"]; ok {
		if isNull(v) {
			in.ClearQuantityG = true
		} else {
			var f float64
			if err := json.Unmarshal(v, &f); err != nil {
				return in, "quantity_g_invalid"
			}
			in.QuantityG = &f
		}
	}
	if v, ok := fields["notes"]; ok {
		if isNull(v) {
			in.ClearNotes = true
		} else {
			s, ok := str(v)
			if !ok {
				return in, "invalid_json"
			}
			in.Notes = &s
		}
	}
	return in, ""
}

func respondError(c *gin.Context, status int, code string) {
	c.JSON(status, gin.H{"error": code})
}

func respondServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		respondError(c, http.StatusNotFound, "planned_meal_not_found")
	case errors.Is(err, ErrProductNotFound):
		respondError(c, http.StatusNotFound, "product_not_found")
	case errors.Is(err, ErrSlotInvalid):
		respondError(c, http.StatusBadRequest, "slot_invalid")
	case errors.Is(err, ErrStatusInvalid):
		respondError(c, http.StatusBadRequest, "status_invalid")
	case errors.Is(err, ErrPlanDateInvalid):
		respondError(c, http.StatusBadRequest, "plan_date_invalid")
	case errors.Is(err, ErrProductIDRequired):
		respondError(c, http.StatusBadRequest, "product_id_required")
	case errors.Is(err, ErrQuantityInvalid):
		respondError(c, http.StatusBadRequest, "quantity_g_invalid")
	case errors.Is(err, ErrLoggedAtFuture):
		respondError(c, http.StatusBadRequest, "logged_at_too_far_future")
	case errors.Is(err, ErrAlreadyEaten):
		respondError(c, http.StatusConflict, "plan_entry_already_eaten")
	case errors.Is(err, ErrNotPlanned):
		respondError(c, http.StatusConflict, "plan_entry_not_planned")
	case errors.Is(err, ErrEatenTerminal):
		respondError(c, http.StatusConflict, "plan_entry_eaten_terminal")
	case errors.Is(err, ErrEatenViaEndpoint):
		respondError(c, http.StatusBadRequest, "plan_entry_eaten_via_endpoint_only")
	default:
		respondError(c, http.StatusInternalServerError, "write_failed")
	}
}
