package agenttools

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Identical inputs hash identically; reordering keys / whitespace is invariant;
// different inputs differ. This is the property the retry-replay guarantee rests
// on, shared by both AI surfaces (D12).
func TestDeriveIdempotencyKey_Deterministic(t *testing.T) {
	a := DeriveIdempotencyKey("add_shopping_items", json.RawMessage(`{"items":[{"name":"onion"}]}`))
	b := DeriveIdempotencyKey("add_shopping_items", json.RawMessage(`{"items":[{"name":"onion"}]}`))
	assert.Equal(t, a, b)

	c := DeriveIdempotencyKey("create_planned_meal", json.RawMessage(`{"slot":"dinner","plan_date":"2026-06-12"}`))
	d := DeriveIdempotencyKey("create_planned_meal", json.RawMessage("{ \"plan_date\":\"2026-06-12\" , \"slot\":\"dinner\" }"))
	assert.Equal(t, c, d, "key/whitespace reordering must be invariant")

	e := DeriveIdempotencyKey("add_shopping_items", json.RawMessage(`{"items":[{"name":"garlic"}]}`))
	assert.NotEqual(t, a, e)

	assert.Len(t, a, 64)
	assert.Empty(t, strings.Trim(a, "0123456789abcdef"))
}

// The embedded idempotency_key field is stripped before hashing, so its
// presence/absence does not change the derived key.
func TestDeriveIdempotencyKey_StripsEmbeddedKey(t *testing.T) {
	without := DeriveIdempotencyKey("log_meal", json.RawMessage(`{"product_id":"abc","quantity_g":100}`))
	with := DeriveIdempotencyKey("log_meal", json.RawMessage(`{"product_id":"abc","quantity_g":100,"idempotency_key":"whatever"}`))
	assert.Equal(t, without, with)
}

func TestDeriveIdempotencyKey_DifferentToolDifferentKey(t *testing.T) {
	in := json.RawMessage(`{"product_id":"abc"}`)
	assert.NotEqual(t, DeriveIdempotencyKey("log_meal", in), DeriveIdempotencyKey("delete_meal", in))
}

func TestEffectiveIdempotencyKey(t *testing.T) {
	in := json.RawMessage(`{"product_id":"abc"}`)
	assert.Equal(t, "explicit", EffectiveIdempotencyKey("explicit", "log_meal", in))
	assert.Equal(t, DeriveIdempotencyKey("log_meal", in), EffectiveIdempotencyKey("", "log_meal", in))
}
