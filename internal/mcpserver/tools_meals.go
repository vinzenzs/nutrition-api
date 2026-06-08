package mcpserver

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// FreeformNutriments is the per-100g nutriment estimate the agent supplies.
type FreeformNutriments struct {
	Kcal     *float64 `json:"kcal,omitempty" jsonschema:"kilocalories per 100g"`
	ProteinG *float64 `json:"protein_g,omitempty" jsonschema:"protein grams per 100g"`
	CarbsG   *float64 `json:"carbs_g,omitempty" jsonschema:"carbohydrate grams per 100g"`
	FatG     *float64 `json:"fat_g,omitempty" jsonschema:"fat grams per 100g"`
	FiberG   *float64 `json:"fiber_g,omitempty" jsonschema:"fiber grams per 100g"`
	SugarG   *float64 `json:"sugar_g,omitempty" jsonschema:"sugar grams per 100g"`
	SaltG    *float64 `json:"salt_g,omitempty" jsonschema:"salt grams per 100g"`

	IronMg        *float64 `json:"iron_mg,omitempty" jsonschema:"iron milligrams per 100g"`
	CalciumMg     *float64 `json:"calcium_mg,omitempty" jsonschema:"calcium milligrams per 100g"`
	VitaminDMcg   *float64 `json:"vitamin_d_mcg,omitempty" jsonschema:"vitamin D micrograms per 100g"`
	VitaminB12Mcg *float64 `json:"vitamin_b12_mcg,omitempty" jsonschema:"vitamin B12 micrograms per 100g"`
	VitaminCMg    *float64 `json:"vitamin_c_mg,omitempty" jsonschema:"vitamin C milligrams per 100g"`
	MagnesiumMg   *float64 `json:"magnesium_mg,omitempty" jsonschema:"magnesium milligrams per 100g"`
	PotassiumMg   *float64 `json:"potassium_mg,omitempty" jsonschema:"potassium milligrams per 100g"`
	ZincMg        *float64 `json:"zinc_mg,omitempty" jsonschema:"zinc milligrams per 100g"`
}

type LogMealArgs struct {
	ProductID      string  `json:"product_id" jsonschema:"the product id returned by lookup_product_by_barcode or search_products"`
	QuantityG      float64 `json:"quantity_g" jsonschema:"amount eaten in grams; must be greater than zero"`
	LoggedAt       string  `json:"logged_at" jsonschema:"when the meal was eaten, RFC 3339 timestamp (UTC or with offset)"`
	MealType       string  `json:"meal_type,omitempty" jsonschema:"optional: breakfast | lunch | dinner | snack"`
	Note           string  `json:"note,omitempty" jsonschema:"optional free-text note"`
	WorkoutID      string  `json:"workout_id,omitempty" jsonschema:"optional UUID of an existing workout to link this meal to. The link is metadata; workout_fueling_summary aggregates by logged_at time-window matching, not by this tag."`
	IdempotencyKey string  `json:"idempotency_key,omitempty" jsonschema:"optional retry key; if omitted, a stable key is derived from the other args"`
}

type LogMealFreeformArgs struct {
	Name              string             `json:"name" jsonschema:"the food item described by the user (e.g. 'banana', 'oat porridge')"`
	NutrimentsPer100g FreeformNutriments `json:"nutriments_per_100g" jsonschema:"your best estimate of the per-100g macros; missing fields stay null"`
	QuantityG         float64            `json:"quantity_g" jsonschema:"amount eaten in grams; must be greater than zero"`
	LoggedAt          string             `json:"logged_at" jsonschema:"when the meal was eaten, RFC 3339 timestamp"`
	MealType          string             `json:"meal_type,omitempty" jsonschema:"optional: breakfast | lunch | dinner | snack"`
	Note              string             `json:"note,omitempty" jsonschema:"optional free-text note"`
	SaveAsProduct     bool               `json:"save_as_product,omitempty" jsonschema:"true to also create a reusable product row with these nutriments"`
	WorkoutID         string             `json:"workout_id,omitempty" jsonschema:"optional UUID of an existing workout to link this meal to"`
	IdempotencyKey    string             `json:"idempotency_key,omitempty" jsonschema:"optional retry key; if omitted, a stable key is derived from the other args. To intentionally log the same item twice, pass a distinct key here."`
}

