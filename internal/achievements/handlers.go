package achievements

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/vinzenzs/kazper/internal/numfmt"
)

// Handlers wires the achievements endpoints.
type Handlers struct {
	svc *Service
}

func NewHandlers(svc *Service) *Handlers {
	return &Handlers{svc: svc}
}

func (h *Handlers) Register(rg *gin.RouterGroup) {
	rg.POST("/achievements", h.upsert)
	rg.GET("/achievements", h.list)
}

type upsertRequest struct {
	ExternalID  string     `json:"external_id"`
	Kind        string     `json:"kind"`
	Name        string     `json:"name"`
	EarnedAt    *time.Time `json:"earned_at,omitempty"`
	ProgressPct *float64   `json:"progress_pct,omitempty"`
}

// upsert godoc
// @Summary      Upsert an achievement (by Garmin badge/challenge id)
// @Description  Creates or updates an earned badge or ad-hoc challenge, upserting by `external_id`. Coaching context only; never feeds nutrition computation. Personal records live in the separate personal-records capability.
// @Tags         achievements
// @Accept       json
// @Produce      json
// @Param        Idempotency-Key  header  string         false  "Optional client-supplied idempotency key"
// @Param        body             body    upsertRequest  true   "Achievement"
// @Success      201  {object}  Achievement  "INSERT"
// @Success      200  {object}  Achievement  "UPDATE (external_id already present)"
// @Failure      400  {object}  map[string]string  "external_id_required | kind_invalid | name_required | progress_pct_invalid"
// @Security     BearerAuth
// @Router       /achievements [post]
func (h *Handlers) upsert(c *gin.Context) {
	var req upsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_json")
		return
	}
	a := &Achievement{
		ExternalID:  req.ExternalID,
		Kind:        Kind(req.Kind),
		Name:        req.Name,
		EarnedAt:    req.EarnedAt,
		ProgressPct: req.ProgressPct,
	}
	out, created, err := h.svc.Upsert(c.Request.Context(), a)
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
// @Summary      List achievements
// @Tags         achievements
// @Produce      json
// @Param        kind  query  string  false  "Filter by kind: badge | challenge"
// @Success      200  {object}  map[string]interface{}  "{ achievements: [...] }"
// @Security     BearerAuth
// @Router       /achievements [get]
func (h *Handlers) list(c *gin.Context) {
	var kind *string
	if v := c.Query("kind"); v != "" {
		kind = &v
	}
	rows, err := h.svc.List(c.Request.Context(), kind)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "list_failed")
		return
	}
	out := make([]*Achievement, 0, len(rows))
	for _, a := range rows {
		out = append(out, round(a))
	}
	c.JSON(http.StatusOK, gin.H{"achievements": out})
}

// ----- helpers -----

func respondError(c *gin.Context, status int, code string) {
	c.JSON(status, gin.H{"error": code})
}

func respondServiceError(c *gin.Context, err error) {
	for _, e := range []error{ErrExternalIDRequired, ErrKindInvalid, ErrNameRequired, ErrProgressPctInvalid} {
		if errors.Is(err, e) {
			respondError(c, http.StatusBadRequest, e.Error())
			return
		}
	}
	respondError(c, http.StatusInternalServerError, "write_failed")
}

// round applies 1dp rounding to progress_pct at the boundary.
func round(a *Achievement) *Achievement {
	if a == nil {
		return nil
	}
	out := *a
	out.ProgressPct = numfmt.Round1Ptr(a.ProgressPct)
	return &out
}
