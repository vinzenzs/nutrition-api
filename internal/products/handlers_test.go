package products_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/off"
	"github.com/vinzenzs/kazper/internal/products"
	"github.com/vinzenzs/kazper/internal/store/storetest"
)

func init() {
	gin.SetMode(gin.TestMode)
}

type stubOFF struct {
	products map[string]*off.Product
	errs     map[string]error
	calls    int64
}

func (s *stubOFF) Fetch(_ context.Context, barcode string) (*off.Product, error) {
	atomic.AddInt64(&s.calls, 1)
	if err, ok := s.errs[barcode]; ok {
		return nil, err
	}
	if p, ok := s.products[barcode]; ok {
		// Return a copy so tests can mutate independently.
		dup := *p
		return &dup, nil
	}
	return nil, off.ErrProductNotFound
}

func setup(t *testing.T) (*gin.Engine, *stubOFF, *products.Repo) {
	t.Helper()
	pool := storetest.NewPool(t)
	repo := products.NewRepo(pool)
	stub := &stubOFF{products: map[string]*off.Product{}, errs: map[string]error{}}
	svc := products.NewService(pool, repo, stub)
	r := gin.New()
	rg := r.Group("/")
	products.NewHandlers(svc).Register(rg)
	return r, stub, repo
}

func makeOFFProduct() *off.Product {
	kcal := 539.0
	protein := 6.3
	carbs := 57.5
	fat := 30.9
	salt := 0.107
	serving := 15.0
	return &off.Product{
		Barcode: "3017624010701",
		Name:    "Nutella",
		Brand:   "Ferrero",
		Nutriments: off.Nutriments{
			KcalPer100g:     &kcal,
			ProteinGPer100g: &protein,
			CarbsGPer100g:   &carbs,
			FatGPer100g:     &fat,
			SaltGPer100g:    &salt,
		},
		ServingSizeG: &serving,
		RawPayload:   []byte(`{"code":"3017624010701","status":1}`),
	}
}

func TestLookup_FirstCallFetchesFromOFFAndCaches(t *testing.T) {
	r, stub, _ := setup(t)
	stub.products["3017624010701"] = makeOFFProduct()

	req := httptest.NewRequest(http.MethodPost, "/products/lookup/3017624010701", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, int64(1), atomic.LoadInt64(&stub.calls))

	var p products.Product
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &p))
	assert.Equal(t, "Nutella", p.Name)
	require.NotNil(t, p.Barcode)
	assert.Equal(t, "3017624010701", *p.Barcode)
	assert.Equal(t, products.SourceOFF, p.Source)

	// Second call: should hit cache, no new OFF call.
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, httptest.NewRequest(http.MethodPost, "/products/lookup/3017624010701", nil))
	require.Equal(t, http.StatusOK, rec2.Code)
	assert.Equal(t, int64(1), atomic.LoadInt64(&stub.calls), "cached lookup should not re-call OFF")
}

func TestLookup_RefreshTrueReFetches(t *testing.T) {
	r, stub, _ := setup(t)
	stub.products["3017624010701"] = makeOFFProduct()

	// First call caches it.
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/products/lookup/3017624010701", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	// Mutate the stub: new "Nutella Plus" data.
	updated := makeOFFProduct()
	updated.Name = "Nutella Plus"
	stub.products["3017624010701"] = updated

	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, httptest.NewRequest(http.MethodPost, "/products/lookup/3017624010701?refresh=true", nil))
	require.Equal(t, http.StatusOK, rec2.Code)
	var p products.Product
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &p))
	assert.Equal(t, "Nutella Plus", p.Name)
	assert.Equal(t, int64(2), atomic.LoadInt64(&stub.calls))
}

func TestLookup_UnknownBarcodeReturns404WithNext(t *testing.T) {
	r, _, _ := setup(t)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/products/lookup/9999999999999", nil))
	require.Equal(t, http.StatusNotFound, rec.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "product_not_found", body["error"])
	assert.Equal(t, "9999999999999", body["barcode"])
	assert.Equal(t, "POST /meals/freeform", body["next"])
}

