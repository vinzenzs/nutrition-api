package chat

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/nutrition-api/internal/agenttools"
)

// The chat loop exposes the shared registry's surface and adds the web_search
// server tool. The detailed schema/build/tier assertions live in the
// agenttools package; here we verify the chat-side rendering and that no
// forbidden tool has leaked into the surface yet.
func TestChatToolDefs_SurfaceAndWebSearch(t *testing.T) {
	specs := agenttools.Registry()
	got := make([]string, 0, len(specs))
	for _, s := range specs {
		got = append(got, s.Name)
	}
	want := []string{
		"get_daily_context", "get_race_fueling", "list_planned_meals",
		"list_shopping_items", "search_products", "get_product",
		"import_cookidoo_recipe", "update_product", "create_planned_meal",
		"update_planned_meal", "mark_planned_meal_eaten", "add_shopping_items",
		"update_shopping_item", "clear_checked_shopping_items",
	}
	assert.ElementsMatch(t, want, got)

	// Forbidden surfaces must be absent (until phase 3 broadens the registry).
	forbidden := []string{
		"log_meal", "log_meal_freeform", "log_hydration", "delete_meal",
		"delete_product", "delete_planned_meal", "set_daily_goal_override",
		"delete_shopping_item", "log_workout",
	}
	names := map[string]bool{}
	for _, n := range got {
		names[n] = true
	}
	for _, f := range forbidden {
		assert.Falsef(t, names[f], "forbidden tool present: %s", f)
	}

	// Tool defs include web_search, domain-restricted to Cookidoo.
	defs := anthropicToolDefs(specs)
	require.Len(t, defs, len(specs)+1) // custom tools + web_search
	last := string(defs[len(defs)-1])
	assert.Contains(t, last, "web_search")
	assert.Contains(t, last, "cookidoo.de")
	assert.Contains(t, last, "allowed_domains")
}

// Idempotency keys are deterministic across key reordering / whitespace and
// differ for different inputs — the property the retry-safety guarantee rests on.
func TestDeriveIdempotencyKey_Deterministic(t *testing.T) {
	a := deriveIdempotencyKey("add_shopping_items", json.RawMessage(`{"items":[{"name":"onion"}]}`))
	b := deriveIdempotencyKey("add_shopping_items", json.RawMessage(`{"items":[{"name":"onion"}]}`))
	assert.Equal(t, a, b, "identical calls must hash identically")

	// Reordered keys / whitespace must not change the key.
	c := deriveIdempotencyKey("create_planned_meal", json.RawMessage(`{"slot":"dinner","plan_date":"2026-06-12"}`))
	d := deriveIdempotencyKey("create_planned_meal", json.RawMessage("{ \"plan_date\":\"2026-06-12\" , \"slot\":\"dinner\" }"))
	assert.Equal(t, c, d, "key/whitespace reordering must be invariant")

	// Different input → different key.
	e := deriveIdempotencyKey("add_shopping_items", json.RawMessage(`{"items":[{"name":"garlic"}]}`))
	assert.NotEqual(t, a, e)

	// Hex-encoded sha256.
	assert.Len(t, a, 64)
	assert.Empty(t, strings.Trim(a, "0123456789abcdef"))
}
