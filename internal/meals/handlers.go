package meals

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/vinzenzs/nutrition-api/internal/vision"
)

// VisionParser is the narrow interface the /meals/from_photo handler depends
// on. Production wires *vision.Client; tests can inject a stub without
// spinning up an httptest server or recording fixtures end-to-end.
type VisionParser interface {
	Parse(ctx context.Context, req vision.ParseRequest) (*vision.ParseResult, error)
}

// Handlers wires meal endpoints onto a Gin router group.
type Handlers struct {
	svc *Service

	// visionClient + maxPhotoBytes are wired via SetVision for the
	// /meals/from_photo path. Optional — when visionClient is nil the
	// handler returns 503 vision_unavailable, matching the
	// "ANTHROPIC_API_KEY not configured" branch in design.md.
	visionClient  VisionParser
	maxPhotoBytes int64
}

func NewHandlers(svc *Service) *Handlers {
	return &Handlers{svc: svc}
}

// SetVision wires the Claude vision parser and the max-image-bytes guard for
// the /meals/from_photo endpoint. Production callers pass `*vision.Client`;
// tests pass a stub VisionParser. Calling with vc=nil (e.g. when
// ANTHROPIC_API_KEY is unset) is supported — the handler then returns 503.
func (h *Handlers) SetVision(vc VisionParser, maxBytes int64) {
	h.visionClient = vc
	h.maxPhotoBytes = maxBytes
}

func (h *Handlers) Register(rg *gin.RouterGroup) {
	rg.POST("/meals", h.create)
	rg.POST("/meals/freeform", h.createFreeform)
	rg.POST("/meals/from_photo", h.createFromPhoto)
	rg.GET("/meals/:id", h.get)
	rg.GET("/meals", h.list)
	rg.PATCH("/meals/:id", h.patch)
	rg.DELETE("/meals/:id", h.delete)
}

// ----- POST /meals -----

type createRequest struct {
	ProductID *string  `json:"product_id"`
	QuantityG float64  `json:"quantity_g"`
	LoggedAt  string   `json:"logged_at"`
	MealType  *string  `json:"meal_type,omitempty"`
	Note      *string  `json:"note,omitempty"`
	WorkoutID *string  `json:"workout_id,omitempty"`
}

// create godoc
// @Summary      Log a meal entry against an existing product
// @Tags         meals
// @Accept       json
// @Produce      json
// @Param        Idempotency-Key  header  string         false  "Optional client-supplied idempotency key"
// @Param        body             body    createRequest  true   "Meal entry"
// @Success      201  {object}  MealEntry
// @Failure      400  {object}  map[string]string
// @Failure      404  {object}  map[string]string  "product_not_found"
// @Security     BearerAuth
// @Router       /meals [post]
func (h *Handlers) create(c *gin.Context) {
	var req createRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_json")
		return
	}
	if req.ProductID == nil || *req.ProductID == "" {
		respondError(c, http.StatusBadRequest, "product_id_required")
		return
	}
	pid, err := uuid.Parse(*req.ProductID)
	if err != nil {
		respondError(c, http.StatusBadRequest, "product_id_required")
		return
	}
	ts, err := parseLoggedAt(req.LoggedAt)
	if err != nil {
		respondError(c, http.StatusBadRequest, "logged_at_invalid")
		return
	}
	in := CreateInput{
		ProductID: &pid,
		QuantityG: req.QuantityG,
		LoggedAt:  ts,
		MealType:  req.MealType,
		Note:      req.Note,
	}
	if req.WorkoutID != nil && *req.WorkoutID != "" {
		wid, err := uuid.Parse(*req.WorkoutID)
		if err != nil {
			respondError(c, http.StatusBadRequest, "workout_id_invalid")
			return
		}
		in.WorkoutID = &wid
	}
	m, err := h.svc.Create(c.Request.Context(), in)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, m)
}

// ----- POST /meals/freeform -----

type freeformRequest struct {
	Name              string                  `json:"name"`
	NutrimentsPer100g freeformNutrimentsBody  `json:"nutriments_per_100g"`
	QuantityG         float64                 `json:"quantity_g"`
	LoggedAt          string                  `json:"logged_at"`
	MealType          *string                 `json:"meal_type,omitempty"`
	Note              *string                 `json:"note,omitempty"`
	SaveAsProduct     bool                    `json:"save_as_product"`
	WorkoutID         *string                 `json:"workout_id,omitempty"`
}

