package mcpserver

import (
	"context"
	"net/url"
	"strconv"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// WeeklyEnergySummaryArgs is the input shape for `weekly_energy_summary`.
//
// `from` and `to` are required RFC 3339 window bounds (max 92 days). All other
// fields are optional. `lean_mass_kg` and `body_fat_pct` are explicit
// composition overrides for the FFM resolution; pass them when the agent has
// trustworthy values that aren't in the database yet (e.g. a DEXA result).
type WeeklyEnergySummaryArgs struct {
	From       string   `json:"from" jsonschema:"inclusive RFC 3339 lower bound for the window"`
	To         string   `json:"to" jsonschema:"exclusive RFC 3339 upper bound; max 92 days from 'from'"`
	TZ         string   `json:"tz,omitempty" jsonschema:"IANA timezone for calendar-day boundaries (e.g. Europe/Berlin). If omitted, the REST server uses DEFAULT_USER_TZ."`
	LeanMassKg *float64 `json:"lean_mass_kg,omitempty" jsonschema:"explicit fat-free mass override in kg (highest-trust input); > 0. Wins over body_fat_pct and any stored composition data."`
	BodyFatPct *float64 `json:"body_fat_pct,omitempty" jsonschema:"explicit body-fat percentage override; 0 <= x < 100. Used with the stored body weight to derive FFM."`
}

func handleWeeklyEnergySummary(ctx context.Context, c *apiClient, args WeeklyEnergySummaryArgs) *mcp.CallToolResult {
	q := url.Values{}
	q.Set("from", args.From)
	q.Set("to", args.To)
	if args.TZ != "" {
		q.Set("tz", args.TZ)
	}
	if args.LeanMassKg != nil {
		q.Set("lean_mass_kg", strconv.FormatFloat(*args.LeanMassKg, 'f', -1, 64))
	}
	if args.BodyFatPct != nil {
		q.Set("body_fat_pct", strconv.FormatFloat(*args.BodyFatPct, 'f', -1, 64))
	}
	status, body, err := c.Get(ctx, "/energy/availability", q)
	return toToolResult(status, body, err)
}

func registerEnergyTools(server *mcp.Server, c *apiClient) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "weekly_energy_summary",
		Description: "Compute Energy Availability (EA) over a date window from intake (meals), exercise " +
			"burn (workouts.kcal_burned), and body composition (body_weight_entries). Returns per-day EA " +
			"values + window average classified into Loucks bands: `< 30 kcal/kg FFM/day` is LOW (real " +
			"physiological risk), `30..45` is SUB-OPTIMAL, `>= 45` is ADEQUATE. " +
			"FFM resolution order: `lean_mass_kg` arg > `body_fat_pct` arg + stored weight > most-recent " +
			"stored body_fat_pct + stored weight > body_weight × 0.85 (loud fallback, flagged via " +
			"`composition.composition_estimated: true`). Days with workouts missing `kcal_burned` " +
			"appear in `missing_burn_workout_ids` and are EXCLUDED from `window.avg_ea` — silently zeroing " +
			"would make low-data days look healthier than they are. Read-only; no idempotency-key is sent.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args WeeklyEnergySummaryArgs) (*mcp.CallToolResult, any, error) {
		return handleWeeklyEnergySummary(ctx, c, args), nil, nil
	})
}
