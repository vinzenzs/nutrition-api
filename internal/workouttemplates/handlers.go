package workouttemplates

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers wires the workout-template endpoints.
type Handlers struct {
	svc *Service
}

func NewHandlers(svc *Service) *Handlers {
	return &Handlers{svc: svc}
}

func (h *Handlers) Register(rg *gin.RouterGroup) {
	rg.POST("/workout-templates", h.create)
	rg.GET("/workout-templates", h.list)
	rg.GET("/workout-templates/:id", h.get)
	rg.PATCH("/workout-templates/:id", h.patch)
	rg.DELETE("/workout-templates/:id", h.delete)
}

// createRequest is the create body. Steps is required and non-empty.
type createRequest struct {
	Sport                string  `json:"sport"`
	Name                 string  `json:"name"`
	Description          *string `json:"description,omitempty"`
	EstimatedDurationSec *int    `json:"estimated_duration_sec,omitempty"`
	Steps                []Step  `json:"steps"`
}

// create godoc
// @Summary      Create a workout template
// @Description  Create a reusable structured session (sport + ordered steps with durations and target zones). `Idempotency-Key` supported.
// @Tags         workout-templates
// @Accept       json
// @Produce      json
// @Param        Idempotency-Key  header  string         false  "Optional client-supplied idempotency key"
// @Param        body             body    createRequest  true   "Workout template"
// @Success      201  {object}  Template
// @Failure      400  {object}  map[string]string  "invalid_json | sport_invalid | name_required | estimated_duration_sec_invalid | steps_empty | step_type_invalid | intent_invalid | duration_invalid | target_invalid | target_range_invalid | repeat_invalid | repeat_nested"
// @Security     BearerAuth
// @Router       /workout-templates [post]
func (h *Handlers) create(c *gin.Context) {
	var req createRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_json")
		return
	}
	t := &Template{
		Sport:                req.Sport,
		Name:                 req.Name,
		Description:          req.Description,
		EstimatedDurationSec: req.EstimatedDurationSec,
		Steps:                req.Steps,
	}
	out, err := h.svc.Create(c.Request.Context(), t)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

// list godoc
// @Summary      List workout templates
// @Tags         workout-templates
// @Produce      json
// @Param        sport  query  string  false  "Filter by sport: run | bike | swim | strength | yoga | mobility | other"
// @Success      200  {object}  map[string]interface{}  "{ workout_templates: [...] }"
// @Failure      400  {object}  map[string]string  "sport_invalid"
// @Security     BearerAuth
// @Router       /workout-templates [get]
func (h *Handlers) list(c *gin.Context) {
	out, err := h.svc.List(c.Request.Context(), c.Query("sport"))
	if err != nil {
		if errors.Is(err, ErrSportInvalid) {
			respondError(c, http.StatusBadRequest, "sport_invalid")
			return
		}
		respondError(c, http.StatusInternalServerError, "list_failed")
		return
	}
	templates := make([]*Template, 0, len(out))
	templates = append(templates, out...)
	c.JSON(http.StatusOK, gin.H{"workout_templates": templates})
}

// get godoc
// @Summary      Get a workout template by id
// @Tags         workout-templates
// @Produce      json
// @Param        id  path  string  true  "Template UUID"
// @Success      200  {object}  Template
// @Failure      404  {object}  map[string]string  "workout_template_not_found"
// @Security     BearerAuth
// @Router       /workout-templates/{id} [get]
func (h *Handlers) get(c *gin.Context) {
	out, err := h.svc.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respondError(c, http.StatusNotFound, "workout_template_not_found")
			return
		}
		respondError(c, http.StatusInternalServerError, "get_failed")
		return
	}
	c.JSON(http.StatusOK, out)
}

