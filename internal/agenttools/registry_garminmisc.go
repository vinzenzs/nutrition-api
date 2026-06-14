package agenttools

import (
	"encoding/json"
	"net/url"
)

// Garmin "catch-all mirror" reads — devices, health-vitals, achievements. Each
// mirrors a REST list endpoint 1:1 and forwards the body verbatim. Ported from
// internal/mcpserver (add-garmin-misc-mirror) onto the shared registry
// (unify-mcp-tool-registry). These are MCP-only (not chat-exposed) reads; the
// descriptions and arg structs are byte-identical to the prior bespoke
// registrations so the announced schema is unchanged.

func init() { registerMCPDomain(garminMiscSpecs()) }

// ListDevicesArgs is the (empty) input to devices_list.
type ListDevicesArgs struct{}

// ListHealthVitalsArgs is the input to health_vitals_list.
type ListHealthVitalsArgs struct {
	From string `json:"from" jsonschema:"inclusive lower bound date YYYY-MM-DD"`
	To   string `json:"to" jsonschema:"inclusive upper bound date YYYY-MM-DD; max 92 days from 'from'"`
}

// ListAchievementsArgs is the input to achievements_list.
type ListAchievementsArgs struct {
	Kind string `json:"kind,omitempty" jsonschema:"optional filter: badge | challenge"`
}

func garminMiscSpecs() []Spec {
	return []Spec{
		{
			Name: "devices_list",
			Description: "List the athlete's paired Garmin devices (watches, bike computers, scales) with model, last " +
				"sync time, battery, and firmware. Reference context — e.g. flagging a low-battery or stale-sync device.",
			SchemaType: ListDevicesArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				return HTTPCall{Method: "GET", Path: "/devices"}, nil
			},
		},
		{
			Name: "health_vitals_list",
			Description: "List daily health vitals (blood pressure, all-day resting/min/max heart rate, all-day stress) " +
				"in an inclusive [from, to] date window (YYYY-MM-DD, max 92 days). Distinct from recovery metrics.",
			SchemaType: ListHealthVitalsArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a ListHealthVitalsArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				q.Set("from", a.From)
				q.Set("to", a.To)
				return HTTPCall{Method: "GET", Path: "/health-vitals", Query: q}, nil
			},
		},
		{
			Name: "achievements_list",
			Description: "List the athlete's earned Garmin badges and ad-hoc challenges (most recent first), optionally " +
				"filtered by kind (badge|challenge). Coaching context — e.g. \"you just earned the 100-rides badge\".",
			SchemaType: ListAchievementsArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a ListAchievementsArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				if a.Kind != "" {
					q.Set("kind", a.Kind)
				}
				return HTTPCall{Method: "GET", Path: "/achievements", Query: q}, nil
			},
		},
	}
}
