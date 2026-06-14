package agenttools

import (
	"encoding/json"
	"net/url"
)

// Daily-context aggregate read — the day's full context bundle in one call.
// Ported from internal/mcpserver onto the shared registry
// (unify-mcp-tool-registry). MCP-only read; the description and arg struct are
// byte-identical to the prior bespoke registration so the announced schema is
// unchanged.

func init() { registerMCPDomain(dailyContextSpecs()) }

// DailyContextArgs is the input shape for the daily_context tool. The
// wrapper is read-only — no idempotency_key field.
type DailyContextArgs struct {
	Date string `json:"date" jsonschema:"calendar date in YYYY-MM-DD"`
	TZ   string `json:"tz,omitempty" jsonschema:"IANA timezone (defaults to DEFAULT_USER_TZ)"`
}

func dailyContextSpecs() []Spec {
	return []Spec{
		{
			Name: "daily_context",
			Description: "Get the day's full context bundle in one call: adherence + nutrition totals + hydration ml + " +
				"today's workouts + workout-fuel entries + body-weight state (with carryover from the most recent prior " +
				"entry when no fresh log) + training-phase context + goal-override presence. Recommended as the FIRST " +
				"call of a session — collapses what would otherwise be 5-7 separate tool calls (daily_summary, " +
				"daily_hydration_summary, list_workouts, list_workout_fuel, list_weights, get_daily_goal_override, " +
				"list_phases). For deep dives into one slice — per-entry breakdowns, full meal lists, range queries — " +
				"use the dedicated tools; they include the per-entry detail this aggregator deliberately omits. " +
				"Read-only.",
			SchemaType: DailyContextArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a DailyContextArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				q.Set("date", a.Date)
				if a.TZ != "" {
					q.Set("tz", a.TZ)
				}
				return HTTPCall{Method: "GET", Path: "/context/daily", Query: q}, nil
			},
		},
	}
}
