package mcpserver

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// PhaseRange mirrors goals.GoalRange but lives here to keep the MCP-input
// struct free of cross-package imports beyond what the wrapper needs.

// ----------------------------------------------------------------------
// Phases: 5 tools (create / list / get / patch / delete)
// ----------------------------------------------------------------------

type CreatePhaseArgs struct {
	Name              string  `json:"name" jsonschema:"phase name (user-chosen, e.g. 'build-block-2')"`
	Type              string  `json:"type" jsonschema:"one of: base, build, peak, recovery, race_week, off_season, other"`
	StartDate         string  `json:"start_date" jsonschema:"inclusive start date in YYYY-MM-DD"`
	EndDate           string  `json:"end_date" jsonschema:"inclusive end date in YYYY-MM-DD (must be >= start_date)"`
	DefaultTemplateID *string `json:"default_template_id,omitempty" jsonschema:"optional goal-template UUID; when set, the template's bounds drive adherence on every date in the phase (subject to per-date overrides winning)"`
	Notes             *string `json:"notes,omitempty" jsonschema:"optional free-text notes"`
	IdempotencyKey    string  `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

func handleCreatePhase(ctx context.Context, c *apiClient, args CreatePhaseArgs) *mcp.CallToolResult {
	payload := struct {
		Name              string  `json:"name"`
		Type              string  `json:"type"`
		StartDate         string  `json:"start_date"`
		EndDate           string  `json:"end_date"`
		DefaultTemplateID *string `json:"default_template_id,omitempty"`
		Notes             *string `json:"notes,omitempty"`
	}{
		Name:              args.Name,
		Type:              args.Type,
		StartDate:         args.StartDate,
		EndDate:           args.EndDate,
		DefaultTemplateID: args.DefaultTemplateID,
		Notes:             args.Notes,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return toToolResult(0, nil, &transportError{inner: err})
	}
	key := effectiveIdempotencyKey(args.IdempotencyKey, "create_phase", args)
	status, respBody, err := c.Post(ctx, "/phases", nil, body, key)
	return toToolResult(status, respBody, err)
}

type ListPhasesArgs struct {
	From string `json:"from" jsonschema:"inclusive start date YYYY-MM-DD"`
	To   string `json:"to" jsonschema:"inclusive end date YYYY-MM-DD (max 730 days from from)"`
}

func handleListPhases(ctx context.Context, c *apiClient, args ListPhasesArgs) *mcp.CallToolResult {
	q := url.Values{}
	q.Set("from", args.From)
	q.Set("to", args.To)
	status, body, err := c.Get(ctx, "/phases", q)
	return toToolResult(status, body, err)
}

type GetPhaseArgs struct {
	PhaseID string `json:"phase_id" jsonschema:"phase UUID"`
}

func handleGetPhase(ctx context.Context, c *apiClient, args GetPhaseArgs) *mcp.CallToolResult {
	status, body, err := c.Get(ctx, "/phases/"+url.PathEscape(args.PhaseID), nil)
	return toToolResult(status, body, err)
}

type UpdatePhaseArgs struct {
	PhaseID           string  `json:"phase_id" jsonschema:"phase UUID"`
	Name              *string `json:"name,omitempty"`
	Type              *string `json:"type,omitempty" jsonschema:"one of: base, build, peak, recovery, race_week, off_season, other"`
	StartDate         *string `json:"start_date,omitempty"`
	EndDate           *string `json:"end_date,omitempty"`
	DefaultTemplateID *string `json:"default_template_id,omitempty" jsonschema:"empty string clears, UUID string sets, missing leaves unchanged"`
	Notes             *string `json:"notes,omitempty"`
	IdempotencyKey    string  `json:"idempotency_key,omitempty"`
}

func handleUpdatePhase(ctx context.Context, c *apiClient, args UpdatePhaseArgs) *mcp.CallToolResult {
	payload := struct {
		Name              *string `json:"name,omitempty"`
		Type              *string `json:"type,omitempty"`
		StartDate         *string `json:"start_date,omitempty"`
		EndDate           *string `json:"end_date,omitempty"`
		DefaultTemplateID *string `json:"default_template_id,omitempty"`
		Notes             *string `json:"notes,omitempty"`
	}{
		Name:              args.Name,
		Type:              args.Type,
		StartDate:         args.StartDate,
		EndDate:           args.EndDate,
		DefaultTemplateID: args.DefaultTemplateID,
		Notes:             args.Notes,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return toToolResult(0, nil, &transportError{inner: err})
	}
	key := effectiveIdempotencyKey(args.IdempotencyKey, "update_phase", args)
	status, respBody, err := c.Patch(ctx, "/phases/"+url.PathEscape(args.PhaseID), body, key)
	return toToolResult(status, respBody, err)
}

type DeletePhaseArgs struct {
	PhaseID        string `json:"phase_id" jsonschema:"phase UUID"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

func handleDeletePhase(ctx context.Context, c *apiClient, args DeletePhaseArgs) *mcp.CallToolResult {
	key := effectiveIdempotencyKey(args.IdempotencyKey, "delete_phase", args)
	status, respBody, err := c.Delete(ctx, "/phases/"+url.PathEscape(args.PhaseID), key)
	if err == nil && status == 204 {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: ""}}}
	}
	return toToolResult(status, respBody, err)
}

