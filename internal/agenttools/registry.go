// Package agenttools is the single source of truth for the agent tool surface
// shared between the in-app chat coach (internal/chat) and — in a future change
// — the desktop MCP coach (internal/mcpserver). Each tool maps to exactly one
// loopback REST call and carries a confirmation tier the chat loop uses to
// decide whether a write pauses for human approval.
package agenttools

import (
	"encoding/json"
	"fmt"
	"net/url"
)

// Tier classifies how a tool's execution is gated by the chat loop.
//
//   - TierRead         — never gated; pure reads.
//   - TierWriteAuto    — low-stakes nutrition-planning writes; dispatch inline.
//   - TierWriteConfirm — training/goal/destructive writes; pause for human
//     confirmation before dispatch.
//
// The MCP server ignores the tier (it has its own client-side trust model);
// only the chat loop reads it.
type Tier string

const (
	TierRead         Tier = "read"
	TierWriteAuto    Tier = "write-auto"
	TierWriteConfirm Tier = "write-confirm"
)

// IsWrite reports whether the tier denotes a mutating tool — the ones that get
// an auto-derived Idempotency-Key on dispatch.
func (t Tier) IsWrite() bool { return t == TierWriteAuto || t == TierWriteConfirm }

// HTTPCall is the REST request a tool maps to. Exactly one per tool execution.
type HTTPCall struct {
	Method string
	Path   string
	Query  url.Values
	Body   []byte
}

// Spec defines one custom (client-executed) tool: its Anthropic/MCP schema, its
// confirmation tier, and how its input maps to a single loopback REST call.
type Spec struct {
	Name        string
	Description string
	// Schema is a hand-written JSON Schema (object) for the tool input — the
	// form the in-app chat coach renders into Anthropic tool defs. Tools ported
	// from the MCP server instead carry SchemaType (a typed arg struct the MCP
	// server reflects into an identical schema); exactly one of the two is set.
	Schema string
	// SchemaType, when non-nil, is a zero value of the tool's typed arg struct
	// (jsonschema-tagged). The MCP server reflects it to the announced input
	// schema via the same library the SDK uses, guaranteeing parity with the
	// prior hand-written registration (DD2).
	SchemaType any
	Tier       Tier
	Build      func(input json.RawMessage) (HTTPCall, error)
	// Format, when set, composes the human-readable confirmation preview for a
	// write-confirm tool from its input (D6) — a deterministic, code-composed
	// render of the actual bytes about to be sent, NOT the model's narration.
	// When nil, a generic "<verb> <resource>" line is derived from the name.
	Format func(input json.RawMessage) string
	// ChatExposed marks a tool the in-app chat coach surfaces (a curated,
	// tier-gated subset). MCPExposed marks a tool the desktop MCP coach
	// registers (the full surface, ignoring tier). A tool may be both — the
	// aggregate context reads are dual-surface (DD1).
	ChatExposed bool
	MCPExposed  bool
}

// DecodeInto unmarshals a tool's input JSON into dst, returning a friendly
// error the caller surfaces as a tool error rather than crashing the stream.
func DecodeInto(input json.RawMessage, dst any) error {
	if len(input) == 0 {
		return nil
	}
	if err := json.Unmarshal(input, dst); err != nil {
		return fmt.Errorf("invalid tool input: %w", err)
	}
	return nil
}

// Passthrough forwards the tool input verbatim as the request body.
func Passthrough(method, path string, in json.RawMessage) (HTTPCall, error) {
	body := in
	if len(body) == 0 {
		body = json.RawMessage(`{}`)
	}
	return HTTPCall{Method: method, Path: path, Body: body}, nil
}

// PathParamPassthrough pulls a single id field out of the input, uses it as a
// path segment, and forwards the remaining fields as the request body.
func PathParamPassthrough(in json.RawMessage, idField, method, pathPrefix string) (HTTPCall, error) {
	var generic map[string]json.RawMessage
	if err := DecodeInto(in, &generic); err != nil {
		return HTTPCall{}, err
	}
	idRaw, ok := generic[idField]
	if !ok {
		return HTTPCall{}, fmt.Errorf("%s is required", idField)
	}
	var id string
	if err := json.Unmarshal(idRaw, &id); err != nil || id == "" {
		return HTTPCall{}, fmt.Errorf("%s must be a non-empty string", idField)
	}
	delete(generic, idField)
	body, _ := json.Marshal(generic)
	return HTTPCall{Method: method, Path: pathPrefix + url.PathEscape(id), Body: body}, nil
}

