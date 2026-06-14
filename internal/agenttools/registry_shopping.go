package agenttools

import (
	"encoding/json"
	"net/url"
	"strconv"
)

// Shopping list tools ported from internal/mcpserver onto the shared registry
// (unify-mcp-tool-registry). These are MCP-only; the arg structs, descriptions,
// and REST mappings are byte-identical to the prior bespoke registrations so the
// announced schema and the dispatched HTTP calls are unchanged.
//
// NOTE on dual surface: list_shopping_items, add_shopping_items,
// update_shopping_item, and clear_checked_shopping_items also exist as chat
// tools in nutritionPlannerSpecs() (registry.go). Those are independent Specs on
// the chat surface; the entries below are the byte-faithful MCP counterparts and
// the two surfaces filter independently (ChatRegistry vs MCPRegistry). The chat
// update_shopping_item uses item_id while this MCP one uses id — they are
// deliberately separate.

func init() { registerMCPDomain(shoppingSpecs()) }

// ShoppingItemArg is one item in a bulk shopping add.
type ShoppingItemArg struct {
	Name            string  `json:"name" jsonschema:"item name, e.g. 'Zwiebeln'; required, ≤300 chars"`
	QuantityText    *string `json:"quantity_text,omitempty" jsonschema:"opaque quantity text, e.g. '3 large' or '500 g' — already merged across recipes by you; the API never parses it"`
	RecipeProductID *string `json:"recipe_product_id,omitempty" jsonschema:"optional UUID of the recipe product this came from (soft provenance)"`
	PlanDate        *string `json:"plan_date,omitempty" jsonschema:"optional plan date YYYY-MM-DD this came from (provenance)"`
}

// AddShoppingItemsArgs is the input for add_shopping_items.
type AddShoppingItemsArgs struct {
	Items          []ShoppingItemArg `json:"items" jsonschema:"the consolidated shopping list, 1–200 items, in display order"`
	IdempotencyKey string            `json:"idempotency_key,omitempty" jsonschema:"optional retry key; derived from the items if omitted"`
}

// ListShoppingItemsArgs is the input for list_shopping_items.
type ListShoppingItemsArgs struct {
	IncludeChecked *bool `json:"include_checked,omitempty" jsonschema:"when true, also return checked (bought) items, listed after the unchecked ones"`
}

// UpdateShoppingItemArgs is the input for update_shopping_item.
type UpdateShoppingItemArgs struct {
	ID             string  `json:"id" jsonschema:"shopping item UUID"`
	Name           *string `json:"name,omitempty" jsonschema:"new name"`
	QuantityText   *string `json:"quantity_text,omitempty" jsonschema:"new opaque quantity text"`
	Checked        *bool   `json:"checked,omitempty" jsonschema:"true to mark bought (stamps checked_at), false to un-check"`
	IdempotencyKey string  `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

// DeleteShoppingItemArgs / ClearCheckedShoppingItemsArgs.
type DeleteShoppingItemArgs struct {
	ID             string `json:"id" jsonschema:"shopping item UUID"`
	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

type ClearCheckedShoppingItemsArgs struct {
	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

func shoppingSpecs() []Spec {
	return []Spec{
		{
			Name: "add_shopping_items",
			Description: "Add items to the shopping list in one call (1–200, atomic). IMPORTANT: merge and " +
				"dedupe quantities across the planned recipes YOURSELF before calling — combine '1 Zwiebel' + " +
				"'2 Zwiebeln' into one '3 Zwiebeln' item. The API stores items verbatim and never aggregates or " +
				"parses quantity_text. Set recipe_product_id / plan_date as provenance when known. A single " +
				"invalid item fails the whole batch (the error names the offending index).",
			SchemaType: AddShoppingItemsArgs{},
			Tier:       TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a AddShoppingItemsArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				body, err := json.Marshal(struct {
					Items []ShoppingItemArg `json:"items"`
				}{a.Items})
				if err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "POST", Path: "/shopping/items", Body: body}, nil
			},
		},
		{
			Name:        "list_shopping_items",
			Description: "List shopping items — unchecked (still-to-buy) in order by default; pass include_checked=true to also see bought items (listed last). Read-only.",
			SchemaType:  ListShoppingItemsArgs{},
			Tier:        TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a ListShoppingItemsArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				if a.IncludeChecked != nil {
					q.Set("include_checked", strconv.FormatBool(*a.IncludeChecked))
				}
				return HTTPCall{Method: "GET", Path: "/shopping/items", Query: q}, nil
			},
		},
		{
			Name:        "update_shopping_item",
			Description: "Rename a shopping item or check/uncheck it (checked=true stamps it bought, false un-checks).",
			SchemaType:  UpdateShoppingItemArgs{},
			Tier:        TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a UpdateShoppingItemArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				body, err := json.Marshal(struct {
					Name         *string `json:"name,omitempty"`
					QuantityText *string `json:"quantity_text,omitempty"`
					Checked      *bool   `json:"checked,omitempty"`
				}{a.Name, a.QuantityText, a.Checked})
				if err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "PATCH", Path: "/shopping/items/" + url.PathEscape(a.ID), Body: body}, nil
			},
		},
		{
			Name:        "delete_shopping_item",
			Description: "Delete a single shopping item by id.",
			SchemaType:  DeleteShoppingItemArgs{},
			Tier:        TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a DeleteShoppingItemArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "DELETE", Path: "/shopping/items/" + url.PathEscape(a.ID)}, nil
			},
		},
		{
			Name:        "clear_checked_shopping_items",
			Description: "Delete all checked (bought) items in one call and report how many were removed — the routine post-shop cleanup. There is intentionally no 'clear the whole list' call.",
			SchemaType:  ClearCheckedShoppingItemsArgs{},
			Tier:        TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a ClearCheckedShoppingItemsArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				q.Set("checked", "true")
				return HTTPCall{Method: "DELETE", Path: "/shopping/items", Query: q}, nil
			},
		},
	}
}
