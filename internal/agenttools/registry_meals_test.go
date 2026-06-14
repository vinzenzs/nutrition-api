package agenttools

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// log_meal → POST /meals. Body carries the known-product fields; omitempty
// drops unset optional strings. Idempotency-Key is derived centrally by the
// dispatcher (write tier), not part of the body.
func TestBuild_LogMeal(t *testing.T) {
	specs := ByName(MCPRegistry())

	spec, ok := specs["log_meal"]
	require.True(t, ok, "log_meal must be registered on the MCP surface")
	assert.True(t, spec.Tier.IsWrite())
	assert.Equal(t, TierWriteAuto, spec.Tier)

	in := json.RawMessage(`{"product_id":"abc","quantity_g":100,"logged_at":"2026-06-06T12:00:00Z","meal_type":"lunch"}`)
	call, err := spec.Build(in)
	require.NoError(t, err)
	assert.Equal(t, "POST", call.Method)
	assert.Equal(t, "/meals", call.Path)
	assert.Empty(t, call.Query)
	assert.JSONEq(t,
		`{"product_id":"abc","quantity_g":100,"logged_at":"2026-06-06T12:00:00Z","meal_type":"lunch"}`,
		string(call.Body))
}

// log_meal omits unset optional string fields (meal_type/note/workout_id), and
// the idempotency_key never appears in the body.
func TestBuild_LogMeal_OmitsUnsetOptionalsAndKey(t *testing.T) {
	specs := ByName(MCPRegistry())
	in := json.RawMessage(`{"product_id":"abc","quantity_g":50,"logged_at":"2026-06-06T12:00:00Z","idempotency_key":"k1"}`)
	call, err := specs["log_meal"].Build(in)
	require.NoError(t, err)
	assert.JSONEq(t,
		`{"product_id":"abc","quantity_g":50,"logged_at":"2026-06-06T12:00:00Z"}`,
		string(call.Body))
}

// log_meal_freeform → POST /meals/freeform. nutriments_per_100g is always
// present (non-omitempty struct); save_as_product=true is carried through.
func TestBuild_LogMealFreeform(t *testing.T) {
	specs := ByName(MCPRegistry())

	spec, ok := specs["log_meal_freeform"]
	require.True(t, ok, "log_meal_freeform must be registered on the MCP surface")
	assert.True(t, spec.Tier.IsWrite())
	assert.Equal(t, TierWriteAuto, spec.Tier)

	in := json.RawMessage(`{"name":"banana","nutriments_per_100g":{"kcal":89},"quantity_g":120,"logged_at":"2026-06-06T10:00:00Z","save_as_product":true}`)
	call, err := spec.Build(in)
	require.NoError(t, err)
	assert.Equal(t, "POST", call.Method)
	assert.Equal(t, "/meals/freeform", call.Path)
	assert.JSONEq(t,
		`{"name":"banana","nutriments_per_100g":{"kcal":89},"quantity_g":120,"logged_at":"2026-06-06T10:00:00Z","save_as_product":true}`,
		string(call.Body))
}

// log_meal_freeform with no save_as_product omits the bool (omitempty) and the
// nutriments block is still present as an empty object.
func TestBuild_LogMealFreeform_OmitsSaveAsProductWhenFalse(t *testing.T) {
	specs := ByName(MCPRegistry())
	in := json.RawMessage(`{"name":"banana","nutriments_per_100g":{},"quantity_g":120,"logged_at":"2026-06-06T10:00:00Z"}`)
	call, err := specs["log_meal_freeform"].Build(in)
	require.NoError(t, err)
	assert.JSONEq(t,
		`{"name":"banana","nutriments_per_100g":{},"quantity_g":120,"logged_at":"2026-06-06T10:00:00Z"}`,
		string(call.Body))
}

// patch_meal → PATCH /meals/{id}. Only supplied fields appear in the body.
func TestBuild_PatchMeal_OnlySuppliedFields(t *testing.T) {
	specs := ByName(MCPRegistry())

	spec, ok := specs["patch_meal"]
	require.True(t, ok, "patch_meal must be registered on the MCP surface")
	assert.True(t, spec.Tier.IsWrite())
	assert.Equal(t, TierWriteAuto, spec.Tier)

	in := json.RawMessage(`{"meal_id":"abc","quantity_g":200}`)
	call, err := spec.Build(in)
	require.NoError(t, err)
	assert.Equal(t, "PATCH", call.Method)
	assert.Equal(t, "/meals/abc", call.Path)
	assert.JSONEq(t, `{"quantity_g":200}`, string(call.Body))
}

// patch_meal honors the empty-string sentinel for workout_id (clear the link):
// an explicit "" is forwarded as workout_id:"" in the body.
func TestBuild_PatchMeal_WorkoutIDClearSentinel(t *testing.T) {
	specs := ByName(MCPRegistry())
	in := json.RawMessage(`{"meal_id":"abc","workout_id":""}`)
	call, err := specs["patch_meal"].Build(in)
	require.NoError(t, err)
	assert.JSONEq(t, `{"workout_id":""}`, string(call.Body))
}

// patch_meal requires meal_id.
func TestBuild_PatchMeal_MissingIDErrors(t *testing.T) {
	specs := ByName(MCPRegistry())
	_, err := specs["patch_meal"].Build(json.RawMessage(`{"quantity_g":200}`))
	require.Error(t, err)
}

// delete_meal → DELETE /meals/{id}, no body.
func TestBuild_DeleteMeal(t *testing.T) {
	specs := ByName(MCPRegistry())

	spec, ok := specs["delete_meal"]
	require.True(t, ok, "delete_meal must be registered on the MCP surface")
	assert.True(t, spec.Tier.IsWrite())
	assert.Equal(t, TierWriteAuto, spec.Tier)

	call, err := spec.Build(json.RawMessage(`{"meal_id":"abc"}`))
	require.NoError(t, err)
	assert.Equal(t, "DELETE", call.Method)
	assert.Equal(t, "/meals/abc", call.Path)
	assert.Empty(t, call.Body)
}

// delete_meal requires meal_id.
func TestBuild_DeleteMeal_MissingIDErrors(t *testing.T) {
	specs := ByName(MCPRegistry())
	_, err := specs["delete_meal"].Build(json.RawMessage(`{}`))
	require.Error(t, err)
}

// id path segments are escaped exactly as the bespoke handler did (url.PathEscape).
func TestBuild_Meals_IDPathEscaping(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["delete_meal"].Build(json.RawMessage(`{"meal_id":"a b/c"}`))
	require.NoError(t, err)
	assert.Equal(t, "/meals/a%20b%2Fc", call.Path)
}
