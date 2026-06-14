package meals_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/meals"
	"github.com/vinzenzs/kazper/internal/products"
	"github.com/vinzenzs/kazper/internal/store"
	"github.com/vinzenzs/kazper/internal/store/storetest"
)

func init() {
	gin.SetMode(gin.TestMode)
}

type fixture struct {
	r            *gin.Engine
	productsRepo *products.Repo
	mealsRepo    *meals.Repo
	q            store.Querier
}

func setupMeals(t *testing.T) *fixture {
	t.Helper()
	p := storetest.NewPool(t)
	pRepo := products.NewRepo(p)
	mRepo := meals.NewRepo(p)
	svc := meals.NewService(p, mRepo, pRepo)
	r := gin.New()
	rg := r.Group("/")
	meals.NewHandlers(svc).Register(rg)
	return &fixture{r: r, productsRepo: pRepo, mealsRepo: mRepo, q: p}
}

// makeProduct inserts a sample Nutella-like product and returns its ID.
func makeProduct(t *testing.T, repo *products.Repo) uuid.UUID {
	t.Helper()
	kcal := 539.0
	protein := 6.3
	carbs := 57.5
	fat := 30.9
	p := &products.Product{
		Name:   "Nutella",
		Source: products.SourceManual,
		Nutriments: products.Nutriments{
			KcalPer100g:     &kcal,
			ProteinGPer100g: &protein,
			CarbsGPer100g:   &carbs,
			FatGPer100g:     &fat,
		},
	}
	require.NoError(t, repo.Insert(context.Background(), p))
	return p.ID
}

func doRequest(t *testing.T, r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Buffer
	if body != "" {
		reader = bytes.NewBufferString(body)
	} else {
		reader = bytes.NewBuffer(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// ============================================================================
// POST /meals
// ============================================================================

func TestCreateMeal_HappyPath(t *testing.T) {
	f := setupMeals(t)
	pid := makeProduct(t, f.productsRepo)

	body := fmt.Sprintf(`{"product_id":%q,"quantity_g":150,"logged_at":"2026-06-06T12:30:00Z"}`, pid)
	rec := doRequest(t, f.r, http.MethodPost, "/meals", body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var m meals.MealEntry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &m))
	require.NotNil(t, m.ProductID)
	assert.Equal(t, pid, *m.ProductID)
	assert.InDelta(t, 150.0, m.QuantityG, 0.001)
	assert.Equal(t, "Nutella", m.EffectiveName)
	require.NotNil(t, m.EffectiveNutrimentsPer100g.KcalPer100g)
	assert.InDelta(t, 539.0, *m.EffectiveNutrimentsPer100g.KcalPer100g, 0.001)

	// Product's last_logged_at should advance.
	updated, err := f.productsRepo.GetByID(context.Background(), pid)
	require.NoError(t, err)
	require.NotNil(t, updated.LastLoggedAt)
}

func TestCreateMeal_OptionalMealTypeAndNote(t *testing.T) {
	f := setupMeals(t)
	pid := makeProduct(t, f.productsRepo)

	body := fmt.Sprintf(`{"product_id":%q,"quantity_g":50,"logged_at":"2026-06-06T08:00:00Z","meal_type":"breakfast","note":"toast"}`, pid)
	rec := doRequest(t, f.r, http.MethodPost, "/meals", body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var m meals.MealEntry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &m))
	require.NotNil(t, m.MealType)
	assert.Equal(t, meals.Breakfast, *m.MealType)
	require.NotNil(t, m.Note)
	assert.Equal(t, "toast", *m.Note)
}

func TestCreateMeal_MissingProductIDReturns400(t *testing.T) {
	f := setupMeals(t)
	rec := doRequest(t, f.r, http.MethodPost, "/meals", `{"quantity_g":150,"logged_at":"2026-06-06T12:30:00Z"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"product_id_required"}`, rec.Body.String())
}

func TestCreateMeal_UnknownProductIDReturns404(t *testing.T) {
	f := setupMeals(t)
	body := fmt.Sprintf(`{"product_id":%q,"quantity_g":150,"logged_at":"2026-06-06T12:30:00Z"}`, uuid.New())
	rec := doRequest(t, f.r, http.MethodPost, "/meals", body)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.JSONEq(t, `{"error":"product_not_found"}`, rec.Body.String())
}

func TestCreateMeal_NonPositiveQuantityReturns400(t *testing.T) {
	f := setupMeals(t)
	pid := makeProduct(t, f.productsRepo)
	body := fmt.Sprintf(`{"product_id":%q,"quantity_g":0,"logged_at":"2026-06-06T12:30:00Z"}`, pid)
	rec := doRequest(t, f.r, http.MethodPost, "/meals", body)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"quantity_g_invalid"}`, rec.Body.String())
}

func TestCreateMeal_LoggedAtFarFutureReturns400(t *testing.T) {
	f := setupMeals(t)
	pid := makeProduct(t, f.productsRepo)
	farFuture := time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339)
	body := fmt.Sprintf(`{"product_id":%q,"quantity_g":100,"logged_at":%q}`, pid, farFuture)
	rec := doRequest(t, f.r, http.MethodPost, "/meals", body)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"logged_at_too_far_future"}`, rec.Body.String())
}

