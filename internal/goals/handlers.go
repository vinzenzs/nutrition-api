package goals

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/vinzenzs/nutrition-api/internal/numfmt"
)

// Handlers exposes GET/PUT /goals.
type Handlers struct {
	repo *Repo
}

func NewHandlers(repo *Repo) *Handlers {
	return &Handlers{repo: repo}
}

func (h *Handlers) Register(rg *gin.RouterGroup) {
	rg.GET("/goals", h.get)
	rg.PUT("/goals", h.put)
}

// get godoc
// @Summary      Get current nutrition goals
// @Tags         goals
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "{\"goals\": Goals | null}"
// @Security     BearerAuth
// @Router       /goals [get]
func (h *Handlers) get(c *gin.Context) {
	g, err := h.repo.Get(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "goals_get_failed"})
		return
	}
	if g == nil {
		c.JSON(http.StatusOK, gin.H{"goals": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"goals": roundGoals(g)})
}

// roundGoals returns a copy of g with every Range bound rounded to 1 dp for
// response presentation. Storage stays at full precision.
func roundGoals(g *Goals) *Goals {
	if g == nil {
		return nil
	}
	out := *g
	round := func(r *Range) *Range {
		if r == nil {
			return nil
		}
		return &Range{Min: numfmt.Round1Ptr(r.Min), Max: numfmt.Round1Ptr(r.Max)}
	}
	out.Kcal = round(g.Kcal)
	out.ProteinG = round(g.ProteinG)
	out.CarbsG = round(g.CarbsG)
	out.FatG = round(g.FatG)
	out.FiberG = round(g.FiberG)
	out.SugarG = round(g.SugarG)
	out.SaltG = round(g.SaltG)
	out.IronMg = round(g.IronMg)
	out.CalciumMg = round(g.CalciumMg)
	out.VitaminDMcg = round(g.VitaminDMcg)
	out.VitaminB12Mcg = round(g.VitaminB12Mcg)
	out.VitaminCMg = round(g.VitaminCMg)
	out.MagnesiumMg = round(g.MagnesiumMg)
	out.PotassiumMg = round(g.PotassiumMg)
	out.ZincMg = round(g.ZincMg)
	return &out
}

// put godoc
// @Summary      Set or replace nutrition goals
// @Description  Full-replace semantics: absent fields are stored as NULL (cleared). Each goal is `{min?, max?}` with at least one bound; ranges must satisfy min <= max. Note: `Idempotency-Key` is NOT accepted on PUT — supplying the header returns `400 idempotency_unsupported_for_put`. ETag/If-Match retry-safety is forward-pointed but not implemented.
// @Tags         goals
// @Accept       json
// @Produce      json
// @Param        body  body  Goals  true  "Goals payload"
// @Success      200   {object}  map[string]interface{}  "{\"goals\": Goals}"
// @Failure      400   {object}  map[string]string  "goal_value_invalid | goal_range_invalid | invalid_json | idempotency_unsupported_for_put"
// @Security     BearerAuth
// @Router       /goals [put]
func (h *Handlers) put(c *gin.Context) {
	raw, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
		return
	}
	var g Goals
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&g); err != nil {
		// Detect the legacy kcal_target case so we give the spec-required
		// 400 goal_value_invalid + field hint, rather than a generic
		// invalid_json. Any other unknown field is also rejected as
		// goal_value_invalid so callers learn the contract quickly.
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
	if err := h.repo.Upsert(c.Request.Context(), &g); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "goals_upsert_failed"})
		return
	}
	// Read back so the response reflects stored state (e.g. timestamps).
	stored, err := h.repo.Get(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "goals_get_failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"goals": roundGoals(stored)})
}

// unknownField parses the error message produced by encoding/json's
// DisallowUnknownFields and returns the offending field name.
func unknownField(err error) (string, bool) {
	if err == nil {
		return "", false
	}
	msg := err.Error()
	const prefix = `json: unknown field "`
	if i := strings.Index(msg, prefix); i >= 0 {
		rest := msg[i+len(prefix):]
		if j := strings.Index(rest, `"`); j >= 0 {
			return rest[:j], true
		}
	}
	return "", false
}

// ValidationError carries the spec-defined error code plus the offending field
// name for clients (and the agent) to act on.
type ValidationError struct {
	Code  string
	Field string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Field)
}

func validateGoals(g *Goals) error {
	check := func(name string, r *Range) error {
		if r == nil {
			return nil
		}
		// At least one bound must be present — empty {} is rejected.
		if r.Min == nil && r.Max == nil {
			return &ValidationError{Code: "goal_value_invalid", Field: name}
		}
		if r.Min != nil {
			if math.IsNaN(*r.Min) || math.IsInf(*r.Min, 0) || *r.Min < 0 {
				return &ValidationError{Code: "goal_value_invalid", Field: name + ".min"}
			}
		}
		if r.Max != nil {
			if math.IsNaN(*r.Max) || math.IsInf(*r.Max, 0) || *r.Max < 0 {
				return &ValidationError{Code: "goal_value_invalid", Field: name + ".max"}
			}
		}
		if r.Min != nil && r.Max != nil && *r.Min > *r.Max {
			return &ValidationError{Code: "goal_range_invalid", Field: name}
		}
		return nil
	}
	for _, pair := range []struct {
		name  string
		field *Range
	}{
		{"kcal", g.Kcal},
		{"protein_g", g.ProteinG},
		{"carbs_g", g.CarbsG},
		{"fat_g", g.FatG},
		{"fiber_g", g.FiberG},
		{"sugar_g", g.SugarG},
		{"salt_g", g.SaltG},
		{"iron_mg", g.IronMg},
		{"calcium_mg", g.CalciumMg},
		{"vitamin_d_mcg", g.VitaminDMcg},
		{"vitamin_b12_mcg", g.VitaminB12Mcg},
		{"vitamin_c_mg", g.VitaminCMg},
		{"magnesium_mg", g.MagnesiumMg},
		{"potassium_mg", g.PotassiumMg},
		{"zinc_mg", g.ZincMg},
	} {
		if err := check(pair.name, pair.field); err != nil {
			return err
		}
	}
	return nil
}
