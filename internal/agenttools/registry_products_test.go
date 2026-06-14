package agenttools

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The products domain contributes seven MCP-only tools. Tiers: search_products
// and list_products are reads; the rest are write-auto (they carried an
// idempotency key or mutate).
func TestProducts_RegisteredWithExpectedTiers(t *testing.T) {
	specs := ByName(MCPRegistry())
	want := map[string]Tier{
		"lookup_product_by_barcode": TierWriteAuto,
		"search_products":           TierRead,
		"create_recipe":             TierWriteAuto,
		"recompute_recipe":          TierWriteAuto,
		"import_cookidoo_recipe":    TierWriteAuto,
		"list_products":             TierRead,
		"delete_product":            TierWriteAuto,
	}
	for name, tier := range want {
		s, ok := specs[name]
		require.Truef(t, ok, "tool %s not registered on the MCP surface", name)
		assert.Equalf(t, tier, s.Tier, "tool %s tier", name)
		assert.Truef(t, s.MCPExposed, "tool %s should be MCP-exposed", name)
		assert.NotNilf(t, s.SchemaType, "tool %s should carry a SchemaType", name)
	}
}

func TestProducts_LookupByBarcode_BuildShape(t *testing.T) {
	specs := ByName(MCPRegistry())

	// No refresh → POST /products/lookup/<barcode>, no query.
	call, err := specs["lookup_product_by_barcode"].Build(json.RawMessage(`{"barcode":"3017624010701"}`))
	require.NoError(t, err)
	assert.Equal(t, "POST", call.Method)
	assert.Equal(t, "/products/lookup/3017624010701", call.Path)
	assert.Empty(t, call.Query)
	assert.Empty(t, call.Body)

	// refresh=true → query carries refresh=true.
	call, err = specs["lookup_product_by_barcode"].Build(json.RawMessage(`{"barcode":"X","refresh":true}`))
	require.NoError(t, err)
	assert.Equal(t, "/products/lookup/X", call.Path)
	assert.Equal(t, "true", call.Query.Get("refresh"))
}

func TestProducts_SearchProducts_BuildShape(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["search_products"].Build(json.RawMessage(`{"q":"yogurt"}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/products/search", call.Path)
	assert.Equal(t, "yogurt", call.Query.Get("q"))
}

func TestProducts_CreateRecipe_BuildShape(t *testing.T) {
	specs := ByName(MCPRegistry())

	// idempotency_key must NOT leak into the body; serving_size_g omitted.
	call, err := specs["create_recipe"].Build(json.RawMessage(
		`{"name":"Mix","components":[{"product_id":"p1","quantity_g":100},{"product_id":"p2","quantity_g":50}],"idempotency_key":"k"}`))
	require.NoError(t, err)
	assert.Equal(t, "POST", call.Method)
	assert.Equal(t, "/products/recipes", call.Path)
	assert.JSONEq(t,
		`{"name":"Mix","components":[{"product_id":"p1","quantity_g":100},{"product_id":"p2","quantity_g":50}]}`,
		string(call.Body))

	// serving_size_g present is included.
	call, err = specs["create_recipe"].Build(json.RawMessage(
		`{"name":"X","components":[{"product_id":"p1","quantity_g":10}],"serving_size_g":250}`))
	require.NoError(t, err)
	assert.JSONEq(t,
		`{"name":"X","components":[{"product_id":"p1","quantity_g":10}],"serving_size_g":250}`,
		string(call.Body))
}

func TestProducts_RecomputeRecipe_BuildShape(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["recompute_recipe"].Build(json.RawMessage(`{"product_id":"r1"}`))
	require.NoError(t, err)
	assert.Equal(t, "POST", call.Method)
	assert.Equal(t, "/products/recipes/r1/recompute", call.Path)
	assert.Empty(t, call.Body)
}

func TestProducts_ImportCookidoo_BuildShape(t *testing.T) {
	specs := ByName(MCPRegistry())

	// idempotency_key must NOT leak into the body; serving_size_g omitted.
	call, err := specs["import_cookidoo_recipe"].Build(json.RawMessage(
		`{"url":"https://cookidoo.de/recipes/recipe/de-DE/r1","idempotency_key":"k"}`))
	require.NoError(t, err)
	assert.Equal(t, "POST", call.Method)
	assert.Equal(t, "/products/import/cookidoo", call.Path)
	assert.JSONEq(t, `{"url":"https://cookidoo.de/recipes/recipe/de-DE/r1"}`, string(call.Body))

	// serving_size_g present is included.
	call, err = specs["import_cookidoo_recipe"].Build(json.RawMessage(
		`{"url":"https://cookidoo.de/x","serving_size_g":300}`))
	require.NoError(t, err)
	assert.JSONEq(t, `{"url":"https://cookidoo.de/x","serving_size_g":300}`, string(call.Body))
}

func TestProducts_ListProducts_BuildShape(t *testing.T) {
	specs := ByName(MCPRegistry())

	// No filters → bare GET /products, no query.
	call, err := specs["list_products"].Build(json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/products", call.Path)
	assert.Empty(t, call.Query)

	// All filters forwarded.
	call, err = specs["list_products"].Build(json.RawMessage(`{"source":"manual","limit":20,"offset":40}`))
	require.NoError(t, err)
	assert.Equal(t, "manual", call.Query.Get("source"))
	assert.Equal(t, "20", call.Query.Get("limit"))
	assert.Equal(t, "40", call.Query.Get("offset"))

	// Empty source string is treated as absent.
	call, err = specs["list_products"].Build(json.RawMessage(`{"source":""}`))
	require.NoError(t, err)
	assert.False(t, call.Query.Has("source"))
}

func TestProducts_DeleteProduct_BuildShape(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["delete_product"].Build(json.RawMessage(`{"product_id":"abc"}`))
	require.NoError(t, err)
	assert.Equal(t, "DELETE", call.Method)
	assert.Equal(t, "/products/abc", call.Path)
	assert.Empty(t, call.Body)
}