// patchRequest uses json.RawMessage per field so present-null can be told apart
// from omitted (design D5): a present `description: null` clears it; omission
// leaves it unchanged.
type patchRequest struct {
	Sport                *string         `json:"sport"`
	Name                 *string         `json:"name"`
	Description          json.RawMessage `json:"description"`
	EstimatedDurationSec json.RawMessage `json:"estimated_duration_sec"`
	Steps                *[]Step         `json:"steps"`
}

// patch godoc
// @Summary      Partially update a workout template
// @Description  Any of sport / name / description / estimated_duration_sec / steps may be supplied; omitted fields are unchanged. A supplied `steps` array replaces the prior steps wholesale.
// @Tags         workout-templates
// @Accept       json
// @Produce      json
// @Param        id    path  string        true  "Template UUID"
// @Param        body  body  patchRequest  true  "Fields to update"
// @Success      200  {object}  Template
// @Failure      400  {object}  map[string]string  "invalid_json | sport_invalid | name_required | estimated_duration_sec_invalid | steps_empty | step_type_invalid | intent_invalid | duration_invalid | target_invalid | target_range_invalid | repeat_invalid | repeat_nested"
// @Failure      404  {object}  map[string]string  "workout_template_not_found"
// @Security     BearerAuth
// @Router       /workout-templates/{id} [patch]
func (h *Handlers) patch(c *gin.Context) {
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

	in := PatchInput{Name: req.Name, Sport: req.Sport, Steps: req.Steps}
	// A present key (even `null`) flips the Set flag; null decodes to a nil value
	// which clears the column.
	if req.Description != nil {
		in.SetDescription = true
		if !isJSONNull(req.Description) {
			var v string
			if err := json.Unmarshal(req.Description, &v); err != nil {
				respondError(c, http.StatusBadRequest, "invalid_json")
				return
			}
			in.Description = &v
		}
	}
	if req.EstimatedDurationSec != nil {
		in.SetEstimated = true
		if !isJSONNull(req.EstimatedDurationSec) {
			var v int
			if err := json.Unmarshal(req.EstimatedDurationSec, &v); err != nil {
				respondError(c, http.StatusBadRequest, "invalid_json")
				return
			}
			in.EstimatedDurationSec = &v
		}
	}

	out, err := h.svc.Patch(c.Request.Context(), c.Param("id"), in)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respondError(c, http.StatusNotFound, "workout_template_not_found")
			return
		}
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// delete godoc
// @Summary      Delete a workout template
// @Tags         workout-templates
// @Param        id  path  string  true  "Template UUID"
// @Success      204  "no content"
// @Failure      404  {object}  map[string]string  "workout_template_not_found"
// @Security     BearerAuth
// @Router       /workout-templates/{id} [delete]
func (h *Handlers) delete(c *gin.Context) {
	if err := h.svc.Delete(c.Request.Context(), c.Param("id")); err != nil {
		if errors.Is(err, ErrNotFound) {
			respondError(c, http.StatusNotFound, "workout_template_not_found")
			return
		}
		if errors.Is(err, ErrInUse) {
			respondError(c, http.StatusConflict, "template_in_use")
			return
		}
		respondError(c, http.StatusInternalServerError, "delete_failed")
		return
	}
	c.Status(http.StatusNoContent)
}

// ----- helpers -----

func isJSONNull(raw json.RawMessage) bool {
	return string(raw) == "null"
}

func respondError(c *gin.Context, status int, code string) {
	c.JSON(status, gin.H{"error": code})
}

func respondServiceError(c *gin.Context, err error) {
	for _, e := range []error{
		ErrSportInvalid, ErrNameRequired, ErrEstimatedInvalid, ErrStepsEmpty,
		ErrStepTypeInvalid, ErrIntentInvalid, ErrDurationInvalid, ErrTargetInvalid,
		ErrTargetRangeInvalid, ErrRepeatInvalid, ErrRepeatNested,
	} {
		if errors.Is(err, e) {
			respondError(c, http.StatusBadRequest, e.Error())
			return
		}
	}
	respondError(c, http.StatusInternalServerError, "write_failed")
}
