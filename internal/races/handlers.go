package races

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/vinzenzs/kazper/internal/numfmt"
)

// Handlers wires race endpoints onto a Gin router group.
type Handlers struct {
	svc *Service
}

func NewHandlers(svc *Service) *Handlers {
	return &Handlers{svc: svc}
}

func (h *Handlers) Register(rg *gin.RouterGroup) {
	rg.POST("/races", h.create)
	rg.GET("/races", h.list)
	rg.GET("/races/:id", h.get)
	rg.PATCH("/races/:id", h.patch)
	rg.DELETE("/races/:id", h.delete)
	rg.GET("/races/:id/fueling-plan", h.fuelingPlan)
}

type legRequest struct {
	Ordinal             int      `json:"ordinal"`
	Discipline          string   `json:"discipline"`
	DistanceM           *float64 `json:"distance_m,omitempty"`
	ExpectedDurationMin *int     `json:"expected_duration_min,omitempty"`
	Intensity           *string  `json:"intensity,omitempty"`
}

func (l legRequest) toInput() LegInput {
	return LegInput{
		Ordinal:             l.Ordinal,
		Discipline:          Discipline(l.Discipline),
		DistanceM:           l.DistanceM,
		ExpectedDurationMin: l.ExpectedDurationMin,
		Intensity:           l.Intensity,
	}
}

type createRequest struct {
	Name     string       `json:"name"`
	RaceDate string       `json:"race_date"`
	RaceType *string      `json:"race_type,omitempty"`
	Location *string      `json:"location,omitempty"`
	Notes    *string      `json:"notes,omitempty"`
	Legs     []legRequest `json:"legs,omitempty"`
}

// create godoc
// @Summary      Create a race with legs
// @Description  Persists a race (name + date) and its ordered legs. Each leg is `{ordinal, discipline, distance_m?, expected_duration_min?, intensity?}`; `discipline` is one of swim|bike|run|transition|other and `ordinal` must be unique within the race. This is a planning entity — the per-leg fuelling plan is computed separately via GET /races/{id}/fueling-plan.
// @Tags         races
// @Accept       json
// @Produce      json
// @Param        Idempotency-Key  header  string         false  "Optional client-supplied idempotency key"
// @Param        body             body    createRequest  true   "Race + legs"
// @Success      201  {object}  Race
// @Failure      400  {object}  map[string]string  "race_name_required | race_date_invalid | notes_too_long | leg_ordinal_duplicate | leg_discipline_invalid | leg_expected_duration_min_invalid | leg_distance_m_invalid"
// @Security     BearerAuth
// @Router       /races [post]
func (h *Handlers) create(c *gin.Context) {
	var req createRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_json")
		return
	}
	in := CreateInput{
		Name:     req.Name,
		RaceDate: req.RaceDate,
		RaceType: req.RaceType,
		Location: req.Location,
		Notes:    req.Notes,
		Legs:     toLegInputs(req.Legs),
	}
	race, err := h.svc.Create(c.Request.Context(), in)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, race)
}

// list godoc
// @Summary      List races
// @Tags         races
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "{ races: [...] }"
// @Security     BearerAuth
// @Router       /races [get]
func (h *Handlers) list(c *gin.Context) {
	out, err := h.svc.List(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, "list_failed")
		return
	}
	if out == nil {
		out = []*Race{}
	}
	c.JSON(http.StatusOK, gin.H{"races": out})
}

// get godoc
// @Summary      Get a race with its legs
// @Tags         races
// @Produce      json
// @Param        id  path  string  true  "Race UUID"
// @Success      200  {object}  Race
// @Failure      404  {object}  map[string]string  "race_not_found"
// @Security     BearerAuth
// @Router       /races/{id} [get]
func (h *Handlers) get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusNotFound, "race_not_found")
		return
	}
	race, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, race)
}

type patchRequest struct {
	Name     *string       `json:"name,omitempty"`
	RaceDate *string       `json:"race_date,omitempty"`
	RaceType *string       `json:"race_type,omitempty"`
	Location *string       `json:"location,omitempty"`
	Notes    *string       `json:"notes,omitempty"`
	Legs     *[]legRequest `json:"legs,omitempty"`
}

// patch godoc
// @Summary      Update a race (and optionally replace its legs)
// @Description  Updates the supplied scalar fields. If a `legs` array is present, it REPLACES all existing legs wholesale (an empty array clears them); omit `legs` to leave them unchanged.
// @Tags         races
// @Accept       json
// @Produce      json
// @Param        id    path  string        true  "Race UUID"
// @Param        body  body  patchRequest  true  "Fields to update"
// @Success      200  {object}  Race
// @Failure      400  {object}  map[string]string  "race_name_required | race_date_invalid | notes_too_long | leg_ordinal_duplicate | leg_discipline_invalid | leg_expected_duration_min_invalid | leg_distance_m_invalid"
// @Failure      404  {object}  map[string]string  "race_not_found"
// @Security     BearerAuth
// @Router       /races/{id} [patch]
func (h *Handlers) patch(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusNotFound, "race_not_found")
		return
	}
	var req patchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_json")
		return
	}
	in := UpdateInput{
		Name:     req.Name,
		RaceDate: req.RaceDate,
		RaceType: req.RaceType,
		Location: req.Location,
		Notes:    req.Notes,
	}
	if req.Legs != nil {
		legs := toLegInputs(*req.Legs)
		in.Legs = &legs
	}
	race, err := h.svc.Update(c.Request.Context(), id, in)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, race)
}

