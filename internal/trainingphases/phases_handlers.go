package trainingphases

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const dateFormat = "2006-01-02"

// PhasesHandlers wires POST/GET/PATCH/DELETE for /phases.
type PhasesHandlers struct {
	svc *PhasesService
}

func NewPhasesHandlers(svc *PhasesService) *PhasesHandlers {
	return &PhasesHandlers{svc: svc}
}

func (h *PhasesHandlers) Register(rg *gin.RouterGroup) {
	rg.POST("/phases", h.create)
	rg.GET("/phases", h.list)
	rg.GET("/phases/:id", h.get)
	rg.PATCH("/phases/:id", h.patch)
	rg.DELETE("/phases/:id", h.delete)
}

// createRequest is the JSON shape POST /phases accepts. Date fields are
// YYYY-MM-DD strings.
type createRequest struct {
	Name              string  `json:"name"`
	Type              string  `json:"type"`
	StartDate         string  `json:"start_date"`
	EndDate           string  `json:"end_date"`
	DefaultTemplateID *string `json:"default_template_id,omitempty"`
	Notes             *string `json:"notes,omitempty"`
}

// create godoc
// @Summary      Create a training phase
// @Description  A phase is a named date range tagged with a training type. Optionally points at a goal template via default_template_id, which becomes the default daily goals for every date in [start_date, end_date]. Per-date overrides still win.
// @Tags         training-phases
// @Accept       json
// @Produce      json
// @Param        body  body  createRequest  true  "Phase fields"
// @Success      201   {object}  map[string]interface{}  "{\"phase\": Phase}"
// @Failure      400   {object}  map[string]interface{}  "phase_name_invalid | phase_name_too_long | phase_type_invalid | date_range_invalid | date_invalid | invalid_json | template_not_found"
// @Security     BearerAuth
// @Router       /phases [post]
func (h *PhasesHandlers) create(c *gin.Context) {
	raw, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
		return
	}
	var req createRequest
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
		return
	}
	startDate, err := time.Parse(dateFormat, req.StartDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date_invalid", "field": "start_date"})
		return
	}
	endDate, err := time.Parse(dateFormat, req.EndDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date_invalid", "field": "end_date"})
		return
	}
	in := CreateInput{
		Name:      req.Name,
		Type:      PhaseType(req.Type),
		StartDate: startDate,
		EndDate:   endDate,
		Notes:     req.Notes,
	}
	if req.DefaultTemplateID != nil && *req.DefaultTemplateID != "" {
		tid, err := uuid.Parse(*req.DefaultTemplateID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "default_template_id_invalid"})
			return
		}
		in.DefaultTemplateID = &tid
	}
	p, err := h.svc.Create(c.Request.Context(), in)
	if err != nil {
		respondPhaseError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"phase": p})
}

// get godoc
// @Summary      Get a training phase by id
// @Tags         training-phases
// @Produce      json
// @Param        id  path  string  true  "Phase id (UUID)"
// @Success      200   {object}  map[string]interface{}  "{\"phase\": Phase}"
// @Failure      400   {object}  map[string]string  "phase_id_invalid"
// @Failure      404   {object}  map[string]string  "phase_not_found"
// @Security     BearerAuth
// @Router       /phases/{id} [get]
func (h *PhasesHandlers) get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "phase_id_invalid"})
		return
	}
	p, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		respondPhaseError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"phase": p})
}

// list godoc
// @Summary      List training phases intersecting a date window
// @Tags         training-phases
// @Produce      json
// @Param        from  query  string  true   "Inclusive start date YYYY-MM-DD"
// @Param        to    query  string  true   "Inclusive end date YYYY-MM-DD; max 730 days from from"
// @Success      200   {object}  map[string]interface{}  "{\"phases\": [Phase]}"
// @Failure      400   {object}  map[string]interface{}  "range_required | date_invalid | range_invalid | range_too_large"
// @Security     BearerAuth
// @Router       /phases [get]
func (h *PhasesHandlers) list(c *gin.Context) {
	const maxRangeDays = 730
	fromStr := c.Query("from")
	toStr := c.Query("to")
	if fromStr == "" || toStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "range_required"})
		return
	}
	from, err := time.Parse(dateFormat, fromStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date_invalid", "field": "from"})
		return
	}
	to, err := time.Parse(dateFormat, toStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date_invalid", "field": "to"})
		return
	}
	if from.After(to) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "range_invalid"})
		return
	}
	if days := int(to.Sub(from).Hours()/24) + 1; days > maxRangeDays {
		c.JSON(http.StatusBadRequest, gin.H{"error": "range_too_large", "max_days": maxRangeDays})
		return
	}
	ps, err := h.svc.ListIntersecting(c.Request.Context(), from, to)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "phase_list_failed"})
		return
	}
	if ps == nil {
		ps = []*Phase{}
	}
	c.JSON(http.StatusOK, gin.H{"phases": ps})
}

