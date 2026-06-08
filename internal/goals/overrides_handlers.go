package goals

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	maxOverrideRangeDays = 366
	dateFormat           = "2006-01-02"
)

// OverridesHandlers wires the per-date override CRUD endpoints.
type OverridesHandlers struct {
	repo *OverridesRepo
}

func NewOverridesHandlers(repo *OverridesRepo) *OverridesHandlers {
	return &OverridesHandlers{repo: repo}
}

func (h *OverridesHandlers) Register(rg *gin.RouterGroup) {
	rg.GET("/goals/overrides", h.list)
	rg.PUT("/goals/overrides/:date", h.put)
	rg.GET("/goals/overrides/:date", h.get)
	rg.DELETE("/goals/overrides/:date", h.delete)
}

// put godoc
// @Summary      Upsert a daily goal override
// @Description  Full-replace semantics: absent fields are stored as NULL. Same validation as PUT /goals. `Idempotency-Key` is NOT accepted on PUT — supplying it returns `400 idempotency_unsupported_for_put`.
// @Tags         goals
// @Accept       json
// @Produce      json
// @Param        date  path  string  true  "Override date in YYYY-MM-DD"
// @Param        body  body  Goals   true  "Goals payload"
// @Success      200   {object}  map[string]interface{}  "{\"goals\": Goals}"
// @Failure      400   {object}  map[string]string  "date_invalid | goal_value_invalid | goal_range_invalid | invalid_json | idempotency_unsupported_for_put"
// @Security     BearerAuth
// @Router       /goals/overrides/{date} [put]
func (h *OverridesHandlers) put(c *gin.Context) {
	date, err := time.Parse(dateFormat, c.Param("date"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date_invalid"})
		return
	}
	raw, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
		return
	}
	var g Goals
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&g); err != nil {
		if field, ok := unknownField(err); ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "goal_value_invalid", "field": field})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
		return
	}
	if err := validateGoals(&g); err != nil {
		var verr *ValidationError
		if errors.As(err, &verr) {
			c.JSON(http.StatusBadRequest, gin.H{"error": verr.Code, "field": verr.Field})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.repo.Upsert(c.Request.Context(), date, &g); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "override_upsert_failed"})
		return
	}
	stored, err := h.repo.GetOverride(c.Request.Context(), date)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "override_get_failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"goals": roundGoals(stored)})
}

// get godoc
// @Summary      Get a daily goal override for a specific date
// @Tags         goals
// @Produce      json
// @Param        date  path  string  true  "Override date in YYYY-MM-DD"
// @Success      200  {object}  map[string]interface{}  "{\"goals\": Goals}"
// @Failure      400  {object}  map[string]string  "date_invalid"
// @Failure      404  {object}  map[string]string  "override_not_found"
// @Security     BearerAuth
// @Router       /goals/overrides/{date} [get]
func (h *OverridesHandlers) get(c *gin.Context) {
	date, err := time.Parse(dateFormat, c.Param("date"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date_invalid"})
		return
	}
	g, err := h.repo.GetOverride(c.Request.Context(), date)
	if err != nil {
		if errors.Is(err, ErrOverrideNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "override_not_found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "override_get_failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"goals": roundGoals(g)})
}

// delete godoc
// @Summary      Delete a daily goal override
// @Tags         goals
// @Param        date  path  string  true  "Override date in YYYY-MM-DD"
// @Success      204  "no content"
// @Failure      400  {object}  map[string]string  "date_invalid"
// @Failure      404  {object}  map[string]string  "override_not_found"
// @Security     BearerAuth
// @Router       /goals/overrides/{date} [delete]
func (h *OverridesHandlers) delete(c *gin.Context) {
	date, err := time.Parse(dateFormat, c.Param("date"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date_invalid"})
		return
	}
	if err := h.repo.Delete(c.Request.Context(), date); err != nil {
		if errors.Is(err, ErrOverrideNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "override_not_found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "override_delete_failed"})
		return
	}
	c.Status(http.StatusNoContent)
}

// list godoc
// @Summary      List daily goal overrides in a date range
// @Description  Returns overrides whose `date` falls within `[from, to]` (inclusive). Dates without an override are omitted (the caller can infer the default applies).
// @Tags         goals
// @Produce      json
// @Param        from  query  string  true   "Inclusive start date YYYY-MM-DD"
// @Param        to    query  string  true   "Inclusive end date YYYY-MM-DD; max 366 days from `from`"
// @Success      200  {object}  map[string]interface{}  "{\"overrides\": [{date, goals}]}"
// @Failure      400  {object}  map[string]interface{}  "range_required | date_invalid | range_invalid | range_too_large"
// @Security     BearerAuth
// @Router       /goals/overrides [get]
func (h *OverridesHandlers) list(c *gin.Context) {
	fromStr := c.Query("from")
	toStr := c.Query("to")
	if fromStr == "" || toStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "range_required"})
		return
	}
	from, err := time.Parse(dateFormat, fromStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date_invalid"})
		return
	}
	to, err := time.Parse(dateFormat, toStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date_invalid"})
		return
	}
	if from.After(to) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "range_invalid"})
		return
	}
	if days := int(to.Sub(from).Hours()/24) + 1; days > maxOverrideRangeDays {
		c.JSON(http.StatusBadRequest, gin.H{"error": "range_too_large", "max_days": maxOverrideRangeDays})
		return
	}
	overrides, err := h.repo.List(c.Request.Context(), from, to)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "override_list_failed"})
		return
	}
	type entry struct {
		Date  string `json:"date"`
		Goals *Goals `json:"goals"`
	}
	out := make([]entry, 0, len(overrides))
	for _, o := range overrides {
		out = append(out, entry{
			Date:  o.Date.Format(dateFormat),
			Goals: roundGoals(o.Goals),
		})
	}
	c.JSON(http.StatusOK, gin.H{"overrides": out})
}
