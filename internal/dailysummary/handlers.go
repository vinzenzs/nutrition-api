package dailysummary

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/vinzenzs/kazper/internal/numfmt"
)

const maxWindowDays = 92

// Handlers wires the daily-summary endpoints.
type Handlers struct {
	svc *Service
}

func NewHandlers(svc *Service) *Handlers {
	return &Handlers{svc: svc}
}

func (h *Handlers) Register(rg *gin.RouterGroup) {
	rg.POST("/daily-summary", h.upsert)
	rg.GET("/daily-summary", h.list)
	rg.GET("/daily-summary/:date", h.get)
	rg.DELETE("/daily-summary/:date", h.delete)
}

// upsert godoc
// @Summary      Upsert a whole-day energy/activity snapshot (by date)
// @Description  Creates or full-replaces the daily-summary snapshot for a calendar date (active/resting/total kcal, steps, floors, intensity minutes, distance). "POST every day you see" — re-pushing the same date updates in place, with omitted fields reset to NULL. Standard `Idempotency-Key` header supported. These expenditure/activity totals are unit-isolated: they never merge into nutrition summary totals or the Energy Availability denominator.
// @Tags         daily-summary
// @Accept       json
// @Produce      json
// @Param        Idempotency-Key  header  string    false  "Optional client-supplied idempotency key"
// @Param        body             body    Snapshot  true   "Daily summary (date required; metrics optional)"
// @Success      201  {object}  Snapshot  "INSERT"
// @Success      200  {object}  Snapshot  "UPDATE (date already present)"
// @Failure      400  {object}  map[string]string  "date_invalid | active_kcal_invalid | resting_kcal_invalid | total_kcal_invalid | steps_invalid | floors_invalid | moderate_intensity_minutes_invalid | vigorous_intensity_minutes_invalid | distance_m_invalid"
// @Security     BearerAuth
// @Router       /daily-summary [post]
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
// @Summary      List daily-summary snapshots in a date window
// @Tags         daily-summary
// @Produce      json
// @Param        from  query  string  true  "Inclusive lower bound YYYY-MM-DD"
// @Param        to    query  string  true  "Inclusive upper bound YYYY-MM-DD; max 92-day span"
// @Success      200  {object}  map[string]interface{}  "{ daily_summary: [...] }"
// @Failure      400  {object}  map[string]interface{}  "window_required | window_invalid | range_too_large"
// @Security     BearerAuth
// @Router       /daily-summary [get]
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
	c.JSON(http.StatusOK, gin.H{"daily_summary": out})
}

// get godoc
// @Summary      Get the daily-summary snapshot for a date
// @Tags         daily-summary
// @Produce      json
// @Param        date  path  string  true  "Date YYYY-MM-DD"
// @Success      200  {object}  Snapshot
// @Failure      404  {object}  map[string]string  "daily_summary_not_found"
// @Security     BearerAuth
// @Router       /daily-summary/{date} [get]
func (h *Handlers) get(c *gin.Context) {
	out, err := h.svc.Get(c.Request.Context(), c.Param("date"))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respondError(c, http.StatusNotFound, "daily_summary_not_found")
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
// @Summary      Delete the daily-summary snapshot for a date
// @Tags         daily-summary
// @Param        date  path  string  true  "Date YYYY-MM-DD"
// @Success      204  "no content"
// @Failure      404  {object}  map[string]string  "daily_summary_not_found"
// @Security     BearerAuth
// @Router       /daily-summary/{date} [delete]
func (h *Handlers) delete(c *gin.Context) {
	if err := h.svc.Delete(c.Request.Context(), c.Param("date")); err != nil {
		if errors.Is(err, ErrNotFound) {
			respondError(c, http.StatusNotFound, "daily_summary_not_found")
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
		ErrDateInvalid, ErrActiveKcalInvalid, ErrRestingKcalInvalid, ErrTotalKcalInvalid,
		ErrStepsInvalid, ErrFloorsInvalid, ErrModerateIntensityMinutesInvalid,
		ErrVigorousIntensityMinutesInvalid, ErrDistanceMInvalid,
	} {
		if errors.Is(err, e) {
			respondError(c, http.StatusBadRequest, e.Error())
			return
		}
	}
	respondError(c, http.StatusInternalServerError, "write_failed")
}

// round applies 1dp rounding to the one float metric (distance_m) at the
// boundary; integer metrics are returned as-is.
func round(s *Snapshot) *Snapshot {
	if s == nil {
		return nil
	}
	out := *s
	out.DistanceM = numfmt.Round1Ptr(s.DistanceM)
	return &out
}
