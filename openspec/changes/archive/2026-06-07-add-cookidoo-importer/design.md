## Context

The backend already has the right shape for what we want: a `products` table whose `source` enum permits `recipe`, a `Nutriments` block that covers macros plus the eight micronutrients shipped by `daily-use-essentials`, and a meals path that logs against any product by id. The composed-recipe flow (`product_components`, `UpdateRecipeNutriments`, `NutrimentComputedAt`) is the right answer when the user composes a meal from their own pantry. Cookidoo recipes are something different — they're pre-computed by Vorwerk and we trust their numbers — so they get stored flat, the same way an OFF lookup stores its parsed nutriments without re-deriving them.

The only thing missing on the backend is provenance: where did this product come from, so we can show "Open in Cookidoo" later and tell at a glance whether a recipe is hand-composed or externally sourced. One nullable column does it.

On the frontend, Chrome MV3 + Schema.org JSON-LD is the smallest extension that actually works. JSON-LD is a public, documented schema that Cookidoo (and most other recipe sites) embeds for Google's recipe rich results. Reading it from the page DOM in the user's authenticated session sidesteps the "no public API" problem entirely.

## Goals / Non-Goals

**Goals:**

- A user looking at a Cookidoo recipe in Chrome can save it to nutrition-api in two clicks: toolbar button, then Save.
- The saved product carries the recipe's Cookidoo URL so it can be linked back to.
- Search (`/products/search?q=…`) and the MCP `search_products` tool surface saved recipes the same way they surface OFF products.
- Logging is unchanged: the mobile app and the agent log meals against the recipe product like any other product.
- The backend change is small enough to never need maintenance: one column, one validator, no new endpoint.

**Non-Goals:**

