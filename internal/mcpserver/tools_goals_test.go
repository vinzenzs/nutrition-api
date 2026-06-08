package mcpserver

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetGoals_HitsGoalsEndpoint(t *testing.T) {
	c, rec := newRecordingBodyClient(t, 200, `{"goals":null}`)
	r := handleGetGoals(context.Background(), c, GetGoalsArgs{})
	assert.False(t, r.IsError)
	assert.Equal(t, http.MethodGet, rec.method)
	assert.Equal(t, "/goals", rec.path)
	assert.JSONEq(t, `{"goals":null}`, extractText(t, r))
}

func TestSetGoals_PutsToGoalsEndpoint(t *testing.T) {
	c, rec := newRecordingBodyClient(t, 200, `{"goals":{"kcal":{"min":2090,"max":2310}}}`)
	kmin := 2090.0
	kmax := 2310.0
	pmin := 150.0
	pmax := 190.0
	args := SetGoalsArgs{
		Kcal:     &GoalRange{Min: &kmin, Max: &kmax},
		ProteinG: &GoalRange{Min: &pmin, Max: &pmax},
	}
	r := handleSetGoals(context.Background(), c, args)
	assert.False(t, r.IsError)
	assert.Equal(t, http.MethodPut, rec.method)
	assert.Equal(t, "/goals", rec.path)
	assert.JSONEq(t,
		`{"kcal":{"min":2090,"max":2310},"protein_g":{"min":150,"max":190}}`,
		string(rec.body))
	// After harden-write-paths: set_goals MUST NOT forward any Idempotency-Key
	// header — PUT /goals on the REST side rejects it with 400. The wrapper
	// drops the field from the tool schema entirely.
	assert.Empty(t, rec.idemKey, "set_goals must not forward Idempotency-Key on PUT")
}

func TestSetGoals_NoIdempotencyAcrossRepeatedCalls(t *testing.T) {
	// Bug-fix regression: every set_goals invocation hits the backend live; no
	// auto-derived idempotency key. Drive the tool twice with byte-identical
	// inputs and verify neither call carries a key.
	c, rec := newRecordingBodyClient(t, 200, `{"goals":{}}`)
	kmin := 2090.0
	kmax := 2310.0
	args := SetGoalsArgs{Kcal: &GoalRange{Min: &kmin, Max: &kmax}}

	_ = handleSetGoals(context.Background(), c, args)
	firstKey := rec.idemKey
	_ = handleSetGoals(context.Background(), c, args)
	secondKey := rec.idemKey

	assert.Empty(t, firstKey, "first set_goals call must not carry Idempotency-Key")
	assert.Empty(t, secondKey, "second set_goals call must not carry Idempotency-Key")
}

func TestSetGoals_400ForwardsValidationError(t *testing.T) {
	body := `{"error":"goal_value_invalid","field":"kcal.min"}`
	c, _ := newRecordingBodyClient(t, 400, body)
	bad := -1.0
	r := handleSetGoals(context.Background(), c, SetGoalsArgs{Kcal: &GoalRange{Min: &bad}})
	assert.True(t, r.IsError)
	assert.JSONEq(t, body, extractText(t, r))
}
