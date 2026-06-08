package mcpserver

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type recordedRequest struct {
	method string
	path   string
	rawQS  string
}

func newRecordingClient(t *testing.T, status int, body string) (*apiClient, *recordedRequest) {
	t.Helper()
	rec := &recordedRequest{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.method = r.Method
		rec.path = r.URL.Path
		rec.rawQS = r.URL.RawQuery
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	return &apiClient{
		baseURL:   u,
		token:     "t",
		userAgent: "ua",
		http:      &http.Client{Timeout: 5 * time.Second},
	}, rec
}

func extractText(t *testing.T, r *mcp.CallToolResult) string {
	t.Helper()
	require.Len(t, r.Content, 1)
	tc, ok := r.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	return tc.Text
}

func TestLookupProductByBarcode_HitsRESTEndpoint(t *testing.T) {
	c, rec := newRecordingClient(t, 200, `{"id":"abc","name":"Nutella"}`)
	r := handleLookupProductByBarcode(context.Background(), c, LookupProductByBarcodeArgs{Barcode: "3017624010701"})
	assert.False(t, r.IsError)
	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "/products/lookup/3017624010701", rec.path)
	assert.Empty(t, rec.rawQS)
	assert.JSONEq(t, `{"id":"abc","name":"Nutella"}`, extractText(t, r))
}

func TestLookupProductByBarcode_RefreshAddsQueryParam(t *testing.T) {
	c, rec := newRecordingClient(t, 200, `{}`)
	_ = handleLookupProductByBarcode(context.Background(), c, LookupProductByBarcodeArgs{Barcode: "X", Refresh: true})
	assert.Equal(t, "refresh=true", rec.rawQS)
}

func TestLookupProductByBarcode_404ForwardsBodyWithIsError(t *testing.T) {
	body := `{"error":"product_not_found","barcode":"X","next":"POST /meals/freeform"}`
	c, _ := newRecordingClient(t, 404, body)
	r := handleLookupProductByBarcode(context.Background(), c, LookupProductByBarcodeArgs{Barcode: "X"})
	assert.True(t, r.IsError)
	assert.JSONEq(t, body, extractText(t, r))
}

func TestSearchProducts_HitsRESTEndpointWithQuery(t *testing.T) {
	c, rec := newRecordingClient(t, 200, `{"results":[]}`)
	r := handleSearchProducts(context.Background(), c, SearchProductsArgs{Q: "yogurt"})
	assert.False(t, r.IsError)
	assert.Equal(t, http.MethodGet, rec.method)
	assert.Equal(t, "/products/search", rec.path)
	assert.Equal(t, "q=yogurt", rec.rawQS)
}

// ----- list_products -----

func TestListProducts_NoFiltersIssuesBareGet(t *testing.T) {
	c, rec := newRecordingClient(t, 200, `{"products":[],"total":0,"limit":50,"offset":0}`)
	r := handleListProducts(context.Background(), c, ListProductsArgs{})
	assert.False(t, r.IsError)
	assert.Equal(t, http.MethodGet, rec.method)
	assert.Equal(t, "/products", rec.path)
	assert.Empty(t, rec.rawQS)
}

func TestListProducts_AllFiltersForwarded(t *testing.T) {
	c, rec := newRecordingClient(t, 200, `{}`)
	source := "manual"
	limit := 20
	offset := 40
	_ = handleListProducts(context.Background(), c, ListProductsArgs{Source: &source, Limit: &limit, Offset: &offset})
	values, err := url.ParseQuery(rec.rawQS)
	require.NoError(t, err)
	assert.Equal(t, "manual", values.Get("source"))
	assert.Equal(t, "20", values.Get("limit"))
	assert.Equal(t, "40", values.Get("offset"))
}

func TestListProducts_ResponseBodyForwardedVerbatim(t *testing.T) {
	body := `{"products":[{"id":"X"}],"total":1,"limit":50,"offset":0}`
	c, _ := newRecordingClient(t, 200, body)
	r := handleListProducts(context.Background(), c, ListProductsArgs{})
	assert.JSONEq(t, body, extractText(t, r))
}

// ----- delete_product -----

func TestDeleteProduct_204ReturnsEmptySuccessResult(t *testing.T) {
	c, rec := newRecordingClient(t, 204, "")
	r := handleDeleteProduct(context.Background(), c, DeleteProductArgs{ProductID: "abc"})
	assert.False(t, r.IsError)
	require.Len(t, r.Content, 1)
	tc, ok := r.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	assert.Empty(t, tc.Text)
	assert.Equal(t, http.MethodDelete, rec.method)
	assert.Equal(t, "/products/abc", rec.path)
}

