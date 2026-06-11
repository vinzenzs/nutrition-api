package products_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/nutrition-api/internal/products"
)

// A flat-imported recipe with ingredients round-trips: the array comes back in
// the same order with identical strings on the create response and on GET.
func TestCreateRecipe_WithIngredientsRoundTrips(t *testing.T) {
	r, _, _ := setup(t)
	reqBody := `{
        "name": "Vegetarische Linsen-Lasagne",
        "source": "recipe",
        "external_url": "https://cookidoo.de/recipes/recipe/de-DE/r386806",
        "serving_size_g": 450,
        "nutriments_per_100g": {"kcal": 131},
        "ingredients": ["1 Zwiebel", "100 g Staudensellerie", "3 Dosen stückige Tomaten"]
    }`
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/products", bytes.NewBufferString(reqBody)))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var created products.Product
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	assert.Equal(t, products.SourceRecipe, created.Source)
	require.Equal(t, []string{"1 Zwiebel", "100 g Staudensellerie", "3 Dosen stückige Tomaten"}, created.Ingredients)

	// GET returns the same array.
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/products/"+created.ID.String(), nil))
	require.Equal(t, http.StatusOK, rec2.Code, rec2.Body.String())
	var got products.Product
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &got))
	assert.Equal(t, []string{"1 Zwiebel", "100 g Staudensellerie", "3 Dosen stückige Tomaten"}, got.Ingredients)
}

// Ingredients on a non-recipe product are rejected with the typed code.
func TestCreateManual_IngredientsOnNonRecipeRejected(t *testing.T) {
	r, _, _ := setup(t)
	reqBody := `{
        "name": "Some snack",
        "source": "manual",
        "nutriments_per_100g": {"kcal": 200},
        "ingredients": ["oats", "honey"]
    }`
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/products", bytes.NewBufferString(reqBody)))
	require.Equal(t, http.StatusBadRequest, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "ingredients_require_recipe_source", body["error"])
}

// An oversized ingredients array (>100 entries) is rejected and no row persists.
func TestCreateRecipe_OversizedIngredientsRejected(t *testing.T) {
	r, _, repo := setup(t)
	entries := make([]string, 101)
	for i := range entries {
		entries[i] = "item"
	}
	arr, err := json.Marshal(entries)
	require.NoError(t, err)
	reqBody := `{
        "name": "Too many things",
        "source": "recipe",
        "external_url": "https://cookidoo.de/recipes/recipe/de-DE/r1",
        "nutriments_per_100g": {},
        "ingredients": ` + string(arr) + `
    }`
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/products", bytes.NewBufferString(reqBody)))
	require.Equal(t, http.StatusBadRequest, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "ingredients_invalid", body["error"])
	assert.Equal(t, "too_many", body["reason"])

	// No recipe product was created.
	n, err := repo.Count(t.Context(), "recipe")
	require.NoError(t, err)
	assert.Equal(t, int64(0), n)
}

// An empty-string entry is rejected with the per-entry index.
func TestCreateRecipe_EmptyIngredientEntryRejected(t *testing.T) {
	r, _, _ := setup(t)
	reqBody := `{
        "name": "Has a blank line",
        "source": "recipe",
        "external_url": "https://cookidoo.de/recipes/recipe/de-DE/r2",
        "nutriments_per_100g": {},
        "ingredients": ["onion", "   ", "garlic"]
    }`
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/products", bytes.NewBufferString(reqBody)))
	require.Equal(t, http.StatusBadRequest, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "ingredients_invalid", body["error"])
	assert.Equal(t, "empty_entry", body["reason"])
	assert.Equal(t, float64(1), body["index"])
}

// An entry over the 500-character limit is rejected.
func TestCreateRecipe_OverlongIngredientEntryRejected(t *testing.T) {
	r, _, _ := setup(t)
	long := strings.Repeat("x", 501)
	reqBody := `{
        "name": "Wordy",
        "source": "recipe",
        "external_url": "https://cookidoo.de/recipes/recipe/de-DE/r3",
        "nutriments_per_100g": {},
        "ingredients": ["` + long + `"]
    }`
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/products", bytes.NewBufferString(reqBody)))
	require.Equal(t, http.StatusBadRequest, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "ingredients_invalid", body["error"])
	assert.Equal(t, "entry_too_long", body["reason"])
}

// Products without ingredients omit the field entirely from the JSON response.
func TestCreateManual_NoIngredientsOmitsField(t *testing.T) {
	r, _, _ := setup(t)
	reqBody := `{"name":"Plain","nutriments_per_100g":{"kcal":50}}`
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/products", bytes.NewBufferString(reqBody)))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), "ingredients")

	var created products.Product
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/products/"+created.ID.String(), nil))
	require.Equal(t, http.StatusOK, rec2.Code)
	assert.NotContains(t, rec2.Body.String(), "ingredients")
}