func TestCreateMeal_InvalidMealTypeReturns400(t *testing.T) {
	f := setupMeals(t)
	pid := makeProduct(t, f.productsRepo)
	body := fmt.Sprintf(`{"product_id":%q,"quantity_g":100,"logged_at":"2026-06-06T12:00:00Z","meal_type":"second_breakfast"}`, pid)
	rec := doRequest(t, f.r, http.MethodPost, "/meals", body)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"meal_type_invalid"}`, rec.Body.String())
}

// ============================================================================
// POST /meals/freeform
// ============================================================================

func TestCreateFreeform_WithoutSavingProduct(t *testing.T) {
	f := setupMeals(t)
	body := `{
        "name":"banana",
        "nutriments_per_100g":{"kcal":89,"protein_g":1.1,"carbs_g":22.8,"fat_g":0.3},
        "quantity_g":120,
        "logged_at":"2026-06-06T10:00:00Z"
    }`
	rec := doRequest(t, f.r, http.MethodPost, "/meals/freeform", body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var m meals.MealEntry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &m))
	assert.Nil(t, m.ProductID, "no product should be created without save_as_product")
	assert.Equal(t, "banana", m.EffectiveName)
	require.NotNil(t, m.EffectiveNutrimentsPer100g.KcalPer100g)
	assert.InDelta(t, 89.0, *m.EffectiveNutrimentsPer100g.KcalPer100g, 0.001)
}

func TestCreateFreeform_WithSaveAsProductCreatesReusableProduct(t *testing.T) {
	f := setupMeals(t)
	body := `{
        "name":"banana",
        "nutriments_per_100g":{"kcal":89,"protein_g":1.1},
        "quantity_g":120,
        "logged_at":"2026-06-06T10:00:00Z",
        "save_as_product": true
    }`
	rec := doRequest(t, f.r, http.MethodPost, "/meals/freeform", body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var m meals.MealEntry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &m))
	require.NotNil(t, m.ProductID)

	prod, err := f.productsRepo.GetByID(context.Background(), *m.ProductID)
	require.NoError(t, err)
	assert.Equal(t, "banana", prod.Name)
	assert.Equal(t, products.SourceManual, prod.Source)
	require.NotNil(t, prod.LastLoggedAt, "last_logged_at should be set")
}

func TestCreateFreeform_MissingNameReturns400(t *testing.T) {
	f := setupMeals(t)
	body := `{"nutriments_per_100g":{"kcal":89},"quantity_g":120,"logged_at":"2026-06-06T10:00:00Z"}`
	rec := doRequest(t, f.r, http.MethodPost, "/meals/freeform", body)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"name_required"}`, rec.Body.String())
}

