package agenttools

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Build-shape parity for the ported fitness-metrics domain: each tool must
// reproduce the bespoke handler's exact REST mapping (method, path, query, body).
func TestFitnessMetrics_BuildShapes(t *testing.T) {
	specs := ByName(MCPRegistry())

	// All four tools are present on the MCP surface.
	for _, name := range []string{
		"log_fitness_metrics", "list_fitness_metrics",
		"get_fitness_metrics", "delete_fitness_metrics",
	} {
		_, ok := specs[name]
		require.Truef(t, ok, "tool %s missing from MCPRegistry", name)
	}

	// Tiers: writes get an idempotency key (IsWrite), reads do not.
	assert.Equal(t, TierWriteAuto, specs["log_fitness_metrics"].Tier)
	assert.Equal(t, TierRead, specs["list_fitness_metrics"].Tier)
	assert.Equal(t, TierRead, specs["get_fitness_metrics"].Tier)
	assert.Equal(t, TierWriteAuto, specs["delete_fitness_metrics"].Tier)

	// log_fitness_metrics → POST /fitness-metrics; body carries the snapshot
	// fields and omits idempotency_key.
	call, err := specs["log_fitness_metrics"].Build(json.RawMessage(
		`{"date":"2026-06-14","vo2max_running":58.5,"race_predictor_5k_seconds":1080,"acute_load":42.0,"idempotency_key":"k1"}`))
	require.NoError(t, err)
	assert.Equal(t, "POST", call.Method)
	assert.Equal(t, "/fitness-metrics", call.Path)
	assert.Nil(t, call.Query)
	var logBody map[string]any
	require.NoError(t, json.Unmarshal(call.Body, &logBody))
	assert.Equal(t, "2026-06-14", logBody["date"])
	assert.Equal(t, 58.5, logBody["vo2max_running"])
	assert.EqualValues(t, 1080, logBody["race_predictor_5k_seconds"])
	assert.EqualValues(t, 42.0, logBody["acute_load"])
	// idempotency_key never leaks into the body.
	assert.NotContains(t, logBody, "idempotency_key")
	// omitempty unset pointers are absent.
	assert.NotContains(t, logBody, "vo2max_cycling")
	assert.NotContains(t, logBody, "chronic_load")
	assert.NotContains(t, logBody, "race_predictor_10k_seconds")

	// Minimal log: only date present, all optional fields omitted.
	minCall, err := specs["log_fitness_metrics"].Build(json.RawMessage(`{"date":"2026-06-14"}`))
	require.NoError(t, err)
	assert.JSONEq(t, `{"date":"2026-06-14"}`, string(minCall.Body))

	// list_fitness_metrics → GET /fitness-metrics?from=..&to=..
	lc, err := specs["list_fitness_metrics"].Build(json.RawMessage(
		`{"from":"2026-06-01","to":"2026-06-14"}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", lc.Method)
	assert.Equal(t, "/fitness-metrics", lc.Path)
	assert.Equal(t, "2026-06-01", lc.Query.Get("from"))
	assert.Equal(t, "2026-06-14", lc.Query.Get("to"))
	assert.Nil(t, lc.Body)

	// get_fitness_metrics → GET /fitness-metrics/{date}
	gc, err := specs["get_fitness_metrics"].Build(json.RawMessage(`{"date":"2026-06-14"}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", gc.Method)
	assert.Equal(t, "/fitness-metrics/2026-06-14", gc.Path)
	assert.Nil(t, gc.Query)
	assert.Nil(t, gc.Body)

	// delete_fitness_metrics → DELETE /fitness-metrics/{date}, no body.
	dc, err := specs["delete_fitness_metrics"].Build(json.RawMessage(
		`{"date":"2026-06-14","idempotency_key":"k2"}`))
	require.NoError(t, err)
	assert.Equal(t, "DELETE", dc.Method)
	assert.Equal(t, "/fitness-metrics/2026-06-14", dc.Path)
	assert.Nil(t, dc.Body)
}
