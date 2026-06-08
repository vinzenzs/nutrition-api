package products

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/vinzenzs/nutrition-api/internal/off"
)

// Handlers wires the products service to Gin routes.
type Handlers struct {
	svc *Service
}

func NewHandlers(svc *Service) *Handlers {
	return &Handlers{svc: svc}
}

// Register mounts the products routes onto rg.
func (h *Handlers) Register(rg *gin.RouterGroup) {
	rg.POST("/products/lookup/:barcode", h.lookup)
	// Empty-barcode sibling: the parametrised route does not match an empty
	// :barcode segment; without this Gin returns a generic 404 even though
	// the path is recognised. The right semantic is 400 (validation), not
	// 404 (no such route). See the http-error-shape capability and the
	// products spec's empty-barcode scenario.
	rg.POST("/products/lookup/", h.lookupEmptyBarcode)
	rg.POST("/products", h.createManual)
	rg.POST("/products/recipes", h.createRecipe)
	rg.POST("/products/recipes/:id/recompute", h.recomputeRecipe)
	rg.GET("/products", h.list)
	rg.GET("/products/:id", h.getByID)
	rg.GET("/products/search", h.search)
	rg.DELETE("/products/:id", h.delete)
}

// lookupEmptyBarcode godoc
// @Summary      Reject barcode lookup with no barcode
// @Description  Sibling route to POST /products/lookup/{barcode}. Returns 400 when the path parameter is missing instead of falling through to a generic 404.
// @Tags         products
// @Produce      json
// @Success      400  {object}  map[string]string  "barcode_required"
// @Security     BearerAuth
// @Router       /products/lookup/ [post]
func (h *Handlers) lookupEmptyBarcode(c *gin.Context) {
	c.JSON(http.StatusBadRequest, gin.H{"error": "barcode_required"})
}

// lookup godoc
// @Summary      Look up a product by barcode
// @Description  Fetches a product from the local cache or upstream Open Food Facts. Pass `refresh=true` to force an upstream re-fetch.
// @Tags         products
// @Produce      json
// @Param        barcode  path   string  true   "EAN/UPC barcode"
// @Param        refresh  query  bool    false  "Force a fresh OFF fetch even if cached"
// @Success      200      {object}  Product
// @Failure      404      {object}  map[string]string  "product_not_found"
// @Failure      502      {object}  map[string]string  "upstream_unexpected_response"
// @Failure      504      {object}  map[string]string  "upstream_timeout"
// @Security     BearerAuth
// @Router       /products/lookup/{barcode} [post]
func (h *Handlers) lookup(c *gin.Context) {
	barcode := c.Param("barcode")
	refresh := c.Query("refresh") == "true"
	p, err := h.svc.Lookup(c.Request.Context(), barcode, refresh)
	if err != nil {
		switch {
		case errors.Is(err, off.ErrProductNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "product_not_found",
				"barcode": barcode,
				"next":    "POST /meals/freeform",
			})
		case errors.Is(err, off.ErrUpstreamTimeout):
			c.JSON(http.StatusGatewayTimeout, gin.H{
				"error":               "upstream_timeout",
				"retry_after_seconds": 30,
			})
		case errors.Is(err, off.ErrUpstreamServerError):
			c.JSON(http.StatusGatewayTimeout, gin.H{
				"error":               "upstream_error",
				"retry_after_seconds": 30,
			})
		default:
			if u, ok := off.IsUnexpectedStatus(err); ok {
				c.JSON(http.StatusBadGateway, gin.H{
					"error":  "upstream_unexpected_response",
					"status": u.StatusCode,
				})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "lookup_failed"})
		}
		return
	}
	c.JSON(http.StatusOK, p)
}

