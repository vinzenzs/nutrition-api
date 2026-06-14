package products_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/products"
)

func createManualHelper(t *testing.T, r http.Handler, body string) *products.Product {
	t.Helper()
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/products", bytes.NewBufferString(body)))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var p products.Product
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &p))
	return &p
}

func TestCreateRecipe_HappyPath(t *testing.T) {
	r, _, _ := setup(t)
	skyr := createManualHelper(t, r, `{"name":"Skyr","nutriments_per_100g":{"kcal":60,"protein_g":11}}`)
	oats := createManualHelper(t, r, `{"name":"Oats","nutriments_per_100g":{"kcal":380,"protein_g":13}}`)

	body := fmt.Sprintf(`{
        "name":"Morning skyr bowl",
        "serving_size_g": 250,
        "components": [
            {"product_id":"%s","quantity_g":200},
            {"product_id":"%s","quantity_g":40}
        ]
    }`, skyr.ID, oats.ID)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/products/recipes", bytes.NewBufferString(body)))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "Morning skyr bowl", resp["name"])
	assert.Equal(t, string(products.SourceRecipe), resp["source"])

	// Computed nutriments live on the product itself.
	nutri, ok := resp["nutriments_per_100g"].(map[string]any)
	require.True(t, ok)
	// kcal = (60*200 + 380*40) / 240 = 113.333...
	assert.InDelta(t, 113.333, nutri["kcal"].(float64), 0.05)
	assert.InDelta(t, 11.333, nutri["protein_g"].(float64), 0.05)

	// nutriment_computed_at is set.
	assert.NotEmpty(t, resp["nutriment_computed_at"])

	// Components are echoed.
	comps, ok := resp["components"].([]any)
	require.True(t, ok)
	require.Len(t, comps, 2)
}

