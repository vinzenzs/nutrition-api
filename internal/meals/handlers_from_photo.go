package meals

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/vinzenzs/kazper/internal/vision"
)

// fromPhotoResponse is the envelope returned on 201. The `meal` block is the
// canonical MealEntry shape (same as POST /meals/freeform) so clients that
// already consume freeform responses just call response.Meal. The
// `inference` block carries vision-specific metadata — clients can decide
// whether to surface confidence to the user, log token usage, etc.
type fromPhotoResponse struct {
	Meal      *MealEntry          `json:"meal"`
	Inference fromPhotoInference  `json:"inference"`
}

type fromPhotoInference struct {
	Model              string  `json:"model"`
	Confidence         float64 `json:"confidence"`
	Notes              string  `json:"notes,omitempty"`
	OriginalImageBytes int     `json:"original_image_bytes"`
	ResizedTo          [2]int  `json:"resized_to"`
	ClaudeInputTokens  int     `json:"claude_input_tokens"`
	ClaudeOutputTokens int     `json:"claude_output_tokens"`
}

// createFromPhoto godoc
// @Summary      Log a meal from a photo via Claude Vision
// @Description  Accepts a `multipart/form-data` upload with an `image` part (JPEG or PNG; HEIC is rejected with 415 in v1 because we don't ship libheif). The backend resizes the image to a max edge of 1568px, sends it to Claude Vision with a tool-forced output, and uses the returned `{name, nutriments_per_100g}` to create a freeform meal entry. The response carries both the canonical `meal` block and an `inference` block reporting model, confidence, token usage, and image dimensions. Returns `503 vision_unavailable` when `ANTHROPIC_API_KEY` is not configured.
// @Tags         meals
// @Accept       multipart/form-data
// @Produce      json
// @Param        Idempotency-Key  header    string  false  "Optional client-supplied idempotency key. Hash includes the raw image bytes so byte-identical replays return the original meal without a second Claude call."
// @Param        image            formData  file    true   "Image file (JPEG or PNG; HEIC rejected in v1)."
// @Param        quantity_g       formData  number  false  "Meal quantity in grams. Default 100."
// @Param        logged_at        formData  string  false  "RFC 3339 timestamp; default now()."
// @Param        meal_type        formData  string  false  "breakfast | lunch | dinner | snack."
// @Param        note             formData  string  false  "Optional free-text note."
// @Success      201  {object}  fromPhotoResponse
// @Failure      400  {object}  map[string]string  "image_required | quantity_g_invalid | logged_at_invalid | logged_at_too_far_future | meal_type_invalid | note_too_long"
// @Failure      413  {object}  map[string]interface{}  "image_too_large with max_bytes hint"
// @Failure      415  {object}  map[string]string  "unsupported_media_type"
// @Failure      429  {object}  map[string]interface{}  "vision_rate_limited with retry_after_seconds"
// @Failure      502  {object}  map[string]interface{}  "vision_unexpected_response | vision_response_unparseable"
// @Failure      503  {object}  map[string]string  "vision_unavailable (ANTHROPIC_API_KEY not configured)"
// @Failure      504  {object}  map[string]interface{}  "vision_timeout | vision_upstream_error with retry_after_seconds"
// @Security     BearerAuth
// @Router       /meals/from_photo [post]
func (h *Handlers) createFromPhoto(c *gin.Context) {
	if h.visionClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":  "vision_unavailable",
			"reason": "ANTHROPIC_API_KEY not configured",
		})
		return
	}

	// Limit reader: refuse uploads above MealFromPhotoMaxBytes before
	// allocating memory for them. Multipart parsing applies the same cap
	// transitively because the body reader is the bound.
	if h.maxPhotoBytes > 0 {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.maxPhotoBytes)
	}

	if err := c.Request.ParseMultipartForm(h.maxPhotoBytes); err != nil {
		var mb *http.MaxBytesError
		if errors.As(err, &mb) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{
				"error":     "image_too_large",
				"max_bytes": h.maxPhotoBytes,
			})
			return
		}
		respondError(c, http.StatusBadRequest, "invalid_multipart")
		return
	}

	// --- Extract metadata fields ---

	quantity := 100.0
	if s := c.PostForm("quantity_g"); s != "" {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			respondError(c, http.StatusBadRequest, "quantity_g_invalid")
			return
		}
		quantity = v
	}

	loggedAt := time.Now().UTC()
	if s := c.PostForm("logged_at"); s != "" {
		ts, err := time.Parse(time.RFC3339, s)
		if err != nil {
			respondError(c, http.StatusBadRequest, "logged_at_invalid")
			return
		}
		loggedAt = ts
	}

	var mealType *string
	if s := c.PostForm("meal_type"); s != "" {
		if _, err := ParseMealType(s); err != nil {
			respondError(c, http.StatusBadRequest, "meal_type_invalid")
			return
		}
		// FreeformInput.MealType is a *string of the raw enum value;
		// service-level CreateFreeform re-validates.
		mt := s
		mealType = &mt
	}

	var note *string
	if s := c.PostForm("note"); s != "" {
		note = &s
	}

	// --- Extract the image ---

	fileHeader, err := c.FormFile("image")
	if err != nil {
		respondError(c, http.StatusBadRequest, "image_required")
		return
	}
	if h.maxPhotoBytes > 0 && fileHeader.Size > h.maxPhotoBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{
			"error":     "image_too_large",
			"max_bytes": h.maxPhotoBytes,
		})
		return
	}
	src, err := fileHeader.Open()
	if err != nil {
		respondError(c, http.StatusBadRequest, "image_required")
		return
	}
	defer src.Close()
	originalBytes, err := io.ReadAll(src)
	if err != nil {
		respondError(c, http.StatusBadRequest, "image_required")
		return
	}

	// --- Resize before validation against vision (resize ALSO sniffs
	//     content-type and returns ErrUnsupportedMediaType for HEIC, which
	//     gives us a clean 415 path) ---

	resized, dims, err := vision.Resize(originalBytes)
	if err != nil {
		if errors.Is(err, vision.ErrUnsupportedMediaType) {
			c.JSON(http.StatusUnsupportedMediaType, gin.H{"error": "unsupported_media_type"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "resize_failed"})
		return
	}

	// --- Call Claude Vision ---

	pr, err := h.visionClient.Parse(c.Request.Context(), vision.ParseRequest{
		Image:         resized,
		ResizedTo:     dims,
		OriginalBytes: len(originalBytes),
	})
	if err != nil {
		respondVisionError(c, err)
		return
	}

	// --- Wire the parsed result into the existing freeform-create path ---

	in := FreeformInput{
		Name: pr.Name,
		Nutriments: Nutriments{
			KcalPer100g:     ptrFloat(pr.NutrimentsPer100g.Kcal),
			ProteinGPer100g: ptrFloat(pr.NutrimentsPer100g.ProteinG),
			CarbsGPer100g:   ptrFloat(pr.NutrimentsPer100g.CarbsG),
			FatGPer100g:     ptrFloat(pr.NutrimentsPer100g.FatG),
			FiberGPer100g:   pr.NutrimentsPer100g.FiberG,
			SugarGPer100g:   pr.NutrimentsPer100g.SugarG,
			SaltGPer100g:    pr.NutrimentsPer100g.SaltG,
		},
		QuantityG: quantity,
		LoggedAt:  loggedAt,
		MealType:  mealType,
		Note:      note,
	}
	m, err := h.svc.CreateFreeform(c.Request.Context(), in)
	if err != nil {
		respondServiceError(c, err)
		return
	}

	c.JSON(http.StatusCreated, fromPhotoResponse{
		Meal: m,
		Inference: fromPhotoInference{
			Model:              pr.Model,
			Confidence:         pr.Confidence,
			Notes:              pr.Notes,
			OriginalImageBytes: pr.OriginalBytes,
			ResizedTo:          pr.ResizedTo,
			ClaudeInputTokens:  pr.InputTokens,
			ClaudeOutputTokens: pr.OutputTokens,
		},
	})
}