type createManualRequest struct {
	Name              string             `json:"name"`
	Brand             *string            `json:"brand,omitempty"`
	Barcode           *string            `json:"barcode,omitempty"`
	// Source defaults to "manual". Accepts "manual" or "recipe" — the latter
	// is the flat-imported-recipe path (e.g. the Cookidoo Chrome extension).
	Source *string `json:"source,omitempty"`
	// ExternalURL records the upstream page this product was imported from.
	// Optional; max 2048 chars; trimmed before validation.
	ExternalURL       *string            `json:"external_url,omitempty"`
	ServingSizeG      *float64           `json:"serving_size_g,omitempty"`
	NutrimentsPer100g createManualNutris `json:"nutriments_per_100g"`
}

type createManualNutris struct {
	Kcal     *float64 `json:"kcal,omitempty"`
	ProteinG *float64 `json:"protein_g,omitempty"`
	CarbsG   *float64 `json:"carbs_g,omitempty"`
	FatG     *float64 `json:"fat_g,omitempty"`
	FiberG   *float64 `json:"fiber_g,omitempty"`
	SugarG   *float64 `json:"sugar_g,omitempty"`
	SaltG    *float64 `json:"salt_g,omitempty"`

	IronMg        *float64 `json:"iron_mg,omitempty"`
	CalciumMg     *float64 `json:"calcium_mg,omitempty"`
	VitaminDMcg   *float64 `json:"vitamin_d_mcg,omitempty"`
	VitaminB12Mcg *float64 `json:"vitamin_b12_mcg,omitempty"`
	VitaminCMg    *float64 `json:"vitamin_c_mg,omitempty"`
	MagnesiumMg   *float64 `json:"magnesium_mg,omitempty"`
	PotassiumMg   *float64 `json:"potassium_mg,omitempty"`
	ZincMg        *float64 `json:"zinc_mg,omitempty"`
}

// validateNutrimentsNonNegative returns the JSON field name that failed if any
// non-nil nutriment value is negative; empty string on success.
func (n createManualNutris) validateNutrimentsNonNegative() string {
	checks := []struct {
		field string
		v     *float64
	}{
		{"kcal", n.Kcal},
		{"protein_g", n.ProteinG},
		{"carbs_g", n.CarbsG},
		{"fat_g", n.FatG},
		{"fiber_g", n.FiberG},
		{"sugar_g", n.SugarG},
		{"salt_g", n.SaltG},
		{"iron_mg", n.IronMg},
		{"calcium_mg", n.CalciumMg},
		{"vitamin_d_mcg", n.VitaminDMcg},
		{"vitamin_b12_mcg", n.VitaminB12Mcg},
		{"vitamin_c_mg", n.VitaminCMg},
		{"magnesium_mg", n.MagnesiumMg},
		{"potassium_mg", n.PotassiumMg},
		{"zinc_mg", n.ZincMg},
	}
	for _, c := range checks {
		if c.v != nil && *c.v < 0 {
			return c.field
		}
	}
	return ""
}

