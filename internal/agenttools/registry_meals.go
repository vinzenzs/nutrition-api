package agenttools

import (
	"encoding/json"
	"fmt"
	"net/url"
)

// Meals MCP tools — ported from internal/mcpserver/tools_meals.go onto the
// shared registry (unify-mcp-tool-registry). These are MCP-only writes that
// mirror the REST /meals surface 1:1. The arg structs (and their json +
// jsonschema tags) are byte-identical to the prior bespoke registrations so the
// announced schema is unchanged.
//
// NOTE: log_meal_from_photo is deliberately NOT ported here — it is a multipart
// upload (the one documented exception to the one-HTTP-call mapping) and stays a
// bespoke registration in internal/mcpserver.

func init() { registerMCPDomain(mealsSpecs()) }

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
	MealID    string   `json:"meal_id" jsonschema:"the id of the meal entry to update"`
	QuantityG *float64 `json:"quantity_g,omitempty" jsonschema:"new amount in grams; must be greater than zero if supplied"`
	LoggedAt  *string  `json:"logged_at,omitempty" jsonschema:"new RFC 3339 timestamp"`
	MealType  *string  `json:"meal_type,omitempty" jsonschema:"new meal type: breakfast | lunch | dinner | snack"`
	Note      *string  `json:"note,omitempty" jsonschema:"new note"`
	// WorkoutID supports the empty-string sentinel: \"<uuid>\" sets the link, \"\" clears it, missing leaves it unchanged.
	WorkoutID      *string `json:"workout_id,omitempty" jsonschema:"new workout link: \"<uuid>\" sets, \"\" clears, omit to leave unchanged"`
	IdempotencyKey string  `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

type DeleteMealArgs struct {
	MealID         string `json:"meal_id" jsonschema:"the id of the meal entry to delete"`
	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

func mealsSpecs() []Spec {
	return []Spec{
		{
			Name: "log_meal",
			Description: "Log a meal entry from a known product. Use this after lookup_product_by_barcode " +
				"or search_products has given you a product_id. If you have no product yet (e.g. the user " +
				"described a meal in natural language), use log_meal_freeform instead.",
			SchemaType: LogMealArgs{},
			Tier:       TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a LogMealArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				body, err := json.Marshal(struct {
					ProductID string  `json:"product_id"`
					QuantityG float64 `json:"quantity_g"`
					LoggedAt  string  `json:"logged_at"`
					MealType  string  `json:"meal_type,omitempty"`
					Note      string  `json:"note,omitempty"`
					WorkoutID string  `json:"workout_id,omitempty"`
				}{
					ProductID: a.ProductID,
					QuantityG: a.QuantityG,
					LoggedAt:  a.LoggedAt,
					MealType:  a.MealType,
					Note:      a.Note,
					WorkoutID: a.WorkoutID,
				})
				if err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "POST", Path: "/meals", Body: body}, nil
			},
		},
		{
			Name: "log_meal_freeform",
			Description: "Log a meal entry from an estimated name + nutriment block. Use this for " +
				"natural-language inputs like 'I had a banana' after you estimate the macros yourself. " +
				"Set save_as_product=true to also create a reusable product row so future logs of the " +
				"same food can reuse it. By default, retrying the tool with the same arguments is " +
				"idempotent — to log the same item twice intentionally, pass a distinct idempotency_key. " +
				"For meals you eat repeatedly that have 2+ ingredients (e.g. 'my morning skyr bowl'), " +
				"use create_recipe instead to define the recipe once, then log_meal it as a single unit.",
			SchemaType: LogMealFreeformArgs{},
			Tier:       TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a LogMealFreeformArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
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
					Name:              a.Name,
					NutrimentsPer100g: a.NutrimentsPer100g,
					QuantityG:         a.QuantityG,
					LoggedAt:          a.LoggedAt,
					MealType:          a.MealType,
					Note:              a.Note,
					SaveAsProduct:     a.SaveAsProduct,
					WorkoutID:         a.WorkoutID,
				})
				if err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "POST", Path: "/meals/freeform", Body: body}, nil
			},
		},
		{
			Name:        "patch_meal",
			Description: "Partially update an existing meal entry. Only supplied fields are changed.",
			SchemaType:  PatchMealArgs{},
			Tier:        TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a PatchMealArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				if a.MealID == "" {
					return HTTPCall{}, fmt.Errorf("meal_id is required")
				}
				payload := map[string]any{}
				if a.QuantityG != nil {
					payload["quantity_g"] = *a.QuantityG
				}
				if a.LoggedAt != nil {
					payload["logged_at"] = *a.LoggedAt
				}
				if a.MealType != nil {
					payload["meal_type"] = *a.MealType
				}
				if a.Note != nil {
					payload["note"] = *a.Note
				}
				if a.WorkoutID != nil {
					payload["workout_id"] = *a.WorkoutID
				}
				body, err := json.Marshal(payload)
				if err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "PATCH", Path: "/meals/" + url.PathEscape(a.MealID), Body: body}, nil
			},
		},
		{
			Name:        "delete_meal",
			Description: "Delete a meal entry. Returns an empty result on success.",
			SchemaType:  DeleteMealArgs{},
			Tier:        TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a DeleteMealArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				if a.MealID == "" {
					return HTTPCall{}, fmt.Errorf("meal_id is required")
				}
				return HTTPCall{Method: "DELETE", Path: "/meals/" + url.PathEscape(a.MealID)}, nil
			},
		},
	}
}