func TestLookup_UpstreamTimeoutReturns504(t *testing.T) {
	r, stub, _ := setup(t)
	stub.errs["8888888888888"] = off.ErrUpstreamTimeout

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/products/lookup/8888888888888", nil))
	assert.Equal(t, http.StatusGatewayTimeout, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "upstream_timeout", body["error"])
}

func TestLookup_Upstream4xxReturns502(t *testing.T) {
	r, stub, _ := setup(t)
	stub.errs["7777777777777"] = &off.UnexpectedStatusError{StatusCode: 403}

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/products/lookup/7777777777777", nil))
	assert.Equal(t, http.StatusBadGateway, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "upstream_unexpected_response", body["error"])
}

func TestCreateManual_HappyPath(t *testing.T) {
	r, _, _ := setup(t)
	reqBody := `{
        "name": "Homemade granola",
        "nutriments_per_100g": {"kcal": 420, "protein_g": 12, "carbs_g": 55, "fat_g": 18}
    }`
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/products", bytes.NewBufferString(reqBody)))
	require.Equal(t, http.StatusCreated, rec.Code)
	var p products.Product
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &p))
	assert.Equal(t, "Homemade granola", p.Name)
	assert.Equal(t, products.SourceManual, p.Source)
	assert.Nil(t, p.Barcode)
	require.NotNil(t, p.Nutriments.KcalPer100g)
	assert.InDelta(t, 420.0, *p.Nutriments.KcalPer100g, 0.001)
}

func TestCreateManual_WithBarcodeStoredAsManual(t *testing.T) {
	r, _, _ := setup(t)
	reqBody := `{"name":"Local Yogurt","barcode":"5555555555555","nutriments_per_100g":{"kcal":80}}`
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/products", bytes.NewBufferString(reqBody)))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var p products.Product
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &p))
	assert.Equal(t, products.SourceManual, p.Source)
	require.NotNil(t, p.Barcode)
	assert.Equal(t, "5555555555555", *p.Barcode)
}

func TestCreateManual_DuplicateBarcodeReturns409(t *testing.T) {
	r, _, _ := setup(t)
	first := `{"name":"A","barcode":"4444444444444","nutriments_per_100g":{}}`
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/products", bytes.NewBufferString(first)))
	require.Equal(t, http.StatusCreated, rec.Code)
	var firstP products.Product
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &firstP))

	dup := `{"name":"B","barcode":"4444444444444","nutriments_per_100g":{}}`
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, httptest.NewRequest(http.MethodPost, "/products", bytes.NewBufferString(dup)))
	require.Equal(t, http.StatusConflict, rec2.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &body))
	assert.Equal(t, "barcode_already_exists", body["error"])
	assert.Equal(t, firstP.ID.String(), body["product_id"])
}

func TestCreateManual_MissingNameReturns400(t *testing.T) {
	r, _, _ := setup(t)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/products", bytes.NewBufferString(`{"nutriments_per_100g":{}}`)))
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"name_required"}`, rec.Body.String())
}

func TestSearch_RanksByLastLoggedAtDesc(t *testing.T) {
	r, _, repo := setup(t)

	// Three products, two with non-null last_logged_at.
	now := time.Now().UTC()
	older := now.Add(-72 * time.Hour)
	mustInsert := func(name string, lastLogged *time.Time) uuid.UUID {
		p := &products.Product{
			Name:         name,
			Source:       products.SourceManual,
			LastLoggedAt: lastLogged,
		}
		require.NoError(t, repo.Insert(context.Background(), p))
		return p.ID
	}
	yesterdayID := mustInsert("Yogurt fresh", &now)
	oldID := mustInsert("Yogurt vintage", &older)
	neverID := mustInsert("Yogurt new", nil)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/products/search?q=yogurt", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	var body struct{ Results []products.Product }
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Results, 3)
	assert.Equal(t, yesterdayID, body.Results[0].ID)
	assert.Equal(t, oldID, body.Results[1].ID)
	assert.Equal(t, neverID, body.Results[2].ID)
}

func TestSearch_MissingQReturns400(t *testing.T) {
	r, _, _ := setup(t)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/products/search", nil))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"q_required"}`, rec.Body.String())
}

