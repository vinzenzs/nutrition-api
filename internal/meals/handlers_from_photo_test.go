package meals_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/nutrition-api/internal/auth"
	"github.com/vinzenzs/nutrition-api/internal/idempotency"
	"github.com/vinzenzs/nutrition-api/internal/meals"
	"github.com/vinzenzs/nutrition-api/internal/products"
	"github.com/vinzenzs/nutrition-api/internal/store/storetest"
	"github.com/vinzenzs/nutrition-api/internal/vision"
)

// stubVision implements meals.VisionParser without making any HTTP calls.
// callCount is locked because the idempotency-replay test asserts exactly
// one call across two requests (the second is served from the cache).
type stubVision struct {
	mu        sync.Mutex
	callCount int
	result    *vision.ParseResult
	err       error
	// captured ParseRequest from the most recent call, for assertions.
	last vision.ParseRequest
}

func (s *stubVision) Parse(_ context.Context, req vision.ParseRequest) (*vision.ParseResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.callCount++
	s.last = req
	if s.err != nil {
		return nil, s.err
	}
	// Mimic the real client: overwrite ResizedTo / OriginalBytes from the
	// request so the handler reports the actual upload context, not the
	// stub's pre-canned values.
	cp := *s.result
	cp.ResizedTo = req.ResizedTo
	cp.OriginalBytes = req.OriginalBytes
	return &cp, nil
}

func (s *stubVision) calls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.callCount
}

// setupPhoto wires the meals handler with auth + idempotency + a stub vision
// parser. Returns the gin engine + the stub so individual tests can assert
// against calls() and the captured request.
func setupPhoto(t *testing.T, vc meals.VisionParser) (*gin.Engine, *products.Repo) {
	t.Helper()
	pool := storetest.NewPool(t)
	pRepo := products.NewRepo(pool)
	mRepo := meals.NewRepo(pool)
	svc := meals.NewService(pool, mRepo, pRepo)
	h := meals.NewHandlers(svc)
	h.SetVision(vc, 10*1024*1024)

	idemRepo := idempotency.NewRepo(pool)

	r := gin.New()
	r.Use(auth.Middleware(auth.Config{MobileToken: "mobile-token-abc", AgentToken: "agent-token-def"}))
	r.Use(idempotency.Middleware(idemRepo, time.Hour))
	rg := r.Group("/")
	h.Register(rg)
	return r, pRepo
}

// makeJPEG returns a small in-memory JPEG suitable for the multipart upload.
func makeJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 220, G: 180, B: 140, A: 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}))
	return buf.Bytes()
}

// buildMultipart constructs the body+content-type for a /meals/from_photo upload.
func buildMultipart(t *testing.T, imageBytes []byte, fields map[string]string) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	if imageBytes != nil {
		fw, err := mw.CreateFormFile("image", "plate.jpg")
		require.NoError(t, err)
		_, err = io.Copy(fw, bytes.NewReader(imageBytes))
		require.NoError(t, err)
	}
	for k, v := range fields {
		require.NoError(t, mw.WriteField(k, v))
	}
	require.NoError(t, mw.Close())
	return body, mw.FormDataContentType()
}