func TestDeleteProduct_404Forwarded(t *testing.T) {
	c, _ := newRecordingClient(t, 404, `{"error":"product_not_found"}`)
	r := handleDeleteProduct(context.Background(), c, DeleteProductArgs{ProductID: "missing"})
	assert.True(t, r.IsError)
	assert.JSONEq(t, `{"error":"product_not_found"}`, extractText(t, r))
}

func TestDeleteProduct_409InUseAsComponentForwardedVerbatim(t *testing.T) {
	body := `{"error":"product_in_use_as_component","recipes":[{"id":"r1","name":"Morning bowl"}],"hint":"delete the listed recipes first, or replace this product within them"}`
	c, _ := newRecordingClient(t, 409, body)
	r := handleDeleteProduct(context.Background(), c, DeleteProductArgs{ProductID: "ingredient-A"})
	assert.True(t, r.IsError)
	assert.JSONEq(t, body, extractText(t, r))
}

// newRecordingBodyClient is like newRecordingClient but also captures the
// request body and idempotency-key header.
func newRecordingBodyClient(t *testing.T, status int, body string) (*apiClient, *mealRecord) {
	t.Helper()
	rec := &mealRecord{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		rec.method = r.Method
		rec.path = r.URL.Path
		rec.body = b
		rec.idemKey = r.Header.Get("Idempotency-Key")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	return &apiClient{
		baseURL:   u,
		token:     "t",
		userAgent: "ua",
		http:      &http.Client{Timeout: 5 * time.Second},
	}, rec
}

func TestCreateRecipe_PostsToRecipesEndpoint(t *testing.T) {
	c, rec := newRecordingBodyClient(t, 201, `{"id":"r1","name":"Mix"}`)
	args := CreateRecipeArgs{
		Name: "Mix",
		Components: []CreateRecipeComponent{
			{ProductID: "p1", QuantityG: 100},
			{ProductID: "p2", QuantityG: 50},
		},
	}
	r := handleCreateRecipe(context.Background(), c, args)
	assert.False(t, r.IsError)
	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "/products/recipes", rec.path)
	assert.NotEmpty(t, rec.idemKey)
	assert.JSONEq(t,
		`{"name":"Mix","components":[{"product_id":"p1","quantity_g":100},{"product_id":"p2","quantity_g":50}]}`,
		string(rec.body))
}

func TestCreateRecipe_ExplicitIdempotencyKey(t *testing.T) {
	c, rec := newRecordingBodyClient(t, 201, `{}`)
	_ = handleCreateRecipe(context.Background(), c, CreateRecipeArgs{
		Name:           "X",
		Components:     []CreateRecipeComponent{{ProductID: "p1", QuantityG: 100}},
		IdempotencyKey: "my-key",
	})
	assert.Equal(t, "my-key", rec.idemKey)
}

func TestCreateRecipe_404ForwardsBodyWithIsError(t *testing.T) {
	body := `{"error":"component_not_found","product_id":"missing"}`
	c, _ := newRecordingBodyClient(t, 404, body)
	r := handleCreateRecipe(context.Background(), c, CreateRecipeArgs{
		Name:       "X",
		Components: []CreateRecipeComponent{{ProductID: "missing", QuantityG: 10}},
	})
	assert.True(t, r.IsError)
	assert.JSONEq(t, body, extractText(t, r))
}

func TestRecomputeRecipe_HitsRecomputeEndpoint(t *testing.T) {
	c, rec := newRecordingBodyClient(t, 200, `{"id":"r1"}`)
	_ = handleRecomputeRecipe(context.Background(), c, RecomputeRecipeArgs{ProductID: "r1"})
	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "/products/recipes/r1/recompute", rec.path)
}

func TestRecomputeRecipe_400NotARecipeForwarded(t *testing.T) {
	body := `{"error":"not_a_recipe","product_id":"p1"}`
	c, _ := newRecordingBodyClient(t, 400, body)
	r := handleRecomputeRecipe(context.Background(), c, RecomputeRecipeArgs{ProductID: "p1"})
	assert.True(t, r.IsError)
	assert.JSONEq(t, body, extractText(t, r))
}
