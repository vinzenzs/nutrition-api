package summary

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/vinzenzs/kazper/internal/meals"
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
	rg.GET("/summary/rolling", h.rolling)
	rg.GET("/summary/protein-distribution", h.proteinDistribution)
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

// rolling godoc
// @Summary      Rolling-window nutrition average
// @Description  Returns the trailing-window average of nutrition totals as of `anchor_date`. The window is `[anchor_date − (window_days − 1) days, anchor_date]`, both inclusive, in the requested `tz`. **Averages are computed across days with logged meals (`days_with_data`), NOT across `total_days`** — a 7-day window with 5 logged days returns the 5-day mean, with both divisors exposed so a sparse window is loud. Per-day rows carry `has_data: bool` so "no meals logged" stays distinct from "logged zero." Adherence is computed against the goal resolved at `anchor_date` (honoring per-date overrides). `window_days` is bounded `[2, 30]`.
// @Tags         summary
// @Produce      json
// @Param        anchor_date  query  string   true   "Calendar date in YYYY-MM-DD (the trailing window ends here)"
// @Param        window_days  query  integer  true   "Window size in calendar days; range [2, 30]"
// @Param        tz           query  string   false  "IANA timezone (defaults to DEFAULT_USER_TZ)"
// @Success      200  {object}  Rolling
// @Failure      400  {object}  map[string]interface{}  "anchor_date_required | anchor_date_invalid | window_days_required | window_days_invalid | tz_invalid"
// @Security     BearerAuth
// @Router       /summary/rolling [get]
func (h *Handlers) rolling(c *gin.Context) {
	anchorStr := c.Query("anchor_date")
	if anchorStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "anchor_date_required"})
		return
	}
	anchor, err := time.Parse("2006-01-02", anchorStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "anchor_date_invalid"})
		return
	}

	windowStr := c.Query("window_days")
	if windowStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "window_days_required"})
		return
	}
	windowDays, err := strconv.Atoi(windowStr)
	if err != nil || windowDays < RollingMinWindowDays || windowDays > RollingMaxWindowDays {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "window_days_invalid",
			"range": gin.H{"min": RollingMinWindowDays, "max": RollingMaxWindowDays},
		})
		return
	}

	loc, err := h.resolveTZ(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tz_invalid"})
		return
	}

	out, err := h.svc.RollingFor(c.Request.Context(), RollingParams{
		AnchorDate: anchor, WindowDays: windowDays, Loc: loc,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "summary_failed"})
		return
	}
	c.JSON(http.StatusOK, out)
}

// proteinDistribution godoc
// @Summary      Per-meal protein distribution with MPS-threshold annotations
// @Description  Returns one row per `meal_entries` row on `date`, annotated with `mps_effective: bool` (against the 0.3 g/kg body-weight muscle-protein-synthesis threshold) plus `logged_at_hour` and `gap_minutes_since_previous` for circadian + meal-spacing context. The headline metric is `mps_effective_meal_count / meal_count`. Body weight resolution order: explicit `body_weight_kg` query param > rolling 7-day mean of stored entries ending at `date` (inclusive) > most-recent stored entry strictly before `date`. With no stored data and no override, returns `400 weight_data_missing`.
// @Tags         summary
// @Produce      json
// @Param        date           query  string  true   "Calendar date in YYYY-MM-DD"
// @Param        tz             query  string  false  "IANA timezone (defaults to DEFAULT_USER_TZ)"
// @Param        body_weight_kg query  number  false  "Explicit body weight override; > 0"
// @Success      200  {object}  ProteinDistribution
// @Failure      400  {object}  map[string]string  "date_required | date_invalid | tz_invalid | body_weight_kg_invalid | weight_data_missing"
// @Security     BearerAuth
// @Router       /summary/protein-distribution [get]
func (h *Handlers) proteinDistribution(c *gin.Context) {
	dateStr := c.Query("date")
	if dateStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date_required"})
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

	params := ProteinDistributionParams{Date: date, Loc: loc}
	if s := c.Query("body_weight_kg"); s != "" {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "body_weight_kg_invalid"})
			return
		}
		params.BodyWeightKgOverride = &v
	}

	out, err := h.svc.ProteinDistributionFor(c.Request.Context(), params)
	if err != nil {
		switch {
		case errors.Is(err, ErrWeightDataMissing):
			c.JSON(http.StatusBadRequest, gin.H{"error": "weight_data_missing"})
		case errors.Is(err, ErrBodyWeightInvalid):
			c.JSON(http.StatusBadRequest, gin.H{"error": "body_weight_kg_invalid"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "summary_failed"})
		}
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

