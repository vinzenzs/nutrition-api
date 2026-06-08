package energy

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers wires GET /energy/availability onto a Gin router group.
type Handlers struct {
	svc       *Service
	defaultTZ string
}

func NewHandlers(svc *Service, defaultTZ string) *Handlers {
	return &Handlers{svc: svc, defaultTZ: defaultTZ}
}

func (h *Handlers) Register(rg *gin.RouterGroup) {
	rg.GET("/energy/availability", h.availability)
}

// availability godoc
// @Summary      Energy Availability over a window
// @Description  Per-day EA values + window aggregate, computed from meals (intake), workouts (exercise burn) and body weight (composition). Loucks bands: `< 30 kcal/kg FFM/day = low`, `30-45 = sub_optimal`, `>= 45 = adequate`. FFM resolution order: lean_mass_kg query param → body_fat_pct query param + stored body weight → most-recent in-window stored body_fat_pct → 85% fallback (loud, flagged via `composition.composition_estimated`). Days with workouts missing `kcal_burned` are listed in `missing_burn_workout_ids` and excluded from `window.avg_ea`.
// @Tags         energy
// @Produce      json
// @Param        from           query  string   true   "Inclusive RFC3339 lower bound"
// @Param        to             query  string   true   "Exclusive RFC3339 upper bound; max 92 days from 'from'"
// @Param        tz             query  string   false  "IANA timezone for calendar-day boundaries; defaults to DEFAULT_USER_TZ"
// @Param        lean_mass_kg   query  number   false  "Explicit FFM override (highest-trust); > 0"
// @Param        body_fat_pct   query  number   false  "Explicit body-fat % override; 0 <= x < 100"
// @Success      200  {object}  Availability
// @Failure      400  {object}  map[string]interface{}  "window_required | window_invalid | range_too_large | tz_invalid | lean_mass_kg_invalid | body_fat_pct_invalid | weight_data_missing"
// @Security     BearerAuth
// @Router       /energy/availability [get]
func (h *Handlers) availability(c *gin.Context) {
	fromStr := c.Query("from")
	toStr := c.Query("to")
	if fromStr == "" || toStr == "" {
		respondError(c, http.StatusBadRequest, "window_required")
		return
	}
	from, err := time.Parse(time.RFC3339, fromStr)
	if err != nil {
		respondError(c, http.StatusBadRequest, "window_invalid")
		return
	}
	to, err := time.Parse(time.RFC3339, toStr)
	if err != nil {
		respondError(c, http.StatusBadRequest, "window_invalid")
		return
	}

	tzStr := c.Query("tz")
	if tzStr == "" {
		tzStr = h.defaultTZ
	}
	loc, err := time.LoadLocation(tzStr)
	if err != nil {
		respondError(c, http.StatusBadRequest, "tz_invalid")
		return
	}

	params := AvailabilityParams{From: from, To: to, TZ: loc}

	if s := c.Query("lean_mass_kg"); s != "" {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			respondError(c, http.StatusBadRequest, "lean_mass_kg_invalid")
			return
		}
		params.LeanMassKg = &v
	}
	if s := c.Query("body_fat_pct"); s != "" {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			respondError(c, http.StatusBadRequest, "body_fat_pct_invalid")
			return
		}
		params.BodyFatPct = &v
	}

	out, err := h.svc.Compute(c.Request.Context(), params)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// ----- helpers -----

func respondError(c *gin.Context, status int, code string) {
	c.JSON(status, gin.H{"error": code})
}

func respondServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrWindowInvalid):
		respondError(c, http.StatusBadRequest, "window_invalid")
	case errors.Is(err, ErrRangeTooLarge):
		c.JSON(http.StatusBadRequest, gin.H{"error": "range_too_large", "max_days": maxWindowDays})
	case errors.Is(err, ErrLeanMassInvalid):
		respondError(c, http.StatusBadRequest, "lean_mass_kg_invalid")
	case errors.Is(err, ErrBodyFatInvalid):
		respondError(c, http.StatusBadRequest, "body_fat_pct_invalid")
	case errors.Is(err, ErrWeightDataMissing):
		respondError(c, http.StatusBadRequest, "weight_data_missing")
	default:
		respondError(c, http.StatusInternalServerError, "compute_failed")
	}
}
