package mcpserver

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// LogWorkoutFuelArgs is the input shape for `log_workout_fuel`.
//
// `name` is required. At least one of `quantity_ml`/`carbs_g`/`sodium_mg`/
// `potassium_mg`/`caffeine_mg` must be supplied (the REST backend rejects
// `empty_entry` otherwise). `caffeine_mg: 0` is a meaningful "explicitly zero"
// signal — distinct from omitting the field (NULL = "not measured").
type LogWorkoutFuelArgs struct {
	Name           string   `json:"name" jsonschema:"product/brand name of the gel, drink, salt tab, or caffeine source. Required — rehearsal data depends on knowing WHAT was taken."`
	LoggedAt       string   `json:"logged_at" jsonschema:"when the fueling event happened, RFC 3339 timestamp"`
	QuantityMl     *float64 `json:"quantity_ml,omitempty" jsonschema:"optional volume in millilitres (drinks only); must be greater than zero if supplied"`
	CarbsG         *float64 `json:"carbs_g,omitempty" jsonschema:"optional carbohydrate amount in grams; >= 0"`
	SodiumMg       *float64 `json:"sodium_mg,omitempty" jsonschema:"optional sodium amount in milligrams; >= 0"`
	PotassiumMg    *float64 `json:"potassium_mg,omitempty" jsonschema:"optional potassium amount in milligrams; >= 0"`
	CaffeineMg     *float64 `json:"caffeine_mg,omitempty" jsonschema:"optional caffeine amount in milligrams; >= 0. Pass 0 explicitly to signal 'measured, no caffeine' (e.g. a decaf product) — distinct from omitting (which means 'not measured')."`
	Note           string   `json:"note,omitempty" jsonschema:"optional free-text note (rehearsal observations, flavour, GI feel)"`
	WorkoutID      string   `json:"workout_id,omitempty" jsonschema:"optional UUID of an existing workout to link this entry to. The link is metadata; workout_fueling_summary aggregates by logged_at time-window matching, not by this tag."`
	IdempotencyKey string   `json:"idempotency_key,omitempty" jsonschema:"optional retry key; if omitted, a stable key is derived from the other args."`
}

// ListWorkoutFuelArgs is the input shape for `list_workout_fuel`.
type ListWorkoutFuelArgs struct {
	From string `json:"from" jsonschema:"inclusive RFC 3339 lower bound on logged_at"`
	To   string `json:"to" jsonschema:"exclusive RFC 3339 upper bound on logged_at; max 92 days from 'from'"`
}

