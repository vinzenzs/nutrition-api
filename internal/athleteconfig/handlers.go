package athleteconfig

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/vinzenzs/nutrition-api/internal/numfmt"
)

// Handlers exposes GET/PUT /athlete-config.
type Handlers struct {
	svc *Service
}

func NewHandlers(svc *Service) *Handlers {
	return &Handlers{svc: svc}
}

func (h *Handlers) Register(rg *gin.RouterGroup) {
	rg.GET("/athlete-config", h.get)
	rg.PUT("/athlete-config", h.put)
}

// get godoc
// @Summary      Get the athlete physiology configuration (singleton)
// @Description  Returns the single athlete-config row (FTP, thresholds, max HR, lactate-threshold HR, HR-zone and optional power-zone boundaries), or null before any config has been set. Capture-only mirror; these values feed no fueling computation.
// @Tags         athlete-config
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "{\"athlete_config\": AthleteConfig | null}"
// @Security     BearerAuth
// @Router       /athlete-config [get]
func (h *Handlers) get(c *gin.Context) {
	cfg, err := h.svc.Get(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "athlete_config_get_failed"})
		return
	}
	if cfg == nil {
		c.JSON(http.StatusOK, gin.H{"athlete_config": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"athlete_config": round(cfg)})
}

// put godoc
// @Summary      Set or replace the athlete physiology configuration (singleton)
// @Description  Full-replace semantics: absent fields are stored as NULL (cleared), matching PUT /goals. Every field is optional and must be > 0 when present. `Idempotency-Key` is NOT accepted on PUT — supplying the header returns `400 idempotency_unsupported_for_put`. Garmin is source-of-truth: the daily sync re-issues this PUT and overwrites manual edits.
// @Tags         athlete-config
// @Accept       json
// @Produce      json
// @Param        body  body  AthleteConfig  true  "Athlete config payload"
// @Success      200  {object}  map[string]interface{}  "{\"athlete_config\": AthleteConfig}"
// @Failure      400  {object}  map[string]string  "athlete_config_value_invalid | invalid_json | idempotency_unsupported_for_put"
// @Security     BearerAuth
// @Router       /athlete-config [put]
func (h *Handlers) put(c *gin.Context) {
	raw, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
		return
	}
	var cfg AthleteConfig
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &cfg); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
			return
		}
	}
	stored, err := h.svc.Put(c.Request.Context(), &cfg)
	if err != nil {
		var verr *ValidationError
		if errors.As(err, &verr) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "athlete_config_value_invalid", "field": verr.Field})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "athlete_config_upsert_failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"athlete_config": round(stored)})
}

// round returns a copy with the two float fields rounded to 1dp for the
// response; integer fields and storage stay untouched.
func round(cfg *AthleteConfig) *AthleteConfig {
	if cfg == nil {
		return nil
	}
	out := *cfg
	out.ThresholdPaceSecPerKm = numfmt.Round1Ptr(cfg.ThresholdPaceSecPerKm)
	out.ThresholdSwimPaceSecPer100m = numfmt.Round1Ptr(cfg.ThresholdSwimPaceSecPer100m)
	return &out
}
