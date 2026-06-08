package mcpserver

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// SetDailyGoalOverrideArgs reuses the same per-nutrient shape as SetGoalsArgs.
// `idempotency_key` is intentionally NOT exposed: the REST PUT rejects it, and
// the wrapper omits the header — matching the set_goals posture.
type SetDailyGoalOverrideArgs struct {
	Date string `json:"date" jsonschema:"calendar date for the override in YYYY-MM-DD"`

	Kcal          *GoalRange `json:"kcal,omitempty" jsonschema:"daily kilocalorie target as a range {min, max} for this date"`
	ProteinG      *GoalRange `json:"protein_g,omitempty" jsonschema:"protein grams per day (min/max) for this date"`
	CarbsG        *GoalRange `json:"carbs_g,omitempty" jsonschema:"carb grams per day (min/max) for this date"`
	FatG          *GoalRange `json:"fat_g,omitempty" jsonschema:"fat grams per day (min/max) for this date"`
	FiberG        *GoalRange `json:"fiber_g,omitempty" jsonschema:"fiber grams per day for this date"`
	SugarG        *GoalRange `json:"sugar_g,omitempty" jsonschema:"sugar grams per day for this date"`
	SaltG         *GoalRange `json:"salt_g,omitempty" jsonschema:"salt grams per day for this date"`
	IronMg        *GoalRange `json:"iron_mg,omitempty" jsonschema:"iron mg per day for this date"`
	CalciumMg     *GoalRange `json:"calcium_mg,omitempty" jsonschema:"calcium mg per day for this date"`
	VitaminDMcg   *GoalRange `json:"vitamin_d_mcg,omitempty" jsonschema:"vitamin D mcg per day for this date"`
	VitaminB12Mcg *GoalRange `json:"vitamin_b12_mcg,omitempty" jsonschema:"vitamin B12 mcg per day for this date"`
	VitaminCMg    *GoalRange `json:"vitamin_c_mg,omitempty" jsonschema:"vitamin C mg per day for this date"`
	MagnesiumMg   *GoalRange `json:"magnesium_mg,omitempty" jsonschema:"magnesium mg per day for this date"`
	PotassiumMg   *GoalRange `json:"potassium_mg,omitempty" jsonschema:"potassium mg per day for this date"`
	ZincMg        *GoalRange `json:"zinc_mg,omitempty" jsonschema:"zinc mg per day for this date"`
}

type GetDailyGoalOverrideArgs struct {
	Date string `json:"date" jsonschema:"calendar date in YYYY-MM-DD"`
}

