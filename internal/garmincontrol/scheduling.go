package garmincontrol

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/vinzenzs/kazper/internal/trainingplan"
	"github.com/vinzenzs/kazper/internal/workouts"
	"github.com/vinzenzs/kazper/internal/workouttemplates"
)

// Dependency interfaces — narrow views of the repos/service the scheduling
// orchestration needs, so tests can stub them without a database.
type workoutsRepo interface {
	GetByID(ctx context.Context, id uuid.UUID) (*workouts.Workout, error)
	SetGarminIDs(ctx context.Context, id uuid.UUID, garminWorkoutID, garminScheduleID *string) error
	CreateAdhocPlannedFromTemplate(ctx context.Context, in workouts.AdhocPlannedInput) (*workouts.Workout, error)
}

type templatesRepo interface {
	GetByID(ctx context.Context, id string) (*workouttemplates.Template, error)
}

type planService interface {
	PlannedWorkoutsInScope(ctx context.Context, planID uuid.UUID, scope trainingplan.Scope) ([]uuid.UUID, error)
	// EffectiveProgram resolves a planned workout's template steps with its slot
	// target overrides applied — what the watch compile must build from.
	EffectiveProgram(ctx context.Context, workoutID uuid.UUID) (*trainingplan.Program, error)
}

// Orchestration sentinels.
var (
	errWorkoutNotFound  = errors.New("workout_not_found")
	errNotSchedulable   = errors.New("workout_not_schedulable") // not planned or no template
	errNotScheduled     = errors.New("workout_not_scheduled")   // no stored schedule id
	errTemplateNotFound = errors.New("template_not_found")      // ad-hoc schedule of an unknown template
)

// adhocFallbackDuration is the session length used when a template's program has
// no time-based durations to sum (e.g. rep-based strength/mobility work).
const adhocFallbackDuration = 60 * time.Minute

// ----- handlers -----

type scheduleWorkoutRequest struct {
	WorkoutID string `json:"workout_id"`
}

// scheduleWorkout godoc
// @Summary      Push a planned workout to the Garmin watch
// @Description  Compiles the workout's template into a structured Garmin workout (via the bridge), schedules it on the workout's date, and stores the returned Garmin ids. Re-pushing unschedules the prior entry first. Requires a planned workout with a template_id.
// @Tags         garmin
// @Accept       json
// @Produce      json
// @Param        body  body  scheduleWorkoutRequest  true  "{ workout_id }"
// @Success      200  {object}  map[string]interface{}  "the updated workout"
// @Failure      400  {object}  map[string]string  "invalid_json | workout_not_schedulable"
// @Failure      404  {object}  map[string]string  "workout_not_found"
// @Failure      502  {object}  map[string]string  "garmin_bridge_unreachable | garmin_error"
// @Failure      503  {object}  map[string]string  "garmin_disabled"
// @Security     BearerAuth
// @Router       /garmin/schedule/workout [post]
func (h *Handlers) scheduleWorkout(c *gin.Context) {
	if !h.enabled() {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "garmin_disabled"})
		return
	}
	var req scheduleWorkoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
		return
	}
	id, err := uuid.Parse(req.WorkoutID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "workout_not_found"})
		return
	}
	w, err := h.pushOne(c.Request.Context(), id)
	if err != nil {
		h.respondScheduleErr(c, err)
		return
	}
	c.JSON(http.StatusOK, w)
}

type scheduleTemplateRequest struct {
	TemplateID string `json:"template_id"`
	Date       string `json:"date"`
}

// scheduleTemplate godoc
// @Summary      Schedule a standalone template to a date on the Garmin watch
// @Description  Creates an ad-hoc planned workout from a template (source=manual, status=planned, no plan slot, started_at=date, ended_at=date + the template's summed timed-step duration, falling back to 60 minutes), then compiles and schedules it via the bridge and stores the returned Garmin ids — the server-side replacement for `garmin.py schedule-yoga`. Unschedule via DELETE /garmin/schedule/workout/{id} on the returned workout. Works for any sport the bridge accepts (notably yoga and mobility).
// @Tags         garmin
// @Accept       json
// @Produce      json
// @Param        body  body  scheduleTemplateRequest  true  "{ template_id, date }"
// @Success      200  {object}  map[string]interface{}  "the created workout"
// @Failure      400  {object}  map[string]string  "invalid_json | date_invalid"
// @Failure      404  {object}  map[string]string  "template_not_found"
// @Failure      502  {object}  map[string]string  "garmin_bridge_unreachable | garmin_error"
// @Failure      503  {object}  map[string]string  "garmin_disabled"
// @Security     BearerAuth
// @Router       /garmin/schedule/template [post]
func (h *Handlers) scheduleTemplate(c *gin.Context) {
	if !h.enabled() {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "garmin_disabled"})
		return
	}
	var req scheduleTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
		return
	}
	tmplID, err := uuid.Parse(req.TemplateID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "template_not_found"})
		return
	}
	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date_invalid"})
		return
	}
	w, err := h.scheduleTemplateOne(c.Request.Context(), tmplID, date)
	if err != nil {
		h.respondScheduleErr(c, err)
		return
	}
	c.JSON(http.StatusOK, w)
}