// ----------------------------------------------------------------------
// Goal templates: 4 tools (set / list / get / delete)
// ----------------------------------------------------------------------

type SetGoalTemplateArgs struct {
	Name  string  `json:"name" jsonschema:"template name (user-chosen, kebab-case-ish, e.g. 'weekday-easy-training')"`
	Notes *string `json:"notes,omitempty" jsonschema:"optional free-text notes"`

	Kcal *GoalRange `json:"kcal,omitempty"`

	ProteinG *GoalRange `json:"protein_g,omitempty"`
	CarbsG   *GoalRange `json:"carbs_g,omitempty"`
	FatG     *GoalRange `json:"fat_g,omitempty"`

	FiberG *GoalRange `json:"fiber_g,omitempty"`
	SugarG *GoalRange `json:"sugar_g,omitempty"`
	SaltG  *GoalRange `json:"salt_g,omitempty"`

	IronMg        *GoalRange `json:"iron_mg,omitempty"`
	CalciumMg     *GoalRange `json:"calcium_mg,omitempty"`
	VitaminDMcg   *GoalRange `json:"vitamin_d_mcg,omitempty"`
	VitaminB12Mcg *GoalRange `json:"vitamin_b12_mcg,omitempty"`
	VitaminCMg    *GoalRange `json:"vitamin_c_mg,omitempty"`
	MagnesiumMg   *GoalRange `json:"magnesium_mg,omitempty"`
	PotassiumMg   *GoalRange `json:"potassium_mg,omitempty"`
	ZincMg        *GoalRange `json:"zinc_mg,omitempty"`
}

