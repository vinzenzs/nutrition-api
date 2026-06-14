package agenttools

import (
	"encoding/json"
	"net/url"
	"strconv"
)

// Nutrition summary reads — daily / range / rolling nutriment totals and the
// per-meal protein-distribution analysis. Ported from internal/mcpserver onto
// the shared registry (unify-mcp-tool-registry). These are MCP-only (not
// chat-exposed) reads; the descriptions and arg structs are byte-identical to
// the prior bespoke registrations so the announced schema is unchanged.
//
// NOTE: the file/func are named "nutritionSummary" to avoid colliding with the
// pre-existing Garmin daily-summary domain (registry_dailysummary.go,
// dailySummarySpecs / daily_summary_get). The tool NAMES here (daily_summary,
// range_summary, rolling_summary, protein_distribution) are unchanged.

func init() { registerMCPDomain(nutritionSummarySpecs()) }

// DailySummaryArgs is the input shape for `daily_summary`.
type DailySummaryArgs struct {
	Date     string `json:"date" jsonschema:"calendar date in YYYY-MM-DD"`
	TZ       string `json:"tz,omitempty" jsonschema:"IANA timezone (e.g. Europe/Berlin). If omitted, the REST server uses DEFAULT_USER_TZ."`
	MealType string `json:"meal_type,omitempty" jsonschema:"optional filter: breakfast | lunch | dinner | snack. When set, totals and entries are scoped to that meal type and adherence is omitted."`
}

// RangeSummaryArgs is the input shape for `range_summary`.
type RangeSummaryArgs struct {
	From    string `json:"from" jsonschema:"inclusive start date YYYY-MM-DD"`
	To      string `json:"to" jsonschema:"inclusive end date YYYY-MM-DD; max 92 days from 'from'"`
	TZ      string `json:"tz,omitempty" jsonschema:"IANA timezone (e.g. Europe/Berlin). If omitted, the REST server uses DEFAULT_USER_TZ."`
	GroupBy string `json:"group_by,omitempty" jsonschema:"optional: meal_type. When set, each day returns by_meal_type totals instead of a single totals object; adherence is omitted."`
}

// RollingSummaryArgs is the input shape for `rolling_summary`. The window is
// [anchor_date - (window_days - 1) days, anchor_date], both inclusive, in the
// requested `tz`.
type RollingSummaryArgs struct {
	AnchorDate string `json:"anchor_date" jsonschema:"calendar date in YYYY-MM-DD (the trailing window ends here, inclusive)"`
	WindowDays int    `json:"window_days" jsonschema:"window size in calendar days; range [2, 30]. Typical values: 3 (acute), 7 (weekly trend), 14 (training-block trend), 30 (block-length trend)."`
	TZ         string `json:"tz,omitempty" jsonschema:"IANA timezone (e.g. Europe/Berlin). If omitted, the REST server uses DEFAULT_USER_TZ."`
}

// ProteinDistributionArgs is the input shape for `protein_distribution`.
// Returns per-meal protein with `mps_effective` annotations against the
// 0.3 g/kg body-weight MPS threshold. Body weight is resolved from the stored
// log unless an explicit override is supplied.
type ProteinDistributionArgs struct {
	Date         string   `json:"date" jsonschema:"calendar date in YYYY-MM-DD"`
	TZ           string   `json:"tz,omitempty" jsonschema:"IANA timezone (e.g. Europe/Berlin). If omitted, the REST server uses DEFAULT_USER_TZ."`
	BodyWeightKg *float64 `json:"body_weight_kg,omitempty" jsonschema:"explicit body weight in kg; > 0. Wins over stored body-weight entries. Omit to use the rolling 7-day stored average (or the most-recent stored entry if none in the window)."`
}