// delete godoc
// @Summary      Delete a race (legs cascade)
// @Tags         races
// @Param        id  path  string  true  "Race UUID"
// @Success      204  "no content"
// @Failure      404  {object}  map[string]string  "race_not_found"
// @Security     BearerAuth
// @Router       /races/{id} [delete]
func (h *Handlers) delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusNotFound, "race_not_found")
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		respondServiceError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// fuelingPlan godoc
// @Summary      Compute the per-leg fuelling plan for a race
// @Description  Deterministic per-leg in-event fuelling baseline computed on read (not stored). Carbs band by total race duration (<75 min → 0, 75–150 → 60, ≥150 → 90 g/hr) and scale by discipline intake capacity (swim/transition 0, bike 1.0, run 0.7, other 0.8). Fluid and sodium derive from `sweat_rate_ml_per_hr` when supplied (fluid capped at 1000 ml/hr; sodium = sweat_rate/1000 × 800 mg/L), else a flagged 600 ml/hr and 600 mg/hr. Carbs (g), sodium (mg) and fluid (ml) are reported as distinct unit fields.
// @Tags         races
// @Produce      json
// @Param        id                   path   string  true   "Race UUID"
// @Param        body_weight_kg       query  number  true   "Athlete body weight in kilograms, 30..200"
// @Param        sweat_rate_ml_per_hr query  number  false  "Measured sweat rate in ml/hr; personalises fluid and sodium"
// @Success      200  {object}  FuelingPlan
// @Failure      400  {object}  map[string]string  "body_weight_kg_required | body_weight_kg_out_of_range | sweat_rate_out_of_range | sweat_rate_invalid"
// @Failure      404  {object}  map[string]string  "race_not_found"
// @Security     BearerAuth
// @Router       /races/{id}/fueling-plan [get]
func (h *Handlers) fuelingPlan(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusNotFound, "race_not_found")
		return
	}
	bwStr := c.Query("body_weight_kg")
	if bwStr == "" {
		respondError(c, http.StatusBadRequest, "body_weight_kg_required")
		return
	}
	bw, err := strconv.ParseFloat(bwStr, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "body_weight_kg_out_of_range")
		return
	}
	p := FuelingParams{BodyWeightKg: bw}
	if s := c.Query("sweat_rate_ml_per_hr"); s != "" {
		sr, err := strconv.ParseFloat(s, 64)
		if err != nil {
			respondError(c, http.StatusBadRequest, "sweat_rate_invalid")
			return
		}
		p.SweatRateMlPerHr = &sr
	}
	plan, err := h.svc.PlanFueling(c.Request.Context(), id, p)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, roundPlan(plan))
}

// ----- helpers -----

func toLegInputs(reqs []legRequest) []LegInput {
	out := make([]LegInput, 0, len(reqs))
	for _, l := range reqs {
		out = append(out, l.toInput())
	}
	return out
}

func roundPlan(p *FuelingPlan) *FuelingPlan {
	if p == nil {
		return nil
	}
	for _, l := range p.Legs {
		l.CarbsGPerHr = numfmt.Round1(l.CarbsGPerHr)
		l.CarbsGTotal = numfmt.Round1(l.CarbsGTotal)
		l.SodiumMgPerHr = numfmt.Round1(l.SodiumMgPerHr)
		l.SodiumMgTotal = numfmt.Round1(l.SodiumMgTotal)
		l.FluidMlPerHr = numfmt.Round1(l.FluidMlPerHr)
		l.FluidMlTotal = numfmt.Round1(l.FluidMlTotal)
	}
	p.Total.CarbsGTotal = numfmt.Round1(p.Total.CarbsGTotal)
	p.Total.SodiumMgTotal = numfmt.Round1(p.Total.SodiumMgTotal)
	p.Total.FluidMlTotal = numfmt.Round1(p.Total.FluidMlTotal)
	p.BodyWeightKg = numfmt.Round1(p.BodyWeightKg)
	return p
}

func respondError(c *gin.Context, status int, code string) {
	c.JSON(status, gin.H{"error": code})
}

func respondServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		respondError(c, http.StatusNotFound, "race_not_found")
	case errors.Is(err, ErrNameRequired):
		respondError(c, http.StatusBadRequest, "race_name_required")
	case errors.Is(err, ErrRaceDateInvalid):
		respondError(c, http.StatusBadRequest, "race_date_invalid")
	case errors.Is(err, ErrNotesTooLong):
		respondError(c, http.StatusBadRequest, "notes_too_long")
	case errors.Is(err, ErrLegOrdinalDuplicate):
		respondError(c, http.StatusBadRequest, "leg_ordinal_duplicate")
	case errors.Is(err, ErrLegDisciplineInvalid):
		respondError(c, http.StatusBadRequest, "leg_discipline_invalid")
	case errors.Is(err, ErrLegDurationInvalid):
		respondError(c, http.StatusBadRequest, "leg_expected_duration_min_invalid")
	case errors.Is(err, ErrLegDistanceInvalid):
		respondError(c, http.StatusBadRequest, "leg_distance_m_invalid")
	case errors.Is(err, ErrBodyWeightRequired):
		respondError(c, http.StatusBadRequest, "body_weight_kg_required")
	case errors.Is(err, ErrBodyWeightRange):
		respondError(c, http.StatusBadRequest, "body_weight_kg_out_of_range")
	case errors.Is(err, ErrSweatRateRange):
		respondError(c, http.StatusBadRequest, "sweat_rate_out_of_range")
	default:
		respondError(c, http.StatusInternalServerError, "write_failed")
	}
}
