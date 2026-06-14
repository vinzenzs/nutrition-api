package agenttools

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The garminmisc domain contributes exactly three MCP-only read tools, each
// mirroring a REST list endpoint 1:1.
func TestGarminMisc_RegisteredAndReadOnly(t *testing.T) {
	specs := ByName(MCPRegistry())
	for _, name := range []string{"devices_list", "health_vitals_list", "achievements_list"} {
		s, ok := specs[name]
		require.Truef(t, ok, "tool %s not registered on the MCP surface", name)
		assert.Equalf(t, TierRead, s.Tier, "tool %s should be TierRead", name)
		assert.Truef(t, s.MCPExposed, "tool %s should be MCP-exposed", name)
		assert.NotNilf(t, s.SchemaType, "tool %s should carry a SchemaType", name)
	}
}

func TestGarminMisc_BuildShapes(t *testing.T) {
	specs := ByName(MCPRegistry())

	// devices_list → GET /devices, no query.
	call, err := specs["devices_list"].Build(json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/devices", call.Path)
	assert.Empty(t, call.Query)
	assert.Empty(t, call.Body)

	// health_vitals_list → GET /health-vitals?from=...&to=...
	call, err = specs["health_vitals_list"].Build(json.RawMessage(`{"from":"2026-01-01","to":"2026-02-01"}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/health-vitals", call.Path)
	assert.Equal(t, "2026-01-01", call.Query.Get("from"))
	assert.Equal(t, "2026-02-01", call.Query.Get("to"))

	// health_vitals_list sets from/to unconditionally (empty values still set).
	call, err = specs["health_vitals_list"].Build(json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.True(t, call.Query.Has("from"))
	assert.True(t, call.Query.Has("to"))
	assert.Equal(t, "", call.Query.Get("from"))
	assert.Equal(t, "", call.Query.Get("to"))

	// achievements_list with kind → GET /achievements?kind=badge
	call, err = specs["achievements_list"].Build(json.RawMessage(`{"kind":"badge"}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/achievements", call.Path)
	assert.Equal(t, "badge", call.Query.Get("kind"))

	// achievements_list without kind → no kind query key set.
	call, err = specs["achievements_list"].Build(json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.Equal(t, "/achievements", call.Path)
	assert.False(t, call.Query.Has("kind"))
}
