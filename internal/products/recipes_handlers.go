package products

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/vinzenzs/nutrition-api/internal/cookidoo"
)

// ----- POST /products/recipes -----

type createRecipeRequest struct {
	Name         string                       `json:"name"`
	ServingSizeG *float64                     `json:"serving_size_g,omitempty"`
	Components   []createRecipeComponentBody `json:"components"`
}

type createRecipeComponentBody struct {
	ProductID string  `json:"product_id"`
	QuantityG float64 `json:"quantity_g"`
}

type recipeComponentResponse struct {
	ProductID                 uuid.UUID  `json:"product_id"`
	Name                      string     `json:"name"`
	QuantityG                 float64    `json:"quantity_g"`
	EffectiveNutrimentsPer100g Nutriments `json:"effective_nutriments_per_100g"`
}

type createRecipeResponse struct {
	*Product
	Components []recipeComponentResponse `json:"components"`
}

// createRecipe godoc
// @Summary      Create a composite recipe product
// @Description  Builds a recipe from N component products and per-component grams. Nutriments-per-100g are computed at creation time from components, so existing meal-logging math works unchanged.
// @Tags         products
// @Accept       json
// @Produce      json
// @Param        Idempotency-Key  header  string                false  "Optional client-supplied idempotency key"
// @Param        body             body    createRecipeRequest   true   "Recipe fields"
// @Success      201  {object}  createRecipeResponse
// @Failure      400  {object}  map[string]string  "components_required | component_quantity_g_invalid | recipe_as_component_not_supported | invalid_json"
// @Failure      404  {object}  map[string]string  "component_not_found"
// @Security     BearerAuth
// @Router       /products/recipes [post]
func (h *Handlers) createRecipe(c *gin.Context) {
	var req createRecipeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
		return
	}
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name_required"})
		return
	}
	if len(req.Components) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "components_required"})
		return
	}

	in := CreateRecipeInput{
		Name:         req.Name,
		ServingSizeG: req.ServingSizeG,
		Components:   make([]RecipeComponentInput, 0, len(req.Components)),
	}
	for _, ci := range req.Components {
		pid, err := uuid.Parse(ci.ProductID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "component_not_found", "product_id": ci.ProductID})
			return
		}
		in.Components = append(in.Components, RecipeComponentInput{ProductID: pid, QuantityG: ci.QuantityG})
	}

	result, err := h.svc.CreateRecipe(c.Request.Context(), in)
	if err != nil {
		switch {
		case errors.Is(err, ErrComponentsRequired):
			c.JSON(http.StatusBadRequest, gin.H{"error": "components_required"})
		case errors.Is(err, ErrRecipeAsComponent):
			c.JSON(http.StatusBadRequest, gin.H{"error": "recipe_as_component_not_supported"})
		default:
			var dup *ErrDuplicateComponent
			if errors.As(err, &dup) {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":       "component_duplicate",
					"product_id":  dup.ProductID.String(),
					"occurrences": dup.Occurrences,
					"hint":        "sum the quantities and supply one entry per product",
				})
				return
			}
			var cqi *ErrComponentQuantityInvalid
			if errors.As(err, &cqi) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "component_quantity_g_invalid", "product_id": cqi.ProductID.String()})
				return
			}
			var cnf *ErrComponentNotFound
			if errors.As(err, &cnf) {
				c.JSON(http.StatusNotFound, gin.H{"error": "component_not_found", "product_id": cnf.ProductID.String()})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "create_recipe_failed"})
		}
		return
	}

	c.JSON(http.StatusCreated, buildRecipeResponse(result))
}

// ----- POST /products/recipes/{id}/recompute -----

// recomputeRecipe godoc
// @Summary      Recompute a recipe's nutriments from its current components
// @Tags         products
// @Produce      json
// @Param        id   path  string  true  "Recipe product UUID"
// @Success      200  {object}  createRecipeResponse
// @Failure      400  {object}  map[string]string  "not_a_recipe"
// @Failure      404  {object}  map[string]string  "product_not_found"
// @Security     BearerAuth
// @Router       /products/recipes/{id}/recompute [post]
func (h *Handlers) recomputeRecipe(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "product_not_found"})
		return
	}
	result, err := h.svc.RecomputeRecipe(c.Request.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "product_not_found"})
		case errors.Is(err, ErrNotARecipe):
			c.JSON(http.StatusBadRequest, gin.H{"error": "not_a_recipe", "product_id": id.String()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "recompute_failed"})
		}
		return
	}
	c.JSON(http.StatusOK, buildRecipeResponse(result))
}

func buildRecipeResponse(r *CreateRecipeResult) createRecipeResponse {
	out := createRecipeResponse{
		Product:    r.Product,
		Components: make([]recipeComponentResponse, 0, len(r.Components)),
	}
	for _, c := range r.Components {
		out.Components = append(out.Components, recipeComponentResponse{
			ProductID:                  c.ComponentProductID,
			Name:                       c.ComponentProduct.Name,
			QuantityG:                  c.QuantityG,
			EffectiveNutrimentsPer100g: c.ComponentProduct.Nutriments,
		})
	}
	return out
}

