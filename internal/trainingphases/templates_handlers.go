package trainingphases

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/vinzenzs/nutrition-api/internal/goals"
	"github.com/vinzenzs/nutrition-api/internal/numfmt"
)

// TemplatesHandlers wires PUT/GET/DELETE for /goal-templates.
type TemplatesHandlers struct {
	svc *TemplatesService
}

func NewTemplatesHandlers(svc *TemplatesService) *TemplatesHandlers {
	return &TemplatesHandlers{svc: svc}
}

func (h *TemplatesHandlers) Register(rg *gin.RouterGroup) {
	rg.PUT("/goal-templates/:name", h.put)
	rg.GET("/goal-templates", h.list)
	rg.GET("/goal-templates/:name", h.get)
	rg.DELETE("/goal-templates/:name", h.delete)
}

// templateBody is the JSON shape PUT /goal-templates/{name} accepts. The
// nutrient bound fields match goals.Goals exactly; `notes` is a sibling.
// The URL `:name` is the canonical identifier; this struct does NOT carry
// a name field, so DisallowUnknownFields rejects accidental `name` in body.
type templateBody struct {
	Notes *string `json:"notes,omitempty"`

	Kcal *goals.Range `json:"kcal,omitempty"`

	ProteinG *goals.Range `json:"protein_g,omitempty"`
	CarbsG   *goals.Range `json:"carbs_g,omitempty"`
	FatG     *goals.Range `json:"fat_g,omitempty"`

	FiberG *goals.Range `json:"fiber_g,omitempty"`
	SugarG *goals.Range `json:"sugar_g,omitempty"`
	SaltG  *goals.Range `json:"salt_g,omitempty"`

	IronMg        *goals.Range `json:"iron_mg,omitempty"`
	CalciumMg     *goals.Range `json:"calcium_mg,omitempty"`
	VitaminDMcg   *goals.Range `json:"vitamin_d_mcg,omitempty"`
	VitaminB12Mcg *goals.Range `json:"vitamin_b12_mcg,omitempty"`
	VitaminCMg    *goals.Range `json:"vitamin_c_mg,omitempty"`
	MagnesiumMg   *goals.Range `json:"magnesium_mg,omitempty"`
	PotassiumMg   *goals.Range `json:"potassium_mg,omitempty"`
	ZincMg        *goals.Range `json:"zinc_mg,omitempty"`
}

// put godoc
// @Summary      Upsert a named goal template
// @Description  Full-replace semantics: absent nutrient bounds are stored as NULL. The template's `name` is the URL path segment (kebab-case-ish, user-chosen). `Idempotency-Key` is NOT accepted on PUT.
// @Tags         training-phases
// @Accept       json
// @Produce      json
// @Param        name  path  string  true  "Template name"
// @Param        body  body  templateBody  true  "Template bounds"
// @Success      200   {object}  map[string]interface{}  "{\"template\": Template}"
// @Failure      400   {object}  map[string]interface{}  "template_name_invalid | template_name_too_long | goal_value_invalid | goal_range_invalid | invalid_json | idempotency_unsupported_for_put"
// @Security     BearerAuth
// @Router       /goal-templates/{name} [put]
func (h *TemplatesHandlers) put(c *gin.Context) {
	name := c.Param("name")
	raw, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
		return
	}
	var body templateBody
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
		return
	}
	t := &Template{
		Name:          name,
		Notes:         body.Notes,
		Kcal:          body.Kcal,
		ProteinG:      body.ProteinG,
		CarbsG:        body.CarbsG,
		FatG:          body.FatG,
		FiberG:        body.FiberG,
		SugarG:        body.SugarG,
		SaltG:         body.SaltG,
		IronMg:        body.IronMg,
		CalciumMg:     body.CalciumMg,
		VitaminDMcg:   body.VitaminDMcg,
		VitaminB12Mcg: body.VitaminB12Mcg,
		VitaminCMg:    body.VitaminCMg,
		MagnesiumMg:   body.MagnesiumMg,
		PotassiumMg:   body.PotassiumMg,
		ZincMg:        body.ZincMg,
	}
	stored, err := h.svc.Upsert(c.Request.Context(), name, t)
	if err != nil {
		respondTemplateError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"template": roundTemplate(stored)})
}