func TestCreateRecipe_MissingComponentsReturns400(t *testing.T) {
	r, _, _ := setup(t)
	body := `{"name":"Empty","components":[]}`
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/products/recipes", bytes.NewBufferString(body)))
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"components_required"}`, rec.Body.String())
}

func TestCreateRecipe_UnknownComponentReturns404(t *testing.T) {
	r, _, _ := setup(t)
	missing := uuid.New().String()
	body := fmt.Sprintf(`{"name":"X","components":[{"product_id":"%s","quantity_g":50}]}`, missing)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/products/recipes", bytes.NewBufferString(body)))
	require.Equal(t, http.StatusNotFound, rec.Code)
	var body2 map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body2))
	assert.Equal(t, "component_not_found", body2["error"])
	assert.Equal(t, missing, body2["product_id"])
}

func TestCreateRecipe_ZeroQuantityReturns400(t *testing.T) {
	r, _, _ := setup(t)
	skyr := createManualHelper(t, r, `{"name":"Skyr","nutriments_per_100g":{"kcal":60}}`)
	body := fmt.Sprintf(`{"name":"X","components":[{"product_id":"%s","quantity_g":0}]}`, skyr.ID)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/products/recipes", bytes.NewBufferString(body)))
	require.Equal(t, http.StatusBadRequest, rec.Code)
	var body2 map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body2))
	assert.Equal(t, "component_quantity_g_invalid", body2["error"])
}

func TestCreateRecipe_NestedRecipeRejected(t *testing.T) {
	r, _, _ := setup(t)
	skyr := createManualHelper(t, r, `{"name":"Skyr","nutriments_per_100g":{"kcal":60,"protein_g":11}}`)

	// Create a base recipe.
	body := fmt.Sprintf(`{"name":"Base","components":[{"product_id":"%s","quantity_g":100}]}`, skyr.ID)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/products/recipes", bytes.NewBufferString(body)))
	require.Equal(t, http.StatusCreated, rec.Code)
	var base map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &base))
	baseID := base["id"].(string)

	// Attempt to use the base recipe as a component of another recipe.
	body2 := fmt.Sprintf(`{"name":"Nested","components":[{"product_id":"%s","quantity_g":100}]}`, baseID)
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, httptest.NewRequest(http.MethodPost, "/products/recipes", bytes.NewBufferString(body2)))
	require.Equal(t, http.StatusBadRequest, rec2.Code)
	assert.JSONEq(t, `{"error":"recipe_as_component_not_supported"}`, rec2.Body.String())
}

func TestRecomputeRecipe_HappyPath(t *testing.T) {
	r, _, repo := setup(t)
	a := createManualHelper(t, r, `{"name":"A","nutriments_per_100g":{"kcal":100,"protein_g":5}}`)
	b := createManualHelper(t, r, `{"name":"B","nutriments_per_100g":{"kcal":200,"protein_g":10}}`)

	body := fmt.Sprintf(`{"name":"Mix","components":[{"product_id":"%s","quantity_g":100},{"product_id":"%s","quantity_g":100}]}`, a.ID, b.ID)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/products/recipes", bytes.NewBufferString(body)))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var initial map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &initial))
	recipeID := initial["id"].(string)
	// Initial kcal = (100*100 + 200*100) / 200 = 150
	initialKcal := initial["nutriments_per_100g"].(map[string]any)["kcal"].(float64)
	assert.InDelta(t, 150, initialKcal, 0.05)

	// Mutate component A's kcal directly via repo (simulates an OFF refresh).
	ctx := context.Background()
	aProd, err := repo.GetByID(ctx, a.ID)
	require.NoError(t, err)
	newKcal := 500.0
	aProd.Nutriments.KcalPer100g = &newKcal
	require.NoError(t, repo.UpdateFromOFF(ctx, aProd))

	// Recompute. Expected new kcal = (500*100 + 200*100) / 200 = 350.
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, httptest.NewRequest(http.MethodPost, "/products/recipes/"+recipeID+"/recompute", nil))
	require.Equal(t, http.StatusOK, rec2.Code, rec2.Body.String())
	var refreshed map[string]any
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &refreshed))
	assert.InDelta(t, 350, refreshed["nutriments_per_100g"].(map[string]any)["kcal"].(float64), 0.05)
}

func TestRecomputeRecipe_NonRecipeReturns400(t *testing.T) {
	r, _, _ := setup(t)
	manual := createManualHelper(t, r, `{"name":"plain","nutriments_per_100g":{"kcal":100}}`)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/products/recipes/"+manual.ID.String()+"/recompute", nil))
	require.Equal(t, http.StatusBadRequest, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "not_a_recipe", body["error"])
}

func TestRecomputeRecipe_MissingReturns404(t *testing.T) {
	r, _, _ := setup(t)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/products/recipes/"+uuid.New().String()+"/recompute", nil))
	require.Equal(t, http.StatusNotFound, rec.Code)
	assert.JSONEq(t, `{"error":"product_not_found"}`, rec.Body.String())
}

func TestGetByID_ExpandComponentsOnRecipe(t *testing.T) {
	r, _, _ := setup(t)
	a := createManualHelper(t, r, `{"name":"A","nutriments_per_100g":{"kcal":100}}`)
	b := createManualHelper(t, r, `{"name":"B","nutriments_per_100g":{"kcal":200}}`)
	body := fmt.Sprintf(`{"name":"Mix","components":[{"product_id":"%s","quantity_g":50},{"product_id":"%s","quantity_g":50}]}`, a.ID, b.ID)
	createRec := httptest.NewRecorder()
	r.ServeHTTP(createRec, httptest.NewRequest(http.MethodPost, "/products/recipes", bytes.NewBufferString(body)))
	require.Equal(t, http.StatusCreated, createRec.Code, createRec.Body.String())
	var created map[string]any
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))
	recipeID := created["id"].(string)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/products/"+recipeID+"?expand=components", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	comps, ok := resp["components"].([]any)
	require.True(t, ok)
	require.Len(t, comps, 2)
}

func TestGetByID_ExpandComponentsOnNonRecipeReturnsEmptyArray(t *testing.T) {
	r, _, _ := setup(t)
	p := createManualHelper(t, r, `{"name":"plain","nutriments_per_100g":{"kcal":100}}`)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/products/"+p.ID.String()+"?expand=components", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	comps, ok := resp["components"].([]any)
	require.True(t, ok)
	assert.Empty(t, comps)
}

func TestGetByID_NoExpandOmitsComponentsField(t *testing.T) {
	r, _, _ := setup(t)
	p := createManualHelper(t, r, `{"name":"plain","nutriments_per_100g":{"kcal":100}}`)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/products/"+p.ID.String(), nil))
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.NotContains(t, body, `"components"`)
}