- Firefox and Safari parity.
- Ingredient decomposition (the existing composed-recipe path covers that use case for hand-built recipes).
- Recipe sync (one-shot import; no re-fetch on Cookidoo updates).
- Backend-side scraping (the user's browser holds the session).
- Dedup by external_url, undo/edit-in-place, or any extension UI beyond Preview + Save.

## Decisions

### 1. One column: `products.external_url TEXT NULLABLE`

The column is the entire backend schema change. A nullable TEXT keeps the shape boring: OFF products have it `null`, manual products have it `null`, composed recipes have it `null`, only flat-imported recipes (and other future externally-sourced rows) carry a value.

Length bound is implicit (PostgreSQL TEXT has no practical limit); we'll validate at the handler layer with a soft cap of 2048 characters to prevent pathological payloads. Cookidoo URLs are well under 200.

**Alternatives considered:**
- *Separate `product_sources` table joined 1:1.* Premature normalisation for one nullable string.
- *JSONB blob with arbitrary provenance metadata.* Too open-ended; YAGNI.
- *URL on the `meal_entries` instead.* Wrong layer — the provenance lives with the reusable recipe, not with each portion logged.

### 2. `POST /products` accepts `source` and `external_url` (no new endpoint)

Today the handler hard-codes `source = manual`. We extend the request body:

```json
{
  "name": "Lasagne Bolognese",
  "brand": "Cookidoo",                       // optional
  "source": "recipe",                         // optional, default "manual"
  "external_url": "https://cookidoo.de/recipes/recipe/de-DE/r-xxxxxxx",  // optional
  "serving_size_g": 350,
  "nutriments_per_100g": { "kcal": 166, "protein_g": 9.4, ... }
}
```

Validation rules:
- `source` ∈ {`"manual"`, `"recipe"`}; anything else → `400 source_invalid`.
- `external_url`, when present, must be ≤ 2048 chars; longer → `400 external_url_too_long`.
- `external_url` is allowed regardless of `source` (manual products may carry a URL too — e.g. a homemade dish you wrote up in a note app); the field's presence does not gate behaviour.
- `nutriments_per_100g` is required when `source=recipe` is flat-imported (no components). The handler does not currently require it for manual products either, so the existing leniency is preserved; the extension's popup enforces presence client-side.

The response includes `external_url` either way. The current `Product` JSON shape is additive-compatible.

**Alternatives considered:**
- *New `POST /products/from-recipe-import` endpoint.* Cleaner API surface but more code, more docs, more MCP tool definitions if we ever expose it. The existing endpoint with a `source` field carries its weight.
- *Infer `source=recipe` from `external_url` presence.* Fragile: a manual product whose name field references a URL would silently switch source. Explicit is clearer.

### 3. `source=recipe` permits two distinct shapes

| Shape                       | `external_url` | `nutriment_computed_at` | `product_components` rows |
|-----------------------------|----------------|--------------------------|---------------------------|
| Composed (existing)         | null           | non-null (set by recompute) | one or more                |
| Flat-imported (this change) | non-null       | null (Cookidoo computed it for us) | zero                       |

We do NOT add a check constraint enforcing exclusivity in v1. Two reasons: the existing recompute pipeline already overwrites `nutriment_computed_at`, so if you later add components to a flat-imported recipe and let it recompute, the row mutates cleanly to the composed shape; and the user might legitimately want a "composed recipe I also keep a link to" later. A spec scenario documents the expected pattern; a runtime invariant can come later if drift surfaces.

**Alternatives considered:**
- *Add a `CHECK (external_url IS NULL OR <no components>)` constraint.* Overzealous; would block the legitimate "composed + linked" case.
- *Introduce a separate `recipe_imports` table.* Over-modelling; the column on `products` is the right surface.

### 4. Chrome extension architecture (MV3, vanilla JS)

```
extensions/cookidoo/
├── manifest.json          MV3; host_permissions: ["https://cookidoo.*/*",
│                                                  "https://*.cookidoo.*/*"]
├── content_script.js      runs on cookidoo.*/recipes/recipe/* — reads JSON-LD,
│                          stores extracted recipe on chrome.storage.session
│                          keyed by tab id, broadcasts via runtime.sendMessage
├── popup.html             toolbar button → preview form (name, servings,
│                          serving_size_g, all per-100g nutriments — editable
│                          before save)
├── popup.js               on open: reads stored recipe for current tab,
│                          populates form. On Save: validates, builds the
│                          POST body (per-100g conversion done client-side),
│                          POSTs to /products, shows success or error
├── options.html           one-time setup form
├── options.js             persists API_URL, TOKEN, TOKEN_TYPE to
│                          chrome.storage.sync
├── icons/                 16, 48, 128 px PNGs
└── README.md              install instructions + options + limits
```

No bundler, no npm. Plain ES modules. The whole thing is inspectable.

The service worker (`background.js`) is intentionally absent — MV3 allows extensions with no background script if message routing is only between content script ↔ popup, both of which can talk directly via `chrome.runtime`.

**Alternatives considered:**
- *Use a bundler (esbuild, vite) and TypeScript.* Saves typing typos but adds a build step we don't otherwise need. The extension is ~300 lines total; plain JS is right.
- *Inject a UI overlay onto the Cookidoo page itself.* Cleaner-feeling but tangles us with Cookidoo's CSS and SPA navigation. The toolbar popup is the standard MV3 pattern and stays out of their DOM.

### 5. JSON-LD parsing and per-100g conversion

Schema.org `Recipe` has a `nutrition` block (type `NutritionInformation`) with string-encoded values: `"calories": "580 kcal"`, `"proteinContent": "33 g"`, etc. These are **per serving** by convention. To produce our per-100g shape:

```
per_100g = per_serving × (100 / serving_size_g)
```

`serving_size_g` comes from one of (in order of preference):
1. `recipe.nutrition.servingSize` parsed as a gram value (e.g. `"350 g"`).
2. The recipe's `recipeYield` weight if Cookidoo provides it (some recipes carry a total weight elsewhere in JSON-LD).
3. **Manual entry in the popup** — the form field is pre-empty and `Save` is disabled until the user fills it.

If `recipe.nutrition` is missing entirely, the popup shows the recipe name and yields with all nutriment fields empty; the user can fill them in manually before saving. We never silently insert zeros for missing macros — a `null` is honest about the gap.

Cookidoo URLs follow `https://cookidoo.<tld>/recipes/recipe/<lang>/<slug>` across country instances. The content-script URL match uses `*recipes/recipe/*` to cover the whole spread without enumerating every TLD. The host_permissions block uses two patterns to cover both bare-domain instances (`cookidoo.de`, `cookidoo.international`) and subdomain ones if they appear.

**Alternatives considered:**
- *Use a JSON-LD parser library.* `application/ld+json` blocks are just JSON; we parse with `JSON.parse`. Zero dependencies, zero attack surface.
- *Walk the rendered DOM for nutriment cards.* Brittle to Cookidoo's CSS/SPA layout. JSON-LD is intentionally stable for Google.
- *Treat per-serving values as already per-100g.* Common bug; would inflate kcal by 3-5×. We explicitly convert.

### 6. Token storage and choice

The extension's options page asks for the API URL, the token, and which token type (`mobile` or `agent`). It defaults to **mobile** because the extension is a user-facing input device (the user is manually approving each save), not an autonomous agent. The token type is sent in the `Authorization` header as `Bearer <token>`; the backend doesn't differentiate between the two tokens at authorization time, but logs the resolved `client_id` for audit.

Both tokens are equally valid. The default and the radio just let the user choose how they want the saves to appear in their request logs.

Storage uses `chrome.storage.sync` so the same token follows the user across Chrome instances they're signed in to. For local-only workflows the user can leave it on the default localhost URL; for remote deployments the user pastes the public URL.

**Alternatives considered:**
- *Hardcode the mobile token assumption.* Less flexible; users may prefer separating "extension activity" into the agent log line.
- *Use `chrome.storage.local` so credentials never leave the machine.* Reasonable trade-off; the spec leaves storage backend as an implementation detail with `sync` as the default.

## Risks / Trade-offs

- **Cookidoo could change their page format.** If they drop JSON-LD or change `@type`, the content script silently fails to find a recipe and the popup shows "no recipe detected on this page" — graceful degradation. *Mitigation:* the popup always permits manual entry of every field, so a Cookidoo format change is "extension stops auto-filling" not "extension breaks."
- **JSON-LD per-serving values may use kJ instead of kcal.** Schema.org's `calories` is documented as kcal, but recipe sites violate this. *Mitigation:* the parser checks the unit string; if "kJ" appears, divide by 4.184. Documented in the spec.
- **The extension's token sits in `chrome.storage.sync` which Chrome syncs across devices.** *Mitigation:* the README's "limits" section is explicit; users wanting stricter isolation can switch to `chrome.storage.local`. For localhost defaults the risk is essentially zero.
- **Schema.org `servingSize` is free-text.** Cookidoo may format it as "1 portion", "1 piece", or in a foreign language. *Mitigation:* the popup requires `serving_size_g` to be a positive number before Save is enabled; the auto-parse is best-effort and falls back to manual entry.
- **The first product imported from a non-Cookidoo recipe site that happens to match `*recipes/recipe/*` would still trigger the content script.** *Mitigation:* the manifest's `host_permissions` constrains the script to Cookidoo TLDs; the URL pattern is a path-only filter against an already-narrowed origin set.
- **No dedup means re-imports create duplicate rows.** *Mitigation:* the product search ranks by `last_logged_at` desc — duplicates float to the bottom and the user notices. Adding a `UNIQUE` on `external_url` is a tiny follow-up if it becomes a problem.

## Migration Plan

- Single forward migration adds the `external_url` column. No data backfill required (defaults to NULL).
- Rollback drops the column. No data loss beyond externally-sourced URLs that were stored after the change shipped.
- The extension is shipped unpacked from the repo; users load it via Chrome's "Load unpacked" dev flow. A signed Chrome Web Store listing is a future change.

## Open Questions

- Whether the extension should also support recipe sites beyond Cookidoo on day one (Chefkoch, NYT Cooking, BBC GoodFood). The JSON-LD path is generic; only the `host_permissions` and the URL matcher would need broadening. Tentative answer: ship Cookidoo-only first, broaden when we have a second concrete site we use.
- Whether to add a `UNIQUE` index on `external_url` to enforce single-import semantics. Tentative answer: no in v1 — see "dedup" trade-off — but if the user reports the duplicate problem, a unique index plus an "update existing" code path is a small follow-up.
- Whether to surface the import-recipe path through an MCP tool (e.g., the agent could "remember this recipe" given a URL the user mentions in chat). Out of scope here; needs server-side fetch (which is the very thing this design avoids) or a different agent-to-extension bridge.