func nutritionSummarySpecs() []Spec {
	return []Spec{
		{
			Name: "daily_summary",
			Description: "Get the user's nutriment totals and meal entries for a calendar date in the " +
				"supplied timezone. Returns kcal, protein/carbs/fat/fiber/sugar/salt grams plus each " +
				"meal's effective name and quantity. Omit tz to use the REST server's default timezone.",
			SchemaType: DailySummaryArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a DailySummaryArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				q.Set("date", a.Date)
				if a.TZ != "" {
					q.Set("tz", a.TZ)
				}
				if a.MealType != "" {
					q.Set("meal_type", a.MealType)
				}
				return HTTPCall{Method: "GET", Path: "/summary/daily", Query: q}, nil
			},
		},
		{
			Name: "range_summary",
			Description: "Get per-day nutriment totals across an inclusive date range (max 92 days). " +
				"Useful for 'how did I do this week?' style questions. Omit tz to use the REST server's " +
				"default timezone.",
			SchemaType: RangeSummaryArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a RangeSummaryArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				q.Set("from", a.From)
				q.Set("to", a.To)
				if a.TZ != "" {
					q.Set("tz", a.TZ)
				}
				if a.GroupBy != "" {
					q.Set("group_by", a.GroupBy)
				}
				return HTTPCall{Method: "GET", Path: "/summary/range", Query: q}, nil
			},
		},
		{
			Name: "protein_distribution",
			Description: "Return per-meal protein for one calendar date with `mps_effective: bool` flags " +
				"against the muscle-protein-synthesis (MPS) threshold of ~0.3 g protein per kg body weight " +
				"per meal. The headline metric is `mps_effective_meal_count / meal_count` — surface that " +
				"to the user when it's not 1.0. Each row also carries `gap_minutes_since_previous` (null on " +
				"the first meal) and `logged_at_hour` (local hour in the requested `tz`) so you can flag " +
				"meal-timing issues — the MPS sweet spot is 3–5h between protein doses; gaps under 3h aren't " +
				"independent triggers and gaps over 5h close the MPS window. Body-weight resolution order: " +
				"explicit `body_weight_kg` arg > rolling 7-day mean of stored weights ending at `date` " +
				"(inclusive) > most-recent stored entry strictly before `date`. With no stored data and no " +
				"override, returns `400 weight_data_missing`. Read-only; no idempotency-key.",
			SchemaType: ProteinDistributionArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a ProteinDistributionArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				q.Set("date", a.Date)
				if a.TZ != "" {
					q.Set("tz", a.TZ)
				}
				if a.BodyWeightKg != nil {
					q.Set("body_weight_kg", strconv.FormatFloat(*a.BodyWeightKg, 'f', -1, 64))
				}
				return HTTPCall{Method: "GET", Path: "/summary/protein-distribution", Query: q}, nil
			},
		},
		{
			Name: "rolling_summary",
			Description: "Get the trailing-window average of nutrition totals as of `anchor_date`. " +
				"The window is `[anchor_date - (window_days - 1) days, anchor_date]`, BOTH INCLUSIVE, " +
				"in the requested `tz` (omit to use DEFAULT_USER_TZ). IMPORTANT: averages are computed " +
				"across DAYS WITH LOGGED MEALS (`days_with_data`), NOT across `total_days` — a 7-day " +
				"window with 5 days logged returns the 5-day mean. The `days_with_data` and `total_days` " +
				"fields expose the divisor so you can spot sparse windows; surface that to the user when " +
				"they differ. Per-day rows carry `has_data: bool` distinguishing 'no meal logged' from " +
				"'logged a zero-kcal meal.' Typical windows: 3 (acute), 7 (weekly trend), 14 (training-" +
				"block trend), 30 (block-length trend). Adherence is computed against the goal that " +
				"applies AT `anchor_date` (honoring per-date overrides).",
			SchemaType: RollingSummaryArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a RollingSummaryArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				q.Set("anchor_date", a.AnchorDate)
				q.Set("window_days", strconv.Itoa(a.WindowDays))
				if a.TZ != "" {
					q.Set("tz", a.TZ)
				}
				return HTTPCall{Method: "GET", Path: "/summary/rolling", Query: q}, nil
			},
		},
	}
}
