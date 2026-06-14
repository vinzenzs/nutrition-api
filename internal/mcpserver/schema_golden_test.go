package mcpserver

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/agenttools"
)

// TestMCPSchemas_MatchGolden is the safety gate for the bespoke→registry port
// (unify-mcp-tool-registry, DD2). testdata/announced_schemas.json is a frozen
// snapshot of every tool's announced inputSchema, captured from the pre-port
// surface (see golden_capture_test.go). For each MCP-exposed registry entry,
// the schema reflected from its typed arg struct MUST equal the frozen one —
// so moving a tool onto the registry cannot silently change the contract the
// desktop coach relies on. As domains are ported, MCPRegistry() grows and each
// new tool is checked here.
func TestMCPSchemas_MatchGolden(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "announced_schemas.json"))
	require.NoError(t, err, "golden baseline missing; regenerate with -tags=goldengen")
	var golden map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &golden))

	specs := agenttools.MCPRegistry()
	require.NotEmpty(t, specs, "no MCP-exposed tools in the registry")

	for _, s := range specs {
		if s.Name == multipartPhotoTool {
			continue // registered bespoke (DD5); not yet a reflected entry
		}
		want, ok := golden[s.Name]
		require.Truef(t, ok, "tool %q has no golden schema — was the baseline captured before porting it?", s.Name)
		got, err := json.Marshal(mustReflectSchema(s))
		require.NoErrorf(t, err, "marshal reflected schema for %q", s.Name)
		assert.JSONEqf(t, string(want), string(got),
			"reflected schema for %q drifted from the announced baseline", s.Name)
	}
}
