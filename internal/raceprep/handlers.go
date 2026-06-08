package raceprep

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers wires the race-prep endpoints onto a Gin router group.
type Handlers struct {
	svc    *Service
	logger *slog.Logger
}

func NewHandlers(svc *Service) *Handlers {
	return &Handlers{svc: svc, logger: slog.Default()}
}

// SetLogger overrides the default logger used for apply-success log lines.
// Exposed so the server wiring can pass the configured slog.Logger.
func (h *Handlers) SetLogger(l *slog.Logger) {
	if l != nil {
		h.logger = l
	}
}

func (h *Handlers) Register(rg *gin.RouterGroup) {
	rg.GET("/race-prep/carb-load", h.carbLoad)
	rg.POST("/race-prep/carb-load/apply", h.carbLoadApply)
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

// applyRequestBody is the JSON shape POST /race-prep/carb-load/apply accepts.
// Optional fields are pointers so we can distinguish absent from zero (zero
// would mean "use the default 0" but the validator rejects 0 for some fields).
type applyRequestBody struct {
	RaceDate          string   `json:"race_date"`
	BodyWeightKg      *float64 `json:"body_weight_kg"`
	DaysBefore        *int     `json:"days_before,omitempty"`
	CarbsPerKgPerDay  *float64 `json:"carbs_per_kg_per_day,omitempty"`
	RaceDayCarbsPerKg *float64 `json:"race_day_carbs_per_kg,omitempty"`
}

// carbLoadApply godoc
// @Summary      Compute AND apply a carb-load schedule into per-date goal overrides
// @Description  Same inputs as `GET /race-prep/carb-load`. Computes the schedule, then in a single transaction writes the per-day carb target into the `daily_goal_overrides` row for each schedule date — merging into existing overrides (preserving non-carb fields) or creating new rows. Returns the schedule plus a per-date `applied` outcome (`{date, carbs_g_min, created}`). On any per-date failure the whole transaction rolls back and zero overrides are persisted.
// @Tags         race-prep
// @Accept       json
// @Produce      json
// @Param        body  body  applyRequestBody  true  "Carb-load parameters (same as the GET endpoint)"
// @Success      200  {object}  ApplyResponse
// @Failure      400  {object}  map[string]interface{}  "race_date_required | body_weight_kg_required | race_date_invalid | race_date_in_past | body_weight_kg_invalid | days_before_invalid | carbs_per_kg_per_day_invalid | race_day_carbs_per_kg_invalid | invalid_json"
// @Failure      500  {object}  map[string]string  "apply_failed"
// @Security     BearerAuth
// @Router       /race-prep/carb-load/apply [post]
func (h *Handlers) carbLoadApply(c *gin.Context) {
	var body applyRequestBody
	if err := json.NewDecoder(c.Request.Body).Decode(&body); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_json", nil)
		return
	}
	if body.RaceDate == "" {
		respondError(c, http.StatusBadRequest, "race_date_required", nil)
		return
	}
	if body.BodyWeightKg == nil {
		respondError(c, http.StatusBadRequest, "body_weight_kg_required", nil)
		return
	}
	raceDate, err := time.ParseInLocation("2006-01-02", body.RaceDate, h.svc.TZ())
	if err != nil {
		respondError(c, http.StatusBadRequest, "race_date_invalid", nil)
		return
	}
	daysBefore := DefaultDaysBefore
	if body.DaysBefore != nil {
		daysBefore = *body.DaysBefore
	}
	carbsPerKgPerDay := DefaultCarbsPerKgPerDay
	if body.CarbsPerKgPerDay != nil {
		carbsPerKgPerDay = *body.CarbsPerKgPerDay
	}
	raceDayCarbsPerKg := DefaultRaceDayCarbsPerKg
	if body.RaceDayCarbsPerKg != nil {
		raceDayCarbsPerKg = *body.RaceDayCarbsPerKg
	}

	out, err := h.svc.ApplyCarbLoad(c.Request.Context(), ApplyRequest{
		RaceDate:          raceDate,
		BodyWeightKg:      *body.BodyWeightKg,
		DaysBefore:        daysBefore,
		CarbsPerKgPerDay:  carbsPerKgPerDay,
		RaceDayCarbsPerKg: raceDayCarbsPerKg,
	})
	if err != nil {
		// Service-level validation errors map 1:1 to 400s. Anything else is a 500.
		if isValidationErr(err) {
			respondServiceError(c, err)
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "apply_failed"})
		return
	}

	newCount := 0
	for _, a := range out.Applied {
		if a.Created {
			newCount++
		}
	}
	h.logger.Info("carb_load_applied",
		"race_date", out.RaceDate,
		"days_before", out.Params.DaysBefore,
		"applied_count", len(out.Applied),
		"new_count", newCount,
	)

	c.JSON(http.StatusOK, out)
}

func isValidationErr(err error) bool {
	return errors.Is(err, ErrRaceDateInPast) ||
		errors.Is(err, ErrBodyWeightKgInvalid) ||
		errors.Is(err, ErrDaysBeforeInvalid) ||
		errors.Is(err, ErrCarbsPerKgPerDayInvalid) ||
		errors.Is(err, ErrRaceDayCarbsPerKgInvalid)
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