// scheduleTemplateOne creates an ad-hoc planned workout from a template on the
// given date, then delegates to pushOne to compile/schedule/track it. Because the
// row carries no plan_slot_id, EffectiveProgram compiles the raw template steps.
func (h *Handlers) scheduleTemplateOne(ctx context.Context, tmplID uuid.UUID, date time.Time) (*workouts.Workout, error) {
	tmpl, err := h.templatesRepo.GetByID(ctx, tmplID.String())
	if err != nil {
		return nil, errTemplateNotFound
	}
	dur := time.Duration(workouttemplates.SumTimedDurationSec(tmpl.Steps)) * time.Second
	if dur <= 0 {
		dur = adhocFallbackDuration
	}
	var name *string
	if tmpl.Name != "" {
		n := tmpl.Name
		name = &n
	}
	w, err := h.workoutsRepo.CreateAdhocPlannedFromTemplate(ctx, workouts.AdhocPlannedInput{
		TemplateID: tmplID,
		Sport:      tmpl.Sport,
		Name:       name,
		StartedAt:  date,
		EndedAt:    date.Add(dur),
	})
	if err != nil {
		return nil, err
	}
	return h.pushOne(ctx, w.ID)
}

// unscheduleWorkout godoc
// @Summary      Remove a workout from the Garmin calendar
// @Description  Deletes the scheduled calendar entry AND the structured workout object via the bridge (closing the library-orphan leak), then clears both stored Garmin ids. No-op success when the workout was never scheduled.
// @Tags         garmin
// @Produce      json
// @Param        id  path  string  true  "Workout UUID"
// @Success      200  {object}  map[string]interface{}  "the updated workout"
// @Failure      404  {object}  map[string]string  "workout_not_found"
// @Failure      502  {object}  map[string]string  "garmin_bridge_unreachable | garmin_error"
// @Failure      503  {object}  map[string]string  "garmin_disabled"
// @Security     BearerAuth
// @Router       /garmin/schedule/workout/{id} [delete]
func (h *Handlers) unscheduleWorkout(c *gin.Context) {
	if !h.enabled() {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "garmin_disabled"})
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "workout_not_found"})
		return
	}
	ctx := c.Request.Context()
	w, err := h.workoutsRepo.GetByID(ctx, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "workout_not_found"})
		return
	}
	if w.GarminScheduleID == nil {
		// Nothing scheduled — idempotent no-op success.
		c.JSON(http.StatusOK, gin.H{"unscheduled": false, "workout": w})
		return
	}
	if err := h.bridgeUnschedule(ctx, *w.GarminScheduleID); err != nil {
		h.respondScheduleErr(c, err)
		return
	}
	// Reap the workout object too, so it doesn't orphan in the library. A
	// failed object delete leaves both ids intact so a retry re-attempts it
	// (the prior unschedule's already-gone entry is a no-op).
	if w.GarminWorkoutID != nil {
		if err := h.bridgeDeleteWorkout(ctx, *w.GarminWorkoutID); err != nil {
			h.respondScheduleErr(c, err)
			return
		}
	}
	if err := h.workoutsRepo.SetGarminIDs(ctx, id, nil, nil); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update_failed"})
		return
	}
	updated, _ := h.workoutsRepo.GetByID(ctx, id)
	c.JSON(http.StatusOK, gin.H{"unscheduled": true, "workout": updated})
}

type schedulePlanRequest struct {
	PlanID string  `json:"plan_id"`
	Scope  string  `json:"scope"`
	Week   *int    `json:"week,omitempty"`
	From   *string `json:"from,omitempty"`
	To     *string `json:"to,omitempty"`
}

