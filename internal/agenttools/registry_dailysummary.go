package agenttools

import (
	"encoding/json"
	"net/url"
)

// Garmin daily-summary read — Garmin's whole-day energy/activity snapshot for a
// single date. Ported from internal/mcpserver onto the shared registry
// (unify-mcp-tool-registry). MCP-only (not chat-exposed) read; the description
// and arg struct are byte-identical to the prior bespoke registration so the
// announced schema is unchanged.

func init() { registerMCPDomain(dailySummarySpecs()) }

// GetDailySummaryArgs is the input to daily_summary_get.
type GetDailySummaryArgs struct {
	Date string `json:"date" jsonschema:"the snapshot date YYYY-MM-DD"`
}

func dailySummarySpecs() []Spec {
	return []Spec{
		{
			Name: "daily_summary_get",
			Description: "Fetch Garmin's whole-day energy/activity snapshot for a single date (YYYY-MM-DD): " +
				"active vs resting vs total kcal, steps, floors, intensity minutes, distance. This is the " +
				"total-daily-expenditure context — including non-workout movement (NEAT) — that the " +
				"energy-availability number deliberately excludes. Read it alongside EA, never as a substitute " +
				"for EA's exercise-burn denominator.",
			SchemaType: GetDailySummaryArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a GetDailySummaryArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "GET", Path: "/daily-summary/" + url.PathEscape(a.Date)}, nil
			},
		},
	}
}
