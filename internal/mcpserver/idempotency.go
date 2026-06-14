package mcpserver

import (
	"github.com/vinzenzs/nutrition-api/internal/agenttools"
)

// deriveIdempotencyKey / effectiveIdempotencyKey delegate to the shared
// implementation in internal/agenttools (D12): one canonicalization +
// hashing path for both the MCP server and the in-app chat dispatcher, so a
// write replayed from either surface hits the same REST idempotency record.

func deriveIdempotencyKey(toolName string, args any) string {
	return agenttools.DeriveIdempotencyKey(toolName, args)
}

func effectiveIdempotencyKey(explicit, toolName string, args any) string {
	return agenttools.EffectiveIdempotencyKey(explicit, toolName, args)
}
