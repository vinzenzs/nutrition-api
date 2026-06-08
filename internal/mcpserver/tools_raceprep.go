package mcpserver

import (
	"context"
	"net/url"
	"strconv"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// PlanCarbLoadArgs reflects the GET /race-prep/carb-load query parameters.
// Optional fields are pointers so the wrapper can omit them from the URL when
// the agent did not supply them — letting the REST defaults apply.
type PlanCarbLoadArgs struct {
	RaceDate          string   `json:"race_date" jsonschema:"race date in YYYY-MM-DD (must be today or later in the user's timezone)"`
	BodyWeightKg      float64  `json:"body_weight_kg" jsonschema:"athlete body weight in kilograms, 30..200"`
	DaysBefore        *int     `json:"days_before,omitempty" jsonschema:"carb-load days before race day, 0..7 (default 3). Sprint tri / short races: 1-2. 70.3: 3. Ironman: 3-4."`
	CarbsPerKgPerDay  *float64 `json:"carbs_per_kg_per_day,omitempty" jsonschema:"load-day multiplier, 1..20 g/kg (default 10, mid-range of the documented 8-12 g/kg; lower for athletes with GI sensitivity)"`
	RaceDayCarbsPerKg *float64 `json:"race_day_carbs_per_kg,omitempty" jsonschema:"race-morning multiplier, 0..10 g/kg (default 2)"`
}

func handlePlanCarbLoad(ctx context.Context, c *apiClient, args PlanCarbLoadArgs) *mcp.CallToolResult {
	q := url.Values{}
	q.Set("race_date", args.RaceDate)
	q.Set("body_weight_kg", strconv.FormatFloat(args.BodyWeightKg, 'f', -1, 64))
	if args.DaysBefore != nil {
		q.Set("days_before", strconv.Itoa(*args.DaysBefore))
	}
	if args.CarbsPerKgPerDay != nil {
		q.Set("carbs_per_kg_per_day", strconv.FormatFloat(*args.CarbsPerKgPerDay, 'f', -1, 64))
	}
	if args.RaceDayCarbsPerKg != nil {
		q.Set("race_day_carbs_per_kg", strconv.FormatFloat(*args.RaceDayCarbsPerKg, 'f', -1, 64))
	}
	status, body, err := c.Get(ctx, "/race-prep/carb-load", q)
	return toToolResult(status, body, err)
}

func registerRacePrepTools(server *mcp.Server, c *apiClient) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "plan_carb_load",
		Description: "Compute the carb-load schedule for a race. Returns a daily schedule of carb " +
			"targets in grams: 'days_before' load days plus race day. The natural follow-up is to " +
			"translate each entry into a goal override via set_daily_goal_override, so adherence on " +
			"those days reflects the carb-load target. For sprint tri / short races, consider " +
			"days_before=1 or 2 (carb-load benefit plateaus). For 70.3 use the default 3. For Ironman " +
			"consider 3-4 days. The carbs_per_kg_per_day default of 10 sits in the middle of the " +
			"documented 8-12 g/kg range; lower for athletes who handle GI distress.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args PlanCarbLoadArgs) (*mcp.CallToolResult, any, error) {
		return handlePlanCarbLoad(ctx, c, args), nil, nil
	})
}