// ByName indexes specs by tool name for dispatch.
func ByName(specs []Spec) map[string]Spec {
	m := make(map[string]Spec, len(specs))
	for _, s := range specs {
		m[s.Name] = s
	}
	return m
}

// Registry returns the full agent-tool union in a stable order: the in-app
// chat coach surface plus the MCP-only tools ported from internal/mcpserver
// (unify-mcp-tool-registry). It is the single source of truth both surfaces
// originate from. Each consumer filters to its surface — ChatRegistry() for the
// in-app coach, MCPRegistry() for the desktop MCP coach (DD1).
func Registry() []Spec {
	specs := chatSpecs()
	specs = append(specs, mcpOnlySpecs()...)
	return specs
}

// ChatRegistry returns only the chat-exposed tools — the curated, tier-gated
// coach surface the in-app chat loop renders and dispatches.
func ChatRegistry() []Spec {
	return filterSpecs(Registry(), func(s Spec) bool { return s.ChatExposed })
}

// MCPRegistry returns only the MCP-exposed tools — the desktop MCP server
// registers exactly these via one generic handler, ignoring tier and
// chat-visibility (DD1/DD3).
func MCPRegistry() []Spec {
	return filterSpecs(Registry(), func(s Spec) bool { return s.MCPExposed })
}

func filterSpecs(specs []Spec, keep func(Spec) bool) []Spec {
	out := make([]Spec, 0, len(specs))
	for _, s := range specs {
		if keep(s) {
			out = append(out, s)
		}
	}
	return out
}

// chatSpecs is the curated coach surface the in-app chat loop exposes: the
// nutrition-planning subset (reads + write-auto), the aggregate coaching
// context reads, and the gated write-confirm coaching actions. All are
// chat-exposed by construction.
func chatSpecs() []Spec {
	specs := nutritionPlannerSpecs()
	specs = append(specs, coachReadSpecs()...)
	specs = append(specs, coachWriteConfirmSpecs()...)
	for i := range specs {
		specs[i].ChatExposed = true
	}
	return specs
}

// mcpOnlySpecs is the desktop MCP coach surface being ported onto the shared
// registry one domain at a time (unify-mcp-tool-registry). Each domain
// contributes a slice; every tool here is MCP-exposed.
func mcpOnlySpecs() []Spec {
	var specs []Spec
	specs = append(specs, garminInventorySpecs()...)
	for i := range specs {
		specs[i].MCPExposed = true
	}
	return specs
}