// ----- POST /products/import/cookidoo -----

type importCookidooRequest struct {
	URL          string   `json:"url"`
	ServingSizeG *float64 `json:"serving_size_g,omitempty"`
}

type nutritionPerServingResponse struct {
	Kcal     *float64 `json:"kcal,omitempty"`
	ProteinG *float64 `json:"protein_g,omitempty"`
	CarbsG   *float64 `json:"carbs_g,omitempty"`
	FatG     *float64 `json:"fat_g,omitempty"`
	FiberG   *float64 `json:"fiber_g,omitempty"`
	SugarG   *float64 `json:"sugar_g,omitempty"`
	SaltG    *float64 `json:"salt_g,omitempty"`
}

type importCookidooResponse struct {
	*Product
	AlreadyImported     bool                         `json:"already_imported,omitempty"`
	NeedsNutriments     bool                         `json:"needs_nutriments,omitempty"`
	NutritionPerServing *nutritionPerServingResponse `json:"nutrition_per_serving,omitempty"`
}

// importCookidoo godoc
// @Summary      Import a Cookidoo recipe by URL
// @Description  Fetches a Cookidoo recipe page server-side, parses its Schema.org Recipe JSON-LD, and creates a flat-imported source=recipe product with the verbatim ingredient list and provenance external_url. Cookidoo reports nutrition per serving with no mass: pass `serving_size_g` to convert to per-100g, or omit it to create the product without nutriments (the response then carries `needs_nutriments: true` and a `nutrition_per_serving` echo to convert and PATCH later). Re-importing a URL already present returns 200 with the existing product and `already_imported: true`.
// @Tags         products
// @Accept       json
// @Produce      json
// @Param        Idempotency-Key  header  string                 false  "Optional client-supplied idempotency key"
// @Param        body             body    importCookidooRequest  true   "Cookidoo recipe URL and optional serving size"
// @Success      201  {object}  importCookidooResponse  "imported (new product)"
// @Success      200  {object}  importCookidooResponse  "already imported (existing product)"
// @Failure      400  {object}  map[string]string  "invalid_json | invalid_cookidoo_url | ingredients_invalid"
// @Failure      502  {object}  map[string]string  "cookidoo_unavailable"
// @Failure      503  {object}  map[string]string  "cookidoo_not_configured"
// @Security     BearerAuth
// @Router       /products/import/cookidoo [post]
func (h *Handlers) importCookidoo(c *gin.Context) {
	var req importCookidooRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
		return
	}

	result, err := h.svc.ImportCookidoo(c.Request.Context(), ImportCookidooInput{
		URL:          req.URL,
		ServingSizeG: req.ServingSizeG,
	})
	if err != nil {
		h.respondImportError(c, err)
		return
	}

	resp := importCookidooResponse{
		Product:         result.Product,
		AlreadyImported: result.AlreadyImported,
		NeedsNutriments: result.NeedsNutriments,
	}
	if result.NutritionPerServing != nil {
		n := result.NutritionPerServing
		resp.NutritionPerServing = &nutritionPerServingResponse{
			Kcal:     n.Kcal,
			ProteinG: n.ProteinG,
			CarbsG:   n.CarbsG,
			FatG:     n.FatG,
			FiberG:   n.FiberG,
			SugarG:   n.SugarG,
			SaltG:    n.SaltG,
		}
	}
	if result.AlreadyImported {
		c.JSON(http.StatusOK, resp)
		return
	}
	c.JSON(http.StatusCreated, resp)
}

// respondImportError maps import errors to REST shapes. cookidoo fetch/parse
// failures collapse to 502 cookidoo_unavailable with a reason distinguishing a
// transport/status failure from a missing-recipe page.
func (h *Handlers) respondImportError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, cookidoo.ErrNotCookidooURL):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_cookidoo_url"})
	case errors.Is(err, ErrCookidooNotConfigured):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "cookidoo_not_configured"})
	case errors.Is(err, cookidoo.ErrNoRecipeJSONLD):
		c.JSON(http.StatusBadGateway, gin.H{"error": "cookidoo_unavailable", "reason": "no_recipe_on_page"})
	case errors.Is(err, ErrIngredientsRequireRecipeSource):
		// Defensive: import always sets source=recipe, so this should not occur.
		c.JSON(http.StatusBadRequest, gin.H{"error": "ingredients_require_recipe_source"})
	default:
		var ff *cookidoo.ErrFetchFailed
		if errors.As(err, &ff) {
			c.JSON(http.StatusBadGateway, gin.H{
				"error":  "cookidoo_unavailable",
				"reason": "fetch_failed",
				"status": ff.StatusCode,
			})
			return
		}
		var badIng *ErrIngredientsInvalid
		if errors.As(err, &badIng) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "ingredients_invalid", "reason": badIng.Reason, "index": badIng.Index})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "import_failed"})
	}
}
