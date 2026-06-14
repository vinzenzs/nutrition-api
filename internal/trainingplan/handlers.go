package trainingplan

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/vinzenzs/kazper/internal/workouts"
)

// Handlers wires the training-plan endpoints.
type Handlers struct {
	svc *Service
}

func NewHandlers(svc *Service) *Handlers {
	return &Handlers{svc: svc}
}

func (h *Handlers) Register(rg *gin.RouterGroup) {
	rg.POST("/training-plans", h.createPlan)
	rg.GET("/training-plans", h.listPlans)
	rg.GET("/training-plans/:id", h.getPlan)
	rg.PATCH("/training-plans/:id", h.patchPlan)
	rg.DELETE("/training-plans/:id", h.deletePlan)

	rg.POST("/training-plans/:id/weeks", h.createWeek)
	rg.PATCH("/training-plans/:id/weeks/:weekId", h.patchWeek)
	rg.DELETE("/training-plans/:id/weeks/:weekId", h.deleteWeek)

	rg.POST("/training-plans/:id/weeks/:weekId/slots", h.createSlot)
	rg.PATCH("/training-plans/:id/slots/:slotId", h.patchSlot)
	rg.DELETE("/training-plans/:id/slots/:slotId", h.deleteSlot)

	rg.POST("/training-plans/:id/materialize", h.materialize)

	rg.GET("/workouts/:id/program", h.workoutProgram)
}

// ----- plan -----

type createPlanRequest struct {
	Name      string     `json:"name"`
	RaceID    *uuid.UUID `json:"race_id,omitempty"`
	StartDate string     `json:"start_date"`
	Notes     *string    `json:"notes,omitempty"`
}

// createPlan godoc
// @Summary      Create a training plan
// @Tags         training-plans
// @Accept       json
// @Produce      json
// @Param        Idempotency-Key  header  string             false  "Optional idempotency key"
// @Param        body             body    createPlanRequest  true   "Plan"
// @Success      201  {object}  Plan
// @Failure      400  {object}  map[string]string  "invalid_json | name_required | start_date_invalid"
// @Security     BearerAuth
// @Router       /training-plans [post]
func (h *Handlers) createPlan(c *gin.Context) {
	var req createPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_json")
		return
	}
	out, err := h.svc.CreatePlan(c.Request.Context(), PlanInput{Name: req.Name, RaceID: req.RaceID, StartDate: req.StartDate, Notes: req.Notes})
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

// listPlans godoc
// @Summary      List training plans
// @Tags         training-plans
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "{ training_plans: [...] }"
// @Security     BearerAuth
// @Router       /training-plans [get]
func (h *Handlers) listPlans(c *gin.Context) {
	out, err := h.svc.ListPlans(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, "list_failed")
		return
	}
	plans := make([]*Plan, 0, len(out))
	plans = append(plans, out...)
	c.JSON(http.StatusOK, gin.H{"training_plans": plans})
}

