package agenttools

import (
	"encoding/json"
	"net/url"
	"strconv"
)

// Products domain — packaged-product lookup/search, recipe creation/recompute,
// Cookidoo import, product listing and deletion. Ported from
// internal/mcpserver/tools_products.go onto the shared registry
// (unify-mcp-tool-registry). These are MCP-only specs; the arg structs and
// descriptions are byte-identical to the prior bespoke registrations so the
// announced schema and tool surface are unchanged.
//
// NOTE: search_products shares its NAME with a chat-surface tool already in the
// registry (nutritionPlannerSpecs). That is expected — the two surfaces filter
// independently (ChatRegistry vs MCPRegistry). This is a SEPARATE, byte-faithful
// MCP entry, not a merge.

func init() { registerMCPDomain(productsSpecs()) }

// LookupProductByBarcodeArgs is the input schema for lookup_product_by_barcode.
type LookupProductByBarcodeArgs struct {
	Barcode string `json:"barcode" jsonschema:"the product barcode (EAN/UPC) to look up"`
	Refresh bool   `json:"refresh,omitempty" jsonschema:"force a fresh Open Food Facts fetch even if cached locally"`
}

// SearchProductsArgs is the input schema for search_products.
type SearchProductsArgs struct {
	Q string `json:"q" jsonschema:"substring to match against product name and brand; case-insensitive"`
}

// CreateRecipeComponent is one ingredient of a CreateRecipeArgs.
type CreateRecipeComponent struct {
	ProductID string  `json:"product_id" jsonschema:"id of the component product (manual or OFF-cached, not a recipe)"`
	QuantityG float64 `json:"quantity_g" jsonschema:"grams of this component in the recipe; must be greater than zero"`
}

// CreateRecipeArgs is the input schema for create_recipe.
type CreateRecipeArgs struct {
	Name           string                  `json:"name" jsonschema:"name of the composite meal/recipe (e.g. 'Morning skyr bowl')"`
	Components     []CreateRecipeComponent `json:"components" jsonschema:"non-empty list of {product_id, quantity_g} entries; nested recipes not supported in v1"`
	ServingSizeG   *float64                `json:"serving_size_g,omitempty" jsonschema:"optional total grams of one serving; defaults to per-100g semantics"`
	IdempotencyKey string                  `json:"idempotency_key,omitempty" jsonschema:"optional retry key; if omitted, a stable key is derived from the args"`
}

