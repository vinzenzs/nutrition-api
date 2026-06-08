package mcpserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type logMealArgs struct {
	ProductID      string  `json:"product_id"`
	QuantityG      float64 `json:"quantity_g"`
	LoggedAt       string  `json:"logged_at"`
	IdempotencyKey string  `json:"idempotency_key,omitempty"`
}

func TestDeriveIdempotencyKey_SameArgsSameKey(t *testing.T) {
	a := logMealArgs{ProductID: "abc", QuantityG: 100, LoggedAt: "2026-06-06T12:00:00Z"}
	b := logMealArgs{ProductID: "abc", QuantityG: 100, LoggedAt: "2026-06-06T12:00:00Z"}
	assert.Equal(t, deriveIdempotencyKey("log_meal", a), deriveIdempotencyKey("log_meal", b))
}

func TestDeriveIdempotencyKey_DifferentArgsDifferentKey(t *testing.T) {
	a := logMealArgs{ProductID: "abc", QuantityG: 100, LoggedAt: "2026-06-06T12:00:00Z"}
	b := logMealArgs{ProductID: "abc", QuantityG: 200, LoggedAt: "2026-06-06T12:00:00Z"}
	assert.NotEqual(t, deriveIdempotencyKey("log_meal", a), deriveIdempotencyKey("log_meal", b))
}

func TestDeriveIdempotencyKey_DifferentToolNameDifferentKey(t *testing.T) {
	a := logMealArgs{ProductID: "abc", QuantityG: 100, LoggedAt: "2026-06-06T12:00:00Z"}
	assert.NotEqual(t,
		deriveIdempotencyKey("log_meal", a),
		deriveIdempotencyKey("log_meal_freeform", a),
	)
}

func TestDeriveIdempotencyKey_IdempotencyKeyFieldIsIgnored(t *testing.T) {
	withoutKey := logMealArgs{ProductID: "abc", QuantityG: 100, LoggedAt: "2026-06-06T12:00:00Z"}
	withKey := withoutKey
	withKey.IdempotencyKey = "explicit-key-doesnt-matter-here"
	assert.Equal(t,
		deriveIdempotencyKey("log_meal", withoutKey),
		deriveIdempotencyKey("log_meal", withKey),
	)
}

func TestEffectiveIdempotencyKey_ExplicitWins(t *testing.T) {
	a := logMealArgs{ProductID: "abc", QuantityG: 100, LoggedAt: "2026-06-06T12:00:00Z"}
	derived := deriveIdempotencyKey("log_meal", a)
	got := effectiveIdempotencyKey("explicit-abc", "log_meal", a)
	assert.Equal(t, "explicit-abc", got)
	assert.NotEqual(t, derived, got)
}

func TestEffectiveIdempotencyKey_DerivesWhenEmpty(t *testing.T) {
	a := logMealArgs{ProductID: "abc", QuantityG: 100, LoggedAt: "2026-06-06T12:00:00Z"}
	got := effectiveIdempotencyKey("", "log_meal", a)
	assert.Equal(t, deriveIdempotencyKey("log_meal", a), got)
}
