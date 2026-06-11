package mcpserver

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// LookupProductByBarcodeArgs is the input schema for lookup_product_by_barcode.
type LookupProductByBarcodeArgs struct {
	Barcode string `json:"barcode" jsonschema:"the product barcode (EAN/UPC) to look up"`
	Refresh bool   `json:"refresh,omitempty" jsonschema:"force a fresh Open Food Facts fetch even if cached locally"`
}

// SearchProductsArgs is the input schema for search_products.
type SearchProductsArgs struct {
	Q string `json:"q" jsonschema:"substring to match against product name and brand; case-insensitive"`
}

func handleLookupProductByBarcode(ctx context.Context, c *apiClient, args LookupProductByBarcodeArgs) *mcp.CallToolResult {
	q := url.Values{}
	if args.Refresh {
		q.Set("refresh", "true")
	}
	status, body, err := c.Post(ctx, "/products/lookup/"+url.PathEscape(args.Barcode), q, nil, "")
	return toToolResult(status, body, err)
}

func handleSearchProducts(ctx context.Context, c *apiClient, args SearchProductsArgs) *mcp.CallToolResult {
	q := url.Values{}
	q.Set("q", args.Q)
	status, body, err := c.Get(ctx, "/products/search", q)
	return toToolResult(status, body, err)
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

func handleCreateRecipe(ctx context.Context, c *apiClient, args CreateRecipeArgs) *mcp.CallToolResult {
	body, err := json.Marshal(struct {
		Name         string                  `json:"name"`
		Components   []CreateRecipeComponent `json:"components"`
		ServingSizeG *float64                `json:"serving_size_g,omitempty"`
	}{
		Name:         args.Name,
		Components:   args.Components,
		ServingSizeG: args.ServingSizeG,
	})
	if err != nil {
		return toToolResult(0, nil, &transportError{inner: err})
	}
	key := effectiveIdempotencyKey(args.IdempotencyKey, "create_recipe", args)
	status, respBody, err := c.Post(ctx, "/products/recipes", nil, body, key)
	return toToolResult(status, respBody, err)
}

func handleRecomputeRecipe(ctx context.Context, c *apiClient, args RecomputeRecipeArgs) *mcp.CallToolResult {
	key := effectiveIdempotencyKey(args.IdempotencyKey, "recompute_recipe", args)
	status, respBody, err := c.Post(ctx, "/products/recipes/"+url.PathEscape(args.ProductID)+"/recompute", nil, nil, key)
	return toToolResult(status, respBody, err)
}

// ImportCookidooRecipeArgs is the input schema for import_cookidoo_recipe.
type ImportCookidooRecipeArgs struct {
	URL            string   `json:"url" jsonschema:"a Cookidoo recipe URL of the form https://cookidoo.<tld>/recipes/recipe/<locale>/<id>"`
	ServingSizeG   *float64 `json:"serving_size_g,omitempty" jsonschema:"optional grams of one serving; supply it to convert the page's per-serving nutrition to per-100g. Omit to import ingredients and metadata only — the response then carries needs_nutriments and a per-serving echo to convert and update later."`
	IdempotencyKey string   `json:"idempotency_key,omitempty" jsonschema:"optional retry key; if omitted, a stable key is derived from the args"`
}

func handleImportCookidooRecipe(ctx context.Context, c *apiClient, args ImportCookidooRecipeArgs) *mcp.CallToolResult {
	body, err := json.Marshal(struct {
		URL          string   `json:"url"`
		ServingSizeG *float64 `json:"serving_size_g,omitempty"`
	}{
		URL:          args.URL,
		ServingSizeG: args.ServingSizeG,
	})
	if err != nil {
		return toToolResult(0, nil, &transportError{inner: err})
	}
	key := effectiveIdempotencyKey(args.IdempotencyKey, "import_cookidoo_recipe", args)
	status, respBody, err := c.Post(ctx, "/products/import/cookidoo", nil, body, key)
	return toToolResult(status, respBody, err)
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

func handleListProducts(ctx context.Context, c *apiClient, args ListProductsArgs) *mcp.CallToolResult {
	q := url.Values{}
	if args.Source != nil && *args.Source != "" {
		q.Set("source", *args.Source)
	}
	if args.Limit != nil {
		q.Set("limit", strconv.Itoa(*args.Limit))
	}
	if args.Offset != nil {
		q.Set("offset", strconv.Itoa(*args.Offset))
	}
	status, body, err := c.Get(ctx, "/products", q)
	return toToolResult(status, body, err)
}

func handleDeleteProduct(ctx context.Context, c *apiClient, args DeleteProductArgs) *mcp.CallToolResult {
	key := effectiveIdempotencyKey(args.IdempotencyKey, "delete_product", args)
	status, respBody, err := c.Delete(ctx, "/products/"+url.PathEscape(args.ProductID), key)
	if err == nil && status == 204 {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: ""}}}
	}
	return toToolResult(status, respBody, err)
}

