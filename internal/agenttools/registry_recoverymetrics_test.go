package agenttools

import (
	"encoding/json"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The recovery-metrics domain contributes exactly these four MCP tools.
func TestRecoveryMetrics_Registered(t *testing.T) {
	byName := ByName(MCPRegistry())
	for _, name := range []string{
		"log_recovery_metrics",
		"list_recovery_metrics",
		"get_recovery_metrics",
		"delete_recovery_metrics",
	} {
		_, ok := byName[name]
		assert.Truef(t, ok, "expected MCP tool %q to be registered", name)
	}
}

func TestRecoveryMetrics_Tiers(t *testing.T) {
	byName := ByName(MCPRegistry())
	assert.Equal(t, TierWriteAuto, byName["log_recovery_metrics"].Tier)
	assert.Equal(t, TierRead, byName["list_recovery_metrics"].Tier)
	assert.Equal(t, TierRead, byName["get_recovery_metrics"].Tier)
	assert.Equal(t, TierWriteAuto, byName["delete_recovery_metrics"].Tier)
}

func TestRecoveryMetrics_LogBuild(t *testing.T) {
	spec := ByName(MCPRegistry())["log_recovery_metrics"]
	require.NotNil(t, spec.Build)

	in := json.RawMessage(`{
		"date":"2026-06-13",
		"sleep_seconds":27000,
		"sleep_score":82,
		"hrv_ms":58.5,
		"resting_hr":46,
		"stress_avg":31,
		"body_battery_charged":74,
		"body_battery_drained":60,
		"training_readiness":68,
		"idempotency_key":"client-supplied-key"
	}`)
	call, err := spec.Build(in)
	require.NoError(t, err)

	assert.Equal(t, "POST", call.Method)
	assert.Equal(t, "/recovery-metrics", call.Path)
	assert.Nil(t, call.Query)

	// Body is the snapshot struct WITHOUT idempotency_key.
	var got map[string]any
	require.NoError(t, json.Unmarshal(call.Body, &got))
	assert.NotContains(t, got, "idempotency_key")
	assert.Equal(t, "2026-06-13", got["date"])
	assert.EqualValues(t, 27000, got["sleep_seconds"])
	assert.EqualValues(t, 82, got["sleep_score"])
	assert.EqualValues(t, 58.5, got["hrv_ms"])
	assert.EqualValues(t, 46, got["resting_hr"])
	assert.EqualValues(t, 31, got["stress_avg"])
	assert.EqualValues(t, 74, got["body_battery_charged"])
	assert.EqualValues(t, 60, got["body_battery_drained"])
	assert.EqualValues(t, 68, got["training_readiness"])
}

func TestRecoveryMetrics_LogBuild_OmitsAbsentOptionals(t *testing.T) {
	spec := ByName(MCPRegistry())["log_recovery_metrics"]
	call, err := spec.Build(json.RawMessage(`{"date":"2026-06-13"}`))
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(call.Body, &got))
	// Only the required, non-omitempty date field is present.
	assert.Equal(t, map[string]any{"date": "2026-06-13"}, got)
}

func TestRecoveryMetrics_ListBuild(t *testing.T) {
	spec := ByName(MCPRegistry())["list_recovery_metrics"]
	require.NotNil(t, spec.Build)

	call, err := spec.Build(json.RawMessage(`{"from":"2026-06-01","to":"2026-06-13"}`))
	require.NoError(t, err)

	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/recovery-metrics", call.Path)
	assert.Equal(t, url.Values{"from": {"2026-06-01"}, "to": {"2026-06-13"}}, call.Query)
	assert.Nil(t, call.Body)
}

func TestRecoveryMetrics_GetBuild(t *testing.T) {
	spec := ByName(MCPRegistry())["get_recovery_metrics"]
	require.NotNil(t, spec.Build)

	call, err := spec.Build(json.RawMessage(`{"date":"2026-06-13"}`))
	require.NoError(t, err)

	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/recovery-metrics/2026-06-13", call.Path)
	assert.Nil(t, call.Query)
	assert.Nil(t, call.Body)
}

func TestRecoveryMetrics_GetBuild_MissingDate(t *testing.T) {
	spec := ByName(MCPRegistry())["get_recovery_metrics"]
	_, err := spec.Build(json.RawMessage(`{}`))
	assert.Error(t, err)
}

func TestRecoveryMetrics_DeleteBuild(t *testing.T) {
	spec := ByName(MCPRegistry())["delete_recovery_metrics"]
	require.NotNil(t, spec.Build)

	call, err := spec.Build(json.RawMessage(`{"date":"2026-06-13","idempotency_key":"k"}`))
	require.NoError(t, err)

	assert.Equal(t, "DELETE", call.Method)
	assert.Equal(t, "/recovery-metrics/2026-06-13", call.Path)
	assert.Nil(t, call.Query)
	assert.Nil(t, call.Body)
}

func TestRecoveryMetrics_DeleteBuild_MissingDate(t *testing.T) {
	spec := ByName(MCPRegistry())["delete_recovery_metrics"]
	_, err := spec.Build(json.RawMessage(`{}`))
	assert.Error(t, err)
}
