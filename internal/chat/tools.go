package chat

import (
	"encoding/json"

	"github.com/vinzenzs/kazper/internal/agenttools"
)

// cookidooSearchDomains restricts the web_search server tool to Cookidoo hosts
// so the agent can only discover importable recipes, not arbitrary pages.
var cookidooSearchDomains = []string{
	"cookidoo.de", "cookidoo.com", "cookidoo.at", "cookidoo.ch",
	"cookidoo.co.uk", "cookidoo.fr", "cookidoo.es", "cookidoo.it",
	"cookidoo.nl", "cookidoo.be", "cookidoo.com.au", "cookidoo.pl",
}

// anthropicToolDefs renders the shared registry plus the web_search server tool
// into the Messages API `tools` array.
func anthropicToolDefs(specs []agenttools.Spec) []json.RawMessage {
	out := make([]json.RawMessage, 0, len(specs)+1)
	for _, s := range specs {
		def := map[string]any{
			"name":         s.Name,
			"description":  s.Description,
			"input_schema": json.RawMessage(s.Schema),
		}
		raw, _ := json.Marshal(def)
		out = append(out, raw)
	}
	// web_search server tool, domain-restricted to Cookidoo.
	ws := map[string]any{
		"type":            "web_search_20250305",
		"name":            "web_search",
		"allowed_domains": cookidooSearchDomains,
		"max_uses":        5,
	}
	raw, _ := json.Marshal(ws)
	out = append(out, raw)
	return out
}