type freeformNutrimentsBody struct {
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

// createFreeform godoc
// @Summary      Log a freeform meal entry (no product lookup)
// @Description  Use when the user is logging a meal that doesn't map to a barcode. Optionally persists the item as a manual product via `save_as_product`.
// @Tags         meals
// @Accept       json
// @Produce      json
// @Param        Idempotency-Key  header  string           false  "Optional client-supplied idempotency key"
// @Param        body             body    freeformRequest  true   "Freeform meal entry"
// @Success      201  {object}  MealEntry
// @Failure      400  {object}  map[string]string
// @Security     BearerAuth
// @Router       /meals/freeform [post]
func (h *Handlers) createFreeform(c *gin.Context) {
	var req freeformRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_json")
		return
	}
	ts, err := parseLoggedAt(req.LoggedAt)
	if err != nil {
		respondError(c, http.StatusBadRequest, "logged_at_invalid")
		return
	}
	in := FreeformInput{
		Name: req.Name,
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
		QuantityG:     req.QuantityG,
		LoggedAt:      ts,
		MealType:      req.MealType,
		Note:          req.Note,
		SaveAsProduct: req.SaveAsProduct,
	}
	if req.WorkoutID != nil && *req.WorkoutID != "" {
		wid, err := uuid.Parse(*req.WorkoutID)
		if err != nil {
			respondError(c, http.StatusBadRequest, "workout_id_invalid")
			return
		}
		in.WorkoutID = &wid
	}
	m, err := h.svc.CreateFreeform(c.Request.Context(), in)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, m)
}

// ----- GET /meals/{id} -----

// get godoc
// @Summary      Get a meal entry by ID
// @Tags         meals
// @Produce      json
// @Param        id      path   string  true   "Meal entry UUID"
// @Param        expand  query  string  false  "Set to `components` to include recipe breakdown when the meal's product is a recipe"
// @Success      200  {object}  MealEntry
// @Failure      404  {object}  map[string]string  "meal_not_found"
// @Security     BearerAuth
// @Router       /meals/{id} [get]
func (h *Handlers) get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusNotFound, "meal_not_found")
		return
	}

	if c.Query("expand") == "components" {
		m, comps, err := h.svc.GetWithComponents(c.Request.Context(), id)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				respondError(c, http.StatusNotFound, "meal_not_found")
				return
			}
			respondError(c, http.StatusInternalServerError, "get_failed")
			return
		}
		mealJSON, _ := json.Marshal(m)
		var envelope map[string]any
		_ = json.Unmarshal(mealJSON, &envelope)
		if comps == nil {
			comps = []ScaledComponent{}
		}
		envelope["components"] = comps
		c.JSON(http.StatusOK, envelope)
		return
	}

	m, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respondError(c, http.StatusNotFound, "meal_not_found")
			return
		}
		respondError(c, http.StatusInternalServerError, "get_failed")
		return
	}
	c.JSON(http.StatusOK, m)
}

// ----- GET /meals -----

// list godoc
// @Summary      List meal entries in a time window
// @Tags         meals
// @Produce      json
// @Param        from        query  string  true   "Inclusive RFC3339 lower bound"
// @Param        to          query  string  true   "Exclusive RFC3339 upper bound"
// @Param        meal_type   query  string  false  "Optional filter: breakfast|lunch|dinner|snack"
// @Success      200  {object}  map[string]interface{}  "{ meals: [...] }"
// @Failure      400  {object}  map[string]string  "window_required | window_invalid | meal_type_invalid"
// @Security     BearerAuth
// @Router       /meals [get]
func (h *Handlers) list(c *gin.Context) {
	fromStr := c.Query("from")
	toStr := c.Query("to")
	if fromStr == "" || toStr == "" {
		respondError(c, http.StatusBadRequest, "window_required")
		return
	}
	from, err := time.Parse(time.RFC3339, fromStr)
	if err != nil {
		respondError(c, http.StatusBadRequest, "window_invalid")
		return
	}
	to, err := time.Parse(time.RFC3339, toStr)
	if err != nil {
		respondError(c, http.StatusBadRequest, "window_invalid")
		return
	}
	if !from.Before(to) {
		respondError(c, http.StatusBadRequest, "window_invalid")
		return
	}
	var mealType *MealType
	if mt := c.Query("meal_type"); mt != "" {
		if !ValidMealType(mt) {
			respondError(c, http.StatusBadRequest, "meal_type_invalid")
			return
		}
		v := MealType(mt)
		mealType = &v
	}

	out, err := h.svc.List(c.Request.Context(), ListParams{From: from, To: to, MealType: mealType})
	if err != nil {
		respondError(c, http.StatusInternalServerError, "list_failed")
		return
	}
	c.JSON(http.StatusOK, gin.H{"meals": out})
}