func TestGetByID_NotFoundReturns404(t *testing.T) {
	r, _, _ := setup(t)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/products/"+uuid.New().String(), nil))
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.JSONEq(t, `{"error":"product_not_found"}`, rec.Body.String())
}

func TestCreateManual_AcceptsMicrosAndRoundTrips(t *testing.T) {
	r, _, _ := setup(t)
	reqBody := `{
        "name": "Fortified oat milk",
        "nutriments_per_100g": {
            "kcal": 48, "protein_g": 1.5,
            "calcium_mg": 120, "vitamin_d_mcg": 1.5, "vitamin_b12_mcg": 0.4,
            "iron_mg": 1.8
        }
    }`
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/products", bytes.NewBufferString(reqBody)))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var p products.Product
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &p))
	require.NotNil(t, p.Nutriments.CalciumMgPer100g)
	assert.InDelta(t, 120, *p.Nutriments.CalciumMgPer100g, 0.001)
	require.NotNil(t, p.Nutriments.VitaminB12McgPer100g)
	assert.InDelta(t, 0.4, *p.Nutriments.VitaminB12McgPer100g, 0.001)

	// GET round-trip surfaces the same micros, and omits unsupplied ones.
	getRec := httptest.NewRecorder()
	r.ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "/products/"+p.ID.String(), nil))
	require.Equal(t, http.StatusOK, getRec.Code)

	body := getRec.Body.String()
	assert.Contains(t, body, `"calcium_mg":120`)
	assert.Contains(t, body, `"vitamin_b12_mcg":0.4`)
	// Unsupplied micros must not appear in the JSON (omitempty on nil).
	assert.NotContains(t, body, `"potassium_mg"`)
	assert.NotContains(t, body, `"zinc_mg"`)
}

func TestCreateManual_NegativeMicroReturns400(t *testing.T) {
	r, _, _ := setup(t)
	reqBody := `{
        "name": "Bad row",
        "nutriments_per_100g": {"kcal": 100, "iron_mg": -1}
    }`
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/products", bytes.NewBufferString(reqBody)))
	require.Equal(t, http.StatusBadRequest, rec.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "nutriments_invalid", body["error"])
	assert.Equal(t, "iron_mg", body["field"])
}

func TestLookup_OFFMicrosFlowThrough(t *testing.T) {
	// When the OFF stub returns a product with micros, the cached row and the
	// API response must expose them.
	r, stub, _ := setup(t)

	iron := 1.8
	calcium := 120.0
	vitD := 1.5
	vitB12 := 0.38
	stub.products["5060337640008"] = &off.Product{
		Barcode: "5060337640008",
		Name:    "Fortified Oat Milk",
		Nutriments: off.Nutriments{
			KcalPer100g:          ptrFloat(48),
			IronMgPer100g:        &iron,
			CalciumMgPer100g:     &calcium,
			VitaminDMcgPer100g:   &vitD,
			VitaminB12McgPer100g: &vitB12,
		},
		RawPayload: []byte(`{"status":1}`),
	}

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/products/lookup/5060337640008", nil))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var p products.Product
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &p))
	require.NotNil(t, p.Nutriments.IronMgPer100g)
	assert.InDelta(t, 1.8, *p.Nutriments.IronMgPer100g, 0.001)
	require.NotNil(t, p.Nutriments.CalciumMgPer100g)
	assert.InDelta(t, 120, *p.Nutriments.CalciumMgPer100g, 0.001)
	require.NotNil(t, p.Nutriments.VitaminB12McgPer100g)
	assert.InDelta(t, 0.38, *p.Nutriments.VitaminB12McgPer100g, 0.001)
	// Unsupplied micros stay nil.
	assert.Nil(t, p.Nutriments.PotassiumMgPer100g)
	assert.Nil(t, p.Nutriments.ZincMgPer100g)
}

func ptrFloat(v float64) *float64 { return &v }