type PatchMealArgs struct {
	MealID         string   `json:"meal_id" jsonschema:"the id of the meal entry to update"`
	QuantityG      *float64 `json:"quantity_g,omitempty" jsonschema:"new amount in grams; must be greater than zero if supplied"`
	LoggedAt       *string  `json:"logged_at,omitempty" jsonschema:"new RFC 3339 timestamp"`
	MealType       *string  `json:"meal_type,omitempty" jsonschema:"new meal type: breakfast | lunch | dinner | snack"`
	Note           *string  `json:"note,omitempty" jsonschema:"new note"`
	// WorkoutID supports the empty-string sentinel: \"<uuid>\" sets the link, \"\" clears it, missing leaves it unchanged.
	WorkoutID      *string  `json:"workout_id,omitempty" jsonschema:"new workout link: \"<uuid>\" sets, \"\" clears, omit to leave unchanged"`
	IdempotencyKey string   `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

type DeleteMealArgs struct {
	MealID         string `json:"meal_id" jsonschema:"the id of the meal entry to delete"`
	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

func handleLogMeal(ctx context.Context, c *apiClient, args LogMealArgs) *mcp.CallToolResult {
	body, err := json.Marshal(struct {
		ProductID string  `json:"product_id"`
		QuantityG float64 `json:"quantity_g"`
		LoggedAt  string  `json:"logged_at"`
		MealType  string  `json:"meal_type,omitempty"`
		Note      string  `json:"note,omitempty"`
		WorkoutID string  `json:"workout_id,omitempty"`
	}{
		ProductID: args.ProductID,
		QuantityG: args.QuantityG,
		LoggedAt:  args.LoggedAt,
		MealType:  args.MealType,
		Note:      args.Note,
		WorkoutID: args.WorkoutID,
	})
	if err != nil {
		return toToolResult(0, nil, &transportError{inner: err})
	}
	key := effectiveIdempotencyKey(args.IdempotencyKey, "log_meal", args)
	status, respBody, err := c.Post(ctx, "/meals", nil, body, key)
	return toToolResult(status, respBody, err)
}

func handleLogMealFreeform(ctx context.Context, c *apiClient, args LogMealFreeformArgs) *mcp.CallToolResult {
	body, err := json.Marshal(struct {
		Name              string             `json:"name"`
		NutrimentsPer100g FreeformNutriments `json:"nutriments_per_100g"`
		QuantityG         float64            `json:"quantity_g"`
		LoggedAt          string             `json:"logged_at"`
		MealType          string             `json:"meal_type,omitempty"`
		Note              string             `json:"note,omitempty"`
		SaveAsProduct     bool               `json:"save_as_product,omitempty"`
		WorkoutID         string             `json:"workout_id,omitempty"`
	}{
		Name:              args.Name,
		NutrimentsPer100g: args.NutrimentsPer100g,
		QuantityG:         args.QuantityG,
		LoggedAt:          args.LoggedAt,
		MealType:          args.MealType,
		Note:              args.Note,
		SaveAsProduct:     args.SaveAsProduct,
		WorkoutID:         args.WorkoutID,
	})
	if err != nil {
		return toToolResult(0, nil, &transportError{inner: err})
	}
	key := effectiveIdempotencyKey(args.IdempotencyKey, "log_meal_freeform", args)
	status, respBody, err := c.Post(ctx, "/meals/freeform", nil, body, key)
	return toToolResult(status, respBody, err)
}

func handlePatchMeal(ctx context.Context, c *apiClient, args PatchMealArgs) *mcp.CallToolResult {
	payload := map[string]any{}
	if args.QuantityG != nil {
		payload["quantity_g"] = *args.QuantityG
	}
	if args.LoggedAt != nil {
		payload["logged_at"] = *args.LoggedAt
	}
	if args.MealType != nil {
		payload["meal_type"] = *args.MealType
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
	key := effectiveIdempotencyKey(args.IdempotencyKey, "patch_meal", args)
	status, respBody, err := c.Patch(ctx, "/meals/"+url.PathEscape(args.MealID), body, key)
	return toToolResult(status, respBody, err)
}

func handleDeleteMeal(ctx context.Context, c *apiClient, args DeleteMealArgs) *mcp.CallToolResult {
	key := effectiveIdempotencyKey(args.IdempotencyKey, "delete_meal", args)
	status, respBody, err := c.Delete(ctx, "/meals/"+url.PathEscape(args.MealID), key)
	if err == nil && status == 204 {
		// 204 No Content → empty tool result content (but not isError).
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: ""}}}
	}
	return toToolResult(status, respBody, err)
}

func registerMealsTools(server *mcp.Server, c *apiClient) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "log_meal",
		Description: "Log a meal entry from a known product. Use this after lookup_product_by_barcode " +
			"or search_products has given you a product_id. If you have no product yet (e.g. the user " +
			"described a meal in natural language), use log_meal_freeform instead.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args LogMealArgs) (*mcp.CallToolResult, any, error) {
		return handleLogMeal(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "log_meal_freeform",
		Description: "Log a meal entry from an estimated name + nutriment block. Use this for " +
			"natural-language inputs like 'I had a banana' after you estimate the macros yourself. " +
			"Set save_as_product=true to also create a reusable product row so future logs of the " +
			"same food can reuse it. By default, retrying the tool with the same arguments is " +
			"idempotent — to log the same item twice intentionally, pass a distinct idempotency_key. " +
			"For meals you eat repeatedly that have 2+ ingredients (e.g. 'my morning skyr bowl'), " +
			"use create_recipe instead to define the recipe once, then log_meal it as a single unit.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args LogMealFreeformArgs) (*mcp.CallToolResult, any, error) {
		return handleLogMealFreeform(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "patch_meal",
		Description: "Partially update an existing meal entry. Only supplied fields are changed.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args PatchMealArgs) (*mcp.CallToolResult, any, error) {
		return handlePatchMeal(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_meal",
		Description: "Delete a meal entry. Returns an empty result on success.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args DeleteMealArgs) (*mcp.CallToolResult, any, error) {
		return handleDeleteMeal(ctx, c, args), nil, nil
	})
}