func TestCreateFreeform_NegativeNutrimentReturns400(t *testing.T) {
	f := setupMeals(t)
	body := `{
        "name":"weird",
        "nutriments_per_100g":{"kcal":-1},
        "quantity_g":50,
        "logged_at":"2026-06-06T10:00:00Z"
    }`
	rec := doRequest(t, f.r, http.MethodPost, "/meals/freeform", body)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	var body2 map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body2))
	assert.Equal(t, "nutriments_invalid", body2["error"])
	assert.Equal(t, "kcal", body2["field"])
}

// ============================================================================
// GET /meals/{id}
// ============================================================================

func TestGetMeal_ReturnsEffectiveNutriments(t *testing.T) {
	f := setupMeals(t)
	pid := makeProduct(t, f.productsRepo)
	body := fmt.Sprintf(`{"product_id":%q,"quantity_g":100,"logged_at":"2026-06-06T12:00:00Z"}`, pid)
	createRec := doRequest(t, f.r, http.MethodPost, "/meals", body)
	require.Equal(t, http.StatusCreated, createRec.Code)
	var created meals.MealEntry
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))

	rec := doRequest(t, f.r, http.MethodGet, "/meals/"+created.ID.String(), "")
	require.Equal(t, http.StatusOK, rec.Code)
	var fetched meals.MealEntry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &fetched))
	assert.Equal(t, created.ID, fetched.ID)
	assert.Equal(t, "Nutella", fetched.EffectiveName)
}

func TestGetMeal_MissingReturns404(t *testing.T) {
	f := setupMeals(t)
	rec := doRequest(t, f.r, http.MethodGet, "/meals/"+uuid.New().String(), "")
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.JSONEq(t, `{"error":"meal_not_found"}`, rec.Body.String())
}

// ============================================================================
// GET /meals (list)
// ============================================================================

func TestListMeals_WindowFiltering(t *testing.T) {
	f := setupMeals(t)
	pid := makeProduct(t, f.productsRepo)

	insertAt := func(ts string) {
		body := fmt.Sprintf(`{"product_id":%q,"quantity_g":50,"logged_at":%q}`, pid, ts)
		rec := doRequest(t, f.r, http.MethodPost, "/meals", body)
		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	}
	insertAt("2026-05-31T12:00:00Z") // before window
	insertAt("2026-06-01T08:00:00Z") // in window
	insertAt("2026-06-07T12:00:00Z") // after window (window is [..., to))

	rec := doRequest(t, f.r, http.MethodGet, "/meals?from=2026-06-01T00:00:00Z&to=2026-06-07T00:00:00Z", "")
	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Meals []meals.MealEntry `json:"meals"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Len(t, body.Meals, 1)
}

func TestListMeals_MissingWindowReturns400(t *testing.T) {
	f := setupMeals(t)
	rec := doRequest(t, f.r, http.MethodGet, "/meals", "")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"window_required"}`, rec.Body.String())
}

func TestListMeals_InvertedWindowReturns400(t *testing.T) {
	f := setupMeals(t)
	rec := doRequest(t, f.r, http.MethodGet, "/meals?from=2026-06-07T00:00:00Z&to=2026-06-01T00:00:00Z", "")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"window_invalid"}`, rec.Body.String())
}

func TestListMeals_FilterByMealType(t *testing.T) {
	f := setupMeals(t)
	pid := makeProduct(t, f.productsRepo)
	bodyTpl := `{"product_id":%q,"quantity_g":50,"logged_at":%q,"meal_type":%q}`
	for _, e := range []struct{ ts, mt string }{
		{"2026-06-01T08:00:00Z", "breakfast"},
		{"2026-06-01T13:00:00Z", "lunch"},
		{"2026-06-01T20:00:00Z", "dinner"},
	} {
		rec := doRequest(t, f.r, http.MethodPost, "/meals", fmt.Sprintf(bodyTpl, pid, e.ts, e.mt))
		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	}
	rec := doRequest(t, f.r, http.MethodGet, "/meals?from=2026-06-01T00:00:00Z&to=2026-06-02T00:00:00Z&meal_type=lunch", "")
	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Meals []meals.MealEntry `json:"meals"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Meals, 1)
	require.NotNil(t, body.Meals[0].MealType)
	assert.Equal(t, meals.Lunch, *body.Meals[0].MealType)
}