// schedulePlan godoc
// @Summary      Push every planned workout in a plan scope to the watch
// @Description  Resolves the planned workouts in the scope (all | week | range) and pushes each via the single-workout path. Per-workout failures are collected, not fatal.
// @Tags         garmin
// @Accept       json
// @Produce      json
// @Param        body  body  schedulePlanRequest  true  "{ plan_id, scope, week?, from?, to? }"
// @Success      200  {object}  map[string]interface{}  "{ results: [{workout_id, ok, error?}] }"
// @Failure      400  {object}  map[string]string  "invalid_json | scope_invalid"
// @Failure      404  {object}  map[string]string  "training_plan_not_found"
// @Failure      503  {object}  map[string]string  "garmin_disabled"
// @Security     BearerAuth
// @Router       /garmin/schedule/plan [post]
func (h *Handlers) schedulePlan(c *gin.Context) {
	if !h.enabled() {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "garmin_disabled"})
		return
	}
	var req schedulePlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
		return
	}
	planID, err := uuid.Parse(req.PlanID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "training_plan_not_found"})
		return
	}
	ctx := c.Request.Context()
	ids, err := h.planSvc.PlannedWorkoutsInScope(ctx, planID, trainingplan.Scope{Kind: req.Scope, Week: req.Week, From: req.From, To: req.To})
	if err != nil {
		if errors.Is(err, trainingplan.ErrScopeInvalid) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "scope_invalid"})
			return
		}
		if errors.Is(err, trainingplan.ErrPlanNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "training_plan_not_found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "scope_resolution_failed"})
		return
	}
	// Loop the single-workout path; collect per-item results (partial success).
	results := make([]gin.H, 0, len(ids))
	for _, id := range ids {
		if _, perr := h.pushOne(ctx, id); perr != nil {
			results = append(results, gin.H{"workout_id": id, "ok": false, "error": scheduleErrCode(perr)})
		} else {
			results = append(results, gin.H{"workout_id": id, "ok": true})
		}
	}
	c.JSON(http.StatusOK, gin.H{"results": results})
}

// calendar godoc
// @Summary      Read the Garmin calendar through the bridge
// @Tags         garmin
// @Produce      json
// @Param        from  query  string  true  "Inclusive lower bound YYYY-MM-DD"
// @Param        to    query  string  true  "Inclusive upper bound YYYY-MM-DD"
// @Success      200  {object}  map[string]interface{}  "the bridge calendar response verbatim"
// @Failure      502  {object}  map[string]string  "garmin_bridge_unreachable"
// @Failure      503  {object}  map[string]string  "garmin_disabled"
// @Security     BearerAuth
// @Router       /garmin/calendar [get]
func (h *Handlers) calendar(c *gin.Context) {
	if !h.enabled() {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "garmin_disabled"})
		return
	}
	q := url.Values{}
	q.Set("from", c.Query("from"))
	q.Set("to", c.Query("to"))
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, h.bridgeURL+"/calendar?"+q.Encode(), nil)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "garmin_bridge_unreachable"})
		return
	}
	resp, err := h.client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "garmin_bridge_unreachable"})
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "garmin_bridge_unreachable"})
		return
	}
	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = "application/json"
	}
	c.Data(resp.StatusCode, ct, body)
}

// ----- orchestration -----

// pushOne compiles, (re)schedules, and tracks ids for one planned workout.
func (h *Handlers) pushOne(ctx context.Context, id uuid.UUID) (*workouts.Workout, error) {
	w, err := h.workoutsRepo.GetByID(ctx, id)
	if err != nil {
		return nil, errWorkoutNotFound
	}
	if string(w.Status) != string(workouts.StatusPlanned) || w.TemplateID == nil {
		return nil, errNotSchedulable
	}
	tmpl, err := h.templatesRepo.GetByID(ctx, w.TemplateID.String())
	if err != nil {
		return nil, errNotSchedulable
	}
	// Compile from the workout's EFFECTIVE program — the template steps with the
	// plan slot's per-intent target overrides applied — not the raw template.
	prog, err := h.planSvc.EffectiveProgram(ctx, id)
	if err != nil {
		return nil, errNotSchedulable
	}
	// Re-push: remove the prior calendar entry first so no orphan is left.
	if w.GarminScheduleID != nil {
		if err := h.bridgeUnschedule(ctx, *w.GarminScheduleID); err != nil {
			return nil, err
		}
	}
	// And delete the prior workout OBJECT — unscheduling only removes the
	// calendar entry; without this the object orphans in the Garmin library on
	// every re-push. Order: unschedule old entry → delete old object → create.
	if w.GarminWorkoutID != nil {
		if err := h.bridgeDeleteWorkout(ctx, *w.GarminWorkoutID); err != nil {
			return nil, err
		}
	}
	garminWorkoutID, err := h.bridgeCreateWorkout(ctx, tmpl.Sport, tmpl.Name, prog.Steps)
	if err != nil {
		return nil, err
	}
	date := w.StartedAt.Format("2006-01-02")
	scheduleID, err := h.bridgeSchedule(ctx, garminWorkoutID, date)
	if err != nil {
		return nil, err
	}
	if err := h.workoutsRepo.SetGarminIDs(ctx, id, &garminWorkoutID, &scheduleID); err != nil {
		return nil, err
	}
	return h.workoutsRepo.GetByID(ctx, id)
}

