package agenttools

import (
	"encoding/json"
	"fmt"
	"net/url"
)

// Meal-plan tools — the desktop MCP coach's planned-meal CRUD plus the
// mark-eaten transition. Ported from internal/mcpserver/tools_mealplan.go onto
// the shared registry (unify-mcp-tool-registry). These are MCP-only; the arg
// structs and descriptions are byte-identical to the prior bespoke
// registrations so the announced schema is unchanged.
//
// NOTE: the in-app chat coach already exposes tools that SHARE these names
// (create_planned_meal, list_planned_meals, update_planned_meal,
// mark_planned_meal_eaten) on the ChatRegistry surface. Those are SEPARATE
// Spec entries (nutritionPlannerSpecs) with hand-written schemas and a
// different body/path convention (plan_id path param via Passthrough). The two
// surfaces filter independently (ChatRegistry vs MCPRegistry), so the shared
// names are not a conflict. The MCP entries below faithfully reproduce the
// bespoke MCP handler shapes (id path param, struct-marshalled bodies).

func init() { registerMCPDomain(mealPlanSpecs()) }

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

func mealPlanSpecs() []Spec {
	return []Spec{
		{
			Name: "create_planned_meal",
			Description: "Persist a planned meal — a selection for a date+slot that is NOT yet logged. Use this " +
				"for 'what should I eat today / the next 3 days'. `product_id` is required (import a Cookidoo " +
				"recipe first if needed). A plan does not affect adherence or meal history until it is marked " +
				"eaten with mark_planned_meal_eaten. Two plans may share a date+slot (options to choose between).",
			SchemaType: CreatePlannedMealArgs{},
			Tier:       TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a CreatePlannedMealArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				body, err := json.Marshal(struct {
					PlanDate  string   `json:"plan_date"`
					Slot      string   `json:"slot"`
					ProductID string   `json:"product_id"`
					QuantityG *float64 `json:"quantity_g,omitempty"`
					Notes     *string  `json:"notes,omitempty"`
				}{a.PlanDate, a.Slot, a.ProductID, a.QuantityG, a.Notes})
				if err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "POST", Path: "/plan", Body: body}, nil
			},
		},
		{
			Name:        "list_planned_meals",
			Description: "List planned meals with plan_date in [from, to] inclusive, ordered by date then slot, each with its product name. Read-only.",
			SchemaType:  ListPlannedMealsArgs{},
			Tier:        TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a ListPlannedMealsArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				q.Set("from", a.From)
				q.Set("to", a.To)
				return HTTPCall{Method: "GET", Path: "/plan", Query: q}, nil
			},
		},
		{
			Name: "update_planned_meal",
			Description: "Update a planned meal's fields. `status` may move planned↔skipped only; `eaten` is set " +
				"exclusively via mark_planned_meal_eaten and is terminal (this tool cannot change an eaten entry).",
			SchemaType: UpdatePlannedMealArgs{},
			Tier:       TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a UpdatePlannedMealArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				if a.ID == "" {
					return HTTPCall{}, fmt.Errorf("id is required")
				}
				body, err := json.Marshal(struct {
					PlanDate  *string  `json:"plan_date,omitempty"`
					Slot      *string  `json:"slot,omitempty"`
					ProductID *string  `json:"product_id,omitempty"`
					QuantityG *float64 `json:"quantity_g,omitempty"`
					Status    *string  `json:"status,omitempty"`
					Notes     *string  `json:"notes,omitempty"`
				}{a.PlanDate, a.Slot, a.ProductID, a.QuantityG, a.Status, a.Notes})
				if err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "PATCH", Path: "/plan/" + url.PathEscape(a.ID), Body: body}, nil
			},
		},
		{
			Name:        "delete_planned_meal",
			Description: "Delete a planned meal by id. Does not touch any meal entry already logged from it.",
			SchemaType:  DeletePlannedMealArgs{},
			Tier:        TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a DeletePlannedMealArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				if a.ID == "" {
					return HTTPCall{}, fmt.Errorf("id is required")
				}
				return HTTPCall{Method: "DELETE", Path: "/plan/" + url.PathEscape(a.ID)}, nil
			},
		},
		{
			Name: "mark_planned_meal_eaten",
			Description: "Record a planned meal as eaten NOW — this logs a real meal entry (logged_at = now, " +
				"meal_type = the plan's slot, quantity = override → plan quantity → product serving) and flips the " +
				"plan to eaten atomically. This is the ONLY correct way to turn a plan into meal history; do NOT " +
				"call log_meal separately for a planned meal. Marking an already-eaten entry returns a 409 conflict.",
			SchemaType: MarkPlannedMealEatenArgs{},
			Tier:       TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a MarkPlannedMealEatenArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				if a.ID == "" {
					return HTTPCall{}, fmt.Errorf("id is required")
				}
				body, err := json.Marshal(struct {
					QuantityG *float64 `json:"quantity_g,omitempty"`
					LoggedAt  *string  `json:"logged_at,omitempty"`
				}{a.QuantityG, a.LoggedAt})
				if err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "POST", Path: "/plan/" + url.PathEscape(a.ID) + "/eaten", Body: body}, nil
			},
		},
	}
}
