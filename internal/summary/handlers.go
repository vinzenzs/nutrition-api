package summary

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/vinzenzs/nutrition-api/internal/meals"
)

const maxRangeDays = 92

// Handlers wires the summary endpoints onto a Gin router group.
type Handlers struct {
	svc           *Service
	defaultTZName string
	logger        *slog.Logger
}

// NewHandlers constructs Handlers with a default TZ used when clients omit
// the `tz` query param.
func NewHandlers(svc *Service, defaultTZ string, logger *slog.Logger) *Handlers {
	return &Handlers{svc: svc, defaultTZName: defaultTZ, logger: logger}
}

func (h *Handlers) Register(rg *gin.RouterGroup) {
	rg.GET("/summary/daily", h.daily)
	rg.GET("/summary/range", h.rng)
}

// daily godoc
// @Summary      Daily nutrition totals
// @Description  Returns aggregated totals for a single calendar date in the supplied timezone. When `meal_type` is set, only entries of that type are summed and adherence is omitted.
// @Tags         summary
// @Produce      json
// @Param        date       query  string  true   "Calendar date in YYYY-MM-DD"
// @Param        tz         query  string  false  "IANA timezone (defaults to DEFAULT_USER_TZ)"
// @Param        meal_type  query  string  false  "Optional filter: breakfast|lunch|dinner|snack"
// @Success      200  {object}  Daily
// @Failure      400  {object}  map[string]string  "date_invalid | tz_invalid | meal_type_invalid"
// @Security     BearerAuth
// @Router       /summary/daily [get]
func (h *Handlers) daily(c *gin.Context) {
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

	var mealType *meals.MealType
	if mtStr := c.Query("meal_type"); mtStr != "" {
		mt, err := meals.ParseMealType(mtStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "meal_type_invalid"})
			return
		}
		mealType = &mt
	}

	out, err := h.svc.DailyFor(c.Request.Context(), DailyParams{Date: date, Loc: loc, MealType: mealType})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "summary_failed"})
		return
	}
	c.JSON(http.StatusOK, out)
}

// rng godoc
// @Summary      Daily totals across a date range
// @Description  Returns one row per day in `[from, to]`. Range is capped at 92 days inclusive.
// @Tags         summary
// @Produce      json
// @Param        from  query  string  true   "Inclusive start date YYYY-MM-DD"
// @Param        to    query  string  true   "Inclusive end date YYYY-MM-DD"
// @Param        tz    query  string  false  "IANA timezone (defaults to DEFAULT_USER_TZ)"
// @Success      200  {object}  Range
// @Failure      400  {object}  map[string]interface{}  "range_required | range_invalid | range_too_large | date_invalid | tz_invalid"
// @Security     BearerAuth
// @Router       /summary/range [get]
func (h *Handlers) rng(c *gin.Context) {
	fromStr := c.Query("from")
	toStr := c.Query("to")
	if fromStr == "" || toStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "range_required"})
		return
	}
	from, err := time.Parse("2006-01-02", fromStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date_invalid"})
		return
	}
	to, err := time.Parse("2006-01-02", toStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date_invalid"})
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
	loc, err := h.resolveTZ(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tz_invalid"})
		return
	}

	groupBy := c.Query("group_by")
	if groupBy != "" && groupBy != "meal_type" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "group_by_invalid"})
		return
	}

	out, err := h.svc.RangeFor(c.Request.Context(), RangeParams{From: from, To: to, Loc: loc, GroupBy: groupBy})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "summary_failed"})
		return
	}
	c.JSON(http.StatusOK, out)
}

// resolveTZ returns the time.Location for the `tz` query param, or the
// configured default. Logs a WARN when the default is used.
func (h *Handlers) resolveTZ(c *gin.Context) (*time.Location, error) {
	tz := c.Query("tz")
	if tz == "" {
		tz = h.defaultTZName
		h.logger.Warn("summary used default tz", "route", c.FullPath(), "default_tz", tz)
	}
	return time.LoadLocation(tz)
}

