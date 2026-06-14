package agenttools

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The shopping domain contributes five MCP tools: one read (list) and four
// writes (add/update/delete/clear). Each mirrors its bespoke REST mapping 1:1.
func TestShopping_Registered(t *testing.T) {
	specs := ByName(MCPRegistry())
	cases := map[string]Tier{
		"add_shopping_items":           TierWriteAuto,
		"list_shopping_items":          TierRead,
		"update_shopping_item":         TierWriteAuto,
		"delete_shopping_item":         TierWriteAuto,
		"clear_checked_shopping_items": TierWriteAuto,
	}
	for name, tier := range cases {
		s, ok := specs[name]
		require.Truef(t, ok, "tool %s not registered on the MCP surface", name)
		assert.Equalf(t, tier, s.Tier, "tool %s tier", name)
		assert.Truef(t, s.MCPExposed, "tool %s should be MCP-exposed", name)
		assert.NotNilf(t, s.SchemaType, "tool %s should carry a SchemaType", name)
	}
}

func TestShopping_AddShoppingItems_PostsItemsArrayOnly(t *testing.T) {
	specs := ByName(MCPRegistry())
	in := json.RawMessage(`{"items":[{"name":"Zwiebeln","quantity_text":"3"},{"name":"Hackfleisch"}],"idempotency_key":"k1"}`)
	call, err := specs["add_shopping_items"].Build(in)
	require.NoError(t, err)
	assert.Equal(t, "POST", call.Method)
	assert.Equal(t, "/shopping/items", call.Path)
	assert.Empty(t, call.Query)
	body := string(call.Body)
	assert.Contains(t, body, `"items":[`)
	assert.Contains(t, body, `"name":"Zwiebeln"`)
	assert.Contains(t, body, `"quantity_text":"3"`)
	// The idempotency_key is dropped from the body; only items is marshalled.
	assert.NotContains(t, body, "idempotency_key")
}

func TestShopping_ListShoppingItems_IncludeCheckedQuery(t *testing.T) {
	specs := ByName(MCPRegistry())

	// No flag → no query key.
	call, err := specs["list_shopping_items"].Build(json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/shopping/items", call.Path)
	assert.False(t, call.Query.Has("include_checked"))

	// include_checked=true → "true".
	call, err = specs["list_shopping_items"].Build(json.RawMessage(`{"include_checked":true}`))
	require.NoError(t, err)
	assert.Equal(t, "true", call.Query.Get("include_checked"))

	// include_checked=false → "false" (pointer set, formatted via strconv).
	call, err = specs["list_shopping_items"].Build(json.RawMessage(`{"include_checked":false}`))
	require.NoError(t, err)
	assert.True(t, call.Query.Has("include_checked"))
	assert.Equal(t, "false", call.Query.Get("include_checked"))
}

func TestShopping_UpdateShoppingItem_PatchesCheckedOnly(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["update_shopping_item"].Build(json.RawMessage(`{"id":"i1","checked":true}`))
	require.NoError(t, err)
	assert.Equal(t, "PATCH", call.Method)
	assert.Equal(t, "/shopping/items/i1", call.Path)
	body := string(call.Body)
	assert.Contains(t, body, `"checked":true`)
	// Omitted fields are not sent.
	assert.NotContains(t, body, "name")
	assert.NotContains(t, body, "quantity_text")
	// The id is in the path, never in the body.
	assert.NotContains(t, body, `"id"`)
	assert.NotContains(t, body, "idempotency_key")
}

func TestShopping_UpdateShoppingItem_NameAndQuantity(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["update_shopping_item"].Build(json.RawMessage(`{"id":"i2","name":"Onions","quantity_text":"5"}`))
	require.NoError(t, err)
	assert.Equal(t, "/shopping/items/i2", call.Path)
	body := string(call.Body)
	assert.Contains(t, body, `"name":"Onions"`)
	assert.Contains(t, body, `"quantity_text":"5"`)
	assert.NotContains(t, body, "checked")
}

func TestShopping_DeleteShoppingItem(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["delete_shopping_item"].Build(json.RawMessage(`{"id":"i9"}`))
	require.NoError(t, err)
	assert.Equal(t, "DELETE", call.Method)
	assert.Equal(t, "/shopping/items/i9", call.Path)
	assert.Empty(t, call.Query)
	assert.Empty(t, call.Body)
}

func TestShopping_ClearCheckedShoppingItems(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["clear_checked_shopping_items"].Build(json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.Equal(t, "DELETE", call.Method)
	assert.Equal(t, "/shopping/items", call.Path)
	assert.Equal(t, "true", call.Query.Get("checked"))
	assert.Empty(t, call.Body)
}