// PatchWorkoutFuelArgs is the input shape for `patch_workout_fuel`.
//
// Quantitative fields use `*float64`: omit = leave unchanged, set = update.
// The REST backend additionally accepts JSON `null` on these fields to clear
// the column — but expressing that through MCP requires using the raw HTTP
// API; the typed MCP shape only models set/omit. `workout_id` is a plain
// string so the empty-string clear sentinel works (omit = leave, "<uuid>" =
// set, "" = clear).
type PatchWorkoutFuelArgs struct {
	ID             string   `json:"id" jsonschema:"the id of the workout-fuel entry to update"`
	Name           *string  `json:"name,omitempty" jsonschema:"new name"`
	LoggedAt       *string  `json:"logged_at,omitempty" jsonschema:"new RFC 3339 timestamp"`
	QuantityMl     *float64 `json:"quantity_ml,omitempty" jsonschema:"new volume in millilitres; must be greater than zero"`
	CarbsG         *float64 `json:"carbs_g,omitempty" jsonschema:"new carbohydrate amount in grams; >= 0"`
	SodiumMg       *float64 `json:"sodium_mg,omitempty" jsonschema:"new sodium amount in milligrams; >= 0"`
	PotassiumMg    *float64 `json:"potassium_mg,omitempty" jsonschema:"new potassium amount in milligrams; >= 0"`
	CaffeineMg     *float64 `json:"caffeine_mg,omitempty" jsonschema:"new caffeine amount in milligrams; >= 0"`
	Note           *string  `json:"note,omitempty" jsonschema:"new note"`
	WorkoutID      *string  `json:"workout_id,omitempty" jsonschema:"new workout link: \"<uuid>\" sets, \"\" clears, omit to leave unchanged"`
	IdempotencyKey string   `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

// DeleteWorkoutFuelArgs is the input shape for `delete_workout_fuel`.
type DeleteWorkoutFuelArgs struct {
	ID             string `json:"id" jsonschema:"the id of the workout-fuel entry to delete"`
	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

func handleLogWorkoutFuel(ctx context.Context, c *apiClient, args LogWorkoutFuelArgs) *mcp.CallToolResult {
	body, err := json.Marshal(struct {
		Name        string   `json:"name"`
		LoggedAt    string   `json:"logged_at"`
		QuantityMl  *float64 `json:"quantity_ml,omitempty"`
		CarbsG      *float64 `json:"carbs_g,omitempty"`
		SodiumMg    *float64 `json:"sodium_mg,omitempty"`
		PotassiumMg *float64 `json:"potassium_mg,omitempty"`
		CaffeineMg  *float64 `json:"caffeine_mg,omitempty"`
		Note        string   `json:"note,omitempty"`
		WorkoutID   string   `json:"workout_id,omitempty"`
	}{
		Name:        args.Name,
		LoggedAt:    args.LoggedAt,
		QuantityMl:  args.QuantityMl,
		CarbsG:      args.CarbsG,
		SodiumMg:    args.SodiumMg,
		PotassiumMg: args.PotassiumMg,
		CaffeineMg:  args.CaffeineMg,
		Note:        args.Note,
		WorkoutID:   args.WorkoutID,
	})
	if err != nil {
		return toToolResult(0, nil, &transportError{inner: err})
	}
	key := effectiveIdempotencyKey(args.IdempotencyKey, "log_workout_fuel", args)
	status, respBody, err := c.Post(ctx, "/workout-fuel", nil, body, key)
	return toToolResult(status, respBody, err)
}

func handleListWorkoutFuel(ctx context.Context, c *apiClient, args ListWorkoutFuelArgs) *mcp.CallToolResult {
	q := url.Values{}
	q.Set("from", args.From)
	q.Set("to", args.To)
	status, body, err := c.Get(ctx, "/workout-fuel", q)
	return toToolResult(status, body, err)
}

func handlePatchWorkoutFuel(ctx context.Context, c *apiClient, args PatchWorkoutFuelArgs) *mcp.CallToolResult {
	payload := map[string]any{}
	if args.Name != nil {
		payload["name"] = *args.Name
	}
	if args.LoggedAt != nil {
		payload["logged_at"] = *args.LoggedAt
	}
	if args.QuantityMl != nil {
		payload["quantity_ml"] = *args.QuantityMl
	}
	if args.CarbsG != nil {
		payload["carbs_g"] = *args.CarbsG
	}
	if args.SodiumMg != nil {
		payload["sodium_mg"] = *args.SodiumMg
	}
	if args.PotassiumMg != nil {
		payload["potassium_mg"] = *args.PotassiumMg
	}
	if args.CaffeineMg != nil {
		payload["caffeine_mg"] = *args.CaffeineMg
	}
	if args.Note != nil {
		payload["note"] = *args.Note
	}
	if args.WorkoutID != nil {
		payload["workout_id"] = *args.WorkoutID
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return toToolResult(0, nil, &transportError{inner: err})
	}
	key := effectiveIdempotencyKey(args.IdempotencyKey, "patch_workout_fuel", args)
	status, respBody, err := c.Patch(ctx, "/workout-fuel/"+url.PathEscape(args.ID), body, key)
	return toToolResult(status, respBody, err)
}

func handleDeleteWorkoutFuel(ctx context.Context, c *apiClient, args DeleteWorkoutFuelArgs) *mcp.CallToolResult {
	key := effectiveIdempotencyKey(args.IdempotencyKey, "delete_workout_fuel", args)
	status, respBody, err := c.Delete(ctx, "/workout-fuel/"+url.PathEscape(args.ID), key)
	if err == nil && status == 204 {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: ""}}}
	}
	return toToolResult(status, respBody, err)
}

func registerWorkoutFuelTools(server *mcp.Server, c *apiClient) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "log_workout_fuel",
		Description: "Record an in-session fueling event — gel, electrolyte drink, salt tab, " +
			"caffeine pill, pre-race espresso. ROUTING RULE: plain water / juice (volume only) " +
			"goes to log_hydration; anything with electrolytes / carbs / caffeine goes here. " +
			"`name` is REQUIRED (rehearsal data depends on knowing WHAT was taken). At least " +
			"one of quantity_ml/carbs_g/sodium_mg/potassium_mg/caffeine_mg must be supplied — " +
			"the API rejects empty entries. Pass `caffeine_mg: 0` explicitly to signal " +
			"'measured, no caffeine' (decaf product); omit the field to mean 'not measured'. " +
			"The optional workout_id link is metadata; workout_fueling_summary aggregates by " +
			"logged_at time-window matching, not by this tag.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args LogWorkoutFuelArgs) (*mcp.CallToolResult, any, error) {
		return handleLogWorkoutFuel(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "list_workout_fuel",
		Description: "List workout-fuel entries whose logged_at falls within the half-open " +
			"[from, to) RFC 3339 window. Window is capped at 92 days. Use workout_fueling_summary " +
			"instead when you want the per-workout pre/intra/post composition.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args ListWorkoutFuelArgs) (*mcp.CallToolResult, any, error) {
		return handleListWorkoutFuel(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "patch_workout_fuel",
		Description: "Partially update an existing workout-fuel entry. Only supplied fields " +
			"are changed. `workout_id`: pass \"<uuid>\" to set, \"\" to clear, omit to leave " +
			"unchanged.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args PatchWorkoutFuelArgs) (*mcp.CallToolResult, any, error) {
		return handlePatchWorkoutFuel(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_workout_fuel",
		Description: "Delete a workout-fuel entry. Returns an empty result on success.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args DeleteWorkoutFuelArgs) (*mcp.CallToolResult, any, error) {
		return handleDeleteWorkoutFuel(ctx, c, args), nil, nil
	})
}
