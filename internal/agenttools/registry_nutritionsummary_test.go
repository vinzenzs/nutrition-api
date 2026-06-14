package agenttools

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Build-shape parity for the ported nutrition-summary domain: each tool must
// reproduce the bespoke handler's exact REST mapping (method, path, query).
// All four tools are pure reads (no body, no idempotency key).
func TestNutritionSummary_BuildShapes(t *testing.T) {
	specs := ByName(MCPRegistry())

	for _, name := range []string{
		"daily_summary", "range_summary", "rolling_summary", "protein_distribution",
	} {
		_, ok := specs[name]
		require.Truef(t, ok, "tool %s missing from MCPRegistry", name)
		assert.Equalf(t, TierRead, specs[name].Tier, "tool %s must be TierRead", name)
	}

	// ----- daily_summary -----
	// Full query: date + tz + meal_type.
	dc, err := specs["daily_summary"].Build(json.RawMessage(
		`{"date":"2026-06-06","tz":"Europe/Berlin","meal_type":"breakfast"}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", dc.Method)
	assert.Equal(t, "/summary/daily", dc.Path)
	assert.Equal(t, "2026-06-06", dc.Query.Get("date"))
	assert.Equal(t, "Europe/Berlin", dc.Query.Get("tz"))
	assert.Equal(t, "breakfast", dc.Query.Get("meal_type"))
	assert.Nil(t, dc.Body)

	// Optionals omitted when empty.
	dc2, err := specs["daily_summary"].Build(json.RawMessage(`{"date":"2026-06-06"}`))
	require.NoError(t, err)
	assert.Equal(t, "2026-06-06", dc2.Query.Get("date"))
	_, hasTZ := dc2.Query["tz"]
	assert.False(t, hasTZ, "tz must be omitted when empty")
	_, hasMealType := dc2.Query["meal_type"]
	assert.False(t, hasMealType, "meal_type must be omitted when empty")

	// ----- range_summary -----
	rc, err := specs["range_summary"].Build(json.RawMessage(
		`{"from":"2026-06-01","to":"2026-06-07","tz":"UTC","group_by":"meal_type"}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", rc.Method)
	assert.Equal(t, "/summary/range", rc.Path)
	assert.Equal(t, "2026-06-01", rc.Query.Get("from"))
	assert.Equal(t, "2026-06-07", rc.Query.Get("to"))
	assert.Equal(t, "UTC", rc.Query.Get("tz"))
	assert.Equal(t, "meal_type", rc.Query.Get("group_by"))
	assert.Nil(t, rc.Body)

	rc2, err := specs["range_summary"].Build(json.RawMessage(
		`{"from":"2026-06-01","to":"2026-06-07"}`))
	require.NoError(t, err)
	_, hasTZ = rc2.Query["tz"]
	assert.False(t, hasTZ, "tz must be omitted when empty")
	_, hasGroupBy := rc2.Query["group_by"]
	assert.False(t, hasGroupBy, "group_by must be omitted when empty")

	// ----- rolling_summary -----
	roc, err := specs["rolling_summary"].Build(json.RawMessage(
		`{"anchor_date":"2026-06-08","window_days":7,"tz":"Europe/Berlin"}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", roc.Method)
	assert.Equal(t, "/summary/rolling", roc.Path)
	assert.Equal(t, "2026-06-08", roc.Query.Get("anchor_date"))
	assert.Equal(t, "7", roc.Query.Get("window_days"))
	assert.Equal(t, "Europe/Berlin", roc.Query.Get("tz"))
	assert.Nil(t, roc.Body)

	roc2, err := specs["rolling_summary"].Build(json.RawMessage(
		`{"anchor_date":"2026-06-08","window_days":7}`))
	require.NoError(t, err)
	// window_days is always set (strconv.Itoa, even of 0).
	assert.Equal(t, "7", roc2.Query.Get("window_days"))
	_, hasTZ = roc2.Query["tz"]
	assert.False(t, hasTZ, "tz must be omitted when empty")

	// window_days defaults to "0" when unset (strconv.Itoa of zero value),
	// matching the bespoke handler which always emitted the field.
	roc3, err := specs["rolling_summary"].Build(json.RawMessage(`{"anchor_date":"2026-06-08"}`))
	require.NoError(t, err)
	assert.Equal(t, "0", roc3.Query.Get("window_days"))

	// ----- protein_distribution -----
	bw := 72.5
	pdIn, _ := json.Marshal(ProteinDistributionArgs{
		Date: "2026-06-09", TZ: "Europe/Berlin", BodyWeightKg: &bw,
	})
	pc, err := specs["protein_distribution"].Build(json.RawMessage(pdIn))
	require.NoError(t, err)
	assert.Equal(t, "GET", pc.Method)
	assert.Equal(t, "/summary/protein-distribution", pc.Path)
	assert.Equal(t, "2026-06-09", pc.Query.Get("date"))
	assert.Equal(t, "Europe/Berlin", pc.Query.Get("tz"))
	// strconv.FormatFloat(_, 'f', -1, 64) renders 72.5 as "72.5".
	assert.Equal(t, "72.5", pc.Query.Get("body_weight_kg"))
	assert.Nil(t, pc.Body)

	// Optionals omitted when unset.
	pc2, err := specs["protein_distribution"].Build(json.RawMessage(`{"date":"2026-06-09"}`))
	require.NoError(t, err)
	assert.Equal(t, "2026-06-09", pc2.Query.Get("date"))
	_, hasTZ = pc2.Query["tz"]
	assert.False(t, hasTZ, "tz must be omitted when empty")
	_, hasBW := pc2.Query["body_weight_kg"]
	assert.False(t, hasBW, "body_weight_kg must be omitted when nil")
}
