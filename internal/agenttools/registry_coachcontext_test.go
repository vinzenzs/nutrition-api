package agenttools

import (
	"encoding/json"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The coach-context domain contributes exactly these two MCP tools.
func TestCoachContext_Registered(t *testing.T) {
	byName := ByName(MCPRegistry())
	for _, name := range []string{
		"get_training_context",
		"get_recovery_context",
	} {
		_, ok := byName[name]
		assert.Truef(t, ok, "expected MCP tool %q to be registered", name)
	}
}

func TestCoachContext_Tiers(t *testing.T) {
	byName := ByName(MCPRegistry())
	assert.Equal(t, TierRead, byName["get_training_context"].Tier)
	assert.Equal(t, TierRead, byName["get_recovery_context"].Tier)
}

func TestCoachContext_TrainingBuild_Full(t *testing.T) {
	spec := ByName(MCPRegistry())["get_training_context"]
	require.NotNil(t, spec.Build)

	in := json.RawMessage(`{"date":"2026-06-13","tz":"Europe/Vienna","lookback_days":30,"lookahead_days":10}`)
	call, err := spec.Build(in)
	require.NoError(t, err)

	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/context/training", call.Path)
	assert.Equal(t, url.Values{
		"date":           {"2026-06-13"},
		"tz":             {"Europe/Vienna"},
		"lookback_days":  {"30"},
		"lookahead_days": {"10"},
	}, call.Query)
	assert.Nil(t, call.Body)
}

func TestCoachContext_TrainingBuild_Empty(t *testing.T) {
	spec := ByName(MCPRegistry())["get_training_context"]
	call, err := spec.Build(json.RawMessage(`{}`))
	require.NoError(t, err)

	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/context/training", call.Path)
	// No optional fields set: empty query (no keys), zero-value days omitted.
	assert.Equal(t, url.Values{}, call.Query)
	assert.Nil(t, call.Body)
}

func TestCoachContext_TrainingBuild_ZeroDaysOmitted(t *testing.T) {
	spec := ByName(MCPRegistry())["get_training_context"]
	call, err := spec.Build(json.RawMessage(`{"date":"2026-06-13","lookback_days":0,"lookahead_days":0}`))
	require.NoError(t, err)

	// Zero day windows are NOT emitted (handler only sets when > 0).
	assert.Equal(t, url.Values{"date": {"2026-06-13"}}, call.Query)
}

func TestCoachContext_RecoveryBuild_Full(t *testing.T) {
	spec := ByName(MCPRegistry())["get_recovery_context"]
	require.NotNil(t, spec.Build)

	in := json.RawMessage(`{"date":"2026-06-13","tz":"Europe/Vienna","days":14}`)
	call, err := spec.Build(in)
	require.NoError(t, err)

	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/context/recovery", call.Path)
	assert.Equal(t, url.Values{
		"date": {"2026-06-13"},
		"tz":   {"Europe/Vienna"},
		"days": {"14"},
	}, call.Query)
	assert.Nil(t, call.Body)
}

func TestCoachContext_RecoveryBuild_Empty(t *testing.T) {
	spec := ByName(MCPRegistry())["get_recovery_context"]
	call, err := spec.Build(json.RawMessage(`{}`))
	require.NoError(t, err)

	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/context/recovery", call.Path)
	assert.Equal(t, url.Values{}, call.Query)
	assert.Nil(t, call.Body)
}

func TestCoachContext_RecoveryBuild_ZeroDaysOmitted(t *testing.T) {
	spec := ByName(MCPRegistry())["get_recovery_context"]
	call, err := spec.Build(json.RawMessage(`{"date":"2026-06-13","days":0}`))
	require.NoError(t, err)

	assert.Equal(t, url.Values{"date": {"2026-06-13"}}, call.Query)
}