func registerProductsTools(server *mcp.Server, c *apiClient) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "lookup_product_by_barcode",
		Description: "Look up a packaged product by its barcode. First call hits Open Food Facts " +
			"and the result is cached locally; subsequent calls serve from cache. Set refresh=true " +
			"to force re-fetching upstream. On 404 with error=product_not_found, the response body " +
			"includes a 'next' hint pointing you at the log_meal_freeform tool to log the meal " +
			"with user-supplied nutriment estimates instead.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args LookupProductByBarcodeArgs) (*mcp.CallToolResult, any, error) {
		return handleLookupProductByBarcode(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "search_products",
		Description: "Search the locally cached product list by name or brand. Results include " +
			"products with `source` of `off`, `manual`, or `recipe`. Results are ranked " +
			"by most recently logged first — use this to find products the user has previously " +
			"recorded before deciding whether to lookup_product_by_barcode or log_meal_freeform.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args SearchProductsArgs) (*mcp.CallToolResult, any, error) {
		return handleSearchProducts(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "create_recipe",
		Description: "Create a reusable composite product (a recipe) from N component products plus " +
			"per-component grams. Use this for repeated multi-ingredient meals the user eats often " +
			"(e.g. their daily skyr-and-oats breakfast) — afterwards a single log_meal call covers " +
			"the whole meal. Nested recipes are not supported in v1: a component must be a manual " +
			"or OFF-sourced product, not another recipe.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args CreateRecipeArgs) (*mcp.CallToolResult, any, error) {
		return handleCreateRecipe(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "recompute_recipe",
		Description: "Recompute a recipe's stored nutriments from the current effective nutriments " +
			"of its components. Use this after a component product's nutriments changed (e.g. you " +
			"refreshed it from Open Food Facts) — meal entries already logged are unaffected, but " +
			"future logs of this recipe will reflect the new component values.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args RecomputeRecipeArgs) (*mcp.CallToolResult, any, error) {
		return handleRecomputeRecipe(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
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
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args ImportCookidooRecipeArgs) (*mcp.CallToolResult, any, error) {
		return handleImportCookidooRecipe(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "list_products",
		Description: "List products in the local cache, ordered most-recently-used first (recently " +
			"logged products surface at the top; stale leftovers sink). Optional source filter: " +
			"'off' | 'manual' | 'recipe'. Pagination via limit (1..200, default 50) and offset " +
			"(default 0). Pair with delete_product to clean up leftover test products from prior " +
			"sessions: list_products to enumerate, delete_product to remove the ids you don't want.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args ListProductsArgs) (*mcp.CallToolResult, any, error) {
		return handleListProducts(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "delete_product",
		Description: "Permanently delete a product. Historical meal entries that logged this " +
			"product are preserved: their snapshot columns are populated from the product's current " +
			"name and nutriments before the FK is nulled, so daily/range summaries still report " +
			"those meals correctly. If the product is used as a component of any recipe, the " +
			"request is rejected with 409 product_in_use_as_component and the body lists the " +
			"using recipes — delete or replace this product within those recipes first, then retry. " +
			"Deletion is idempotent under retry (subsequent calls return 404 product_not_found).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args DeleteProductArgs) (*mcp.CallToolResult, any, error) {
		return handleDeleteProduct(ctx, c, args), nil, nil
	})
}