// respondVisionError maps the vision package's typed errors to the documented
// HTTP responses (see design.md § Error mapping).
func respondVisionError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, vision.ErrVisionTimeout):
		c.JSON(http.StatusGatewayTimeout, gin.H{
			"error":               "vision_timeout",
			"retry_after_seconds": 30,
		})
	case errors.Is(err, vision.ErrVisionUpstreamError):
		c.JSON(http.StatusGatewayTimeout, gin.H{
			"error":               "vision_upstream_error",
			"retry_after_seconds": 30,
		})
	case errors.Is(err, vision.ErrVisionResponseUnparseable):
		c.JSON(http.StatusBadGateway, gin.H{"error": "vision_response_unparseable"})
	}
	var rl *vision.ErrVisionRateLimited
	if errors.As(err, &rl) {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error":               "vision_rate_limited",
			"retry_after_seconds": rl.RetryAfterSeconds,
		})
		return
	}
	var ur *vision.ErrVisionUnexpectedResponse
	if errors.As(err, &ur) {
		c.JSON(http.StatusBadGateway, gin.H{
			"error":  "vision_unexpected_response",
			"status": ur.StatusCode,
		})
		return
	}
	if c.Writer.Status() == http.StatusOK {
		// No matching sentinel — last-resort 500. Shouldn't happen because
		// the vision client only returns the documented errors.
		c.JSON(http.StatusInternalServerError, gin.H{"error": "vision_failed"})
	}
}

// ptrFloat returns &v when v != 0, else nil. Treats 0 as "model didn't fill
// it in" — defensible because a real food has non-zero macros and the
// nutriments_per_100g schema requires kcal/protein/carbs/fat anyway.
//
// (The trace nutriments fiber/sugar/salt are already nullable from the
// vision package; this helper applies only to the required macros.)
func ptrFloat(v float64) *float64 {
	if v == 0 {
		return nil
	}
	return &v
}