// ============================================================================
// PATCH /meals/{id}
// ============================================================================

func TestPatchMeal_PartialUpdateOnlyChangesGivenFields(t *testing.T) {
	f := setupMeals(t)
	pid := makeProduct(t, f.productsRepo)
	body := fmt.Sprintf(`{"product_id":%q,"quantity_g":100,"logged_at":"2026-06-06T12:00:00Z","meal_type":"lunch"}`, pid)
	createRec := doRequest(t, f.r, http.MethodPost, "/meals", body)
	require.Equal(t, http.StatusCreated, createRec.Code)
	var created meals.MealEntry
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))

	rec := doRequest(t, f.r, http.MethodPatch, "/meals/"+created.ID.String(), `{"quantity_g":200}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var patched meals.MealEntry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &patched))
	assert.InDelta(t, 200.0, patched.QuantityG, 0.001)
	require.NotNil(t, patched.MealType)
	assert.Equal(t, meals.Lunch, *patched.MealType, "meal_type should be unchanged")
}

func TestPatchMeal_UnknownFieldsAreIgnored(t *testing.T) {
	f := setupMeals(t)
	pid := makeProduct(t, f.productsRepo)
	body := fmt.Sprintf(`{"product_id":%q,"quantity_g":100,"logged_at":"2026-06-06T12:00:00Z"}`, pid)
	createRec := doRequest(t, f.r, http.MethodPost, "/meals", body)
	require.Equal(t, http.StatusCreated, createRec.Code)
	var created meals.MealEntry
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))

	rec := doRequest(t, f.r, http.MethodPatch, "/meals/"+created.ID.String(),
		`{"quantity_g":175,"product_id":"00000000-0000-0000-0000-000000000000","snapshot_kcal_per_100g":42}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var patched meals.MealEntry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &patched))
	assert.InDelta(t, 175.0, patched.QuantityG, 0.001)
	require.NotNil(t, patched.ProductID)
	assert.Equal(t, pid, *patched.ProductID, "product_id should not have been changed")
}

func TestPatchMeal_InvalidQuantityReturns400(t *testing.T) {
	f := setupMeals(t)
	pid := makeProduct(t, f.productsRepo)
	body := fmt.Sprintf(`{"product_id":%q,"quantity_g":100,"logged_at":"2026-06-06T12:00:00Z"}`, pid)
	createRec := doRequest(t, f.r, http.MethodPost, "/meals", body)
	require.Equal(t, http.StatusCreated, createRec.Code)
	var created meals.MealEntry
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))

	rec := doRequest(t, f.r, http.MethodPatch, "/meals/"+created.ID.String(), `{"quantity_g":-1}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"quantity_g_invalid"}`, rec.Body.String())
}

func TestPatchMeal_AdvancesProductLastLoggedAt(t *testing.T) {
	f := setupMeals(t)
	pid := makeProduct(t, f.productsRepo)

	earlier := "2026-06-01T12:00:00Z"
	body := fmt.Sprintf(`{"product_id":%q,"quantity_g":100,"logged_at":%q}`, pid, earlier)
	createRec := doRequest(t, f.r, http.MethodPost, "/meals", body)
	require.Equal(t, http.StatusCreated, createRec.Code)
	var created meals.MealEntry
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))

	later := "2026-06-05T12:00:00Z"
	rec := doRequest(t, f.r, http.MethodPatch, "/meals/"+created.ID.String(),
		fmt.Sprintf(`{"logged_at":%q}`, later))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	updated, err := f.productsRepo.GetByID(context.Background(), pid)
	require.NoError(t, err)
	require.NotNil(t, updated.LastLoggedAt)
	assert.True(t, updated.LastLoggedAt.UTC().Equal(time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)))
}

