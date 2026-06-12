package fitnessmetrics

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/vinzenzs/nutrition-api/internal/numfmt"
)

const maxWindowDays = 92

// Handlers wires the fitness-metrics endpoints.
type Handlers struct {
	svc *Service
}

func NewHandlers(svc *Service) *Handlers {
	return &Handlers{svc: svc}
}

func (h *Handlers) Register(rg *gin.RouterGroup) {
	rg.POST("/fitness-metrics", h.upsert)
	rg.GET("/fitness-metrics", h.list)
	rg.GET("/fitness-metrics/:date", h.get)
	rg.DELETE("/fitness-metrics/:date", h.delete)
}

// upsert godoc
// @Summary      Upsert a daily fitness snapshot (by date)
// @Description  Creates or full-replaces the fitness snapshot for a calendar date. Re-pushing the same date updates in place. Race predictions are seconds. Standard `Idempotency-Key` header supported.
// @Tags         fitness-metrics
// @Accept       json
// @Produce      json
// @Param        Idempotency-Key  header  string    false  "Optional client-supplied idempotency key"
// @Param        body             body    Snapshot  true   "Fitness snapshot (date required; metrics optional)"
// @Success      201  {object}  Snapshot  "INSERT"
// @Success      200  {object}  Snapshot  "UPDATE (date already present)"
// @Failure      400  {object}  map[string]string  "date_invalid | vo2max_running_invalid | vo2max_cycling_invalid | race_predictor_5k_seconds_invalid | race_predictor_10k_seconds_invalid | race_predictor_half_seconds_invalid | race_predictor_full_seconds_invalid | acute_load_invalid | chronic_load_invalid | endurance_score_invalid | hill_score_invalid | fitness_age_invalid | training_status_invalid"
// @Security     BearerAuth
// @Router       /fitness-metrics [post]
func (h *Handlers) upsert(c *gin.Context) {
	var in Snapshot
	if err := c.ShouldBindJSON(&in); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_json")
		return
	}
	out, created, err := h.svc.Upsert(c.Request.Context(), &in)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	c.JSON(status, round(out))
}

// list godoc
// @Summary      List fitness snapshots in a date window
// @Tags         fitness-metrics
// @Produce      json
// @Param        from  query  string  true  "Inclusive lower bound YYYY-MM-DD"
// @Param        to    query  string  true  "Inclusive upper bound YYYY-MM-DD; max 92-day span"
// @Success      200  {object}  map[string]interface{}  "{ fitness_metrics: [...] }"
// @Failure      400  {object}  map[string]interface{}  "window_required | window_invalid | range_too_large"
// @Security     BearerAuth
// @Router       /fitness-metrics [get]
func (h *Handlers) list(c *gin.Context) {
	fromStr := c.Query("from")
	toStr := c.Query("to")
	if fromStr == "" || toStr == "" {
		respondError(c, http.StatusBadRequest, "window_required")
		return
	}
	from, err := time.Parse(dateLayout, fromStr)
	if err != nil {
		respondError(c, http.StatusBadRequest, "window_invalid")
		return
	}
	to, err := time.Parse(dateLayout, toStr)
	if err != nil {
		respondError(c, http.StatusBadRequest, "window_invalid")
		return
	}
	if to.Before(from) {
		respondError(c, http.StatusBadRequest, "window_invalid")
		return
	}
	if to.Sub(from) > time.Duration(maxWindowDays)*24*time.Hour {
		c.JSON(http.StatusBadRequest, gin.H{"error": "range_too_large", "max_days": maxWindowDays})
		return
	}
	rows, err := h.svc.ListWindow(c.Request.Context(), fromStr, toStr)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "list_failed")
		return
	}
	out := make([]*Snapshot, 0, len(rows))
	for _, s := range rows {
		out = append(out, round(s))
	}
	c.JSON(http.StatusOK, gin.H{"fitness_metrics": out})
}

// get godoc
// @Summary      Get the fitness snapshot for a date
// @Tags         fitness-metrics
// @Produce      json
// @Param        date  path  string  true  "Date YYYY-MM-DD"
// @Success      200  {object}  Snapshot
// @Failure      404  {object}  map[string]string  "fitness_metrics_not_found"
// @Security     BearerAuth
// @Router       /fitness-metrics/{date} [get]
func (h *Handlers) get(c *gin.Context) {
	out, err := h.svc.Get(c.Request.Context(), c.Param("date"))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respondError(c, http.StatusNotFound, "fitness_metrics_not_found")
			return
		}
		if errors.Is(err, ErrDateInvalid) {
			respondError(c, http.StatusBadRequest, "date_invalid")
			return
		}
		respondError(c, http.StatusInternalServerError, "get_failed")
		return
	}
	c.JSON(http.StatusOK, round(out))
}

// delete godoc
// @Summary      Delete the fitness snapshot for a date
// @Tags         fitness-metrics
// @Param        date  path  string  true  "Date YYYY-MM-DD"
// @Success      204  "no content"
// @Failure      404  {object}  map[string]string  "fitness_metrics_not_found"
// @Security     BearerAuth
// @Router       /fitness-metrics/{date} [delete]
func (h *Handlers) delete(c *gin.Context) {
	if err := h.svc.Delete(c.Request.Context(), c.Param("date")); err != nil {
		if errors.Is(err, ErrNotFound) {
			respondError(c, http.StatusNotFound, "fitness_metrics_not_found")
			return
		}
		if errors.Is(err, ErrDateInvalid) {
			respondError(c, http.StatusBadRequest, "date_invalid")
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
	for _, e := range []error{
		ErrDateInvalid, ErrVO2MaxRunningInvalid, ErrVO2MaxCyclingInvalid,
		ErrRacePredictor5kInvalid, ErrRacePredictor10kInvalid,
		ErrRacePredictorHalfInvalid, ErrRacePredictorFullInvalid,
		ErrAcuteLoadInvalid, ErrChronicLoadInvalid,
		ErrEnduranceScoreInvalid, ErrHillScoreInvalid, ErrFitnessAgeInvalid,
		ErrTrainingStatusInvalid,
	} {
		if errors.Is(err, e) {
			respondError(c, http.StatusBadRequest, e.Error())
			return
		}
	}
	respondError(c, http.StatusInternalServerError, "write_failed")
}

// round applies 1dp rounding to the float metrics at the response boundary.
// endurance_score / hill_score are integers and training_status is text — both
// pass through unrounded.
func round(s *Snapshot) *Snapshot {
	if s == nil {
		return nil
	}
	out := *s
	out.VO2MaxRunning = numfmt.Round1Ptr(s.VO2MaxRunning)
	out.VO2MaxCycling = numfmt.Round1Ptr(s.VO2MaxCycling)
	out.AcuteLoad = numfmt.Round1Ptr(s.AcuteLoad)
	out.ChronicLoad = numfmt.Round1Ptr(s.ChronicLoad)
	out.FitnessAge = numfmt.Round1Ptr(s.FitnessAge)
	return &out
}
