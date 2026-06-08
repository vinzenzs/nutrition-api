package raceprep

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers wires the race-prep endpoints onto a Gin router group.
type Handlers struct {
	svc *Service
}

func NewHandlers(svc *Service) *Handlers {
	return &Handlers{svc: svc}
}

func (h *Handlers) Register(rg *gin.RouterGroup) {
	rg.GET("/race-prep/carb-load", h.carbLoad)
}

// carbLoad godoc
// @Summary      Compute a carb-load schedule for a race
// @Description  Stateless, deterministic carb-load math: given a race date and a body weight, returns daily carbohydrate targets for the load window and race morning. No persistence. The natural follow-up is to translate each entry into a goal override via `PUT /goals/overrides/{date}`.
// @Tags         race-prep
// @Produce      json
// @Param        race_date              query  string   true   "Race date YYYY-MM-DD (must be today or later in DEFAULT_USER_TZ)"
// @Param        body_weight_kg         query  number   true   "Athlete body weight in kilograms, 30..200"
// @Param        days_before            query  integer  false  "Carb-load days before race day, 0..7 (default 3)"
// @Param        carbs_per_kg_per_day   query  number   false  "Load-day multiplier, 1..20 g/kg (default 10)"
// @Param        race_day_carbs_per_kg  query  number   false  "Race-morning multiplier, 0..10 g/kg (default 2)"
// @Success      200  {object}  CarbLoadSchedule
// @Failure      400  {object}  map[string]interface{}  "race_date_required | body_weight_kg_required | race_date_invalid | race_date_in_past | body_weight_kg_invalid | days_before_invalid | carbs_per_kg_per_day_invalid | race_day_carbs_per_kg_invalid"
// @Security     BearerAuth
// @Router       /race-prep/carb-load [get]
func (h *Handlers) carbLoad(c *gin.Context) {
	raceDateStr := c.Query("race_date")
	if raceDateStr == "" {
		respondError(c, http.StatusBadRequest, "race_date_required", nil)
		return
	}
	bodyWeightStr := c.Query("body_weight_kg")
	if bodyWeightStr == "" {
		respondError(c, http.StatusBadRequest, "body_weight_kg_required", nil)
		return
	}

	// Parse race_date as a calendar date in the configured user TZ so
	// "today" in race_date_in_past comparisons resolves consistently.
	raceDate, err := time.ParseInLocation("2006-01-02", raceDateStr, h.svc.TZ())
	if err != nil {
		respondError(c, http.StatusBadRequest, "race_date_invalid", nil)
		return
	}

	bodyWeight, err := strconv.ParseFloat(bodyWeightStr, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "body_weight_kg_invalid",
			rangeHint(BodyWeightKgMin, BodyWeightKgMax))
		return
	}

	daysBefore := DefaultDaysBefore
	if s := c.Query("days_before"); s != "" {
		v, err := strconv.Atoi(s)
		if err != nil {
			respondError(c, http.StatusBadRequest, "days_before_invalid",
				rangeHint(float64(DaysBeforeMin), float64(DaysBeforeMax)))
			return
		}
		daysBefore = v
	}

	carbsPerKgPerDay := DefaultCarbsPerKgPerDay
	if s := c.Query("carbs_per_kg_per_day"); s != "" {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			respondError(c, http.StatusBadRequest, "carbs_per_kg_per_day_invalid",
				rangeHint(CarbsPerKgPerDayMin, CarbsPerKgPerDayMax))
			return
		}
		carbsPerKgPerDay = v
	}

	raceDayCarbsPerKg := DefaultRaceDayCarbsPerKg
	if s := c.Query("race_day_carbs_per_kg"); s != "" {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			respondError(c, http.StatusBadRequest, "race_day_carbs_per_kg_invalid",
				rangeHint(RaceDayCarbsPerKgMin, RaceDayCarbsPerKgMax))
			return
		}
		raceDayCarbsPerKg = v
	}

	out, err := h.svc.Plan(CarbLoadParams{
		RaceDate:          raceDate,
		BodyWeightKg:      bodyWeight,
		DaysBefore:        daysBefore,
		CarbsPerKgPerDay:  carbsPerKgPerDay,
		RaceDayCarbsPerKg: raceDayCarbsPerKg,
	})
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func respondError(c *gin.Context, status int, code string, rangeHint gin.H) {
	body := gin.H{"error": code}
	if rangeHint != nil {
		body["range"] = rangeHint
	}
	c.JSON(status, body)
}

func respondServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrRaceDateInPast):
		respondError(c, http.StatusBadRequest, "race_date_in_past", nil)
	case errors.Is(err, ErrBodyWeightKgInvalid):
		respondError(c, http.StatusBadRequest, "body_weight_kg_invalid",
			rangeHint(BodyWeightKgMin, BodyWeightKgMax))
	case errors.Is(err, ErrDaysBeforeInvalid):
		respondError(c, http.StatusBadRequest, "days_before_invalid",
			rangeHint(float64(DaysBeforeMin), float64(DaysBeforeMax)))
	case errors.Is(err, ErrCarbsPerKgPerDayInvalid):
		respondError(c, http.StatusBadRequest, "carbs_per_kg_per_day_invalid",
			rangeHint(CarbsPerKgPerDayMin, CarbsPerKgPerDayMax))
	case errors.Is(err, ErrRaceDayCarbsPerKgInvalid):
		respondError(c, http.StatusBadRequest, "race_day_carbs_per_kg_invalid",
			rangeHint(RaceDayCarbsPerKgMin, RaceDayCarbsPerKgMax))
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "plan_failed"})
	}
}

func rangeHint(min, max float64) gin.H {
	return gin.H{"min": min, "max": max}
}