// createManual godoc
// @Summary      Create a product (manual or flat-imported recipe)
// @Description  Creates a product not sourced from Open Food Facts. Default source is "manual". Pass `source: "recipe"` together with an `external_url` to register a flat-imported recipe (e.g. from the Cookidoo Chrome extension); the row has no `product_components` and `nutriment_computed_at` stays null — distinguishing it from composed recipes built via POST /products/recipes.
// @Tags         products
// @Accept       json
// @Produce      json
// @Param        body  body  createManualRequest  true  "Product fields"
// @Success      201   {object}  Product
// @Failure      400   {object}  map[string]string  "invalid_json | name_required | source_invalid | external_url_too_long | external_url_invalid | nutriments_invalid"
// @Failure      409   {object}  map[string]string  "barcode_already_exists"
// @Security     BearerAuth
// @Router       /products [post]
func (h *Handlers) createManual(c *gin.Context) {
	var req createManualRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name_required"})
		return
	}
	if bad := req.NutrimentsPer100g.validateNutrimentsNonNegative(); bad != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "nutriments_invalid", "field": bad})
		return
	}

	// Source: default "manual"; allow-list rejects anything else.
	source := SourceManual
	if req.Source != nil && *req.Source != "" {
		switch *req.Source {
		case "manual":
			source = SourceManual
		case "recipe":
			source = SourceRecipe
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "source_invalid"})
			return
		}
	}

	// ExternalURL: trim, reject empty-after-trim, reject > 2048 chars. Allowed
	// for any source so a manual product can also carry a provenance link.
	var externalURL *string
	if req.ExternalURL != nil {
		trimmed := strings.TrimSpace(*req.ExternalURL)
		if trimmed == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "external_url_invalid"})
			return
		}
		if len(trimmed) > 2048 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "external_url_too_long"})
			return
		}
		externalURL = &trimmed
	}

	in := CreateManualInput{
		Name:         strings.TrimSpace(req.Name),
		Brand:        req.Brand,
		Barcode:      req.Barcode,
		Source:       source,
		ExternalURL:  externalURL,
		ServingSizeG: req.ServingSizeG,
		Nutriments: Nutriments{
			KcalPer100g:     req.NutrimentsPer100g.Kcal,
			ProteinGPer100g: req.NutrimentsPer100g.ProteinG,
			CarbsGPer100g:   req.NutrimentsPer100g.CarbsG,
			FatGPer100g:     req.NutrimentsPer100g.FatG,
			FiberGPer100g:   req.NutrimentsPer100g.FiberG,
			SugarGPer100g:   req.NutrimentsPer100g.SugarG,
			SaltGPer100g:    req.NutrimentsPer100g.SaltG,

			IronMgPer100g:        req.NutrimentsPer100g.IronMg,
			CalciumMgPer100g:     req.NutrimentsPer100g.CalciumMg,
			VitaminDMcgPer100g:   req.NutrimentsPer100g.VitaminDMcg,
			VitaminB12McgPer100g: req.NutrimentsPer100g.VitaminB12Mcg,
			VitaminCMgPer100g:    req.NutrimentsPer100g.VitaminCMg,
			MagnesiumMgPer100g:   req.NutrimentsPer100g.MagnesiumMg,
			PotassiumMgPer100g:   req.NutrimentsPer100g.PotassiumMg,
			ZincMgPer100g:        req.NutrimentsPer100g.ZincMg,
		},
	}
	p, err := h.svc.CreateManual(c.Request.Context(), in)
	if err != nil {
		var dup *ErrBarcodeExists
		if errors.As(err, &dup) {
			c.JSON(http.StatusConflict, gin.H{
				"error":      "barcode_already_exists",
				"product_id": dup.ExistingID.String(),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "create_failed"})
		return
	}
	c.JSON(http.StatusCreated, p)
}

// delete godoc
// @Summary      Delete a product
// @Description  Hard-deletes a product. Historical meal_entries get a snapshot of the product's current name + nutriments before the FK is nulled, so daily/range summaries continue to reflect those meals. If the product is referenced as a component of any recipe, the request is rejected with 409 listing the using recipes — delete or replace within those recipes first.
// @Tags         products
// @Produce      json
// @Param        id   path  string  true  "Product UUID"
// @Success      204  "no content"
// @Failure      404  {object}  map[string]string  "product_not_found"
// @Failure      409  {object}  map[string]interface{}  "product_in_use_as_component (body includes the using recipes list and a hint)"
// @Security     BearerAuth
// @Router       /products/{id} [delete]
func (h *Handlers) delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "product_not_found"})
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		var inUse *ErrProductInUseAsComponent
		if errors.As(err, &inUse) {
			recipes := inUse.Recipes
			if recipes == nil {
				recipes = []RecipeRef{}
			}
			c.JSON(http.StatusConflict, gin.H{
				"error":   "product_in_use_as_component",
				"recipes": recipes,
				"hint":    "delete the listed recipes first, or replace this product within them",
			})
			return
		}
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "product_not_found"})
			return
		}
		// Belt-and-suspenders: a raw FK violation from the DB (race window
		// between RecipesUsing and Delete) maps to the same 409 with an
		// empty recipes list.
		c.JSON(http.StatusInternalServerError, gin.H{"error": "delete_failed"})
		return
	}
	c.Status(http.StatusNoContent)
}

