package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/auth"
	"github.com/vinzenzs/kazper/internal/goals"
	"github.com/vinzenzs/kazper/internal/idempotency"
	"github.com/vinzenzs/kazper/internal/meals"
	"github.com/vinzenzs/kazper/internal/products"
	"github.com/vinzenzs/kazper/internal/store/storetest"
	"github.com/vinzenzs/kazper/internal/summary"
	"github.com/vinzenzs/kazper/internal/vision"
)

// stubVisionParser implements meals.VisionParser without calling Anthropic.
type stubVisionParser struct {
	result *vision.ParseResult
}

func (s *stubVisionParser) Parse(_ context.Context, req vision.ParseRequest) (*vision.ParseResult, error) {
	cp := *s.result
	cp.ResizedTo = req.ResizedTo
	cp.OriginalBytes = req.OriginalBytes
	return &cp, nil
}

// bootServerWithVision is bootServer + an injected vision stub. The OFF
// stub from the regular bootServer is irrelevant for this test, so we don't
// wire products lookup; we wire only what the from_photo path needs.
func bootServerWithVision(t *testing.T, vc meals.VisionParser) *gin.Engine {
	t.Helper()
	pool := storetest.NewPool(t)

	pRepo := products.NewRepo(pool)
	mRepo := meals.NewRepo(pool)
	mSvc := meals.NewService(pool, mRepo, pRepo)
	gRepo := goals.NewRepo(pool)
	goRepo := goals.NewOverridesRepo(pool)
	resolver := goals.NewResolver(gRepo, goRepo, nil, nil)
	sSvc := summary.NewService(pool, mRepo, resolver)
	idRepo := idempotency.NewRepo(pool)

	mHandlers := meals.NewHandlers(mSvc)
	mHandlers.SetVision(vc, 10*1024*1024)

	r := gin.New()
	r.Use(gin.Recovery())
	api := r.Group("/")
	api.Use(auth.Middleware(auth.Config{MobileToken: mobileToken, AgentToken: agentToken}))
	api.Use(idempotency.Middleware(idRepo, time.Hour))
	mHandlers.Register(api)
	summary.NewHandlers(sSvc, "UTC", slog.New(slog.NewTextHandler(io.Discard, nil))).Register(api)
	return r
}

func makeE2EJPEG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 400, 300))
	for y := 0; y < 300; y++ {
		for x := 0; x < 400; x++ {
			img.Set(x, y, color.RGBA{R: 150, G: 200, B: 100, A: 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}))
	return buf.Bytes()
}

// TestE2E_PhotoToMealLandsInDailySummary posts a stub-vision-backed photo
// upload, then confirms the meal entry shows up in the same day's summary.
// This is the round-trip test that proves the photo path materializes a real
// meal_entries row reachable by every downstream summary endpoint.
func TestE2E_PhotoToMealLandsInDailySummary(t *testing.T) {
	loggedAt := "2026-06-09T12:00:00Z"
	stub := &stubVisionParser{
		result: &vision.ParseResult{
			Name: "Grilled chicken caesar salad",
			NutrimentsPer100g: vision.Nutriments{
				Kcal:     165,
				ProteinG: 14.2,
				CarbsG:   5.1,
				FatG:     9.8,
			},
			Confidence:   0.85,
			Model:        "claude-sonnet-4-6",
			InputTokens:  1240,
			OutputTokens: 89,
		},
	}
	r := bootServerWithVision(t, stub)

	// --- POST /meals/from_photo ---

	imageBytes := makeE2EJPEG(t)
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	fw, err := mw.CreateFormFile("image", "plate.jpg")
	require.NoError(t, err)
	_, err = io.Copy(fw, bytes.NewReader(imageBytes))
	require.NoError(t, err)
	require.NoError(t, mw.WriteField("quantity_g", "250"))
	require.NoError(t, mw.WriteField("meal_type", "lunch"))
	require.NoError(t, mw.WriteField("logged_at", loggedAt))
	require.NoError(t, mw.Close())

	req := httptest.NewRequest(http.MethodPost, "/meals/from_photo", body)
	req.Header.Set("Authorization", "Bearer "+mobileToken)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var postResp struct {
		Meal      json.RawMessage `json:"meal"`
		Inference struct {
			Model      string  `json:"model"`
			Confidence float64 `json:"confidence"`
		} `json:"inference"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &postResp))
	assert.Equal(t, "claude-sonnet-4-6", postResp.Inference.Model)
	assert.Equal(t, 0.85, postResp.Inference.Confidence)

	var createdMeal map[string]any
	require.NoError(t, json.Unmarshal(postResp.Meal, &createdMeal))
	createdName, _ := createdMeal["effective_name"].(string)
	require.Equal(t, "Grilled chicken caesar salad", createdName)

	// --- GET /summary/daily?date=2026-06-09 ---

	req = httptest.NewRequest(http.MethodGet,
		"/summary/daily?date=2026-06-09&tz=UTC", nil)
	req.Header.Set("Authorization", "Bearer "+mobileToken)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var daily struct {
		Date    string `json:"date"`
		Entries []struct {
			EffectiveName string `json:"effective_name"`
		} `json:"entries"`
		Totals struct {
			Kcal     float64 `json:"kcal"`
			ProteinG float64 `json:"protein_g"`
		} `json:"totals"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &daily))

	// 250g × 165 kcal/100g = 412.5 kcal; 250g × 14.2g/100g = 35.5g protein.
	require.Len(t, daily.Entries, 1)
	assert.Equal(t, "Grilled chicken caesar salad", daily.Entries[0].EffectiveName)
	assert.InDelta(t, 412.5, daily.Totals.Kcal, 0.1,
		"photo-derived meal contributes to the daily kcal total")
	assert.InDelta(t, 35.5, daily.Totals.ProteinG, 0.1,
		"photo-derived meal contributes to the daily protein total")
}
