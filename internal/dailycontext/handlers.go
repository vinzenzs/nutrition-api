package dailycontext

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

const dateFormat = "2006-01-02"

// Handlers wires GET /context/daily.
type Handlers struct {
	svc           *Service
	defaultTZName string
	logger        *slog.Logger
}

func NewHandlers(svc *Service, defaultTZ string, logger *slog.Logger) *Handlers {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handlers{svc: svc, defaultTZName: defaultTZ, logger: logger}
}

func (h *Handlers) Register(rg *gin.RouterGroup) {
	rg.GET("/context/daily", h.daily)
}

// daily godoc
// @Summary      Daily context bundle
// @Description  One read that returns the day's adherence + nutrition totals + hydration ml + workouts + workout-fuel entries + body-weight state + training-phase context + goal-override presence. Composition-only over existing primitives — use this as the first call of a session; for deep dives into one slice, use the dedicated tools (daily_summary, list_workouts, etc.) which include the per-entry detail this aggregator omits.
// @Tags         daily-context
// @Produce      json
// @Param        date  query  string  true   "Calendar date in YYYY-MM-DD"
// @Param        tz    query  string  false  "IANA timezone (defaults to DEFAULT_USER_TZ)"
// @Success      200  {object}  DailyContext
// @Failure      400  {object}  map[string]string  "date_invalid | tz_invalid"
// @Failure      500  {object}  map[string]string  "context_failed"
// @Security     BearerAuth
// @Router       /context/daily [get]
func (h *Handlers) daily(c *gin.Context) {
	dateStr := c.Query("date")
	if dateStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date_invalid"})
		return
	}
	date, err := time.Parse(dateFormat, dateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date_invalid"})
		return
	}

	tzName := c.Query("tz")
	if tzName == "" {
		tzName = h.defaultTZName
	}
	loc, err := time.LoadLocation(tzName)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tz_invalid"})
		return
	}

	out, err := h.svc.BuildFor(c.Request.Context(), date, loc)
	if err != nil {
		h.logger.Warn("daily context build failed", "date", dateStr, "tz", tzName, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "context_failed"})
		return
	}
	// Echo the request's date verbatim (loc.String() handles "UTC"/"Local"
	// normalization for the tz field, matching loc resolution).
	out.TZ = loc.String()
	c.JSON(http.StatusOK, out)
}