// get godoc
// @Summary      Get a named goal template
// @Tags         training-phases
// @Produce      json
// @Param        name  path  string  true  "Template name"
// @Success      200   {object}  map[string]interface{}  "{\"template\": Template}"
// @Failure      404   {object}  map[string]string  "template_not_found"
// @Security     BearerAuth
// @Router       /goal-templates/{name} [get]
func (h *TemplatesHandlers) get(c *gin.Context) {
	t, err := h.svc.GetByName(c.Request.Context(), c.Param("name"))
	if err != nil {
		if errors.Is(err, ErrTemplateNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "template_not_found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "template_get_failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"template": roundTemplate(t)})
}

// list godoc
// @Summary      List all goal templates
// @Tags         training-phases
// @Produce      json
// @Success      200   {object}  map[string]interface{}  "{\"templates\": [Template]}"
// @Security     BearerAuth
// @Router       /goal-templates [get]
func (h *TemplatesHandlers) list(c *gin.Context) {
	ts, err := h.svc.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "template_list_failed"})
		return
	}
	out := make([]*Template, 0, len(ts))
	for _, t := range ts {
		out = append(out, roundTemplate(t))
	}
	c.JSON(http.StatusOK, gin.H{"templates": out})
}

// delete godoc
// @Summary      Delete a goal template
// @Description  Refused with 409 template_in_use if any phase references the template via default_template_id.
// @Tags         training-phases
// @Produce      json
// @Param        name  path  string  true  "Template name"
// @Success      204   "no content"
// @Failure      404   {object}  map[string]string  "template_not_found"
// @Failure      409   {object}  map[string]interface{}  "template_in_use with referencing_phases array"
// @Security     BearerAuth
// @Router       /goal-templates/{name} [delete]
func (h *TemplatesHandlers) delete(c *gin.Context) {
	err := h.svc.Delete(c.Request.Context(), c.Param("name"))
	if err == nil {
		c.Status(http.StatusNoContent)
		return
	}
	respondTemplateError(c, err)
}

// respondTemplateError maps service-level errors to HTTP status + body.
func respondTemplateError(c *gin.Context, err error) {
	var verr *goals.ValidationError
	if errors.As(err, &verr) {
		c.JSON(http.StatusBadRequest, gin.H{"error": verr.Code, "field": verr.Field})
		return
	}
	var inUse *InUseError
	if errors.As(err, &inUse) {
		c.JSON(http.StatusConflict, gin.H{
			"error":              "template_in_use",
			"referencing_phases": inUse.ReferencingPhases,
		})
		return
	}
	switch {
	case errors.Is(err, ErrTemplateNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "template_not_found"})
	case errors.Is(err, ErrTemplateNameInvalid):
		c.JSON(http.StatusBadRequest, gin.H{"error": "template_name_invalid"})
	case errors.Is(err, ErrTemplateNameTooLong):
		c.JSON(http.StatusBadRequest, gin.H{"error": "template_name_too_long", "max_length": MaxTemplateNameLength})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "template_write_failed"})
	}
}

// roundTemplate returns a copy of t with every Range bound rounded to 1 dp
// for response presentation. Storage stays at full precision.
func roundTemplate(t *Template) *Template {
	if t == nil {
		return nil
	}
	out := *t
	round := func(r *goals.Range) *goals.Range {
		if r == nil {
			return nil
		}
		return &goals.Range{Min: numfmt.Round1Ptr(r.Min), Max: numfmt.Round1Ptr(r.Max)}
	}
	out.Kcal = round(t.Kcal)
	out.ProteinG = round(t.ProteinG)
	out.CarbsG = round(t.CarbsG)
	out.FatG = round(t.FatG)
	out.FiberG = round(t.FiberG)
	out.SugarG = round(t.SugarG)
	out.SaltG = round(t.SaltG)
	out.IronMg = round(t.IronMg)
	out.CalciumMg = round(t.CalciumMg)
	out.VitaminDMcg = round(t.VitaminDMcg)
	out.VitaminB12Mcg = round(t.VitaminB12Mcg)
	out.VitaminCMg = round(t.VitaminCMg)
	out.MagnesiumMg = round(t.MagnesiumMg)
	out.PotassiumMg = round(t.PotassiumMg)
	out.ZincMg = round(t.ZincMg)
	return &out
}