// ----- PATCH /meals/{id} -----

type patchRequest struct {
	QuantityG *float64 `json:"quantity_g,omitempty"`
	LoggedAt  *string  `json:"logged_at,omitempty"`
	MealType  *string  `json:"meal_type,omitempty"`
	Note      *string  `json:"note,omitempty"`
	// WorkoutID supports the empty-string sentinel for clear:
	//   omitted   → leave unchanged
	//   "<uuid>"  → set the link
	//   ""        → clear the link
	WorkoutID *string `json:"workout_id,omitempty"`
}

// patch godoc
// @Summary      Update fields on a meal entry
// @Description  Unknown fields are ignored. Send only the keys you want to change.
// @Tags         meals
// @Accept       json
// @Produce      json
// @Param        id    path  string        true   "Meal entry UUID"
// @Param        body  body  patchRequest  true   "Fields to update"
// @Success      200  {object}  MealEntry
// @Failure      400  {object}  map[string]string
// @Failure      404  {object}  map[string]string  "meal_not_found"
// @Security     BearerAuth
// @Router       /meals/{id} [patch]
func (h *Handlers) patch(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusNotFound, "meal_not_found")
		return
	}
	// Decode the body with a tolerant decoder — unknown fields are ignored.
	raw, err := c.GetRawData()
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid_json")
		return
	}
	var req patchRequest
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &req); err != nil {
			respondError(c, http.StatusBadRequest, "invalid_json")
			return
		}
	}
	in := PatchInput{
		QuantityG: req.QuantityG,
		MealType:  req.MealType,
		Note:      req.Note,
	}
	if req.LoggedAt != nil {
		ts, err := time.Parse(time.RFC3339, *req.LoggedAt)
		if err != nil {
			respondError(c, http.StatusBadRequest, "logged_at_invalid")
			return
		}
		in.LoggedAt = &ts
	}
	if req.WorkoutID != nil {
		if *req.WorkoutID == "" {
			in.ClearWorkoutID = true
		} else {
			wid, err := uuid.Parse(*req.WorkoutID)
			if err != nil {
				respondError(c, http.StatusBadRequest, "workout_id_invalid")
				return
			}
			in.WorkoutID = &wid
		}
	}
	m, err := h.svc.Patch(c.Request.Context(), id, in)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respondError(c, http.StatusNotFound, "meal_not_found")
			return
		}
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, m)
}

// ----- DELETE /meals/{id} -----

// delete godoc
// @Summary      Delete a meal entry
// @Tags         meals
// @Param        id   path  string  true  "Meal entry UUID"
// @Success      204  "no content"
// @Failure      404  {object}  map[string]string  "meal_not_found"
// @Security     BearerAuth
// @Router       /meals/{id} [delete]
func (h *Handlers) delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusNotFound, "meal_not_found")
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		if errors.Is(err, ErrNotFound) {
			respondError(c, http.StatusNotFound, "meal_not_found")
			return
		}
		respondError(c, http.StatusInternalServerError, "delete_failed")
		return
	}
	c.Status(http.StatusNoContent)
}

// ----- helpers -----

func respondError(c *gin.Context, status int, code string) {
	c.JSON(status, gin.H{"error": code})
}

func respondServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrProductIDRequired):
		respondError(c, http.StatusBadRequest, "product_id_required")
	case errors.Is(err, ErrProductNotFound):
		respondError(c, http.StatusNotFound, "product_not_found")
	case errors.Is(err, ErrWorkoutNotFound):
		respondError(c, http.StatusBadRequest, "workout_not_found")
	case errors.Is(err, ErrQuantityInvalid):
		respondError(c, http.StatusBadRequest, "quantity_g_invalid")
	case errors.Is(err, ErrLoggedAtFuture):
		respondError(c, http.StatusBadRequest, "logged_at_too_far_future")
	case errors.Is(err, ErrMealTypeInvalid):
		respondError(c, http.StatusBadRequest, "meal_type_invalid")
	case errors.Is(err, ErrNameRequired):
		respondError(c, http.StatusBadRequest, "name_required")
	default:
		var ni *ErrNutrimentInvalid
		if errors.As(err, &ni) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "nutriments_invalid", "field": ni.Field})
			return
		}
		respondError(c, http.StatusInternalServerError, "write_failed")
	}
}

func parseLoggedAt(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, errors.New("empty")
	}
	return time.Parse(time.RFC3339, s)
}
