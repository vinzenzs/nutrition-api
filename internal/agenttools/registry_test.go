package agenttools

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The shared registry exposes EXACTLY the curated coach surface in a stable set:
// the nutrition-planning subset + the aggregate coach reads + the gated
// write-confirm coaching actions.
func TestRegistry_ExactSurface(t *testing.T) {
	got := make([]string, 0)
	for _, s := range Registry() {
		got = append(got, s.Name)
	}
	want := []string{
		// nutrition planner (reads + write-auto)
		"get_daily_context", "get_race_fueling", "list_planned_meals",
		"list_shopping_items", "search_products", "get_product",
		"import_cookidoo_recipe", "update_product", "create_planned_meal",
		"update_planned_meal", "mark_planned_meal_eaten", "add_shopping_items",
		"update_shopping_item", "clear_checked_shopping_items",
		// coach aggregate reads
		"get_training_context", "get_recovery_context",
		// coach write-confirm actions
		"log_workout", "patch_workout", "delete_workout",
		"log_weight", "log_hydration", "log_meal_freeform",
		"set_daily_goal_override", "delete_daily_goal_override",
	}
	assert.ElementsMatch(t, want, got)
}

// Every tool's schema is valid JSON, carries the expected tier, and every
// write-confirm tool has a Format formatter so its confirmation preview is
// code-composed (D6) rather than falling back to the generic verb/resource line.
func TestRegistry_SchemasValidAndTiers(t *testing.T) {
	wantTier := map[string]Tier{
		"get_daily_context":   TierRead,
		"get_race_fueling":    TierRead,
		"list_planned_meals":  TierRead,
		"list_shopping_items": TierRead,
		"search_products":     TierRead,
		"get_product":         TierRead,
		"get_training_context": TierRead,
		"get_recovery_context": TierRead,

		"import_cookidoo_recipe":       TierWriteAuto,
		"update_product":               TierWriteAuto,
		"create_planned_meal":          TierWriteAuto,
		"update_planned_meal":          TierWriteAuto,
		"mark_planned_meal_eaten":      TierWriteAuto,
		"add_shopping_items":           TierWriteAuto,
		"update_shopping_item":         TierWriteAuto,
		"clear_checked_shopping_items": TierWriteAuto,

		"log_workout":                TierWriteConfirm,
		"patch_workout":              TierWriteConfirm,
		"delete_workout":             TierWriteConfirm,
		"log_weight":                 TierWriteConfirm,
		"log_hydration":              TierWriteConfirm,
		"log_meal_freeform":          TierWriteConfirm,
		"set_daily_goal_override":    TierWriteConfirm,
		"delete_daily_goal_override": TierWriteConfirm,
	}
	for _, s := range Registry() {
		var schema any
		require.NoErrorf(t, json.Unmarshal([]byte(s.Schema), &schema), "tool %s schema invalid", s.Name)
		wt, ok := wantTier[s.Name]
		require.Truef(t, ok, "no expected tier declared for %s", s.Name)
		assert.Equalf(t, wt, s.Tier, "tier wrong for %s", s.Name)
		if s.Tier == TierWriteConfirm {
			assert.NotNilf(t, s.Format, "write-confirm tool %s should have a Format formatter (D6)", s.Name)
		}
	}
}

// IsWrite distinguishes mutating tools (which get an Idempotency-Key) from reads.
func TestTier_IsWrite(t *testing.T) {
	assert.False(t, TierRead.IsWrite())
	assert.True(t, TierWriteAuto.IsWrite())
	assert.True(t, TierWriteConfirm.IsWrite())
}

func TestBuild_PathParamAndQueryTools(t *testing.T) {
	specs := ByName(Registry())

	// get_product → GET /products/{id}
	call, err := specs["get_product"].Build(json.RawMessage(`{"product_id":"abc"}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/products/abc", call.Path)

	// get_race_fueling, no id → list; with id → plan
	c1, _ := specs["get_race_fueling"].Build(json.RawMessage(`{}`))
	assert.Equal(t, "/races", c1.Path)
	c2, _ := specs["get_race_fueling"].Build(json.RawMessage(`{"race_id":"r1"}`))
	assert.Equal(t, "/races/r1/fueling-plan", c2.Path)

	// mark_planned_meal_eaten → POST /plan/{id}/eaten, plan_id stripped from body
	c3, err := specs["mark_planned_meal_eaten"].Build(json.RawMessage(`{"plan_id":"p1","quantity_g":450}`))
	require.NoError(t, err)
	assert.Equal(t, "POST", c3.Method)
	assert.Equal(t, "/plan/p1/eaten", c3.Path)
	assert.Contains(t, string(c3.Body), "quantity_g")
	assert.NotContains(t, string(c3.Body), "plan_id")

	// update_shopping_item → PATCH /shopping/items/{id}, item_id stripped
	c4, err := specs["update_shopping_item"].Build(json.RawMessage(`{"item_id":"i1","checked":true}`))
	require.NoError(t, err)
	assert.Equal(t, "PATCH", c4.Method)
	assert.Equal(t, "/shopping/items/i1", c4.Path)
	assert.Contains(t, string(c4.Body), "checked")
	assert.NotContains(t, string(c4.Body), "item_id")

	// clear_checked → DELETE /shopping/items?checked=true
	c5, _ := specs["clear_checked_shopping_items"].Build(json.RawMessage(`{}`))
	assert.Equal(t, "DELETE", c5.Method)
	assert.Equal(t, "true", c5.Query.Get("checked"))

	// missing required path param errors out (no call made)
	_, err = specs["mark_planned_meal_eaten"].Build(json.RawMessage(`{}`))
	assert.Error(t, err)
}
