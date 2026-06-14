package agenttools

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
)

// ToolUseBlock is one tool_use content block parsed from an assistant turn's
// content. It is the wire shape Anthropic persists and resumes verbatim.
type ToolUseBlock struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// ProposalCall is one pending write-confirm call surfaced to the client — in the
// `proposal` SSE event (live) and in `pending_confirmation` on session detail
// (cold-open). Both render identically because both come from this package.
type ProposalCall struct {
	ToolID  string `json:"tool_id"`
	Name    string `json:"name"`
	Tier    Tier   `json:"tier"`
	Preview string `json:"preview"`
}

// ParseToolUseBlocks extracts the tool_use blocks (in order) from an assistant
// turn's content. Plain-string content (user text) yields none.
func ParseToolUseBlocks(assistantContent json.RawMessage) []ToolUseBlock {
	var blocks []struct {
		Type  string          `json:"type"`
		ID    string          `json:"id"`
		Name  string          `json:"name"`
		Input json.RawMessage `json:"input"`
	}
	if err := json.Unmarshal(assistantContent, &blocks); err != nil {
		return nil
	}
	out := make([]ToolUseBlock, 0, len(blocks))
	for _, b := range blocks {
		if b.Type == "tool_use" {
			out = append(out, ToolUseBlock{ID: b.ID, Name: b.Name, Input: b.Input})
		}
	}
	return out
}

// AwaitingConfirmation reports whether an assistant turn's content contains at
// least one write-confirm tool_use block — i.e. the loop paused on it and it is
// the resume anchor for a confirmation, not truncation litter. Tools absent from
// specs are treated as non-confirm (conservative: never wedge on an unknown).
func AwaitingConfirmation(assistantContent json.RawMessage, specs map[string]Spec) bool {
	for _, b := range ParseToolUseBlocks(assistantContent) {
		if specs[b.Name].Tier == TierWriteConfirm {
			return true
		}
	}
	return false
}

// PendingFromContent returns the write-confirm calls (with composed previews)
// awaiting a decision in an assistant turn, or nil if the turn has none. Only
// write-confirm calls appear — the read/write-auto calls in the same turn
// execute on resume without a decision.
func PendingFromContent(assistantContent json.RawMessage, specs map[string]Spec) []ProposalCall {
	var out []ProposalCall
	for _, b := range ParseToolUseBlocks(assistantContent) {
		spec, ok := specs[b.Name]
		if !ok || spec.Tier != TierWriteConfirm {
			continue
		}
		out = append(out, ProposalCall{
			ToolID:  b.ID,
			Name:    b.Name,
			Tier:    spec.Tier,
			Preview: PreviewLine(spec, b.Input),
		})
	}
	return out
}

// TurnID derives a stable identifier for an assistant turn from its tool_use
// block ids, so the live `proposal` event and the cold-open session detail label
// the same paused turn identically. It is informational — the confirm endpoint
// targets the session's trailing turn, keyed by per-call tool_id.
func TurnID(assistantContent json.RawMessage) string {
	blocks := ParseToolUseBlocks(assistantContent)
	ids := make([]string, 0, len(blocks))
	for _, b := range blocks {
		ids = append(ids, b.ID)
	}
	h := sha256.Sum256([]byte(strings.Join(ids, "|")))
	return "turn_" + hex.EncodeToString(h[:])[:16]
}

// PreviewLine composes the confirmation preview for a write-confirm call: the
// tool's own Format formatter when present, otherwise a generic verb/resource
// line derived from the tool name. It never surfaces model-supplied prose.
func PreviewLine(spec Spec, input json.RawMessage) string {
	if spec.Format != nil {
		if s := strings.TrimSpace(spec.Format(input)); s != "" {
			return s
		}
	}
	return genericPreview(spec.Name)
}

// genericPreview humanizes a snake_case tool name into a "Verb resource" line,
// e.g. "delete_planned_meal" → "Delete planned meal".
func genericPreview(name string) string {
	if name == "" {
		return "Apply change"
	}
	words := strings.Split(name, "_")
	for i, w := range words {
		if i == 0 && w != "" {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}