func handleSetGoalTemplate(ctx context.Context, c *apiClient, args SetGoalTemplateArgs) *mcp.CallToolResult {
	payload := struct {
		Notes         *string    `json:"notes,omitempty"`
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
		Notes:         args.Notes,
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
	// PUT — no Idempotency-Key (the backend rejects it on PUT).
	status, respBody, err := c.Put(ctx, "/goal-templates/"+url.PathEscape(args.Name), body, "")
	return toToolResult(status, respBody, err)
}

type ListGoalTemplatesArgs struct{}

func handleListGoalTemplates(ctx context.Context, c *apiClient, _ ListGoalTemplatesArgs) *mcp.CallToolResult {
	status, body, err := c.Get(ctx, "/goal-templates", nil)
	return toToolResult(status, body, err)
}

type GetGoalTemplateArgs struct {
	Name string `json:"name" jsonschema:"template name"`
}

func handleGetGoalTemplate(ctx context.Context, c *apiClient, args GetGoalTemplateArgs) *mcp.CallToolResult {
	status, body, err := c.Get(ctx, "/goal-templates/"+url.PathEscape(args.Name), nil)
	return toToolResult(status, body, err)
}

type DeleteGoalTemplateArgs struct {
	Name           string `json:"name" jsonschema:"template name"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

func handleDeleteGoalTemplate(ctx context.Context, c *apiClient, args DeleteGoalTemplateArgs) *mcp.CallToolResult {
	key := effectiveIdempotencyKey(args.IdempotencyKey, "delete_goal_template", args)
	status, respBody, err := c.Delete(ctx, "/goal-templates/"+url.PathEscape(args.Name), key)
	if err == nil && status == 204 {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: ""}}}
	}
	return toToolResult(status, respBody, err)
}

// ----------------------------------------------------------------------
// Registration
// ----------------------------------------------------------------------

func registerTrainingPhasesTools(server *mcp.Server, c *apiClient) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "create_phase",
		Description: "Create a training phase: a named date range tagged with type (base, build, peak, recovery, race_week, off_season, other). " +
			"Optionally points at a goal template via default_template_id — when set, the template's bounds become the default daily goals for every date in [start_date, end_date]. " +
			"Per-date overrides (set_daily_goal_override) still win over the phase's template. " +
			"Omitting default_template_id creates a phase visible in list_phases that does NOT drive adherence — useful for tagging a date range with a type without committing to a template yet.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args CreatePhaseArgs) (*mcp.CallToolResult, any, error) {
		return handleCreatePhase(ctx, c, args), nil, nil
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_phases",
		Description: "List training phases intersecting [from, to] (both inclusive YYYY-MM-DD; max 730 days). Each result includes default_template_name when a template is attached.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args ListPhasesArgs) (*mcp.CallToolResult, any, error) {
		return handleListPhases(ctx, c, args), nil, nil
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_phase",
		Description: "Fetch a single phase by UUID.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args GetPhaseArgs) (*mcp.CallToolResult, any, error) {
		return handleGetPhase(ctx, c, args), nil, nil
	})
	mcp.AddTool(server, &mcp.Tool{
		Name: "update_phase",
		Description: "Partially update a phase. Tri-state on default_template_id: empty string clears the template link, " +
			"a UUID string sets a new one, and omitting the field leaves it unchanged. Bumps updated_at, " +
			"which is also the tiebreaker when phases overlap (most-recently-updated wins for adherence).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args UpdatePhaseArgs) (*mcp.CallToolResult, any, error) {
		return handleUpdatePhase(ctx, c, args), nil, nil
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_phase",
		Description: "Delete a phase. Adherence on dates that were in the phase falls through to the next step (override, singleton default, or none). Any template the phase pointed at is unaffected.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args DeletePhaseArgs) (*mcp.CallToolResult, any, error) {
		return handleDeletePhase(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "set_goal_template",
		Description: "Create or replace a named goal template (reusable goal-set). Full-replace semantics: absent nutrient bounds are stored as NULL. " +
			"A template is meant to be attached to a phase via create_phase or update_phase's default_template_id — once attached, editing the template's bounds propagates to every phase pointing at it on the next adherence read. " +
			"No apply step required; template edits are intentionally cheap. Retries are NOT safe (PUT rejects Idempotency-Key).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args SetGoalTemplateArgs) (*mcp.CallToolResult, any, error) {
		return handleSetGoalTemplate(ctx, c, args), nil, nil
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_goal_templates",
		Description: "List every goal template ordered by name ascending. Each entry includes its full goal-bound set.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args ListGoalTemplatesArgs) (*mcp.CallToolResult, any, error) {
		return handleListGoalTemplates(ctx, c, args), nil, nil
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_goal_template",
		Description: "Fetch a goal template by name.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args GetGoalTemplateArgs) (*mcp.CallToolResult, any, error) {
		return handleGetGoalTemplate(ctx, c, args), nil, nil
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_goal_template",
		Description: "Delete a goal template by name. Refused with 409 template_in_use (and a referencing_phases list) if any phase points at it via default_template_id. Reassign or delete those phases first.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args DeleteGoalTemplateArgs) (*mcp.CallToolResult, any, error) {
		return handleDeleteGoalTemplate(ctx, c, args), nil, nil
	})
}
