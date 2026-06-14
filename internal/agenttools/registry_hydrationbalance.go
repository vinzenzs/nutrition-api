package agenttools

import (
	"encoding/json"
	"net/url"
)

// Hydration-balance daily water-balance snapshots — one snapshot per calendar
// day (sweat loss, activity intake, goal, all millilitres) keyed by date.
// Ported from internal/mcpserver onto the shared registry
// (unify-mcp-tool-registry). The arg structs and descriptions are byte-identical
// to the prior bespoke registrations so the announced schema and REST mapping
// are unchanged.

func init() { registerMCPDomain(hydrationBalanceSpecs()) }

// LogHydrationBalanceArgs is the input to log_hydration_balance.
type LogHydrationBalanceArgs struct {
	Date             string   `json:"date" jsonschema:"the calendar day YYYY-MM-DD (the snapshot identity; re-posting the same date updates in place)"`
	SweatLossML      *float64 `json:"sweat_loss_ml,omitempty" jsonschema:"estimated daily sweat loss in millilitres (> 0)"`
	ActivityIntakeML *float64 `json:"activity_intake_ml,omitempty" jsonschema:"fluid taken during activities that day in millilitres (>= 0; a real 0 means sweated but drank nothing)"`
	GoalML           *float64 `json:"goal_ml,omitempty" jsonschema:"daily hydration goal in millilitres (> 0)"`
	IdempotencyKey   string   `json:"idempotency_key,omitempty" jsonschema:"optional retry key; if omitted, a stable key is derived from the other args"`
}

// ListHydrationBalanceArgs is the input to list_hydration_balance.
type ListHydrationBalanceArgs struct {
	From string `json:"from" jsonschema:"inclusive lower bound date YYYY-MM-DD"`
	To   string `json:"to" jsonschema:"inclusive upper bound date YYYY-MM-DD; max 92 days from 'from'"`
}

// GetHydrationBalanceArgs is the input to get_hydration_balance.
type GetHydrationBalanceArgs struct {
	Date string `json:"date" jsonschema:"the snapshot date YYYY-MM-DD"`
}

// DeleteHydrationBalanceArgs is the input to delete_hydration_balance.
type DeleteHydrationBalanceArgs struct {
	Date           string `json:"date" jsonschema:"the snapshot date YYYY-MM-DD to delete"`
	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

func hydrationBalanceSpecs() []Spec {
	return []Spec{
		{
			Name: "log_hydration_balance",
			Description: "Record (or replace) the daily water-balance snapshot for a date — estimated sweat " +
				"loss, fluid taken during activity, daily hydration goal (all millilitres). One snapshot per " +
				"calendar day, keyed by `date`; re-posting overwrites it. DISTINCT from log_hydration (per-entry " +
				"logged intake): this is a device's daily estimate. Compute the balance against logged intake " +
				"from the daily hydration summary.",
			SchemaType: LogHydrationBalanceArgs{},
			Tier:       TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a LogHydrationBalanceArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				body, err := json.Marshal(struct {
					Date             string   `json:"date"`
					SweatLossML      *float64 `json:"sweat_loss_ml,omitempty"`
					ActivityIntakeML *float64 `json:"activity_intake_ml,omitempty"`
					GoalML           *float64 `json:"goal_ml,omitempty"`
				}{
					Date:             a.Date,
					SweatLossML:      a.SweatLossML,
					ActivityIntakeML: a.ActivityIntakeML,
					GoalML:           a.GoalML,
				})
				if err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "POST", Path: "/hydration-balance", Body: body}, nil
			},
		},
		{
			Name: "list_hydration_balance",
			Description: "List daily hydration-balance snapshots whose date falls in the inclusive [from, to] " +
				"window (YYYY-MM-DD, max 92 days). Use for 'did my fluid intake keep up with sweat loss this week?'.",
			SchemaType: ListHydrationBalanceArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a ListHydrationBalanceArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				q.Set("from", a.From)
				q.Set("to", a.To)
				return HTTPCall{Method: "GET", Path: "/hydration-balance", Query: q}, nil
			},
		},
		{
			Name:        "get_hydration_balance",
			Description: "Fetch the hydration-balance snapshot for a single date (YYYY-MM-DD).",
			SchemaType:  GetHydrationBalanceArgs{},
			Tier:        TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a GetHydrationBalanceArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "GET", Path: "/hydration-balance/" + url.PathEscape(a.Date)}, nil
			},
		},
		{
			Name:        "delete_hydration_balance",
			Description: "Delete the hydration-balance snapshot for a date. Returns an empty result on success.",
			SchemaType:  DeleteHydrationBalanceArgs{},
			Tier:        TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a DeleteHydrationBalanceArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "DELETE", Path: "/hydration-balance/" + url.PathEscape(a.Date)}, nil
			},
		},
	}
}
