package mcpserver

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/agenttools"
)

// dispatchMCP is the single generic handler every ported tool runs through.
// These tests pin its cross-cutting behavior (idempotency-key attachment,
// explicit-key override, reads get no key, error mapping) once, so the
// per-domain Build-shape tests in agenttools only need to assert the REST shape.

func mcpSpec(t *testing.T, name string) agenttools.Spec {
	t.Helper()
	s, ok := agenttools.ByName(agenttools.MCPRegistry())[name]
	require.Truef(t, ok, "tool %q not on the MCP surface", name)
	return s
}

func TestDispatchMCP_WriteDerivesIdempotencyKey(t *testing.T) {
	c, recs := newWorkoutRecorder(t, 201, `{"date":"2026-06-14"}`)
	res := dispatchMCP(context.Background(), c, mcpSpec(t, "log_hydration_balance"),
		json.RawMessage(`{"date":"2026-06-14","sweat_loss_ml":2400}`))
	require.Len(t, *recs, 1)
	assert.False(t, res.IsError)
	assert.NotEmpty(t, (*recs)[0].idemKey, "a write tool with no explicit key derives one")
}

func TestDispatchMCP_ExplicitKeyWins(t *testing.T) {
	c, recs := newWorkoutRecorder(t, 201, `{"date":"2026-06-14"}`)
	_ = dispatchMCP(context.Background(), c, mcpSpec(t, "log_hydration_balance"),
		json.RawMessage(`{"date":"2026-06-14","sweat_loss_ml":2400,"idempotency_key":"explicit-key"}`))
	require.Len(t, *recs, 1)
	assert.Equal(t, "explicit-key", (*recs)[0].idemKey)
}

func TestDispatchMCP_ReadGetsNoKey(t *testing.T) {
	c, recs := newWorkoutRecorder(t, 200, `[]`)
	_ = dispatchMCP(context.Background(), c, mcpSpec(t, "devices_list"), json.RawMessage(`{}`))
	require.Len(t, *recs, 1)
	assert.Empty(t, (*recs)[0].idemKey, "a read never carries an idempotency key")
}

func TestDispatchMCP_OmitIdempotencyKeyWriteCarriesNoKey(t *testing.T) {
	// materialize_training_plan is a POST but is re-runnable by design, so it
	// dispatches without an Idempotency-Key (Spec.OmitIdempotencyKey) — a
	// derived key would replay a stale response after a plan edit.
	c, recs := newWorkoutRecorder(t, 200, `{"materialized":3}`)
	_ = dispatchMCP(context.Background(), c, mcpSpec(t, "materialize_training_plan"),
		json.RawMessage(`{"plan_id":"p1","scope":"all"}`))
	require.Len(t, *recs, 1)
	assert.Equal(t, "POST", (*recs)[0].method)
	assert.Empty(t, (*recs)[0].idemKey, "a re-runnable write (OmitIdempotencyKey) carries no key")
}

func TestDispatchMCP_SameInputSameKey(t *testing.T) {
	c, recs := newWorkoutRecorder(t, 201, `{}`)
	in := json.RawMessage(`{"date":"2026-06-14","sweat_loss_ml":2400}`)
	spec := mcpSpec(t, "log_hydration_balance")
	_ = dispatchMCP(context.Background(), c, spec, in)
	_ = dispatchMCP(context.Background(), c, spec, in)
	require.Len(t, *recs, 2)
	assert.Equal(t, (*recs)[0].idemKey, (*recs)[1].idemKey, "identical input derives a stable key")
}