func TestPatchMeal_OlderLoggedAtDoesNotRegressLastLoggedAt(t *testing.T) {
	f := setupMeals(t)
	pid := makeProduct(t, f.productsRepo)

	body := fmt.Sprintf(`{"product_id":%q,"quantity_g":100,"logged_at":"2026-06-05T12:00:00Z"}`, pid)
	createRec := doRequest(t, f.r, http.MethodPost, "/meals", body)
	require.Equal(t, http.StatusCreated, createRec.Code)
	var created meals.MealEntry
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))

	rec := doRequest(t, f.r, http.MethodPatch, "/meals/"+created.ID.String(),
		`{"logged_at":"2026-06-01T12:00:00Z"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	updated, err := f.productsRepo.GetByID(context.Background(), pid)
	require.NoError(t, err)
	require.NotNil(t, updated.LastLoggedAt)
	assert.True(t, updated.LastLoggedAt.UTC().Equal(time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)))
}

// ============================================================================
// DELETE /meals/{id}
// ============================================================================

func TestDeleteMeal_Returns204OnSuccess(t *testing.T) {
	f := setupMeals(t)
	pid := makeProduct(t, f.productsRepo)
	body := fmt.Sprintf(`{"product_id":%q,"quantity_g":100,"logged_at":"2026-06-06T12:00:00Z"}`, pid)
	createRec := doRequest(t, f.r, http.MethodPost, "/meals", body)
	require.Equal(t, http.StatusCreated, createRec.Code)
	var created meals.MealEntry
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))

	rec := doRequest(t, f.r, http.MethodDelete, "/meals/"+created.ID.String(), "")
	require.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, rec.Body.String())

	getRec := doRequest(t, f.r, http.MethodGet, "/meals/"+created.ID.String(), "")
	assert.Equal(t, http.StatusNotFound, getRec.Code)
}

func TestDeleteMeal_UnknownIDReturns404(t *testing.T) {
	f := setupMeals(t)
	rec := doRequest(t, f.r, http.MethodDelete, "/meals/"+uuid.New().String(), "")
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.JSONEq(t, `{"error":"meal_not_found"}`, rec.Body.String())
}

func TestDeleteMeal_DoesNotRevertProductLastLoggedAt(t *testing.T) {
	f := setupMeals(t)
	pid := makeProduct(t, f.productsRepo)
	body := fmt.Sprintf(`{"product_id":%q,"quantity_g":100,"logged_at":"2026-06-06T12:00:00Z"}`, pid)
	createRec := doRequest(t, f.r, http.MethodPost, "/meals", body)
	require.Equal(t, http.StatusCreated, createRec.Code)
	var created meals.MealEntry
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))

	rec := doRequest(t, f.r, http.MethodDelete, "/meals/"+created.ID.String(), "")
	require.Equal(t, http.StatusNoContent, rec.Code)

	updated, err := f.productsRepo.GetByID(context.Background(), pid)
	require.NoError(t, err)
	require.NotNil(t, updated.LastLoggedAt, "last_logged_at should still be set after delete")
}

// ============================================================================
// Micros + expand=components (daily-use-essentials section 6)
// ============================================================================

func TestCreateFreeform_AcceptsMicrosAndRoundTrips(t *testing.T) {
	f := setupMeals(t)
	body := `{
        "name":"banana",
        "nutriments_per_100g":{"kcal":89,"potassium_mg":358,"vitamin_c_mg":8.7},
        "quantity_g":120,
        "logged_at":"2026-06-06T10:00:00Z"
    }`
	rec := doRequest(t, f.r, http.MethodPost, "/meals/freeform", body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var m meals.MealEntry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &m))
	require.NotNil(t, m.EffectiveNutrimentsPer100g.PotassiumMgPer100g)
	assert.InDelta(t, 358.0, *m.EffectiveNutrimentsPer100g.PotassiumMgPer100g, 0.001)
	require.NotNil(t, m.EffectiveNutrimentsPer100g.VitaminCMgPer100g)
	assert.InDelta(t, 8.7, *m.EffectiveNutrimentsPer100g.VitaminCMgPer100g, 0.001)
	// Unsupplied micros must stay nil.
	assert.Nil(t, m.EffectiveNutrimentsPer100g.IronMgPer100g)
}

func TestCreateFreeform_SaveAsProductPropagatesMicros(t *testing.T) {
	f := setupMeals(t)
	body := `{
        "name":"banana",
        "nutriments_per_100g":{"kcal":89,"potassium_mg":358},
        "quantity_g":120,
        "logged_at":"2026-06-06T10:00:00Z",
        "save_as_product": true
    }`
	rec := doRequest(t, f.r, http.MethodPost, "/meals/freeform", body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var m meals.MealEntry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &m))
	require.NotNil(t, m.ProductID)

	prod, err := f.productsRepo.GetByID(context.Background(), *m.ProductID)
	require.NoError(t, err)
	require.NotNil(t, prod.Nutriments.PotassiumMgPer100g, "saved product should carry micros from freeform")
	assert.InDelta(t, 358.0, *prod.Nutriments.PotassiumMgPer100g, 0.001)
}

func TestCreateFreeform_NegativeMicroReturns400(t *testing.T) {
	f := setupMeals(t)
	body := `{
        "name":"bad",
        "nutriments_per_100g":{"iron_mg":-1},
        "quantity_g":50,
        "logged_at":"2026-06-06T10:00:00Z"
    }`
	rec := doRequest(t, f.r, http.MethodPost, "/meals/freeform", body)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	var body2 map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body2))
	assert.Equal(t, "nutriments_invalid", body2["error"])
	assert.Equal(t, "iron_mg", body2["field"])
}

func TestGetMeal_ExpandComponentsOnRecipe(t *testing.T) {
	f := setupMeals(t)
	ctx := context.Background()

	// Build two component products + a recipe wrapping them.
	a := makeProductNamed(t, f.productsRepo, "Component A", 100, 5)
	b := makeProductNamed(t, f.productsRepo, "Component B", 200, 10)
	serving := 200.0
	recipe := &products.Product{
		Name:         "Test Mix",
		Source:       products.SourceRecipe,
		ServingSizeG: &serving,
	}
	require.NoError(t, f.productsRepo.Insert(ctx, recipe))

	crepo := products.NewComponentsRepo(f.q)
	require.NoError(t, crepo.InsertComponents(ctx, recipe.ID, []products.Component{
		{ComponentProductID: a, QuantityG: 100},
		{ComponentProductID: b, QuantityG: 100},
	}))

	// Log a meal for this recipe at 400g (2x the 200g serving). Components
	// should scale by 2.
	body := fmt.Sprintf(`{"product_id":%q,"quantity_g":400,"logged_at":"2026-06-06T12:00:00Z"}`, recipe.ID)
	createRec := doRequest(t, f.r, http.MethodPost, "/meals", body)
	require.Equal(t, http.StatusCreated, createRec.Code, createRec.Body.String())
	var created meals.MealEntry
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))

	rec := doRequest(t, f.r, http.MethodGet, "/meals/"+created.ID.String()+"?expand=components", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var envelope map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	comps, ok := envelope["components"].([]any)
	require.True(t, ok)
	require.Len(t, comps, 2)
	// Each component was 100g per 200g recipe-serving, meal is 400g, so each scales to 200g.
	first := comps[0].(map[string]any)
	assert.InDelta(t, 200.0, first["quantity_g"].(float64), 0.001)
}

func TestGetMeal_ExpandComponentsOnFreeformReturnsEmpty(t *testing.T) {
	f := setupMeals(t)

	body := `{"name":"banana","nutriments_per_100g":{"kcal":89},"quantity_g":120,"logged_at":"2026-06-06T10:00:00Z"}`
	createRec := doRequest(t, f.r, http.MethodPost, "/meals/freeform", body)
	require.Equal(t, http.StatusCreated, createRec.Code)
	var created meals.MealEntry
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))

	rec := doRequest(t, f.r, http.MethodGet, "/meals/"+created.ID.String()+"?expand=components", "")
	require.Equal(t, http.StatusOK, rec.Code)
	var envelope map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	comps, ok := envelope["components"].([]any)
	require.True(t, ok)
	assert.Empty(t, comps)
}

// makeProductNamed inserts a simple manual product with a given name and kcal/protein.
func makeProductNamed(t *testing.T, repo *products.Repo, name string, kcal, protein float64) uuid.UUID {
	t.Helper()
	p := &products.Product{
		Name:   name,
		Source: products.SourceManual,
		Nutriments: products.Nutriments{
			KcalPer100g:     &kcal,
			ProteinGPer100g: &protein,
		},
	}
	require.NoError(t, repo.Insert(context.Background(), p))
	return p.ID
}

// ============================================================================
// last_logged_quantity_g tracking (add-last-logged-quantity)
// ============================================================================

func TestCreateMeal_AdvancesLastLoggedQuantity(t *testing.T) {
	f := setupMeals(t)
	pid := makeProduct(t, f.productsRepo)

	body := fmt.Sprintf(`{"product_id":%q,"quantity_g":200,"logged_at":"2026-06-06T12:30:00Z"}`, pid)
	rec := doRequest(t, f.r, http.MethodPost, "/meals", body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	updated, err := f.productsRepo.GetByID(context.Background(), pid)
	require.NoError(t, err)
	require.NotNil(t, updated.LastLoggedQuantityG, "creating a meal entry must populate last_logged_quantity_g")
	assert.InDelta(t, 200.0, *updated.LastLoggedQuantityG, 0.001)
}

func TestCreateMeal_BackdatedDoesNotRegressLastLoggedQuantity(t *testing.T) {
	f := setupMeals(t)
	pid := makeProduct(t, f.productsRepo)

	// First, more recent meal at 200g.
	body := fmt.Sprintf(`{"product_id":%q,"quantity_g":200,"logged_at":"2026-06-06T12:30:00Z"}`, pid)
	rec := doRequest(t, f.r, http.MethodPost, "/meals", body)
	require.Equal(t, http.StatusCreated, rec.Code)

	// Then a backdated 50g entry; the product should still reflect 200.
	older := fmt.Sprintf(`{"product_id":%q,"quantity_g":50,"logged_at":"2026-06-05T08:00:00Z"}`, pid)
	rec = doRequest(t, f.r, http.MethodPost, "/meals", older)
	require.Equal(t, http.StatusCreated, rec.Code)

	updated, err := f.productsRepo.GetByID(context.Background(), pid)
	require.NoError(t, err)
	require.NotNil(t, updated.LastLoggedQuantityG)
	assert.InDelta(t, 200.0, *updated.LastLoggedQuantityG, 0.001, "older meal must not regress last_logged_quantity_g")
}

func TestPatchMeal_QuantityOnMostRecentUpdatesProduct(t *testing.T) {
	f := setupMeals(t)
	pid := makeProduct(t, f.productsRepo)

	body := fmt.Sprintf(`{"product_id":%q,"quantity_g":200,"logged_at":"2026-06-06T12:30:00Z"}`, pid)
	rec := doRequest(t, f.r, http.MethodPost, "/meals", body)
	require.Equal(t, http.StatusCreated, rec.Code)
	var created meals.MealEntry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))

	patch := `{"quantity_g":300}`
	rec = doRequest(t, f.r, http.MethodPatch, "/meals/"+created.ID.String(), patch)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	updated, err := f.productsRepo.GetByID(context.Background(), pid)
	require.NoError(t, err)
	require.NotNil(t, updated.LastLoggedQuantityG)
	assert.InDelta(t, 300.0, *updated.LastLoggedQuantityG, 0.001)
}

func TestPatchMeal_QuantityOnOlderMealDoesNotUpdateProduct(t *testing.T) {
	f := setupMeals(t)
	pid := makeProduct(t, f.productsRepo)

	// Older meal at 100g.
	older := fmt.Sprintf(`{"product_id":%q,"quantity_g":100,"logged_at":"2026-06-05T08:00:00Z"}`, pid)
	rec := doRequest(t, f.r, http.MethodPost, "/meals", older)
	require.Equal(t, http.StatusCreated, rec.Code)
	var olderMeal meals.MealEntry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &olderMeal))

	// More recent meal at 200g — this becomes the "last logged" anchor.
	newer := fmt.Sprintf(`{"product_id":%q,"quantity_g":200,"logged_at":"2026-06-06T12:30:00Z"}`, pid)
	rec = doRequest(t, f.r, http.MethodPost, "/meals", newer)
	require.Equal(t, http.StatusCreated, rec.Code)

	// Now PATCH the OLDER meal's quantity_g to 500 — must not propagate.
	rec = doRequest(t, f.r, http.MethodPatch, "/meals/"+olderMeal.ID.String(), `{"quantity_g":500}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	updated, err := f.productsRepo.GetByID(context.Background(), pid)
	require.NoError(t, err)
	require.NotNil(t, updated.LastLoggedQuantityG)
	assert.InDelta(t, 200.0, *updated.LastLoggedQuantityG, 0.001, "PATCH on an older meal must NOT update the product's last_logged_quantity_g")
}

