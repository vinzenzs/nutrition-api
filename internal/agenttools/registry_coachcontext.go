package agenttools

import (
	"encoding/json"
	"net/url"
	"strconv"
)

// Coach-context aggregate reads — single-call bundles the desktop MCP coach
// reads before giving training/recovery advice. Ported from internal/mcpserver
// onto the shared registry (unify-mcp-tool-registry). These are MCP-only reads;
// the descriptions and arg structs are byte-identical to the prior bespoke
// registrations so the announced schema is unchanged.
//
// Note: get_training_context shares a NAME with a chat-surface coach read, but
// this is a SEPARATE MCP Spec (the two surfaces filter independently) — it is
// byte-faithful to the bespoke MCP handler in tools_coachcontext.go.

func init() { registerMCPDomain(coachContextSpecs()) }

// TrainingContextArgs is the input for the get_training_context tool. Read-only.
type TrainingContextArgs struct {
	Date          string `json:"date,omitempty" jsonschema:"calendar date YYYY-MM-DD (defaults to today)"`
	TZ            string `json:"tz,omitempty" jsonschema:"IANA timezone (defaults to DEFAULT_USER_TZ)"`
	LookbackDays  int    `json:"lookback_days,omitempty" jsonschema:"completed-workout/fitness lookback window (default 14, max 90)"`
	LookaheadDays int    `json:"lookahead_days,omitempty" jsonschema:"planned-workout lookahead window (default 7, max 60)"`
}

// RecoveryContextArgs is the input for the get_recovery_context tool. Read-only.
type RecoveryContextArgs struct {
	Date string `json:"date,omitempty" jsonschema:"calendar date YYYY-MM-DD (defaults to today)"`
	TZ   string `json:"tz,omitempty" jsonschema:"IANA timezone (defaults to DEFAULT_USER_TZ)"`
	Days int    `json:"days,omitempty" jsonschema:"trend window in days (default 7, max 90)"`
}

func coachContextSpecs() []Spec {
	return []Spec{
		{
			Name: "get_training_context",
			Description: "Get the training context bundle in one call: the covering training phase, the latest fitness " +
				"snapshot (VO2max, acute/chronic load, training status, race predictors) with derived ACWR, a recent-load " +
				"summary plus recent completed workouts (lookback_days, default 14), and upcoming planned workouts " +
				"(lookahead_days, default 7). Recommended as the FIRST call before giving training advice — collapses many " +
				"granular reads (list_workouts, list_fitness_metrics, list_phases). For per-entry detail use the dedicated " +
				"tools. Read-only.",
			SchemaType: TrainingContextArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a TrainingContextArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				if a.Date != "" {
					q.Set("date", a.Date)
				}
				if a.TZ != "" {
					q.Set("tz", a.TZ)
				}
				if a.LookbackDays > 0 {
					q.Set("lookback_days", strconv.Itoa(a.LookbackDays))
				}
				if a.LookaheadDays > 0 {
					q.Set("lookahead_days", strconv.Itoa(a.LookaheadDays))
				}
				return HTTPCall{Method: "GET", Path: "/context/training", Query: q}, nil
			},
		},
		{
			Name: "get_recovery_context",
			Description: "Get the recovery context bundle in one call: the latest recovery snapshot on/before the date " +
				"(sleep, HRV, resting HR, body battery, training readiness, …) plus the recent trend over `days` " +
				"(default 7). Recommended before advising on a hard session or rest day. For per-day detail use " +
				"list_recovery_metrics. Read-only.",
			SchemaType: RecoveryContextArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a RecoveryContextArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				if a.Date != "" {
					q.Set("date", a.Date)
				}
				if a.TZ != "" {
					q.Set("tz", a.TZ)
				}
				if a.Days > 0 {
					q.Set("days", strconv.Itoa(a.Days))
				}
				return HTTPCall{Method: "GET", Path: "/context/recovery", Query: q}, nil
			},
		},
	}
}
