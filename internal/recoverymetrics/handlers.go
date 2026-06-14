package recoverymetrics

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/vinzenzs/kazper/internal/numfmt"
)

const maxWindowDays = 92

// Handlers wires the recovery-metrics endpoints.
type Handlers struct {
	svc *Service
}

func NewHandlers(svc *Service) *Handlers {
	return &Handlers{svc: svc}
}

func (h *Handlers) Register(rg *gin.RouterGroup) {
	rg.POST("/recovery-metrics", h.upsert)
	rg.GET("/recovery-metrics", h.list)
	rg.GET("/recovery-metrics/:date", h.get)
	rg.DELETE("/recovery-metrics/:date", h.delete)
}

// upsert godoc
// @Summary      Upsert a daily recovery snapshot (by date)
// @Description  Creates or full-replaces the recovery snapshot for a calendar date. "POST every day you see" — re-pushing the same date updates in place. Standard `Idempotency-Key` header supported.
// @Tags         recovery-metrics
// @Accept       json
// @Produce      json
// @Param        Idempotency-Key  header  string    false  "Optional client-supplied idempotency key"
// @Param        body             body    Snapshot  true   "Recovery snapshot (date required; metrics optional)"
// @Success      201  {object}  Snapshot  "INSERT"
// @Success      200  {object}  Snapshot  "UPDATE (date already present)"
// @Failure      400  {object}  map[string]string  "date_invalid | sleep_seconds_invalid | sleep_score_invalid | hrv_ms_invalid | resting_hr_invalid | stress_avg_invalid | body_battery_charged_invalid | body_battery_drained_invalid | training_readiness_invalid | spo2_avg_invalid | spo2_lowest_invalid | respiration_avg_invalid | respiration_lowest_invalid | deep_sleep_seconds_invalid | light_sleep_seconds_invalid | rem_sleep_seconds_invalid | awake_seconds_invalid"
// @Security     BearerAuth
// @Router       /recovery-metrics [post]
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
// @Summary      List recovery snapshots in a date window
// @Tags         recovery-metrics
// @Produce      json
// @Param        from  query  string  true  "Inclusive lower bound YYYY-MM-DD"
// @Param        to    query  string  true  "Inclusive upper bound YYYY-MM-DD; max 92-day span"
// @Success      200  {object}  map[string]interface{}  "{ recovery_metrics: [...] }"
// @Failure      400  {object}  map[string]interface{}  "window_required | window_invalid | range_too_large"
// @Security     BearerAuth
// @Router       /recovery-metrics [get]
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
	c.JSON(http.StatusOK, gin.H{"recovery_metrics": out})
}

// get godoc
// @Summary      Get the recovery snapshot for a date
// @Tags         recovery-metrics
// @Produce      json
// @Param        date  path  string  true  "Date YYYY-MM-DD"
// @Success      200  {object}  Snapshot
// @Failure      404  {object}  map[string]string  "recovery_metrics_not_found"
// @Security     BearerAuth
// @Router       /recovery-metrics/{date} [get]
func (h *Handlers) get(c *gin.Context) {
	out, err := h.svc.Get(c.Request.Context(), c.Param("date"))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respondError(c, http.StatusNotFound, "recovery_metrics_not_found")
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
// @Summary      Delete the recovery snapshot for a date
// @Tags         recovery-metrics
// @Param        date  path  string  true  "Date YYYY-MM-DD"
// @Success      204  "no content"
// @Failure      404  {object}  map[string]string  "recovery_metrics_not_found"
// @Security     BearerAuth
// @Router       /recovery-metrics/{date} [delete]
func (h *Handlers) delete(c *gin.Context) {
	if err := h.svc.Delete(c.Request.Context(), c.Param("date")); err != nil {
		if errors.Is(err, ErrNotFound) {
			respondError(c, http.StatusNotFound, "recovery_metrics_not_found")
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
		ErrDateInvalid, ErrSleepSecondsInvalid, ErrSleepScoreInvalid, ErrHRVInvalid,
		ErrRestingHRInvalid, ErrStressAvgInvalid, ErrBodyBatteryChargedInvalid,
		ErrBodyBatteryDrainedInvalid, ErrTrainingReadinessInvalid,
		ErrSpo2AvgInvalid, ErrSpo2LowestInvalid, ErrRespirationAvgInvalid,
		ErrRespirationLowestInvalid, ErrDeepSleepSecondsInvalid, ErrLightSleepSecondsInvalid,
		ErrRemSleepSecondsInvalid, ErrAwakeSecondsInvalid,
	} {
		if errors.Is(err, e) {
			respondError(c, http.StatusBadRequest, e.Error())
			return
		}
	}
	respondError(c, http.StatusInternalServerError, "write_failed")
}

// round applies 1dp rounding to the float metrics (hrv_ms, respiration_*) at the
// boundary. SpO2 and sleep-stage seconds are integers and pass through unrounded.
func round(s *Snapshot) *Snapshot {
	if s == nil {
		return nil
	}
	out := *s
	out.HRVMs = numfmt.Round1Ptr(s.HRVMs)
	out.RespirationAvg = numfmt.Round1Ptr(s.RespirationAvg)
	out.RespirationLowest = numfmt.Round1Ptr(s.RespirationLowest)
	return &out
}
