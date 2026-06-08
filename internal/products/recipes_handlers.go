package products

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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
