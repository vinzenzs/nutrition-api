package mcpserver

import (
	"sort"

	"github.com/vinzenzs/nutrition-api/internal/agenttools"
)

// AnnouncedToolNames returns the exact set of tool names the MCP server
// announces, derived from the shared registry rather than hand-maintained
// (unify-mcp-tool-registry, DD6). It is every MCP-exposed tool in
// agenttools.Registry() plus the one tool registered outside the generic loop:
// the multipart photo upload (DD5). Because both the registration loop and this
// list read from the same registry, the announced surface and its checklist
// can no longer drift — the old name-level drift guard is retired.
func AnnouncedToolNames() []string {
	specs := agenttools.MCPRegistry()
	names := make([]string, 0, len(specs)+1)
	for _, s := range specs {
		names = append(names, s.Name)
	}
	names = append(names, multipartPhotoTool) // log_meal_from_photo, registered bespoke
	sort.Strings(names)
	return names
}