// getPlan godoc
// @Summary      Get a training plan with its nested weeks and slots
// @Tags         training-plans
// @Produce      json
// @Param        id  path  string  true  "Plan UUID"
// @Success      200  {object}  Plan
// @Failure      404  {object}  map[string]string  "training_plan_not_found"
// @Security     BearerAuth
// @Router       /training-plans/{id} [get]
func (h *Handlers) getPlan(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		respondError(c, http.StatusNotFound, "training_plan_not_found")
		return
	}
	out, err := h.svc.GetPlan(c.Request.Context(), id)
	if err != nil {
		respondNotFoundOr(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// patchPlan godoc
// @Summary      Update a training plan
// @Tags         training-plans
// @Accept       json
// @Produce      json
// @Param        id    path  string  true  "Plan UUID"
// @Param        body  body  map[string]interface{}  true  "Fields to update (race_id / notes accept null to clear)"
// @Success      200  {object}  Plan
// @Failure      404  {object}  map[string]string  "training_plan_not_found"
// @Security     BearerAuth
// @Router       /training-plans/{id} [patch]
func (h *Handlers) patchPlan(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		respondError(c, http.StatusNotFound, "training_plan_not_found")
		return
	}
	var raw map[string]json.RawMessage
	if !bindPatch(c, &raw) {
		return
	}
	var p PlanPatch
	if err := decodeStr(raw, "name", &p.Name); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_json")
		return
	}
	if err := decodeStr(raw, "start_date", &p.StartDate); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_json")
		return
	}
	if v, present := raw["race_id"]; present {
		p.SetRace = true
		if !isNull(v) {
			var u uuid.UUID
			if json.Unmarshal(v, &u) != nil {
				respondError(c, http.StatusBadRequest, "invalid_json")
				return
			}
			p.RaceID = &u
		}
	}
	if v, present := raw["notes"]; present {
		p.SetNotes = true
		if !isNull(v) {
			var sv string
			if json.Unmarshal(v, &sv) != nil {
				respondError(c, http.StatusBadRequest, "invalid_json")
				return
			}
			p.Notes = &sv
		}
	}
	if v, present := raw["methodology"]; present {
		p.SetMethodology = true
		if !isNull(v) {
			var sv string
			if json.Unmarshal(v, &sv) != nil {
				respondError(c, http.StatusBadRequest, "invalid_json")
				return
			}
			p.Methodology = &sv
		}
	}
	out, err := h.svc.PatchPlan(c.Request.Context(), id, p)
	if err != nil {
		respondNotFoundOr(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handlers) deletePlan(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		respondError(c, http.StatusNotFound, "training_plan_not_found")
		return
	}
	if err := h.svc.DeletePlan(c.Request.Context(), id); err != nil {
		respondNotFoundOr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ----- week -----

type createWeekRequest struct {
	Ordinal int        `json:"ordinal"`
	PhaseID *uuid.UUID `json:"phase_id,omitempty"`
	Notes   *string    `json:"notes,omitempty"`
}

// createWeek godoc
// @Summary      Add a week to a plan
// @Tags         training-plans
// @Accept       json
// @Produce      json
// @Param        id    path  string             true  "Plan UUID"
// @Param        body  body  createWeekRequest  true  "Week"
// @Success      201  {object}  PlanWeek
// @Failure      400  {object}  map[string]string  "invalid_json | ordinal_invalid | week_ordinal_taken"
// @Failure      404  {object}  map[string]string  "training_plan_not_found"
// @Security     BearerAuth
// @Router       /training-plans/{id}/weeks [post]
func (h *Handlers) createWeek(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		respondError(c, http.StatusNotFound, "training_plan_not_found")
		return
	}
	var req createWeekRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_json")
		return
	}
	out, err := h.svc.CreateWeek(c.Request.Context(), id, WeekInput{Ordinal: req.Ordinal, PhaseID: req.PhaseID, Notes: req.Notes})
	if err != nil {
		respondNotFoundOr(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *Handlers) patchWeek(c *gin.Context) {
	weekID, ok := parseID(c, "weekId")
	if !ok {
		respondError(c, http.StatusNotFound, "plan_week_not_found")
		return
	}
	var raw map[string]json.RawMessage
	if !bindPatch(c, &raw) {
		return
	}
	var p WeekPatch
	if v, present := raw["ordinal"]; present {
		var n int
		if json.Unmarshal(v, &n) != nil {
			respondError(c, http.StatusBadRequest, "invalid_json")
			return
		}
		p.Ordinal = &n
	}
	if v, present := raw["phase_id"]; present {
		p.SetPhase = true
		if !isNull(v) {
			var u uuid.UUID
			if json.Unmarshal(v, &u) != nil {
				respondError(c, http.StatusBadRequest, "invalid_json")
				return
			}
			p.PhaseID = &u
		}
	}
	if v, present := raw["notes"]; present {
		p.SetNotes = true
		if !isNull(v) {
			var sv string
			if json.Unmarshal(v, &sv) != nil {
				respondError(c, http.StatusBadRequest, "invalid_json")
				return
			}
			p.Notes = &sv
		}
	}
	out, err := h.svc.PatchWeek(c.Request.Context(), weekID, p)
	if err != nil {
		respondNotFoundOr(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handlers) deleteWeek(c *gin.Context) {
	weekID, ok := parseID(c, "weekId")
	if !ok {
		respondError(c, http.StatusNotFound, "plan_week_not_found")
		return
	}
	if err := h.svc.DeleteWeek(c.Request.Context(), weekID); err != nil {
		respondNotFoundOr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ----- slot -----

type createSlotRequest struct {
	Weekday           int                    `json:"weekday"`
	Ordinal           int                    `json:"ordinal"`
	TemplateID        uuid.UUID              `json:"template_id"`
	TimeOfDay         *string                `json:"time_of_day,omitempty"`
	TargetOverrides   []SlotTargetOverride   `json:"target_overrides,omitempty"`
	DurationOverrides []SlotDurationOverride `json:"duration_overrides,omitempty"`
}

// createSlot godoc
// @Summary      Add a slot to a plan week
// @Tags         training-plans
// @Accept       json
// @Produce      json
// @Param        id      path  string             true  "Plan UUID"
// @Param        weekId  path  string             true  "Week UUID"
// @Param        body    body  createSlotRequest  true  "Slot"
// @Success      201  {object}  PlanSlot
// @Failure      400  {object}  map[string]string  "invalid_json | weekday_invalid | template_id_required | time_of_day_invalid | template_not_found | override_intent_invalid | override_intent_duplicate | override_target_invalid | override_duration_invalid"
// @Failure      404  {object}  map[string]string  "plan_week_not_found"
// @Security     BearerAuth
// @Router       /training-plans/{id}/weeks/{weekId}/slots [post]
func (h *Handlers) createSlot(c *gin.Context) {
	weekID, ok := parseID(c, "weekId")
	if !ok {
		respondError(c, http.StatusNotFound, "plan_week_not_found")
		return
	}
	var req createSlotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_json")
		return
	}
	out, err := h.svc.CreateSlot(c.Request.Context(), weekID, SlotInput{Weekday: req.Weekday, Ordinal: req.Ordinal, TemplateID: req.TemplateID, TimeOfDay: req.TimeOfDay, TargetOverrides: req.TargetOverrides, DurationOverrides: req.DurationOverrides})
	if err != nil {
		respondNotFoundOr(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *Handlers) patchSlot(c *gin.Context) {
	slotID, ok := parseID(c, "slotId")
	if !ok {
		respondError(c, http.StatusNotFound, "plan_slot_not_found")
		return
	}
	var raw map[string]json.RawMessage
	if !bindPatch(c, &raw) {
		return
	}
	var p SlotPatch
	if v, present := raw["weekday"]; present {
		var n int
		if json.Unmarshal(v, &n) != nil {
			respondError(c, http.StatusBadRequest, "invalid_json")
			return
		}
		p.Weekday = &n
	}
	if v, present := raw["ordinal"]; present {
		var n int
		if json.Unmarshal(v, &n) != nil {
			respondError(c, http.StatusBadRequest, "invalid_json")
			return
		}
		p.Ordinal = &n
	}
	if v, present := raw["template_id"]; present {
		var u uuid.UUID
		if json.Unmarshal(v, &u) != nil {
			respondError(c, http.StatusBadRequest, "invalid_json")
			return
		}
		p.TemplateID = &u
	}
	if v, present := raw["time_of_day"]; present {
		p.SetTime = true
		if !isNull(v) {
			var sv string
			if json.Unmarshal(v, &sv) != nil {
				respondError(c, http.StatusBadRequest, "invalid_json")
				return
			}
			p.TimeOfDay = &sv
		}
	}
	if v, present := raw["target_overrides"]; present {
		p.SetOverrides = true
		if !isNull(v) {
			if json.Unmarshal(v, &p.TargetOverrides) != nil {
				respondError(c, http.StatusBadRequest, "invalid_json")
				return
			}
		}
	}
	if v, present := raw["duration_overrides"]; present {
		p.SetDurations = true
		if !isNull(v) {
			if json.Unmarshal(v, &p.DurationOverrides) != nil {
				respondError(c, http.StatusBadRequest, "invalid_json")
				return
			}
		}
	}
	out, err := h.svc.PatchSlot(c.Request.Context(), slotID, p)
	if err != nil {
		respondNotFoundOr(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handlers) deleteSlot(c *gin.Context) {
	slotID, ok := parseID(c, "slotId")
	if !ok {
		respondError(c, http.StatusNotFound, "plan_slot_not_found")
		return
	}
	if err := h.svc.DeleteSlot(c.Request.Context(), slotID); err != nil {
		respondNotFoundOr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ----- materialize -----

type materializeRequest struct {
	Scope string  `json:"scope"`
	Week  *int    `json:"week,omitempty"`
	From  *string `json:"from,omitempty"`
	To    *string `json:"to,omitempty"`
}

// materialize godoc
// @Summary      Expand a plan (or a scope of it) into planned workouts
// @Description  Idempotent, slot-keyed. scope: {"scope":"all"} | {"scope":"week","week":5} | {"scope":"range","from":"YYYY-MM-DD","to":"YYYY-MM-DD"}.
// @Tags         training-plans
// @Accept       json
// @Produce      json
// @Param        id    path  string              true  "Plan UUID"
// @Param        body  body  materializeRequest  true  "Scope"
// @Success      200  {object}  map[string]interface{}  "{ workouts: [...] }"
// @Failure      400  {object}  map[string]string  "invalid_json | scope_invalid | start_date_invalid"
// @Failure      404  {object}  map[string]string  "training_plan_not_found"
// @Security     BearerAuth
// @Router       /training-plans/{id}/materialize [post]
func (h *Handlers) materialize(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		respondError(c, http.StatusNotFound, "training_plan_not_found")
		return
	}
	var req materializeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_json")
		return
	}
	out, err := h.svc.Materialize(c.Request.Context(), id, Scope{Kind: req.Scope, Week: req.Week, From: req.From, To: req.To})
	if err != nil {
		respondNotFoundOr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"workouts": out})
}

// ----- workout program -----

// workoutProgram godoc
// @Summary      Get a planned workout's effective program (template steps + slot target/duration overrides)
// @Description  Resolves the workout's template steps with its plan-slot target and duration overrides applied per intent. A workout with no template returns its sport/name and an empty step list.
// @Tags         workouts
// @Produce      json
// @Param        id  path  string  true  "Workout UUID"
// @Success      200  {object}  Program
// @Failure      404  {object}  map[string]string  "workout_not_found"
// @Security     BearerAuth
// @Router       /workouts/{id}/program [get]
func (h *Handlers) workoutProgram(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		respondError(c, http.StatusNotFound, "workout_not_found")
		return
	}
	out, err := h.svc.EffectiveProgram(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, workouts.ErrNotFound) {
			respondError(c, http.StatusNotFound, "workout_not_found")
			return
		}
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// ----- helpers -----

func parseID(c *gin.Context, param string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(param))
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}

// bindPatch reads the raw body into a key→raw map for presence detection.
func bindPatch(c *gin.Context, dst *map[string]json.RawMessage) bool {
	raw, err := c.GetRawData()
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid_json")
		return false
	}
	*dst = map[string]json.RawMessage{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, dst); err != nil {
			respondError(c, http.StatusBadRequest, "invalid_json")
			return false
		}
	}
	return true
}

func decodeStr(raw map[string]json.RawMessage, key string, dst **string) error {
	v, present := raw[key]
	if !present || isNull(v) {
		return nil
	}
	var s string
	if err := json.Unmarshal(v, &s); err != nil {
		return err
	}
	*dst = &s
	return nil
}

func isNull(raw json.RawMessage) bool { return string(raw) == "null" }

func respondError(c *gin.Context, status int, code string) {
	c.JSON(status, gin.H{"error": code})
}

// respondNotFoundOr maps the not-found sentinels to 404 and the validation
// sentinels to 400, falling back to 500.
func respondNotFoundOr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrPlanNotFound):
		respondError(c, http.StatusNotFound, "training_plan_not_found")
	case errors.Is(err, ErrWeekNotFound):
		respondError(c, http.StatusNotFound, "plan_week_not_found")
	case errors.Is(err, ErrSlotNotFound):
		respondError(c, http.StatusNotFound, "plan_slot_not_found")
	default:
		respondServiceError(c, err)
	}
}

func respondServiceError(c *gin.Context, err error) {
	for _, e := range []error{
		ErrNameRequired, ErrStartDateInvalid, ErrOrdinalInvalid, ErrWeekdayInvalid,
		ErrTimeOfDayInvalid, ErrTemplateRequired, ErrScopeInvalid,
		ErrTemplateMissing, ErrWeekOrdinalTaken,
		ErrOverrideIntentInvalid, ErrOverrideDuplicate, ErrOverrideTargetInvalid,
		ErrOverrideDurationInvalid,
	} {
		if errors.Is(err, e) {
			respondError(c, http.StatusBadRequest, e.Error())
			return
		}
	}
	respondError(c, http.StatusInternalServerError, "write_failed")
}
