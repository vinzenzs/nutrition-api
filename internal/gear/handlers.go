package gear

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/vinzenzs/nutrition-api/internal/numfmt"
)

// Handlers wires the gear endpoints.
type Handlers struct {
	svc *Service
}

func NewHandlers(svc *Service) *Handlers {
	return &Handlers{svc: svc}
}

func (h *Handlers) Register(rg *gin.RouterGroup) {
	rg.POST("/gear", h.upsert)
	rg.GET("/gear", h.list)
	rg.GET("/gear/:id", h.get)
}

// upsertRequest mirrors the POST /gear body. external_id/gear_type/display_name
// are required; the rest are optional.
type upsertRequest struct {
	ExternalID      string   `json:"external_id"`
	GearType        string   `json:"gear_type"`
	DisplayName     string   `json:"display_name"`
	TotalDistanceM  *float64 `json:"total_distance_m,omitempty"`
	TotalActivities *int     `json:"total_activities,omitempty"`
	Retired         bool     `json:"retired,omitempty"`
	DateBegin       *string  `json:"date_begin,omitempty"`
	DateEnd         *string  `json:"date_end,omitempty"`
}

// upsert godoc
// @Summary      Upsert a gear record (by Garmin gear id)
// @Description  Creates or updates a piece of gear, upserting by `external_id` (the Garmin gear uuid). Slowly-changing inventory — re-observing the same gear updates it in place. Standard `Idempotency-Key` header supported. Gear is coaching context only; its distance never feeds nutrition computation.
// @Tags         gear
// @Accept       json
// @Produce      json
// @Param        Idempotency-Key  header  string         false  "Optional client-supplied idempotency key"
// @Param        body             body    upsertRequest  true   "Gear record"
// @Success      201  {object}  Gear  "INSERT"
// @Success      200  {object}  Gear  "UPDATE (external_id already present)"
// @Failure      400  {object}  map[string]string  "external_id_required | gear_type_invalid | display_name_required | total_distance_m_invalid | total_activities_invalid"
// @Security     BearerAuth
// @Router       /gear [post]
func (h *Handlers) upsert(c *gin.Context) {
	var req upsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_json")
		return
	}
	g := &Gear{
		ExternalID:      req.ExternalID,
		GearType:        Type(req.GearType),
		DisplayName:     req.DisplayName,
		TotalDistanceM:  req.TotalDistanceM,
		TotalActivities: req.TotalActivities,
		Retired:         req.Retired,
		DateBegin:       req.DateBegin,
		DateEnd:         req.DateEnd,
	}
	out, created, err := h.svc.Upsert(c.Request.Context(), g)
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
// @Summary      List gear records
// @Tags         gear
// @Produce      json
// @Param        retired  query  bool  false  "Filter by retirement state (true|false)"
// @Success      200  {object}  map[string]interface{}  "{ gear: [...] }"
// @Security     BearerAuth
// @Router       /gear [get]
func (h *Handlers) list(c *gin.Context) {
	var retired *bool
	if v := c.Query("retired"); v != "" {
		b := v == "true" || v == "1"
		retired = &b
	}
	rows, err := h.svc.List(c.Request.Context(), retired)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "list_failed")
		return
	}
	out := make([]*Gear, 0, len(rows))
	for _, g := range rows {
		out = append(out, round(g))
	}
	c.JSON(http.StatusOK, gin.H{"gear": out})
}

// get godoc
// @Summary      Get a gear record by id
// @Tags         gear
// @Produce      json
// @Param        id   path  string  true  "Gear UUID"
// @Success      200  {object}  Gear
// @Failure      404  {object}  map[string]string  "gear_not_found"
// @Security     BearerAuth
// @Router       /gear/{id} [get]
func (h *Handlers) get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusNotFound, "gear_not_found")
		return
	}
	out, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respondError(c, http.StatusNotFound, "gear_not_found")
			return
		}
		respondError(c, http.StatusInternalServerError, "get_failed")
		return
	}
	c.JSON(http.StatusOK, round(out))
}

// ----- helpers -----

func respondError(c *gin.Context, status int, code string) {
	c.JSON(status, gin.H{"error": code})
}

func respondServiceError(c *gin.Context, err error) {
	for _, e := range []error{
		ErrExternalIDRequired, ErrGearTypeInvalid, ErrDisplayNameRequired,
		ErrTotalDistanceMInvalid, ErrTotalActivitiesInvalid,
	} {
		if errors.Is(err, e) {
			respondError(c, http.StatusBadRequest, e.Error())
			return
		}
	}
	respondError(c, http.StatusInternalServerError, "write_failed")
}

// round applies 1dp rounding to total_distance_m at the boundary.
func round(g *Gear) *Gear {
	if g == nil {
		return nil
	}
	out := *g
	out.TotalDistanceM = numfmt.Round1Ptr(g.TotalDistanceM)
	return &out
}