func TestLookup_EmptyBarcodeReturns400(t *testing.T) {
	r, _, _ := setup(t)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/products/lookup/", nil))
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"barcode_required"}`, rec.Body.String())
}

// ----- list endpoint -----

// listResponse mirrors the GET /products body for test deserialisation.
type listResponse struct {
	Products []*products.Product `json:"products"`
	Total    int64               `json:"total"`
	Limit    int                 `json:"limit"`
	Offset   int                 `json:"offset"`
}

func TestList_DefaultPaginationAndOrdering(t *testing.T) {
	r, _, repo := setup(t)
	// Insert three manual products with deliberately staggered last_logged_at
	// to lock down the recency-then-name ordering.
	now := time.Now().UTC()
	older := now.Add(-72 * time.Hour)
	mustInsert := func(name string, lastLogged *time.Time) uuid.UUID {
		p := &products.Product{Name: name, Source: products.SourceManual, LastLoggedAt: lastLogged}
		require.NoError(t, repo.Insert(context.Background(), p))
		return p.ID
	}
	freshID := mustInsert("Yogurt fresh", &now)
	vintID := mustInsert("Yogurt vintage", &older)
	newID := mustInsert("Yogurt new", nil)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/products", nil))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var body listResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, int64(3), body.Total)
	assert.Equal(t, 50, body.Limit)
	assert.Equal(t, 0, body.Offset)
	require.Len(t, body.Products, 3)
	assert.Equal(t, freshID, body.Products[0].ID)
	assert.Equal(t, vintID, body.Products[1].ID)
	assert.Equal(t, newID, body.Products[2].ID)
}

func TestList_SourceFilter(t *testing.T) {
	r, _, repo := setup(t)
	require.NoError(t, repo.Insert(context.Background(), &products.Product{Name: "A", Source: products.SourceManual}))
	require.NoError(t, repo.Insert(context.Background(), &products.Product{Name: "B", Source: products.SourceManual}))
	// Insert one OFF product (with a barcode so it's distinguishable).
	b := "1111111111111"
	require.NoError(t, repo.Insert(context.Background(), &products.Product{Name: "Z", Source: products.SourceOFF, Barcode: &b}))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/products?source=manual", nil))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body listResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, int64(2), body.Total)
	assert.Len(t, body.Products, 2)
}

func TestList_PaginationLimitAndOffset(t *testing.T) {
	r, _, repo := setup(t)
	for i := 0; i < 5; i++ {
		require.NoError(t, repo.Insert(context.Background(), &products.Product{Name: fmt.Sprintf("P%02d", i), Source: products.SourceManual}))
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/products?limit=2&offset=2", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var body listResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, int64(5), body.Total)
	assert.Equal(t, 2, body.Limit)
	assert.Equal(t, 2, body.Offset)
	assert.Len(t, body.Products, 2)
}

func TestList_InvalidSourceReturns400(t *testing.T) {
	r, _, _ := setup(t)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/products?source=nope", nil))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"source_invalid"}`, rec.Body.String())
}

func TestList_LimitTooLargeReturns400(t *testing.T) {
	r, _, _ := setup(t)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/products?limit=300", nil))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), `"error":"limit_too_large"`)
	assert.Contains(t, rec.Body.String(), `"max":200`)
}

func TestList_NegativeLimitReturns400(t *testing.T) {
	r, _, _ := setup(t)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/products?limit=-1", nil))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"pagination_invalid"}`, rec.Body.String())
}

func TestList_NonNumericOffsetReturns400(t *testing.T) {
	r, _, _ := setup(t)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/products?offset=abc", nil))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"pagination_invalid"}`, rec.Body.String())
}

// ----- delete endpoint -----

func TestDelete_HappyPathReturns204(t *testing.T) {
	r, _, repo := setup(t)
	p := &products.Product{Name: "delete-me", Source: products.SourceManual}
	require.NoError(t, repo.Insert(context.Background(), p))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/products/"+p.ID.String(), nil))
	require.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, rec.Body.String())

	// Confirm the row is actually gone.
	_, err := repo.GetByID(context.Background(), p.ID)
	assert.ErrorIs(t, err, products.ErrNotFound)
}

func TestDelete_UnknownIDReturns404(t *testing.T) {
	r, _, _ := setup(t)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/products/"+uuid.New().String(), nil))
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.JSONEq(t, `{"error":"product_not_found"}`, rec.Body.String())
}

