package agenttools

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The weight domain contributes exactly these five MCP tools, with the tiers
// the bespoke registrations implied (reads → TierRead; mutations → write-auto).
func TestWeight_SurfaceAndTiers(t *testing.T) {
	specs := ByName(MCPRegistry())
	wantTier := map[string]Tier{
		"log_weight":    TierWriteAuto,
		"list_weights":  TierRead,
		"patch_weight":  TierWriteAuto,
		"delete_weight": TierWriteAuto,
		"weight_trend":  TierRead,
	}
	for name, tier := range wantTier {
		s, ok := specs[name]
		require.Truef(t, ok, "missing MCP tool %s", name)
		assert.Equalf(t, tier, s.Tier, "tier wrong for %s", name)
		assert.NotNilf(t, s.SchemaType, "tool %s must carry a SchemaType for MCP schema reflection", name)
		assert.Truef(t, s.MCPExposed, "tool %s should be MCP-exposed", name)
	}
}

// log_weight → POST /weight; always emits weight_kg + logged_at, drops the
// idempotency_key from the body, and includes supplied optionals.
func TestWeight_Log(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["log_weight"].Build(json.RawMessage(
		`{"weight_kg":72.5,"logged_at":"2026-06-07T07:00:00Z","body_fat_pct":14.2,` +
			`"note":"morning, fasted","idempotency_key":"k1"}`))
	require.NoError(t, err)
	assert.Equal(t, "POST", call.Method)
	assert.Equal(t, "/weight", call.Path)
	assert.Empty(t, call.Query)
	assert.JSONEq(t,
		`{"weight_kg":72.5,"logged_at":"2026-06-07T07:00:00Z","body_fat_pct":14.2,"note":"morning, fasted"}`,
		string(call.Body))
	assert.NotContains(t, string(call.Body), "idempotency_key")
}

// log_weight omits unset optional fields (omitempty) but always emits the two
// required scalars.
func TestWeight_LogOmitsOptionals(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["log_weight"].Build(json.RawMessage(
		`{"weight_kg":72.5,"logged_at":"2026-06-07T07:00:00Z"}`))
	require.NoError(t, err)
	assert.JSONEq(t,
		`{"weight_kg":72.5,"logged_at":"2026-06-07T07:00:00Z"}`,
		string(call.Body))
}

// list_weights → GET /weight with from/to query, no idempotency.
func TestWeight_List(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["list_weights"].Build(json.RawMessage(
		`{"from":"2026-06-01T00:00:00Z","to":"2026-06-08T00:00:00Z"}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/weight", call.Path)
	assert.Equal(t, "2026-06-01T00:00:00Z", call.Query.Get("from"))
	assert.Equal(t, "2026-06-08T00:00:00Z", call.Query.Get("to"))
}

// patch_weight → PATCH /weight/{id}; only supplied fields in the body, id used
// as the path segment, idempotency_key dropped.
func TestWeight_Patch(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["patch_weight"].Build(json.RawMessage(
		`{"id":"abc","body_fat_pct":13.8,"idempotency_key":"k1"}`))
	require.NoError(t, err)
	assert.Equal(t, "PATCH", call.Method)
	assert.Equal(t, "/weight/abc", call.Path)
	assert.JSONEq(t, `{"body_fat_pct":13.8}`, string(call.Body))
	assert.NotContains(t, string(call.Body), "idempotency_key")
	assert.NotContains(t, string(call.Body), `"id"`)
}

// delete_weight → DELETE /weight/{id}.
func TestWeight_Delete(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["delete_weight"].Build(json.RawMessage(`{"id":"abc"}`))
	require.NoError(t, err)
	assert.Equal(t, "DELETE", call.Method)
	assert.Equal(t, "/weight/abc", call.Path)
}

// weight_trend → GET /weight/trend with from/to plus optional window_days/tz.
func TestWeight_Trend(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["weight_trend"].Build(json.RawMessage(
		`{"from":"2026-05-01","to":"2026-06-07","window_days":7,"tz":"Europe/Berlin"}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/weight/trend", call.Path)
	assert.Equal(t, "2026-05-01", call.Query.Get("from"))
	assert.Equal(t, "2026-06-07", call.Query.Get("to"))
	assert.Equal(t, "7", call.Query.Get("window_days"))
	assert.Equal(t, "Europe/Berlin", call.Query.Get("tz"))
}

// weight_trend omits window_days and tz when not supplied.
func TestWeight_TrendOmitsOptionals(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["weight_trend"].Build(json.RawMessage(
		`{"from":"2026-05-01","to":"2026-06-07"}`))
	require.NoError(t, err)
	assert.False(t, call.Query.Has("window_days"))
	assert.False(t, call.Query.Has("tz"))
}
