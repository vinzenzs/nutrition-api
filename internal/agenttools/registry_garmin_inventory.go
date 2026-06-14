package agenttools

import (
	"encoding/json"
	"net/url"
	"strconv"
)

// Garmin inventory reads — singleton/list reference data the desktop coach
// reads for context. Ported from internal/mcpserver onto the shared registry
// (unify-mcp-tool-registry, pilot domain). These are MCP-only (not chat-exposed)
// reads; the descriptions and arg structs are byte-identical to the prior
// bespoke registrations so the announced schema is unchanged.

func init() { registerMCPDomain(garminInventorySpecs()) }

// ListGearArgs is the input to gear_list.
type ListGearArgs struct {
	Retired *bool `json:"retired,omitempty" jsonschema:"optional filter by retirement state (true returns only retired gear, false only active)"`
}

// GetAthleteConfigArgs is the (empty) input to athlete_config_get.
type GetAthleteConfigArgs struct{}

// ListPersonalRecordsArgs is the input to personal_records_list.
type ListPersonalRecordsArgs struct {
	PRType string `json:"pr_type,omitempty" jsonschema:"optional filter to a single PR type (e.g. 5k, 10k, longest-ride)"`
}

func garminInventorySpecs() []Spec {
	return []Spec{
		{
			Name: "gear_list",
			Description: "List the athlete's Garmin gear inventory (shoes, bikes, other equipment) with " +
				"accumulated distance, activity count, and retirement state. Use for gear-rotation context — " +
				"e.g. flagging shoes that are near or past their mileage budget. Optional `retired` filter.",
			SchemaType: ListGearArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a ListGearArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				if a.Retired != nil {
					q.Set("retired", strconv.FormatBool(*a.Retired))
				}
				return HTTPCall{Method: "GET", Path: "/gear", Query: q}, nil
			},
		},
		{
			Name: "athlete_config_get",
			Description: "Fetch the athlete's physiology configuration (singleton): FTP, threshold HR and " +
				"run/swim paces, max HR, lactate-threshold HR, and HR-zone (and optional power-zone) " +
				"boundaries. Returns null before any config has been set. Use to interpret workout " +
				"detail — e.g. to know what heart rate a zone-4 second actually corresponds to.",
			SchemaType: GetAthleteConfigArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				return HTTPCall{Method: "GET", Path: "/athlete-config"}, nil
			},
		},
		{
			Name: "personal_records_list",
			Description: "List the athlete's Garmin personal records (fastest 5k/10k, longest ride, …) with " +
				"value, unit, and when each was achieved, most recent first. Use for PR-freshness coaching " +
				"context — e.g. framing race-prep advice around how sharp the athlete's top-end is. Optional " +
				"`pr_type` filter.",
			SchemaType: ListPersonalRecordsArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a ListPersonalRecordsArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				if a.PRType != "" {
					q.Set("pr_type", a.PRType)
				}
				return HTTPCall{Method: "GET", Path: "/personal-records", Query: q}, nil
			},
		},
	}
}