func TestDelete_InUseAsComponentReturns409(t *testing.T) {
	r, _, repo := setup(t)
	ctx := context.Background()

	// Two components and a recipe that references them.
	a := &products.Product{Name: "ingredient-A", Source: products.SourceManual, Nutriments: products.Nutriments{KcalPer100g: ptrFloat(100), ProteinGPer100g: ptrFloat(10)}}
	require.NoError(t, repo.Insert(ctx, a))
	b := &products.Product{Name: "ingredient-B", Source: products.SourceManual, Nutriments: products.Nutriments{KcalPer100g: ptrFloat(50), ProteinGPer100g: ptrFloat(2)}}
	require.NoError(t, repo.Insert(ctx, b))

	recipeBody := fmt.Sprintf(
		`{"name":"reference recipe","serving_size_g":100,"components":[{"product_id":%q,"quantity_g":100},{"product_id":%q,"quantity_g":50}]}`,
		a.ID, b.ID,
	)
	createRec := httptest.NewRecorder()
	r.ServeHTTP(createRec, httptest.NewRequest(http.MethodPost, "/products/recipes", bytes.NewBufferString(recipeBody)))
	require.Equal(t, http.StatusCreated, createRec.Code, createRec.Body.String())

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/products/"+a.ID.String(), nil))
	require.Equal(t, http.StatusConflict, rec.Code)
	var body struct {
		Error   string `json:"error"`
		Recipes []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"recipes"`
		Hint string `json:"hint"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "product_in_use_as_component", body.Error)
	require.Len(t, body.Recipes, 1)
	assert.Equal(t, "reference recipe", body.Recipes[0].Name)
	assert.Contains(t, body.Hint, "delete the listed recipes")
}

func TestDelete_MaterialisesSnapshotIntoHistoricalMeals(t *testing.T) {
	// Set up a meal that referenced a product (no freeform snapshot), delete
	// the product, then assert the meal's snapshot columns were populated so
	// the meal retains its name + nutriments.
	pool := storetest.NewPool(t)
	ctx := context.Background()
	repo := products.NewRepo(pool)
	stub := &stubOFF{products: map[string]*off.Product{}}
	svc := products.NewService(pool, repo, stub)

	p := &products.Product{
		Name:       "Future Deleted Nutella",
		Source:     products.SourceManual,
		Nutriments: products.Nutriments{KcalPer100g: ptrFloat(539), ProteinGPer100g: ptrFloat(6.3)},
	}
	require.NoError(t, repo.Insert(ctx, p))

	// Log a meal entry directly via SQL since exposing the meals service
	// here would couple this test to that package. The product_id link is
	// what matters; snapshot columns start null.
	mealID := uuid.New()
	_, err := pool.Exec(ctx, `
        INSERT INTO meal_entries (id, product_id, logged_at, quantity_g, created_at, updated_at)
        VALUES ($1, $2, $3, $4, now(), now())
    `, mealID, p.ID, time.Date(2026, 6, 6, 8, 0, 0, 0, time.UTC), 100.0)
	require.NoError(t, err)

	require.NoError(t, svc.Delete(ctx, p.ID))

	var snapshotName *string
	var snapshotKcal *float64
	var snapshotProtein *float64
	var productID *uuid.UUID
	err = pool.QueryRow(ctx, `
        SELECT product_id, snapshot_name, snapshot_kcal_per_100g, snapshot_protein_g_per_100g
        FROM meal_entries WHERE id = $1
    `, mealID).Scan(&productID, &snapshotName, &snapshotKcal, &snapshotProtein)
	require.NoError(t, err)
	assert.Nil(t, productID, "FK ON DELETE SET NULL should clear product_id")
	require.NotNil(t, snapshotName)
	assert.Equal(t, "Future Deleted Nutella", *snapshotName)
	require.NotNil(t, snapshotKcal)
	assert.InDelta(t, 539.0, *snapshotKcal, 0.001)
	require.NotNil(t, snapshotProtein)
	assert.InDelta(t, 6.3, *snapshotProtein, 0.001)
}

