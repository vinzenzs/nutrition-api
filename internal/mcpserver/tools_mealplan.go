package mcpserver

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// CreatePlannedMealArgs is the input for create_planned_meal.
type CreatePlannedMealArgs struct {
	PlanDate       string   `json:"plan_date" jsonschema:"date the meal is planned for, YYYY-MM-DD"`
	Slot           string   `json:"slot" jsonschema:"one of breakfast|lunch|dinner|snack"`
	ProductID      string   `json:"product_id" jsonschema:"UUID of an existing product (recipe products included); required"`
	QuantityG      *float64 `json:"quantity_g,omitempty" jsonschema:"optional planned quantity in grams; > 0. Defaults at eating time to the product's serving size."`
	Notes          *string  `json:"notes,omitempty" jsonschema:"optional free-text note"`
	IdempotencyKey string   `json:"idempotency_key,omitempty" jsonschema:"optional retry key; derived from the other args if omitted"`
}

// ListPlannedMealsArgs is the input for list_planned_meals.
type ListPlannedMealsArgs struct {
	From string `json:"from" jsonschema:"inclusive lower bound YYYY-MM-DD"`
	To   string `json:"to" jsonschema:"inclusive upper bound YYYY-MM-DD"`
}

// UpdatePlannedMealArgs is the input for update_planned_meal. Status may move
// planned↔skipped only; use mark_planned_meal_eaten to record eating.
type UpdatePlannedMealArgs struct {
	ID             string   `json:"id" jsonschema:"planned meal UUID"`
	PlanDate       *string  `json:"plan_date,omitempty" jsonschema:"new date YYYY-MM-DD"`
	Slot           *string  `json:"slot,omitempty" jsonschema:"new slot breakfast|lunch|dinner|snack"`
	ProductID      *string  `json:"product_id,omitempty" jsonschema:"new product UUID"`
	QuantityG      *float64 `json:"quantity_g,omitempty" jsonschema:"new planned quantity in grams; > 0"`
	Status         *string  `json:"status,omitempty" jsonschema:"planned or skipped (eaten is set only via mark_planned_meal_eaten and is terminal)"`
	Notes          *string  `json:"notes,omitempty" jsonschema:"new note"`
	IdempotencyKey string   `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

// DeletePlannedMealArgs is the input for delete_planned_meal.
type DeletePlannedMealArgs struct {
	ID             string `json:"id" jsonschema:"planned meal UUID"`
	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

// MarkPlannedMealEatenArgs is the input for mark_planned_meal_eaten.
type MarkPlannedMealEatenArgs struct {
	ID             string   `json:"id" jsonschema:"planned meal UUID"`
	QuantityG      *float64 `json:"quantity_g,omitempty" jsonschema:"optional quantity override in grams; > 0"`
	LoggedAt       *string  `json:"logged_at,omitempty" jsonschema:"optional RFC 3339 eating time; defaults to now, never in the future"`
	IdempotencyKey string   `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

func handleCreatePlannedMeal(ctx context.Context, c *apiClient, args CreatePlannedMealArgs) *mcp.CallToolResult {
	body, err := json.Marshal(struct {
		PlanDate  string   `json:"plan_date"`
		Slot      string   `json:"slot"`
		ProductID string   `json:"product_id"`
		QuantityG *float64 `json:"quantity_g,omitempty"`
		Notes     *string  `json:"notes,omitempty"`
	}{args.PlanDate, args.Slot, args.ProductID, args.QuantityG, args.Notes})
	if err != nil {
		return toToolResult(0, nil, &transportError{inner: err})
	}
	key := effectiveIdempotencyKey(args.IdempotencyKey, "create_planned_meal", args)
	status, respBody, err := c.Post(ctx, "/plan", nil, body, key)
	return toToolResult(status, respBody, err)
}

func handleUpdatePlannedMeal(ctx context.Context, c *apiClient, args UpdatePlannedMealArgs) *mcp.CallToolResult {
	body, err := json.Marshal(struct {
		PlanDate  *string  `json:"plan_date,omitempty"`
		Slot      *string  `json:"slot,omitempty"`
		ProductID *string  `json:"product_id,omitempty"`
		QuantityG *float64 `json:"quantity_g,omitempty"`
		Status    *string  `json:"status,omitempty"`
		Notes     *string  `json:"notes,omitempty"`
	}{args.PlanDate, args.Slot, args.ProductID, args.QuantityG, args.Status, args.Notes})
	if err != nil {
		return toToolResult(0, nil, &transportError{inner: err})
	}
	key := effectiveIdempotencyKey(args.IdempotencyKey, "update_planned_meal", args)
	status, respBody, err := c.Patch(ctx, "/plan/"+url.PathEscape(args.ID), body, key)
	return toToolResult(status, respBody, err)
}

func handleMarkPlannedMealEaten(ctx context.Context, c *apiClient, args MarkPlannedMealEatenArgs) *mcp.CallToolResult {
	body, err := json.Marshal(struct {
		QuantityG *float64 `json:"quantity_g,omitempty"`
		LoggedAt  *string  `json:"logged_at,omitempty"`
	}{args.QuantityG, args.LoggedAt})
	if err != nil {
		return toToolResult(0, nil, &transportError{inner: err})
	}
	key := effectiveIdempotencyKey(args.IdempotencyKey, "mark_planned_meal_eaten", args)
	status, respBody, err := c.Post(ctx, "/plan/"+url.PathEscape(args.ID)+"/eaten", nil, body, key)
	return toToolResult(status, respBody, err)
}

func registerMealPlanTools(server *mcp.Server, c *apiClient) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "create_planned_meal",
		Description: "Persist a planned meal — a selection for a date+slot that is NOT yet logged. Use this " +
			"for 'what should I eat today / the next 3 days'. `product_id` is required (import a Cookidoo " +
			"recipe first if needed). A plan does not affect adherence or meal history until it is marked " +
			"eaten with mark_planned_meal_eaten. Two plans may share a date+slot (options to choose between).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args CreatePlannedMealArgs) (*mcp.CallToolResult, any, error) {
		return handleCreatePlannedMeal(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_planned_meals",
		Description: "List planned meals with plan_date in [from, to] inclusive, ordered by date then slot, each with its product name. Read-only.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args ListPlannedMealsArgs) (*mcp.CallToolResult, any, error) {
		q := url.Values{}
		q.Set("from", args.From)
		q.Set("to", args.To)
		status, body, err := c.Get(ctx, "/plan", q)
		return toToolResult(status, body, err), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "update_planned_meal",
		Description: "Update a planned meal's fields. `status` may move planned↔skipped only; `eaten` is set " +
			"exclusively via mark_planned_meal_eaten and is terminal (this tool cannot change an eaten entry).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args UpdatePlannedMealArgs) (*mcp.CallToolResult, any, error) {
		return handleUpdatePlannedMeal(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_planned_meal",
		Description: "Delete a planned meal by id. Does not touch any meal entry already logged from it.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args DeletePlannedMealArgs) (*mcp.CallToolResult, any, error) {
		key := effectiveIdempotencyKey(args.IdempotencyKey, "delete_planned_meal", args)
		status, body, err := c.Delete(ctx, "/plan/"+url.PathEscape(args.ID), key)
		return toToolResult(status, body, err), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "mark_planned_meal_eaten",
		Description: "Record a planned meal as eaten NOW — this logs a real meal entry (logged_at = now, " +
			"meal_type = the plan's slot, quantity = override → plan quantity → product serving) and flips the " +
			"plan to eaten atomically. This is the ONLY correct way to turn a plan into meal history; do NOT " +
			"call log_meal separately for a planned meal. Marking an already-eaten entry returns a 409 conflict.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args MarkPlannedMealEatenArgs) (*mcp.CallToolResult, any, error) {
		return handleMarkPlannedMealEaten(ctx, c, args), nil, nil
	})
}