func postPhoto(t *testing.T, r *gin.Engine, body *bytes.Buffer, ct string, idemKey string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/meals/from_photo", body)
	req.Header.Set("Authorization", "Bearer mobile-token-abc")
	req.Header.Set("Content-Type", ct)
	if idemKey != "" {
		req.Header.Set("Idempotency-Key", idemKey)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// goodResult is the ParseResult tests use for happy-path cases.
func goodResult() *vision.ParseResult {
	return &vision.ParseResult{
		Name: "Grilled chicken caesar salad",
		NutrimentsPer100g: vision.Nutriments{
			Kcal:     165,
			ProteinG: 14.2,
			CarbsG:   5.1,
			FatG:     9.8,
		},
		Confidence:    0.85,
		Notes:         "Estimate assumes a standard caesar dressing portion.",
		Model:         "claude-sonnet-4-6",
		InputTokens:   1240,
		OutputTokens:  89,
		ResizedTo:     [2]int{1568, 1045},
		OriginalBytes: 4_128_719,
	}
}

// ============================================================================
// Happy path
// ============================================================================

func TestFromPhoto_HappyPath(t *testing.T) {
	stub := &stubVision{result: goodResult()}
	r, _ := setupPhoto(t, stub)
	imgBytes := makeJPEG(t, 800, 600)
	body, ct := buildMultipart(t, imgBytes, map[string]string{
		"quantity_g": "250",
		"meal_type":  "lunch",
	})

	rec := postPhoto(t, r, body, ct, "")
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var out struct {
		Meal      json.RawMessage `json:"meal"`
		Inference struct {
			Model              string  `json:"model"`
			Confidence         float64 `json:"confidence"`
			Notes              string  `json:"notes"`
			OriginalImageBytes int     `json:"original_image_bytes"`
			ResizedTo          [2]int  `json:"resized_to"`
			ClaudeInputTokens  int     `json:"claude_input_tokens"`
			ClaudeOutputTokens int     `json:"claude_output_tokens"`
		} `json:"inference"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))

	assert.Equal(t, "claude-sonnet-4-6", out.Inference.Model)
	assert.Equal(t, 0.85, out.Inference.Confidence)
	assert.Equal(t, 1240, out.Inference.ClaudeInputTokens)
	assert.Equal(t, 89, out.Inference.ClaudeOutputTokens)
	// The handler reports the local image size (after re-encode), not the
	// stub's 4MB claim, because the handler ignores stub-supplied
	// OriginalBytes — that field is overwritten by the handler from the
	// actual upload bytes. Loose check: it's the size of the uploaded JPEG.
	assert.Equal(t, len(imgBytes), out.Inference.OriginalImageBytes)

	// Meal block is the canonical freeform-meal shape.
	var meal map[string]any
	require.NoError(t, json.Unmarshal(out.Meal, &meal))
	assert.Equal(t, "Grilled chicken caesar salad", meal["effective_name"])
	assert.Equal(t, "lunch", meal["meal_type"])
	assert.Equal(t, 250.0, meal["quantity_g"])

	assert.Equal(t, 1, stub.calls())
}

// ============================================================================
// Validation errors (no vision call should happen)
// ============================================================================

func TestFromPhoto_MissingImageReturns400(t *testing.T) {
	stub := &stubVision{result: goodResult()}
	r, _ := setupPhoto(t, stub)
	body, ct := buildMultipart(t, nil, map[string]string{"quantity_g": "100"})

	rec := postPhoto(t, r, body, ct, "")
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"error":"image_required"}`, rec.Body.String())
	assert.Equal(t, 0, stub.calls(), "no vision call before validation passes")
}

func TestFromPhoto_OversizedImageReturns413(t *testing.T) {
	stub := &stubVision{result: goodResult()}
	pool := storetest.NewPool(t)
	pRepo := products.NewRepo(pool)
	mRepo := meals.NewRepo(pool)
	svc := meals.NewService(pool, mRepo, pRepo)
	h := meals.NewHandlers(svc)
	// Cap at 1KB to make the test fast — any reasonable JPEG exceeds this.
	h.SetVision(stub, 1024)
	r := gin.New()
	rg := r.Group("/")
	h.Register(rg)

	imgBytes := makeJPEG(t, 800, 600) // >1KB
	body, ct := buildMultipart(t, imgBytes, nil)

	rec := postPhoto(t, r, body, ct, "")
	require.Equal(t, http.StatusRequestEntityTooLarge, rec.Code, rec.Body.String())
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "image_too_large", resp["error"])
	assert.EqualValues(t, 1024, resp["max_bytes"])
	assert.Equal(t, 0, stub.calls())
}

func TestFromPhoto_InvalidMealTypeReturns400Pre_Vision(t *testing.T) {
	stub := &stubVision{result: goodResult()}
	r, _ := setupPhoto(t, stub)
	imgBytes := makeJPEG(t, 400, 300)
	body, ct := buildMultipart(t, imgBytes, map[string]string{"meal_type": "elevenses"})

	rec := postPhoto(t, r, body, ct, "")
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"error":"meal_type_invalid"}`, rec.Body.String())
	assert.Equal(t, 0, stub.calls(), "validation runs BEFORE the vision call")
}

func TestFromPhoto_UnsupportedMediaTypeReturns415(t *testing.T) {
	stub := &stubVision{result: goodResult()}
	r, _ := setupPhoto(t, stub)
	// Send gibberish bytes — Resize's content-type sniff fails and returns
	// ErrUnsupportedMediaType, which maps to 415.
	body, ct := buildMultipart(t, []byte("definitely not an image"), nil)

	rec := postPhoto(t, r, body, ct, "")
	require.Equal(t, http.StatusUnsupportedMediaType, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"error":"unsupported_media_type"}`, rec.Body.String())
	assert.Equal(t, 0, stub.calls())
}

// ============================================================================
// Vision error mapping
// ============================================================================