// ----- bridge client -----

func (h *Handlers) bridgeCreateWorkout(ctx context.Context, sport, name string, steps []workouttemplates.Step) (string, error) {
	body, _ := json.Marshal(map[string]any{"sport": sport, "name": name, "steps": steps})
	var out struct {
		GarminWorkoutID string `json:"garmin_workout_id"`
	}
	if err := h.bridgeJSON(ctx, http.MethodPost, "/workouts", body, &out); err != nil {
		return "", err
	}
	return out.GarminWorkoutID, nil
}

func (h *Handlers) bridgeSchedule(ctx context.Context, garminWorkoutID, date string) (string, error) {
	body, _ := json.Marshal(map[string]any{"garmin_workout_id": garminWorkoutID, "date": date})
	var out struct {
		GarminScheduleID string `json:"garmin_schedule_id"`
	}
	if err := h.bridgeJSON(ctx, http.MethodPost, "/schedule", body, &out); err != nil {
		return "", err
	}
	return out.GarminScheduleID, nil
}

func (h *Handlers) bridgeUnschedule(ctx context.Context, scheduleID string) error {
	return h.bridgeJSON(ctx, http.MethodDelete, "/schedule?schedule_id="+url.QueryEscape(scheduleID), nil, nil)
}

// bridgeDeleteWorkout deletes a structured workout OBJECT from the Garmin
// library by its id. Idempotent: the bridge returns 2xx for an already-absent
// object (404 → no-op), so re-push and unschedule reaps stay safe to retry.
func (h *Handlers) bridgeDeleteWorkout(ctx context.Context, garminWorkoutID string) error {
	return h.bridgeJSON(ctx, http.MethodDelete, "/workouts/"+url.PathEscape(garminWorkoutID), nil, nil)
}

// bridgeJSON issues a request to the bridge and decodes a JSON response into out
// (when non-nil). A transport failure is errGarminBridgeUnreachable; a non-2xx
// is errGarminError.
func (h *Handlers) bridgeJSON(ctx context.Context, method, path string, body []byte, out any) error {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, h.bridgeURL+path, reader)
	if err != nil {
		return errGarminBridgeUnreachable
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := h.client.Do(req)
	if err != nil {
		return errGarminBridgeUnreachable
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%w: bridge %d", errGarminError, resp.StatusCode)
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("%w: decode", errGarminError)
		}
	}
	return nil
}

var (
	errGarminBridgeUnreachable = errors.New("garmin_bridge_unreachable")
	errGarminError             = errors.New("garmin_error")
)

func (h *Handlers) respondScheduleErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, errWorkoutNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "workout_not_found"})
	case errors.Is(err, errTemplateNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "template_not_found"})
	case errors.Is(err, errNotSchedulable):
		c.JSON(http.StatusBadRequest, gin.H{"error": "workout_not_schedulable"})
	case errors.Is(err, errNotScheduled):
		c.JSON(http.StatusBadRequest, gin.H{"error": "workout_not_scheduled"})
	case errors.Is(err, errGarminBridgeUnreachable):
		c.JSON(http.StatusBadGateway, gin.H{"error": "garmin_bridge_unreachable"})
	default:
		c.JSON(http.StatusBadGateway, gin.H{"error": "garmin_error"})
	}
}

func scheduleErrCode(err error) string {
	switch {
	case errors.Is(err, errWorkoutNotFound):
		return "workout_not_found"
	case errors.Is(err, errNotSchedulable):
		return "workout_not_schedulable"
	case errors.Is(err, errGarminBridgeUnreachable):
		return "garmin_bridge_unreachable"
	default:
		return "garmin_error"
	}
}