func TestDelete_DoesNotOverwriteFreeformSnapshot(t *testing.T) {
	// A freeform meal that ALSO links a product (save_as_product=true) carries
	// its own snapshot — deletion of the linked product must leave the
	// freeform snapshot intact rather than overwriting with product values.
	pool := storetest.NewPool(t)
	ctx := context.Background()
	repo := products.NewRepo(pool)
	stub := &stubOFF{products: map[string]*off.Product{}}
	svc := products.NewService(pool, repo, stub)

	p := &products.Product{Name: "Pristine Product", Source: products.SourceManual, Nutriments: products.Nutriments{KcalPer100g: ptrFloat(200)}}
	require.NoError(t, repo.Insert(ctx, p))

	// Meal stores its own snapshot AND links the product.
	mealID := uuid.New()
	snapshotName := "Freeform Custom Name"
	freeformKcal := 95.5
	_, err := pool.Exec(ctx, `
        INSERT INTO meal_entries (id, product_id, logged_at, quantity_g, snapshot_name, snapshot_kcal_per_100g, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, now(), now())
    `, mealID, p.ID, time.Now().UTC(), 80.0, snapshotName, freeformKcal)
	require.NoError(t, err)

	require.NoError(t, svc.Delete(ctx, p.ID))

	var gotName *string
	var gotKcal *float64
	err = pool.QueryRow(ctx, `SELECT snapshot_name, snapshot_kcal_per_100g FROM meal_entries WHERE id = $1`, mealID).Scan(&gotName, &gotKcal)
	require.NoError(t, err)
	require.NotNil(t, gotName)
	assert.Equal(t, "Freeform Custom Name", *gotName, "pre-existing snapshot must be preserved")
	require.NotNil(t, gotKcal)
	assert.InDelta(t, 95.5, *gotKcal, 0.001, "pre-existing snapshot kcal must be preserved")
}

func TestDelete_InUseRecipesAreDedupedAcrossLegacyDuplicates(t *testing.T) {
	// Regression: when a recipe contains the same component product more
	// than once (only possible for rows created BEFORE the
	// add-product-management-tools change added the upfront duplicate-component
	// rejection), the RecipesUsing query joins through product_components
	// and would naturally produce one ref per join row. SELECT DISTINCT
	// keeps the 409 body honest: one entry per recipe.
	pool := storetest.NewPool(t)
	ctx := context.Background()
	repo := products.NewRepo(pool)
	stub := &stubOFF{products: map[string]*off.Product{}}
	svc := products.NewService(pool, repo, stub)

	component := &products.Product{Name: "legacy-component", Source: products.SourceManual, Nutriments: products.Nutriments{KcalPer100g: ptrFloat(100)}}
	require.NoError(t, repo.Insert(ctx, component))
	recipe := &products.Product{Name: "legacy-dup-recipe", Source: products.SourceRecipe, Nutriments: products.Nutriments{KcalPer100g: ptrFloat(120)}}
	require.NoError(t, repo.Insert(ctx, recipe))

	// Insert two component rows for the same (recipe, component) pair to
	// simulate legacy data. The upfront API validation prevents this today
	// for any recipe created via POST /products/recipes.
	for i := 0; i < 2; i++ {
		_, err := pool.Exec(ctx, `
            INSERT INTO product_components (id, product_id, component_product_id, quantity_g, position, created_at)
            VALUES ($1, $2, $3, $4, $5, now())
        `, uuid.New(), recipe.ID, component.ID, 50.0, i)
		require.NoError(t, err)
	}

	err := svc.Delete(ctx, component.ID)
	require.Error(t, err)
	var inUse *products.ErrProductInUseAsComponent
	require.ErrorAs(t, err, &inUse)
	// Without SELECT DISTINCT the slice would contain two refs to the same
	// recipe id. Assert exactly one.
	require.Len(t, inUse.Recipes, 1, "duplicate component rows must not duplicate the recipe in the 409 body")
	assert.Equal(t, recipe.ID, inUse.Recipes[0].ID)
	assert.Equal(t, "legacy-dup-recipe", inUse.Recipes[0].Name)
}