// RecomputeRecipeArgs is the input schema for recompute_recipe.
type RecomputeRecipeArgs struct {
	ProductID      string `json:"product_id" jsonschema:"id of the recipe product to recompute"`
	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

// ImportCookidooRecipeArgs is the input schema for import_cookidoo_recipe.
type ImportCookidooRecipeArgs struct {
	URL            string   `json:"url" jsonschema:"a Cookidoo recipe URL of the form https://cookidoo.<tld>/recipes/recipe/<locale>/<id>"`
	ServingSizeG   *float64 `json:"serving_size_g,omitempty" jsonschema:"optional grams of one serving; supply it to convert the page's per-serving nutrition to per-100g. Omit to import ingredients and metadata only — the response then carries needs_nutriments and a per-serving echo to convert and update later."`
	IdempotencyKey string   `json:"idempotency_key,omitempty" jsonschema:"optional retry key; if omitted, a stable key is derived from the args"`
}

// ListProductsArgs is the input schema for list_products. All fields optional.
type ListProductsArgs struct {
	Source *string `json:"source,omitempty" jsonschema:"optional filter: 'off' | 'manual' | 'recipe'. Omit to list all."`
	Limit  *int    `json:"limit,omitempty"  jsonschema:"page size 1..200; default 50"`
	Offset *int    `json:"offset,omitempty" jsonschema:"page offset >= 0; default 0"`
}

// DeleteProductArgs is the input schema for delete_product.
type DeleteProductArgs struct {
	ProductID      string `json:"product_id" jsonschema:"id of the product to permanently delete"`
	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key; if omitted, a stable key is derived from the args"`
}

func productsSpecs() []Spec {
	return []Spec{
		{
			Name: "lookup_product_by_barcode",
			Description: "Look up a packaged product by its barcode. First call hits Open Food Facts " +
				"and the result is cached locally; subsequent calls serve from cache. Set refresh=true " +
				"to force re-fetching upstream. On 404 with error=product_not_found, the response body " +
				"includes a 'next' hint pointing you at the log_meal_freeform tool to log the meal " +
				"with user-supplied nutriment estimates instead.",
			SchemaType: LookupProductByBarcodeArgs{},
			Tier:       TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a LookupProductByBarcodeArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				if a.Refresh {
					q.Set("refresh", "true")
				}
				return HTTPCall{Method: "POST", Path: "/products/lookup/" + url.PathEscape(a.Barcode), Query: q}, nil
			},
		},
		{
			Name: "search_products",
			Description: "Search the locally cached product list by name or brand. Results include " +
				"products with `source` of `off`, `manual`, or `recipe`. Results are ranked " +
				"by most recently logged first — use this to find products the user has previously " +
				"recorded before deciding whether to lookup_product_by_barcode or log_meal_freeform.",
			SchemaType: SearchProductsArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a SearchProductsArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				q.Set("q", a.Q)
				return HTTPCall{Method: "GET", Path: "/products/search", Query: q}, nil
			},
		},
		{
			Name: "create_recipe",
			Description: "Create a reusable composite product (a recipe) from N component products plus " +
				"per-component grams. Use this for repeated multi-ingredient meals the user eats often " +
				"(e.g. their daily skyr-and-oats breakfast) — afterwards a single log_meal call covers " +
				"the whole meal. Nested recipes are not supported in v1: a component must be a manual " +
				"or OFF-sourced product, not another recipe.",
			SchemaType: CreateRecipeArgs{},
			Tier:       TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a CreateRecipeArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				body, err := json.Marshal(struct {
					Name         string                  `json:"name"`
					Components   []CreateRecipeComponent `json:"components"`
					ServingSizeG *float64                `json:"serving_size_g,omitempty"`
				}{
					Name:         a.Name,
					Components:   a.Components,
					ServingSizeG: a.ServingSizeG,
				})
				if err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "POST", Path: "/products/recipes", Body: body}, nil
			},
		},
		{
			Name: "recompute_recipe",
			Description: "Recompute a recipe's stored nutriments from the current effective nutriments " +
				"of its components. Use this after a component product's nutriments changed (e.g. you " +
				"refreshed it from Open Food Facts) — meal entries already logged are unaffected, but " +
				"future logs of this recipe will reflect the new component values.",
			SchemaType: RecomputeRecipeArgs{},
			Tier:       TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a RecomputeRecipeArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "POST", Path: "/products/recipes/" + url.PathEscape(a.ProductID) + "/recompute"}, nil
			},
		},
		{
			Name: "import_cookidoo_recipe",
			Description: "Import a Thermomix/Cookidoo recipe into the product library by URL. The server " +
				"fetches the public recipe page, parses its structured data, and creates a source=recipe " +
				"product with the verbatim ingredient list and a link back to Cookidoo. Cookidoo reports " +
				"nutrition per serving with no serving mass: pass serving_size_g to convert to per-100g, " +
				"or omit it to import ingredients + metadata only — the response then has " +
				"needs_nutriments=true plus a nutrition_per_serving echo, so you can estimate the serving " +
				"mass from the ingredients and update the product afterwards. Re-importing the same URL is " +
				"safe: it returns the existing product with already_imported=true and never overwrites " +
				"manual corrections.",
			SchemaType: ImportCookidooRecipeArgs{},
			Tier:       TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a ImportCookidooRecipeArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				body, err := json.Marshal(struct {
					URL          string   `json:"url"`
					ServingSizeG *float64 `json:"serving_size_g,omitempty"`
				}{
					URL:          a.URL,
					ServingSizeG: a.ServingSizeG,
				})
				if err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "POST", Path: "/products/import/cookidoo", Body: body}, nil
			},
		},
		{
			Name: "list_products",
			Description: "List products in the local cache, ordered most-recently-used first (recently " +
				"logged products surface at the top; stale leftovers sink). Optional source filter: " +
				"'off' | 'manual' | 'recipe'. Pagination via limit (1..200, default 50) and offset " +
				"(default 0). Pair with delete_product to clean up leftover test products from prior " +
				"sessions: list_products to enumerate, delete_product to remove the ids you don't want.",
			SchemaType: ListProductsArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a ListProductsArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				if a.Source != nil && *a.Source != "" {
					q.Set("source", *a.Source)
				}
				if a.Limit != nil {
					q.Set("limit", strconv.Itoa(*a.Limit))
				}
				if a.Offset != nil {
					q.Set("offset", strconv.Itoa(*a.Offset))
				}
				return HTTPCall{Method: "GET", Path: "/products", Query: q}, nil
			},
		},
		{
			Name: "delete_product",
			Description: "Permanently delete a product. Historical meal entries that logged this " +
				"product are preserved: their snapshot columns are populated from the product's current " +
				"name and nutriments before the FK is nulled, so daily/range summaries still report " +
				"those meals correctly. If the product is used as a component of any recipe, the " +
				"request is rejected with 409 product_in_use_as_component and the body lists the " +
				"using recipes — delete or replace this product within those recipes first, then retry. " +
				"Deletion is idempotent under retry (subsequent calls return 404 product_not_found).",
			SchemaType: DeleteProductArgs{},
			Tier:       TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a DeleteProductArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "DELETE", Path: "/products/" + url.PathEscape(a.ProductID)}, nil
			},
		},
	}
}
