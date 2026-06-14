package raceprep

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/vinzenzs/kazper/internal/bodyweight"
	"github.com/vinzenzs/kazper/internal/workouts"
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
	rg.GET("/race-prep/recommend-workout-fuel", h.recommendWorkoutFuel)
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

// recommendWorkoutFuel godoc
// @Summary      Pre/intra/post fueling recommendation for a single session
// @Description  Stateless literature-grounded fueling recommendation for one training or race session. Accepts EITHER `workout_id` (pulls sport/duration/intensity from the row, deriving intensity zone from `tss` when present) OR the explicit triplet `sport`+`duration_min`+`intensity_zone`. Body weight resolved via the four-tier rule (explicit override > rolling 7d stored mean > most-recent stored entry > 400). Reuses the 0.3 g/kg MPS threshold from `protein_distribution` for the post-workout protein recommendation.
// @Tags         race-prep
// @Produce      json
// @Param        workout_id      query  string   false  "Workout UUID — pulls sport/duration/intensity from the row"
// @Param        sport           query  string   false  "Sport (bike|run|swim|strength|other). Required in explicit mode."
// @Param        duration_min    query  integer  false  "Duration in minutes (>0). Required in explicit mode."
// @Param        intensity_zone  query  integer  false  "Zone 1–5. Required in explicit mode."
// @Param        body_weight_kg  query  number   false  "Explicit body-weight override (>0). Otherwise resolved from stored entries."
// @Success      200  {object}  FuelRecommendation
// @Failure      400  {object}  map[string]interface{}  "input_required | input_conflict | sport_required | duration_min_required | intensity_zone_required | sport_invalid | duration_min_invalid | intensity_zone_invalid | body_weight_kg_invalid | weight_data_missing | workout_id_invalid"
// @Failure      404  {object}  map[string]string       "workout_not_found"
// @Security     BearerAuth
// @Router       /race-prep/recommend-workout-fuel [get]
func (h *Handlers) recommendWorkoutFuel(c *gin.Context) {
	params := RecommendParams{
		Today: time.Now(),
		Loc:   time.UTC,
	}

	if s := c.Query("workout_id"); s != "" {
		wid, err := uuid.Parse(s)
		if err != nil {
			respondError(c, http.StatusBadRequest, "workout_id_invalid", nil)
			return
		}
		params.WorkoutID = &wid
	}
	if s := c.Query("sport"); s != "" {
		params.Sport = &s
	}
	if s := c.Query("duration_min"); s != "" {
		v, err := strconv.Atoi(s)
		if err != nil {
			respondError(c, http.StatusBadRequest, "duration_min_invalid", nil)
			return
		}
		params.DurationMin = &v
	}
	if s := c.Query("intensity_zone"); s != "" {
		v, err := strconv.Atoi(s)
		if err != nil {
			respondError(c, http.StatusBadRequest, "intensity_zone_invalid", gin.H{"min": 1, "max": 5})
			return
		}
		params.IntensityZone = &v
	}
	if s := c.Query("body_weight_kg"); s != "" {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			respondError(c, http.StatusBadRequest, "body_weight_kg_invalid", nil)
			return
		}
		params.BodyWeightKgOverride = &v
	}

	out, err := h.svc.RecommendFor(c.Request.Context(), params)
	if err != nil {
		h.respondRecommendError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// respondRecommendError maps service errors to the documented response codes.
// Each ErrXxx sentinel maps 1:1 to a per-endpoint error name; the bodyweight
// resolver's ErrWeightDataMissing is shared with protein-distribution and EA.
func (h *Handlers) respondRecommendError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrInputRequired):
		respondError(c, http.StatusBadRequest, "input_required", nil)
	case errors.Is(err, ErrInputConflict):
		respondError(c, http.StatusBadRequest, "input_conflict", nil)
	case errors.Is(err, ErrSportRequired):
		respondError(c, http.StatusBadRequest, "sport_required", nil)
	case errors.Is(err, ErrDurationMinRequired):
		respondError(c, http.StatusBadRequest, "duration_min_required", nil)
	case errors.Is(err, ErrIntensityZoneRequired):
		respondError(c, http.StatusBadRequest, "intensity_zone_required", nil)
	case errors.Is(err, ErrSportInvalid):
		respondError(c, http.StatusBadRequest, "sport_invalid", nil)
	case errors.Is(err, ErrDurationMinInvalid):
		respondError(c, http.StatusBadRequest, "duration_min_invalid", nil)
	case errors.Is(err, ErrIntensityZoneInvalid):
		respondError(c, http.StatusBadRequest, "intensity_zone_invalid", gin.H{"min": 1, "max": 5})
	case errors.Is(err, ErrBodyWeightInvalid):
		respondError(c, http.StatusBadRequest, "body_weight_kg_invalid", nil)
	case errors.Is(err, bodyweight.ErrWeightDataMissing):
		respondError(c, http.StatusBadRequest, "weight_data_missing", nil)
	case errors.Is(err, workouts.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "workout_not_found"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "recommend_failed"})
	}
}