func TestFromPhoto_VisionRateLimited429(t *testing.T) {
	stub := &stubVision{err: &vision.ErrVisionRateLimited{RetryAfterSeconds: 42}}
	r, _ := setupPhoto(t, stub)
	body, ct := buildMultipart(t, makeJPEG(t, 400, 300), nil)

	rec := postPhoto(t, r, body, ct, "")
	require.Equal(t, http.StatusTooManyRequests, rec.Code, rec.Body.String())
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "vision_rate_limited", resp["error"])
	assert.EqualValues(t, 42, resp["retry_after_seconds"])
}

func TestFromPhoto_VisionTimeout504(t *testing.T) {
	stub := &stubVision{err: vision.ErrVisionTimeout}
	r, _ := setupPhoto(t, stub)
	body, ct := buildMultipart(t, makeJPEG(t, 400, 300), nil)

	rec := postPhoto(t, r, body, ct, "")
	require.Equal(t, http.StatusGatewayTimeout, rec.Code, rec.Body.String())
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "vision_timeout", resp["error"])
	assert.EqualValues(t, 30, resp["retry_after_seconds"])
}

func TestFromPhoto_VisionUnparseable502(t *testing.T) {
	stub := &stubVision{err: fmt.Errorf("wrap: %w", vision.ErrVisionResponseUnparseable)}
	r, _ := setupPhoto(t, stub)
	body, ct := buildMultipart(t, makeJPEG(t, 400, 300), nil)

	rec := postPhoto(t, r, body, ct, "")
	require.Equal(t, http.StatusBadGateway, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"error":"vision_response_unparseable"}`, rec.Body.String())
}

// ============================================================================
// No API key wired → 503
// ============================================================================

func TestFromPhoto_NoVisionClient503(t *testing.T) {
	// Pass nil — simulating ANTHROPIC_API_KEY unset at startup.
	r, _ := setupPhoto(t, nil)
	body, ct := buildMultipart(t, makeJPEG(t, 400, 300), nil)

	rec := postPhoto(t, r, body, ct, "")
	require.Equal(t, http.StatusServiceUnavailable, rec.Code, rec.Body.String())
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "vision_unavailable", resp["error"])
	assert.Equal(t, "ANTHROPIC_API_KEY not configured", resp["reason"])
}

// ============================================================================
// Idempotency
// ============================================================================

func TestFromPhoto_IdempotencyReplay_SameImageSameKey(t *testing.T) {
	stub := &stubVision{result: goodResult()}
	r, _ := setupPhoto(t, stub)
	imgBytes := makeJPEG(t, 400, 300)

	// multipart.Writer generates a random boundary per call, so to test
	// "byte-strict idempotency" (a real client replaying their queue sends
	// the same bytes) we build the body once and replay the captured bytes.
	body, ct := buildMultipart(t, imgBytes, map[string]string{"quantity_g": "150"})
	raw := body.Bytes()

	first := postPhoto(t, r, bytes.NewBuffer(append([]byte(nil), raw...)), ct, "photo-key-1")
	require.Equal(t, http.StatusCreated, first.Code, first.Body.String())

	second := postPhoto(t, r, bytes.NewBuffer(append([]byte(nil), raw...)), ct, "photo-key-1")
	require.Equal(t, http.StatusCreated, second.Code, second.Body.String())

	// The idempotency middleware hashes the body verbatim, so byte-identical
	// multipart payloads with the same Idempotency-Key replay the cached
	// response — only one vision call total.
	assert.Equal(t, 1, stub.calls(), "second request must NOT call vision again")
	assert.Equal(t, first.Body.String(), second.Body.String(), "byte-for-byte replay")
}

func TestFromPhoto_IdempotencyKeyConflict_DifferentImage409(t *testing.T) {
	stub := &stubVision{result: goodResult()}
	r, _ := setupPhoto(t, stub)
	imgA := makeJPEG(t, 400, 300)
	imgB := makeJPEG(t, 500, 400)

	bodyA, ctA := buildMultipart(t, imgA, nil)
	first := postPhoto(t, r, bodyA, ctA, "photo-key-2")
	require.Equal(t, http.StatusCreated, first.Code)

	bodyB, ctB := buildMultipart(t, imgB, nil)
	conflict := postPhoto(t, r, bodyB, ctB, "photo-key-2")
	require.Equal(t, http.StatusConflict, conflict.Code, conflict.Body.String())
	assert.JSONEq(t, `{"error":"idempotency_key_conflict"}`, conflict.Body.String())
}