func TestDelete_AfterRecipeDeletedComponentCanBeDeleted(t *testing.T) {
	// The flow from the spec: A & B → recipe(A, B) → DELETE A → 409 →
	// DELETE recipe → 204 → DELETE A again → 204. Verifies the cascade
	// path (deleting the recipe removes its product_components rows so the
	// component is no longer "in use").
	r, _, repo := setup(t)
	ctx := context.Background()

	a := &products.Product{Name: "flow-A", Source: products.SourceManual, Nutriments: products.Nutriments{KcalPer100g: ptrFloat(100)}}
	require.NoError(t, repo.Insert(ctx, a))
	b := &products.Product{Name: "flow-B", Source: products.SourceManual, Nutriments: products.Nutriments{KcalPer100g: ptrFloat(50)}}
	require.NoError(t, repo.Insert(ctx, b))

	recipeBody := fmt.Sprintf(
		`{"name":"flow-recipe","serving_size_g":100,"components":[{"product_id":%q,"quantity_g":100},{"product_id":%q,"quantity_g":50}]}`,
		a.ID, b.ID,
	)
	createRec := httptest.NewRecorder()
	r.ServeHTTP(createRec, httptest.NewRequest(http.MethodPost, "/products/recipes", bytes.NewBufferString(recipeBody)))
	require.Equal(t, http.StatusCreated, createRec.Code)
	var recipeResp struct {
		ID uuid.UUID `json:"id"`
	}
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &recipeResp))

	// Step: 409 on first attempt to delete A.
	blocked := httptest.NewRecorder()
	r.ServeHTTP(blocked, httptest.NewRequest(http.MethodDelete, "/products/"+a.ID.String(), nil))
	require.Equal(t, http.StatusConflict, blocked.Code)

	// Step: delete the recipe; the cascade clears product_components rows.
	delRecipe := httptest.NewRecorder()
	r.ServeHTTP(delRecipe, httptest.NewRequest(http.MethodDelete, "/products/"+recipeResp.ID.String(), nil))
	require.Equal(t, http.StatusNoContent, delRecipe.Code)

	// Step: now A can be deleted cleanly.
	delA := httptest.NewRecorder()
	r.ServeHTTP(delA, httptest.NewRequest(http.MethodDelete, "/products/"+a.ID.String(), nil))
	require.Equal(t, http.StatusNoContent, delA.Code)
}

func TestDelete_RetryAfterDeleteIs404(t *testing.T) {
	r, _, repo := setup(t)
	p := &products.Product{Name: "ephemeral", Source: products.SourceManual}
	require.NoError(t, repo.Insert(context.Background(), p))

	first := httptest.NewRecorder()
	r.ServeHTTP(first, httptest.NewRequest(http.MethodDelete, "/products/"+p.ID.String(), nil))
	require.Equal(t, http.StatusNoContent, first.Code)

	second := httptest.NewRecorder()
	r.ServeHTTP(second, httptest.NewRequest(http.MethodDelete, "/products/"+p.ID.String(), nil))
	assert.Equal(t, http.StatusNotFound, second.Code)
}

// ----- source + external_url on POST /products -----

func TestCreateManual_RecipeWithExternalURLHappyPath(t *testing.T) {
	r, _, _ := setup(t)
	url := "https://cookidoo.de/recipes/recipe/de-DE/r728590"
	body := fmt.Sprintf(`{
        "name":"Rinderfilet mit Ofengemüse",
        "source":"recipe",
        "external_url":%q,
        "serving_size_g":350,
        "nutriments_per_100g":{"kcal":266,"protein_g":13,"carbs_g":5,"fat_g":21}
    }`, url)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/products", bytes.NewBufferString(body)))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var p products.Product
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &p))
	assert.Equal(t, products.SourceRecipe, p.Source)
	require.NotNil(t, p.ExternalURL)
	assert.Equal(t, url, *p.ExternalURL)
	// Flat-imported recipes carry no computed-from-components timestamp.
	assert.Nil(t, p.NutrimentComputedAt)

	// GET-by-id round-trips the same shape.
	getRec := httptest.NewRecorder()
	r.ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "/products/"+p.ID.String(), nil))
	require.Equal(t, http.StatusOK, getRec.Code)
	var got products.Product
	require.NoError(t, json.Unmarshal(getRec.Body.Bytes(), &got))
	assert.Equal(t, products.SourceRecipe, got.Source)
	require.NotNil(t, got.ExternalURL)
	assert.Equal(t, url, *got.ExternalURL)
}