// list godoc
// @Summary      List products with pagination
// @Description  Returns the product cache as a paginated list, ordered last_logged_at desc then name asc. Combine with `delete` to clean up leftovers.
// @Tags         products
// @Produce      json
// @Param        source  query  string  false  "Filter by source: off | manual | recipe"
// @Param        limit   query  int     false  "Page size (1..200, default 50)"
// @Param        offset  query  int     false  "Page offset (>=0, default 0)"
// @Success      200  {object}  map[string]interface{}  "{ products: [...], total: N, limit, offset }"
// @Failure      400  {object}  map[string]string  "source_invalid | limit_too_large | pagination_invalid"
// @Security     BearerAuth
// @Router       /products [get]
func (h *Handlers) list(c *gin.Context) {
	source := c.Query("source")
	switch source {
	case "", "off", "manual", "recipe":
		// ok
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "source_invalid"})
		return
	}

	limit := 50
	if v := c.Query("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "pagination_invalid"})
			return
		}
		if n < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "pagination_invalid"})
			return
		}
		if n > 200 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "limit_too_large", "max": 200})
			return
		}
		limit = n
	}

	offset := 0
	if v := c.Query("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "pagination_invalid"})
			return
		}
		offset = n
	}

	ctx := c.Request.Context()
	rows, err := h.svc.repo.List(ctx, ListParams{Source: source, Limit: limit, Offset: offset})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list_failed"})
		return
	}
	total, err := h.svc.repo.Count(ctx, source)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list_failed"})
		return
	}
	if rows == nil {
		rows = []*Product{}
	}
	c.JSON(http.StatusOK, gin.H{
		"products": rows,
		"total":    total,
		"limit":    limit,
		"offset":   offset,
	})
}

// getByID godoc
// @Summary      Get a product by ID
// @Tags         products
// @Produce      json
// @Param        id      path   string  true   "Product UUID"
// @Param        expand  query  string  false  "Set to `components` to include a recipe's components"
// @Success      200  {object}  Product
// @Failure      404  {object}  map[string]string  "product_not_found"
// @Security     BearerAuth
// @Router       /products/{id} [get]
func (h *Handlers) getByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "product_not_found"})
		return
	}
	p, err := h.svc.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "product_not_found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "get_failed"})
		return
	}

	if c.Query("expand") != "components" {
		c.JSON(http.StatusOK, p)
		return
	}

	// For non-recipe products, return components as an empty array.
	if p.Source != SourceRecipe {
		c.JSON(http.StatusOK, gin.H{
			"id":                   p.ID,
			"barcode":              p.Barcode,
			"name":                 p.Name,
			"brand":                p.Brand,
			"source":               p.Source,
			"serving_size_g":       p.ServingSizeG,
			"nutriments_per_100g":  p.Nutriments,
			"fetched_at":           p.FetchedAt,
			"last_logged_at":       p.LastLoggedAt,
			"nutriment_computed_at": p.NutrimentComputedAt,
			"created_at":           p.CreatedAt,
			"updated_at":           p.UpdatedAt,
			"components":           []any{},
		})
		return
	}

	crepo := NewComponentsRepo(h.svc.pool)
	comps, err := crepo.ListComponents(c.Request.Context(), p.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "get_components_failed"})
		return
	}
	c.JSON(http.StatusOK, buildRecipeResponse(&CreateRecipeResult{Product: p, Components: comps}))
}

// search godoc
// @Summary      Search products
// @Description  Case-insensitive substring match across product name and brand. Returns up to 50 results.
// @Tags         products
// @Produce      json
// @Param        q    query  string  true  "Substring to search for"
// @Success      200  {object}  map[string]interface{}  "{ results: [...] }"
// @Failure      400  {object}  map[string]string  "q_required"
// @Security     BearerAuth
// @Router       /products/search [get]
func (h *Handlers) search(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	if q == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "q_required"})
		return
	}
	results, err := h.svc.repo.Search(c.Request.Context(), q, 50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "search_failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"results": results})
}