type DeleteDailyGoalOverrideArgs struct {
	Date           string `json:"date" jsonschema:"calendar date in YYYY-MM-DD"`
	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

type ListDailyGoalOverridesArgs struct {
	From string `json:"from" jsonschema:"inclusive start date YYYY-MM-DD"`
	To   string `json:"to" jsonschema:"inclusive end date YYYY-MM-DD; max 366 days from from"`
}

func handleSetDailyGoalOverride(ctx context.Context, c *apiClient, args SetDailyGoalOverrideArgs) *mcp.CallToolResult {
	payload := struct {
		Kcal          *GoalRange `json:"kcal,omitempty"`
		ProteinG      *GoalRange `json:"protein_g,omitempty"`
		CarbsG        *GoalRange `json:"carbs_g,omitempty"`
		FatG          *GoalRange `json:"fat_g,omitempty"`
		FiberG        *GoalRange `json:"fiber_g,omitempty"`
		SugarG        *GoalRange `json:"sugar_g,omitempty"`
		SaltG         *GoalRange `json:"salt_g,omitempty"`
		IronMg        *GoalRange `json:"iron_mg,omitempty"`
		CalciumMg     *GoalRange `json:"calcium_mg,omitempty"`
		VitaminDMcg   *GoalRange `json:"vitamin_d_mcg,omitempty"`
		VitaminB12Mcg *GoalRange `json:"vitamin_b12_mcg,omitempty"`
		VitaminCMg    *GoalRange `json:"vitamin_c_mg,omitempty"`
		MagnesiumMg   *GoalRange `json:"magnesium_mg,omitempty"`
		PotassiumMg   *GoalRange `json:"potassium_mg,omitempty"`
		ZincMg        *GoalRange `json:"zinc_mg,omitempty"`
	}{
		Kcal:          args.Kcal,
		ProteinG:      args.ProteinG,
		CarbsG:        args.CarbsG,
		FatG:          args.FatG,
		FiberG:        args.FiberG,
		SugarG:        args.SugarG,
		SaltG:         args.SaltG,
		IronMg:        args.IronMg,
		CalciumMg:     args.CalciumMg,
		VitaminDMcg:   args.VitaminDMcg,
		VitaminB12Mcg: args.VitaminB12Mcg,
		VitaminCMg:    args.VitaminCMg,
		MagnesiumMg:   args.MagnesiumMg,
		PotassiumMg:   args.PotassiumMg,
		ZincMg:        args.ZincMg,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return toToolResult(0, nil, &transportError{inner: err})
	}
	// PUT /goals/overrides/{date} — no Idempotency-Key (same posture as set_goals).
	status, respBody, err := c.Put(ctx, "/goals/overrides/"+url.PathEscape(args.Date), body, "")
	return toToolResult(status, respBody, err)
}

func handleGetDailyGoalOverride(ctx context.Context, c *apiClient, args GetDailyGoalOverrideArgs) *mcp.CallToolResult {
	status, body, err := c.Get(ctx, "/goals/overrides/"+url.PathEscape(args.Date), nil)
	return toToolResult(status, body, err)
}

func handleDeleteDailyGoalOverride(ctx context.Context, c *apiClient, args DeleteDailyGoalOverrideArgs) *mcp.CallToolResult {
	key := effectiveIdempotencyKey(args.IdempotencyKey, "delete_daily_goal_override", args)
	status, respBody, err := c.Delete(ctx, "/goals/overrides/"+url.PathEscape(args.Date), key)
	if err == nil && status == 204 {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: ""}}}
	}
	return toToolResult(status, respBody, err)
}

func handleListDailyGoalOverrides(ctx context.Context, c *apiClient, args ListDailyGoalOverridesArgs) *mcp.CallToolResult {
	q := url.Values{}
	q.Set("from", args.From)
	q.Set("to", args.To)
	status, body, err := c.Get(ctx, "/goals/overrides", q)
	return toToolResult(status, body, err)
}

func registerGoalOverrideTools(server *mcp.Server, c *apiClient) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "set_daily_goal_override",
		Description: "Override the default daily goals for a specific calendar date — full-replace " +
			"semantics: the override completely replaces (not merges with) the default for that date. " +
			"Typical use cases: training days, rest days, race weeks. Same {min?, max?} range shape as " +
			"set_goals; omitted nutrient fields are stored as null (no override for that nutrient on " +
			"that date). Retries are NOT safe (same constraint as set_goals — the backend rejects " +
			"Idempotency-Key on PUT). Date format: YYYY-MM-DD.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args SetDailyGoalOverrideArgs) (*mcp.CallToolResult, any, error) {
		return handleSetDailyGoalOverride(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "get_daily_goal_override",
		Description: "Get the override stored for one specific date. Returns 404 override_not_found " +
			"when no override exists (the date uses the default goals). Useful for confirming what's " +
			"set before changing it.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args GetDailyGoalOverrideArgs) (*mcp.CallToolResult, any, error) {
		return handleGetDailyGoalOverride(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "delete_daily_goal_override",
		Description: "Remove the override for a date. Subsequent adherence on that date falls back " +
			"to the default goals (or to no adherence if no default is set).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args DeleteDailyGoalOverrideArgs) (*mcp.CallToolResult, any, error) {
		return handleDeleteDailyGoalOverride(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "list_daily_goal_overrides",
		Description: "Enumerate dates that have an explicit override in the [from, to] range " +
			"(inclusive, max 366 days). Useful for 'what's set for this week before I add more?' " +
			"Dates without an override are omitted from the response — the caller can infer they " +
			"use the default goals.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args ListDailyGoalOverridesArgs) (*mcp.CallToolResult, any, error) {
		return handleListDailyGoalOverrides(ctx, c, args), nil, nil
	})
}