func TestCreateManual_UnknownSourceReturns400(t *testing.T) {
	r, _, _ := setup(t)
	body := `{"name":"X","source":"cookidoo","nutriments_per_100g":{}}`
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/products", bytes.NewBufferString(body)))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"source_invalid"}`, rec.Body.String())
}

func TestCreateManual_ExternalURLTooLongReturns400(t *testing.T) {
	r, _, _ := setup(t)
	tooLong := strings.Repeat("a", 2049)
	body := fmt.Sprintf(`{"name":"X","external_url":%q,"nutriments_per_100g":{}}`, tooLong)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/products", bytes.NewBufferString(body)))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"external_url_too_long"}`, rec.Body.String())
}

func TestCreateManual_ExternalURLEmptyAfterTrimReturns400(t *testing.T) {
	r, _, _ := setup(t)
	body := `{"name":"X","external_url":"   ","nutriments_per_100g":{}}`
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/products", bytes.NewBufferString(body)))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"external_url_invalid"}`, rec.Body.String())
}

func TestCreateManual_OmittingSourceAndURLPreservesManualDefault(t *testing.T) {
	// Existing-client compatibility: a POST that doesn't supply source or
	// external_url still produces source=manual with no provenance link.
	r, _, _ := setup(t)
	body := `{"name":"Plain manual","nutriments_per_100g":{"kcal":150}}`
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/products", bytes.NewBufferString(body)))
	require.Equal(t, http.StatusCreated, rec.Code)
	var p products.Product
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &p))
	assert.Equal(t, products.SourceManual, p.Source)
	assert.Nil(t, p.ExternalURL)
}

func TestSearch_IncludesExternalURLInResults(t *testing.T) {
	r, _, _ := setup(t)
	url := "https://cookidoo.de/recipes/recipe/de-DE/r-search-me"
	body := fmt.Sprintf(`{
        "name":"Searchable Cookidoo Recipe",
        "source":"recipe",
        "external_url":%q,
        "nutriments_per_100g":{"kcal":300}
    }`, url)
	postRec := httptest.NewRecorder()
	r.ServeHTTP(postRec, httptest.NewRequest(http.MethodPost, "/products", bytes.NewBufferString(body)))
	require.Equal(t, http.StatusCreated, postRec.Code, postRec.Body.String())

	searchRec := httptest.NewRecorder()
	r.ServeHTTP(searchRec, httptest.NewRequest(http.MethodGet, "/products/search?q=cookidoo", nil))
	require.Equal(t, http.StatusOK, searchRec.Code)
	var resp struct {
		Results []products.Product `json:"results"`
	}
	require.NoError(t, json.Unmarshal(searchRec.Body.Bytes(), &resp))
	require.NotEmpty(t, resp.Results)
	// Find the imported recipe in the results.
	var found *products.Product
	for i := range resp.Results {
		if resp.Results[i].Name == "Searchable Cookidoo Recipe" {
			found = &resp.Results[i]
			break
		}
	}
	require.NotNil(t, found, "imported recipe should appear in search results")
	require.NotNil(t, found.ExternalURL)
	assert.Equal(t, url, *found.ExternalURL)
}

// ----- duplicate-component rejection -----

func TestCreateRecipe_DuplicateComponentReturns400(t *testing.T) {
	r, _, repo := setup(t)
	ctx := context.Background()
	x := &products.Product{Name: "ingredient-X", Source: products.SourceManual, Nutriments: products.Nutriments{KcalPer100g: ptrFloat(100)}}
	require.NoError(t, repo.Insert(ctx, x))

	body := fmt.Sprintf(
		`{"name":"dup-recipe","serving_size_g":100,"components":[{"product_id":%q,"quantity_g":100},{"product_id":%q,"quantity_g":50}]}`,
		x.ID, x.ID,
	)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/products/recipes", bytes.NewBufferString(body)))
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "component_duplicate", resp["error"])
	assert.Equal(t, x.ID.String(), resp["product_id"])
	assert.EqualValues(t, 2, resp["occurrences"])
	assert.Contains(t, resp["hint"], "sum the quantities")
}
