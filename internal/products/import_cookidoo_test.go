package products_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/nutrition-api/internal/cookidoo"
	"github.com/vinzenzs/nutrition-api/internal/off"
	"github.com/vinzenzs/nutrition-api/internal/products"
	"github.com/vinzenzs/nutrition-api/internal/store/storetest"
)

// stubCookidoo implements products.CookidooClient. It returns a fixed recipe (or
// error) and counts Fetch calls so tests can assert no outbound call happens for
// invalid URLs.
type stubCookidoo struct {
	recipe *cookidoo.Recipe
	err    error
	calls  int64
}

func (s *stubCookidoo) Fetch(_ context.Context, _ string) (*cookidoo.Recipe, error) {
	atomic.AddInt64(&s.calls, 1)
	if s.err != nil {
		return nil, s.err
	}
	dup := *s.recipe
	return &dup, nil
}

func setupImport(t *testing.T, ck products.CookidooClient) (*gin.Engine, *products.Repo) {
	t.Helper()
	pool := storetest.NewPool(t)
	repo := products.NewRepo(pool)
	stub := &stubOFF{products: map[string]*off.Product{}, errs: map[string]error{}}
	svc := products.NewService(pool, repo, stub)
	svc.SetCookidooClient(ck)
	r := gin.New()
	rg := r.Group("/")
	products.NewHandlers(svc).Register(rg)
	return r, repo
}

func sampleRecipe() *cookidoo.Recipe {
	kcal, protein, carbs, fat := 589.0, 24.0, 45.0, 32.0
	yield := 6
	tt := 90
	return &cookidoo.Recipe{
		Name:        "Vegetarische Linsen-Lasagne",
		Ingredients: []string{"1 Zwiebel", "100 g Staudensellerie", "400 g Feta"},
		NutritionPerServing: cookidoo.NutritionPerServing{
			Kcal: &kcal, ProteinG: &protein, CarbsG: &carbs, FatG: &fat,
		},
		ServingsYield: &yield,
		TotalTimeMin:  &tt,
	}
}

const recipeURL = "https://cookidoo.de/recipes/recipe/de-DE/r386806"

func postImport(t *testing.T, r *gin.Engine, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/products/import/cookidoo", bytes.NewBufferString(body)))
	return rec
}

func TestImportCookidoo_WithServingSizeConvertsToPer100g(t *testing.T) {
	ck := &stubCookidoo{recipe: sampleRecipe()}
	r, _ := setupImport(t, ck)

	rec := postImport(t, r, `{"url":"`+recipeURL+`","serving_size_g":450}`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "recipe", resp["source"])
	assert.Equal(t, recipeURL, resp["external_url"])
	assert.EqualValues(t, 450, resp["serving_size_g"])
	assert.Equal(t, []any{"1 Zwiebel", "100 g Staudensellerie", "400 g Feta"}, resp["ingredients"])
	assert.NotContains(t, rec.Body.String(), "needs_nutriments")

	nut, ok := resp["nutriments_per_100g"].(map[string]any)
	require.True(t, ok)
	// 589 kcal/serving * 100 / 450 g = 130.888...
	assert.InDelta(t, 589.0*100/450, nut["kcal"].(float64), 0.01)
	assert.InDelta(t, 45.0*100/450, nut["carbs_g"].(float64), 0.01)
}

func TestImportCookidoo_WithoutServingSizeSkipsNutriments(t *testing.T) {
	ck := &stubCookidoo{recipe: sampleRecipe()}
	r, _ := setupImport(t, ck)

	rec := postImport(t, r, `{"url":"`+recipeURL+`"}`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, true, resp["needs_nutriments"])
	// Ingredients still present.
	assert.Len(t, resp["ingredients"], 3)
	// Per-serving echo carries the parsed values for later conversion.
	echo, ok := resp["nutrition_per_serving"].(map[string]any)
	require.True(t, ok, "expected nutrition_per_serving echo")
	assert.InDelta(t, 589.0, echo["kcal"].(float64), 0.01)
	// No per-100g nutriments were stored.
	nut, _ := resp["nutriments_per_100g"].(map[string]any)
	assert.Empty(t, nut)
}

func TestImportCookidoo_NonCookidooURLRejectedNoOutboundCall(t *testing.T) {
	ck := &stubCookidoo{recipe: sampleRecipe()}
	r, _ := setupImport(t, ck)

	rec := postImport(t, r, `{"url":"https://evil.example.com/recipes/recipe/de-DE/r1"}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "invalid_cookidoo_url", body["error"])
	assert.Equal(t, int64(0), atomic.LoadInt64(&ck.calls), "no outbound fetch for an invalid URL")
}

func TestImportCookidoo_NoRecipeOnPageReturns502(t *testing.T) {
	ck := &stubCookidoo{err: cookidoo.ErrNoRecipeJSONLD}
	r, _ := setupImport(t, ck)

	rec := postImport(t, r, `{"url":"`+recipeURL+`"}`)
	require.Equal(t, http.StatusBadGateway, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "cookidoo_unavailable", body["error"])
	assert.Equal(t, "no_recipe_on_page", body["reason"])
}

func TestImportCookidoo_FetchFailedReturns502(t *testing.T) {
	ck := &stubCookidoo{err: &cookidoo.ErrFetchFailed{StatusCode: http.StatusInternalServerError}}
	r, _ := setupImport(t, ck)

	rec := postImport(t, r, `{"url":"`+recipeURL+`"}`)
	require.Equal(t, http.StatusBadGateway, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "cookidoo_unavailable", body["error"])
	assert.Equal(t, "fetch_failed", body["reason"])
}

func TestImportCookidoo_DuplicateReturnsExistingUntouched(t *testing.T) {
	ck := &stubCookidoo{recipe: sampleRecipe()}
	r, repo := setupImport(t, ck)

	// First import converts at 450 g/serving.
	rec1 := postImport(t, r, `{"url":"`+recipeURL+`","serving_size_g":450}`)
	require.Equal(t, http.StatusCreated, rec1.Code, rec1.Body.String())
	var first map[string]any
	require.NoError(t, json.Unmarshal(rec1.Body.Bytes(), &first))
	firstID := first["id"]
	firstKcal := first["nutriments_per_100g"].(map[string]any)["kcal"].(float64)

	// Re-import with a DIFFERENT serving size: must NOT overwrite — returns 200
	// with the existing product and its original converted nutriments intact.
	rec2 := postImport(t, r, `{"url":"`+recipeURL+`","serving_size_g":100}`)
	require.Equal(t, http.StatusOK, rec2.Code, rec2.Body.String())
	var second map[string]any
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &second))
	assert.Equal(t, true, second["already_imported"])
	assert.Equal(t, firstID, second["id"], "same product row")
	assert.InDelta(t, firstKcal, second["nutriments_per_100g"].(map[string]any)["kcal"].(float64), 0.001,
		"nutriments must not be overwritten by re-import")

	// Exactly one recipe product exists — the re-import did not create a duplicate.
	count, err := repo.Count(t.Context(), "recipe")
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}