func TestDeleteMeal_DoesNotRevertLastLoggedQuantity(t *testing.T) {
	f := setupMeals(t)
	pid := makeProduct(t, f.productsRepo)

	body := fmt.Sprintf(`{"product_id":%q,"quantity_g":300,"logged_at":"2026-06-06T12:00:00Z"}`, pid)
	createRec := doRequest(t, f.r, http.MethodPost, "/meals", body)
	require.Equal(t, http.StatusCreated, createRec.Code)
	var m meals.MealEntry
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &m))

	rec := doRequest(t, f.r, http.MethodDelete, "/meals/"+m.ID.String(), "")
	require.Equal(t, http.StatusNoContent, rec.Code)

	updated, err := f.productsRepo.GetByID(context.Background(), pid)
	require.NoError(t, err)
	require.NotNil(t, updated.LastLoggedQuantityG, "deleting the most-recent meal must NOT revert last_logged_quantity_g")
	assert.InDelta(t, 300.0, *updated.LastLoggedQuantityG, 0.001)
}

func TestCreateFreeform_SaveAsProductCapturesInitialQuantity(t *testing.T) {
	f := setupMeals(t)
	body := `{
        "name":"banana",
        "nutriments_per_100g":{"kcal":89,"protein_g":1.1},
        "quantity_g":120,
        "logged_at":"2026-06-06T10:00:00Z",
        "save_as_product": true
    }`
	rec := doRequest(t, f.r, http.MethodPost, "/meals/freeform", body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var meal meals.MealEntry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &meal))
	require.NotNil(t, meal.ProductID)

	prod, err := f.productsRepo.GetByID(context.Background(), *meal.ProductID)
	require.NoError(t, err)
	require.NotNil(t, prod.LastLoggedQuantityG, "save_as_product must seed last_logged_quantity_g")
	assert.InDelta(t, 120.0, *prod.LastLoggedQuantityG, 0.001)
}

func TestGet_NullLastLoggedQuantityIsOmittedFromJSON(t *testing.T) {
	f := setupMeals(t)
	// Create a product that has never been logged.
	pid := makeProductNamed(t, f.productsRepo, "Never Logged", 200, 5)

	prod, err := f.productsRepo.GetByID(context.Background(), pid)
	require.NoError(t, err)
	assert.Nil(t, prod.LastLoggedQuantityG)

	// JSON marshal: omitempty should drop the field entirely.
	raw, err := json.Marshal(prod)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "last_logged_quantity_g")
}
