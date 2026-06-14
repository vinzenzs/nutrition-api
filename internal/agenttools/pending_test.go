package agenttools

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testSpecs builds a specs map with one of each tier plus a Format-bearing
// confirm tool, for exercising the pending/preview logic without the real
// registry (which carries no write-confirm tools until phase 3).
func testSpecs() map[string]Spec {
	return ByName([]Spec{
		{Name: "get_daily_context", Tier: TierRead},
		{Name: "add_shopping_items", Tier: TierWriteAuto},
		{Name: "delete_planned_meal", Tier: TierWriteConfirm}, // generic preview
		{
			Name: "schedule_workout",
			Tier: TierWriteConfirm,
			Format: func(in json.RawMessage) string {
				var a struct {
					Date string `json:"date"`
				}
				_ = json.Unmarshal(in, &a)
				return "Schedule a ride on " + a.Date
			},
		},
	})
}

func assistantTurn(blocks ...string) json.RawMessage {
	return json.RawMessage("[" + strings.Join(blocks, ",") + "]")
}

func toolUse(id, name, input string) string {
	return `{"type":"tool_use","id":"` + id + `","name":"` + name + `","input":` + input + `}`
}

func TestParseToolUseBlocks(t *testing.T) {
	content := assistantTurn(
		`{"type":"text","text":"ok"}`,
		toolUse("t1", "get_daily_context", `{"date":"2026-06-14"}`),
		toolUse("t2", "schedule_workout", `{"date":"2026-06-20"}`),
	)
	blocks := ParseToolUseBlocks(content)
	require.Len(t, blocks, 2)
	assert.Equal(t, "t1", blocks[0].ID)
	assert.Equal(t, "schedule_workout", blocks[1].Name)

	// Plain user text yields no blocks.
	assert.Empty(t, ParseToolUseBlocks(json.RawMessage(`"just text"`)))
}

func TestAwaitingConfirmation(t *testing.T) {
	specs := testSpecs()
	// Read + auto only → not awaiting.
	noConfirm := assistantTurn(
		toolUse("t1", "get_daily_context", `{}`),
		toolUse("t2", "add_shopping_items", `{"items":[]}`),
	)
	assert.False(t, AwaitingConfirmation(noConfirm, specs))

	// Any write-confirm → awaiting.
	withConfirm := assistantTurn(
		toolUse("t1", "get_daily_context", `{}`),
		toolUse("t2", "schedule_workout", `{"date":"2026-06-20"}`),
	)
	assert.True(t, AwaitingConfirmation(withConfirm, specs))
}

func TestPendingFromContent_OnlyConfirmCallsWithPreviews(t *testing.T) {
	specs := testSpecs()
	content := assistantTurn(
		toolUse("t1", "get_daily_context", `{}`),               // read — excluded
		toolUse("t2", "add_shopping_items", `{"items":[]}`),     // auto — excluded
		toolUse("t3", "schedule_workout", `{"date":"2026-06-20"}`),
		toolUse("t4", "delete_planned_meal", `{"plan_id":"p1"}`),
	)
	pending := PendingFromContent(content, specs)
	require.Len(t, pending, 2)

	assert.Equal(t, "t3", pending[0].ToolID)
	assert.Equal(t, TierWriteConfirm, pending[0].Tier)
	assert.Equal(t, "Schedule a ride on 2026-06-20", pending[0].Preview, "Format formatter used")

	assert.Equal(t, "t4", pending[1].ToolID)
	assert.Equal(t, "Delete planned meal", pending[1].Preview, "generic fallback when no Format")
}

func TestTurnID_StableAndOrderSensitive(t *testing.T) {
	a := assistantTurn(toolUse("t1", "schedule_workout", `{}`), toolUse("t2", "delete_planned_meal", `{}`))
	// Same ids → same turn id, even if other fields differ.
	a2 := assistantTurn(toolUse("t1", "schedule_workout", `{"x":1}`), toolUse("t2", "delete_planned_meal", `{"y":2}`))
	assert.Equal(t, TurnID(a), TurnID(a2))
	assert.True(t, len(TurnID(a)) > len("turn_"))

	// Different ids → different turn id.
	b := assistantTurn(toolUse("t9", "schedule_workout", `{}`))
	assert.NotEqual(t, TurnID(a), TurnID(b))
}