// nutritionPlannerSpecs is the original meal-planning surface (reads +
// write-auto), retained as a subset of the coach.
func nutritionPlannerSpecs() []Spec {
	return []Spec{
		// ---------- reads ----------
		{
			Name:        "get_daily_context",
			Description: "Read the day's full nutrition state in one call: adherence vs goals, nutrition totals, hydration, workouts, workout-fuel, body weight, training phase, and any goal override. Call this FIRST before recommending meals so suggestions fit the remaining macro budget.",
			Schema:      `{"type":"object","properties":{"date":{"type":"string","description":"YYYY-MM-DD"},"tz":{"type":"string","description":"IANA timezone; optional, defaults to the server's configured zone"}},"required":["date"]}`,
			Tier:        TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a struct {
					Date string `json:"date"`
					TZ   string `json:"tz"`
				}
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				q.Set("date", a.Date)
				if a.TZ != "" {
					q.Set("tz", a.TZ)
				}
				return HTTPCall{Method: "GET", Path: "/context/daily", Query: q}, nil
			},
		},
		{
			Name:        "get_race_fueling",
			Description: "Race-day fuelling context. Call with no arguments to list the user's races and their dates (so you know how near a race is); call with race_id to get that race's per-leg fuelling plan (carbs/sodium/fluid). Use when a race is near and it should shape today's eating.",
			Schema:      `{"type":"object","properties":{"race_id":{"type":"string","description":"optional; omit to list races, supply to get one race's per-leg fuelling plan"}}}`,
			Tier:        TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a struct {
					RaceID string `json:"race_id"`
				}
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				if a.RaceID == "" {
					return HTTPCall{Method: "GET", Path: "/races"}, nil
				}
				return HTTPCall{Method: "GET", Path: "/races/" + url.PathEscape(a.RaceID) + "/fueling-plan"}, nil
			},
		},
		{
			Name:        "list_planned_meals",
			Description: "List planned meals (the user's selected-but-not-yet-eaten dishes) over a date range, inclusive. Use to see what's already planned before adding more.",
			Schema:      `{"type":"object","properties":{"from":{"type":"string","description":"YYYY-MM-DD"},"to":{"type":"string","description":"YYYY-MM-DD"}},"required":["from","to"]}`,
			Tier:        TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a struct {
					From string `json:"from"`
					To   string `json:"to"`
				}
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
			Name:        "list_shopping_items",
			Description: "List the shopping list. By default returns only unchecked (still-to-buy) items; pass include_checked=true to also see bought ones.",
			Schema:      `{"type":"object","properties":{"include_checked":{"type":"boolean"}}}`,
			Tier:        TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a struct {
					IncludeChecked bool `json:"include_checked"`
				}
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				if a.IncludeChecked {
					q.Set("include_checked", "true")
				}
				return HTTPCall{Method: "GET", Path: "/shopping/items", Query: q}, nil
			},
		},
		{
			Name:        "search_products",
			Description: "Search the user's product/recipe library by name or brand (recently-logged first). Check here for an existing recipe before web-searching Cookidoo for a new one.",
			Schema:      `{"type":"object","properties":{"q":{"type":"string"}},"required":["q"]}`,
			Tier:        TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a struct {
					Q string `json:"q"`
				}
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				q.Set("q", a.Q)
				return HTTPCall{Method: "GET", Path: "/products/search", Query: q}, nil
			},
		},
		{
			Name:        "get_product",
			Description: "Fetch a single product/recipe by id, including its nutriments and (for recipes) its ingredient list.",
			Schema:      `{"type":"object","properties":{"product_id":{"type":"string"}},"required":["product_id"]}`,
			Tier:        TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a struct {
					ProductID string `json:"product_id"`
				}
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "GET", Path: "/products/" + url.PathEscape(a.ProductID)}, nil
			},
		},

		// ---------- writes ----------
		{
			Name:        "import_cookidoo_recipe",
			Description: "Import a Cookidoo recipe into the library by URL. ALWAYS estimate the serving mass from the ingredients and pass serving_size_g so the nutriments are computed at import — omitting it leaves the recipe without nutriments. Re-importing the same URL is safe (returns the existing product).",
			Schema:      `{"type":"object","properties":{"url":{"type":"string"},"serving_size_g":{"type":"number","description":"grams per serving; strongly recommended so per-100g nutriments are computed"}},"required":["url"]}`,
			Tier:        TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a struct {
					URL          string   `json:"url"`
					ServingSizeG *float64 `json:"serving_size_g,omitempty"`
				}
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				body, _ := json.Marshal(a)
				return HTTPCall{Method: "POST", Path: "/products/import/cookidoo", Body: body}, nil
			},
		},
		{
			Name:        "update_product",
			Description: "Set a product's editable fields (name, serving_size_g, nutriments_per_100g). Use to fill in nutriments after importing a Cookidoo recipe without a serving size. Only supplied fields change.",
			Schema:      `{"type":"object","properties":{"product_id":{"type":"string"},"name":{"type":"string"},"serving_size_g":{"type":"number"},"nutriments_per_100g":{"type":"object","properties":{"kcal":{"type":"number"},"protein_g":{"type":"number"},"carbs_g":{"type":"number"},"fat_g":{"type":"number"},"fiber_g":{"type":"number"},"sugar_g":{"type":"number"},"salt_g":{"type":"number"}}}},"required":["product_id"]}`,
			Tier:        TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a struct {
					ProductID         string          `json:"product_id"`
					Name              *string         `json:"name,omitempty"`
					ServingSizeG      *float64        `json:"serving_size_g,omitempty"`
					NutrimentsPer100g json.RawMessage `json:"nutriments_per_100g,omitempty"`
				}
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				body := map[string]any{}
				if a.Name != nil {
					body["name"] = *a.Name
				}
				if a.ServingSizeG != nil {
					body["serving_size_g"] = *a.ServingSizeG
				}
				if len(a.NutrimentsPer100g) > 0 {
					body["nutriments_per_100g"] = a.NutrimentsPer100g
				}
				b, _ := json.Marshal(body)
				return HTTPCall{Method: "PATCH", Path: "/products/" + url.PathEscape(a.ProductID), Body: b}, nil
			},
		},
		{
			Name:        "create_planned_meal",
			Description: "Plan a meal: a selected dish for a date and slot. slot is breakfast|lunch|dinner|snack. Use after the user picks an option; product_id must reference an existing product/recipe.",
			Schema:      `{"type":"object","properties":{"plan_date":{"type":"string","description":"YYYY-MM-DD"},"slot":{"type":"string","enum":["breakfast","lunch","dinner","snack"]},"product_id":{"type":"string"},"quantity_g":{"type":"number"},"notes":{"type":"string"}},"required":["plan_date","slot","product_id"]}`,
			Tier:        TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				return Passthrough("POST", "/plan", in)
			},
		},
		{
			Name:        "update_planned_meal",
			Description: "Update a planned meal: change status (planned↔skipped only; use mark_planned_meal_eaten to log), quantity_g, slot, plan_date, or notes.",
			Schema:      `{"type":"object","properties":{"plan_id":{"type":"string"},"status":{"type":"string","enum":["planned","skipped"]},"quantity_g":{"type":"number"},"slot":{"type":"string"},"plan_date":{"type":"string"},"notes":{"type":"string"}},"required":["plan_id"]}`,
			Tier:        TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				return PathParamPassthrough(in, "plan_id", "PATCH", "/plan/")
			},
		},
		{
			Name:        "mark_planned_meal_eaten",
			Description: "Mark a planned meal as eaten NOW — this logs a real meal entry. The only correct way to record that a planned meal was actually eaten. Optionally override quantity_g.",
			Schema:      `{"type":"object","properties":{"plan_id":{"type":"string"},"quantity_g":{"type":"number"},"logged_at":{"type":"string","description":"RFC3339; defaults to now, must not be in the future"}},"required":["plan_id"]}`,
			Tier:        TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a struct {
					PlanID    string          `json:"plan_id"`
					QuantityG json.RawMessage `json:"quantity_g,omitempty"`
					LoggedAt  json.RawMessage `json:"logged_at,omitempty"`
				}
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				if a.PlanID == "" {
					return HTTPCall{}, fmt.Errorf("plan_id is required")
				}
				body := map[string]any{}
				if len(a.QuantityG) > 0 {
					body["quantity_g"] = a.QuantityG
				}
				if len(a.LoggedAt) > 0 {
					body["logged_at"] = a.LoggedAt
				}
				b, _ := json.Marshal(body)
				return HTTPCall{Method: "POST", Path: "/plan/" + url.PathEscape(a.PlanID) + "/eaten", Body: b}, nil
			},
		},
		{
			Name:        "add_shopping_items",
			Description: "Add items to the shopping list in one call. MERGE and DEDUPE across recipes BEFORE calling — the list stores items verbatim and never aggregates (combine '1 onion' + '2 onions' into '3 onions' yourself). quantity_text is free text.",
			Schema:      `{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"name":{"type":"string"},"quantity_text":{"type":"string"},"recipe_product_id":{"type":"string"},"plan_date":{"type":"string"}},"required":["name"]}}},"required":["items"]}`,
			Tier:        TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				return Passthrough("POST", "/shopping/items", in)
			},
		},
		{
			Name:        "update_shopping_item",
			Description: "Update a shopping item: check/uncheck it (checked), or edit name/quantity_text.",
			Schema:      `{"type":"object","properties":{"item_id":{"type":"string"},"checked":{"type":"boolean"},"name":{"type":"string"},"quantity_text":{"type":"string"}},"required":["item_id"]}`,
			Tier:        TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				return PathParamPassthrough(in, "item_id", "PATCH", "/shopping/items/")
			},
		},
		{
			Name:        "clear_checked_shopping_items",
			Description: "Remove all checked (bought) items from the shopping list. Reports how many were cleared.",
			Schema:      `{"type":"object","properties":{}}`,
			Tier:        TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				q := url.Values{}
				q.Set("checked", "true")
				return HTTPCall{Method: "DELETE", Path: "/shopping/items", Query: q}, nil
			},
		},
	}
}
