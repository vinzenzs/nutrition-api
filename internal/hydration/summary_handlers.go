package hydration

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/vinzenzs/nutrition-api/internal/numfmt"
)

// SummaryHandlers wires the daily hydration summary endpoint.
type SummaryHandlers struct {
	svc           *Service
	defaultTZName string
	logger        *slog.Logger
}

func NewSummaryHandlers(svc *Service, defaultTZ string, logger *slog.Logger) *SummaryHandlers {
	return &SummaryHandlers{svc: svc, defaultTZName: defaultTZ, logger: logger}
}

func (h *SummaryHandlers) Register(rg *gin.RouterGroup) {
	rg.GET("/summary/hydration/daily", h.daily)
}

// daily godoc
// @Summary      Daily hydration total + entries
// @Description  Returns total ml and per-entry list for one calendar date in the supplied timezone. Volume-only — separate from /summary/daily, which is nutrient-only.
// @Tags         summary
// @Produce      json
// @Param        date  query  string  true   "Calendar date in YYYY-MM-DD"
// @Param        tz    query  string  false  "IANA timezone (defaults to DEFAULT_USER_TZ)"
// @Success      200  {object}  Daily
// @Failure      400  {object}  map[string]string  "date_invalid | tz_invalid"
// @Security     BearerAuth
// @Router       /summary/hydration/daily [get]
func (h *SummaryHandlers) daily(c *gin.Context) {
	dateStr := c.Query("date")
	if dateStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date_invalid"})
		return
	}
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date_invalid"})
		return
	}
	loc, err := h.resolveTZ(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tz_invalid"})
		return
	}

	out, err := h.svc.DailyFor(c.Request.Context(), DailyParams{Date: date, Loc: loc})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "summary_failed"})
		return
	}
	// Round total to 1dp at the response boundary; per-entry quantities also
	// get the same rounding so the response is self-consistent.
	out.TotalMl = numfmt.Round1(out.TotalMl)
	for _, e := range out.Entries {
		e.QuantityMl = numfmt.Round1(e.QuantityMl)
	}
	c.JSON(http.StatusOK, out)
}

func (h *SummaryHandlers) resolveTZ(c *gin.Context) (*time.Location, error) {
	tz := c.Query("tz")
	if tz == "" {
		tz = h.defaultTZName
		h.logger.Warn("summary used default tz", "route", c.FullPath(), "default_tz", tz)
	}
	return time.LoadLocation(tz)
}
