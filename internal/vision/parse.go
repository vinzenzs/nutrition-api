package vision

import (
	"encoding/json"
	"fmt"
)

// anthropicEnvelope captures the subset of the Anthropic Messages API
// response we care about. Documented at
// https://docs.anthropic.com/en/api/messages — the response is a tagged-union
// across content block types; we only look at the `tool_use` block named
// `report_meal`.
type anthropicEnvelope struct {
	ID      string         `json:"id"`
	Model   string         `json:"model"`
	Role    string         `json:"role"`
	Content []contentBlock `json:"content"`
	Usage   *usage         `json:"usage,omitempty"`
}

type contentBlock struct {
	Type string `json:"type"`
	// Text blocks (fallback path; tool-forced output should produce a tool_use).
	Text string `json:"text,omitempty"`
	// Tool-use blocks.
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

type usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// extractReportMeal walks the response content blocks, finds the
// `tool_use` block named "report_meal", and unmarshals its `input` payload
// into a ParseResult. Returns the parsed result plus the model + token
// counts, or an error if the block is missing/malformed (caller decides
// whether to retry or fail).
func extractReportMeal(raw []byte) (*ParseResult, error) {
	var env anthropicEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("vision: decode envelope: %w", err)
	}
	for _, b := range env.Content {
		if b.Type != "tool_use" || b.Name != reportMealToolName {
			continue
		}
		var pr ParseResult
		if err := json.Unmarshal(b.Input, &pr); err != nil {
			return nil, fmt.Errorf("vision: unmarshal tool_use input: %w", err)
		}
		if pr.Name == "" {
			return nil, fmt.Errorf("vision: tool_use input missing name")
		}
		pr.Model = env.Model
		if env.Usage != nil {
			pr.InputTokens = env.Usage.InputTokens
			pr.OutputTokens = env.Usage.OutputTokens
		}
		return &pr, nil
	}
	return nil, fmt.Errorf("vision: no tool_use block named %q in response", reportMealToolName)
}