// patchRequest is the JSON shape PATCH /phases/{id} accepts. Tri-state for
// default_template_id: missing = leave unchanged, empty string = clear,
// UUID string = set. Mirrors the meal/hydration workout_id convention from
// add-meal-workout-link.
type patchRequest struct {
	Name              *string `json:"name,omitempty"`
	Type              *string `json:"type,omitempty"`
	StartDate         *string `json:"start_date,omitempty"`
	EndDate           *string `json:"end_date,omitempty"`
	DefaultTemplateID *string `json:"default_template_id,omitempty"`
	Notes             *string `json:"notes,omitempty"`
}

// patch godoc
// @Summary      Partially update a training phase
// @Description  Tri-state on default_template_id: empty string clears, UUID sets, missing leaves unchanged.
// @Tags         training-phases
// @Accept       json
// @Produce      json
// @Param        id    path  string  true  "Phase id"
// @Param        body  body  patchRequest  true  "Fields to update"
// @Success      200   {object}  map[string]interface{}  "{\"phase\": Phase}"
// @Failure      400   {object}  map[string]interface{}  "phase_id_invalid | invalid_json | patch_empty | phase_name_invalid | phase_name_too_long | phase_type_invalid | date_invalid | date_range_invalid | default_template_id_invalid | template_not_found"
// @Failure      404   {object}  map[string]string  "phase_not_found"
// @Security     BearerAuth
// @Router       /phases/{id} [patch]
func (h *PhasesHandlers) patch(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "phase_id_invalid"})
		return
	}
	raw, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
		return
	}
	var req patchRequest
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
		return
	}
	p := PatchParams{
		Name:  req.Name,
		Notes: req.Notes,
	}
	if req.Type != nil {
		t := PhaseType(*req.Type)
		p.Type = &t
	}
	if req.StartDate != nil {
		d, err := time.Parse(dateFormat, *req.StartDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "date_invalid", "field": "start_date"})
			return
		}
		p.StartDate = &d
	}
	if req.EndDate != nil {
		d, err := time.Parse(dateFormat, *req.EndDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "date_invalid", "field": "end_date"})
			return
		}
		p.EndDate = &d
	}
	if req.DefaultTemplateID != nil {
		if *req.DefaultTemplateID == "" {
			p.ClearDefaultTemplateID = true
		} else {
			tid, err := uuid.Parse(*req.DefaultTemplateID)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "default_template_id_invalid"})
				return
			}
			p.DefaultTemplateID = &tid
		}
	}
	out, err := h.svc.Patch(c.Request.Context(), id, p)
	if err != nil {
		respondPhaseError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"phase": out})
}

// delete godoc
// @Summary      Delete a training phase
// @Tags         training-phases
// @Param        id  path  string  true  "Phase id"
// @Success      204   "no content"
// @Failure      400   {object}  map[string]string  "phase_id_invalid"
// @Failure      404   {object}  map[string]string  "phase_not_found"
// @Security     BearerAuth
// @Router       /phases/{id} [delete]
func (h *PhasesHandlers) delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "phase_id_invalid"})
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		respondPhaseError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func respondPhaseError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrPhaseNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "phase_not_found"})
	case errors.Is(err, ErrPhaseNameInvalid):
		c.JSON(http.StatusBadRequest, gin.H{"error": "phase_name_invalid"})
	case errors.Is(err, ErrPhaseNameTooLong):
		c.JSON(http.StatusBadRequest, gin.H{"error": "phase_name_too_long", "max_length": MaxPhaseNameLength})
	case errors.Is(err, ErrPhaseTypeInvalid):
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "phase_type_invalid",
			"allowed": AllowedPhaseTypeStrings(),
		})
	case errors.Is(err, ErrDateRangeInvalid):
		c.JSON(http.StatusBadRequest, gin.H{"error": "date_range_invalid"})
	case errors.Is(err, ErrPatchEmpty):
		c.JSON(http.StatusBadRequest, gin.H{"error": "patch_empty"})
	case errors.Is(err, ErrTemplateNotFound):
		c.JSON(http.StatusBadRequest, gin.H{"error": "template_not_found"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "phase_write_failed"})
	}
}
