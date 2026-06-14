package agenttools

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Build-shape parity for the ported energy domain: the single tool must
// reproduce the bespoke handler's exact REST mapping (method, path, query).
func TestEnergy_BuildShapes(t *testing.T) {
	specs := ByName(MCPRegistry())

	spec, ok := specs["weekly_energy_summary"]
	require.True(t, ok, "tool weekly_energy_summary missing from MCPRegistry")

	// Read-only tool: no idempotency key.
	assert.Equal(t, TierRead, spec.Tier)

	// Required params only → GET /energy/availability?from=..&to=..; optional
	// query keys are omitted when unset.
	call, err := spec.Build(json.RawMessage(
		`{"from":"2026-06-01T00:00:00Z","to":"2026-06-08T00:00:00Z"}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/energy/availability", call.Path)
	assert.Equal(t, "2026-06-01T00:00:00Z", call.Query.Get("from"))
	assert.Equal(t, "2026-06-08T00:00:00Z", call.Query.Get("to"))
	assert.Empty(t, call.Query.Get("tz"))
	assert.Empty(t, call.Query.Get("lean_mass_kg"))
	assert.Empty(t, call.Query.Get("body_fat_pct"))
	assert.Nil(t, call.Body)

	// Optional overrides are forwarded; floats use the minimal -1 precision form.
	lean := 62.0
	bf := 15.0
	full, err := spec.Build(mustMarshal(t, WeeklyEnergySummaryArgs{
		From:       "2026-06-01T00:00:00Z",
		To:         "2026-06-08T00:00:00Z",
		TZ:         "Europe/Berlin",
		LeanMassKg: &lean,
		BodyFatPct: &bf,
	}))
	require.NoError(t, err)
	assert.Equal(t, "Europe/Berlin", full.Query.Get("tz"))
	assert.Equal(t, "62", full.Query.Get("lean_mass_kg"))
	assert.Equal(t, "15", full.Query.Get("body_fat_pct"))

	// body_fat_pct is omitted when only lean_mass_kg is set.
	leanOnly, err := spec.Build(mustMarshal(t, WeeklyEnergySummaryArgs{
		From:       "2026-06-01T00:00:00Z",
		To:         "2026-06-08T00:00:00Z",
		LeanMassKg: &lean,
	}))
	require.NoError(t, err)
	assert.Equal(t, "62", leanOnly.Query.Get("lean_mass_kg"))
	assert.Empty(t, leanOnly.Query.Get("body_fat_pct"))
}

func mustMarshal(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}
