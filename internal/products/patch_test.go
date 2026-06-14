package products_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/products"
)

// PATCH sets nutriments on a recipe imported without a serving size, merging
// individual fields and leaving omitted ones (and name/serving) intact.
func TestPatchProduct_SetsNutrimentsAndServing(t *testing.T) {
	r, _, _ := setup(t)
	// Create a flat recipe with no nutriments (the serving-size-less import shape).
	create := `{"name":"Linsen-Lasagne","source":"recipe","external_url":"https://cookidoo.de/recipes/recipe/de-DE/r1","nutriments_per_100g":{},"ingredients":["1 Zwiebel"]}`
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/products", bytes.NewBufferString(create)))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var created products.Product
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	require.Nil(t, created.Nutriments.KcalPer100g)

	// PATCH in serving size + kcal/carbs only.
	patch := `{"serving_size_g":450,"nutriments_per_100g":{"kcal":131,"carbs_g":10}}`
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, httptest.NewRequest(http.MethodPatch, "/products/"+created.ID.String(), bytes.NewBufferString(patch)))
	require.Equal(t, http.StatusOK, rec2.Code, rec2.Body.String())
	var patched products.Product
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &patched))
	assert.Equal(t, "Linsen-Lasagne", patched.Name) // unchanged
	require.NotNil(t, patched.ServingSizeG)
	assert.InDelta(t, 450, *patched.ServingSizeG, 0.001)
	require.NotNil(t, patched.Nutriments.KcalPer100g)
	assert.InDelta(t, 131, *patched.Nutriments.KcalPer100g, 0.001)
	require.NotNil(t, patched.Nutriments.CarbsGPer100g)
	assert.InDelta(t, 10, *patched.Nutriments.CarbsGPer100g, 0.001)
	assert.Nil(t, patched.Nutriments.FatGPer100g) // omitted, stays unset

	// A second PATCH of just protein merges without clearing kcal.
	rec3 := httptest.NewRecorder()
	r.ServeHTTP(rec3, httptest.NewRequest(http.MethodPatch, "/products/"+created.ID.String(), bytes.NewBufferString(`{"nutriments_per_100g":{"protein_g":7}}`)))
	require.Equal(t, http.StatusOK, rec3.Code)
	var p3 products.Product
	require.NoError(t, json.Unmarshal(rec3.Body.Bytes(), &p3))
	require.NotNil(t, p3.Nutriments.ProteinGPer100g)
	assert.InDelta(t, 7, *p3.Nutriments.ProteinGPer100g, 0.001)
	require.NotNil(t, p3.Nutriments.KcalPer100g) // still there
	assert.InDelta(t, 131, *p3.Nutriments.KcalPer100g, 0.001)
}

func TestPatchProduct_UnknownReturns404(t *testing.T) {
	r, _, _ := setup(t)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/products/00000000-0000-0000-0000-000000000000", bytes.NewBufferString(`{"name":"x"}`)))
	require.Equal(t, http.StatusNotFound, rec.Code)
	assert.JSONEq(t, `{"error":"product_not_found"}`, rec.Body.String())
}

func TestPatchProduct_EmptyNameRejected(t *testing.T) {
	r, _, _ := setup(t)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/products", bytes.NewBufferString(`{"name":"Snack","nutriments_per_100g":{"kcal":50}}`)))
	require.Equal(t, http.StatusCreated, rec.Code)
	var p products.Product
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &p))

	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, httptest.NewRequest(http.MethodPatch, "/products/"+p.ID.String(), bytes.NewBufferString(`{"name":"   "}`)))
	require.Equal(t, http.StatusBadRequest, rec2.Code)
	assert.JSONEq(t, `{"error":"name_required"}`, rec2.Body.String())
}
