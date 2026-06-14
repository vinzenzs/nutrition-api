package mcpserver

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/vinzenzs/nutrition-api/internal/agenttools"
)

// chatBespokeTools are the agenttools surface entries whose names intentionally
// differ from the MCP server's equivalents (the chat coach merges/renames a few
// reads for ergonomics). They are exempt from the name-match drift guard. As
// phase 3 broadens the chat surface, newly-added coach tools should reuse the
// MCP names (and thus stay OUT of this allowlist); this set should only shrink.
//
//   - get_daily_context → MCP daily_context
//   - get_race_fueling  → merges MCP list_races + plan_race_fueling
//   - get_product       → fetch-by-id read (no single MCP equivalent name)
//   - update_product    → product PATCH (no single MCP equivalent name)
var chatBespokeTools = map[string]bool{
	"get_daily_context": true,
	"get_race_fueling":  true,
	"get_product":       true,
	"update_product":    true,
}

// TestSharedRegistry_NoDriftFromMCPSurface guards against the two AI surfaces
// silently diverging: every tool the shared agenttools registry exposes (which
// internal/chat consumes) must also be announced by the MCP server, except the
// documented chat-bespoke convenience tools. Until internal/mcpserver itself
// consumes agenttools (a future change), this name-level check is the anti-drift
// contract.
func TestSharedRegistry_NoDriftFromMCPSurface(t *testing.T) {
	announced := make(map[string]bool, len(AnnouncedToolNames))
	for _, n := range AnnouncedToolNames {
		announced[n] = true
	}

	for _, spec := range agenttools.Registry() {
		if chatBespokeTools[spec.Name] {
			continue
		}
		assert.Truef(t, announced[spec.Name],
			"shared tool %q is not in the MCP announced surface (AnnouncedToolNames); "+
				"either it moved/renamed in mcpserver or it is a new chat-bespoke tool "+
				"that should be added to chatBespokeTools", spec.Name)
	}
}
