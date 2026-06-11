# Proposal: add-recipe-ingredients

## Why

Recipe products imported from Cookidoo carry nutriments and an `external_url`, but their ingredient lists are thrown away at import time — the Schema.org `Recipe` JSON-LD on every Cookidoo page includes a `recipeIngredient` string array that the Chrome extension never captures. Without ingredients, no shopping list can ever be derived from a recipe, which blocks the planned meal-recommendation + shopping-list flow (see `add-shopping-list`, `add-chat-backend`). A spike confirmed Cookidoo recipe pages serve full JSON-LD (name, ingredients, per-serving nutrition, yield, total time) to anonymous server-side fetches — no login session required.

## What Changes

- Recipe products gain an optional `ingredients` field: an ordered array of free-text ingredient strings (e.g. `"100 g Staudensellerie"`), stored verbatim, no product linking.
- New server-side import endpoint `POST /products/import/cookidoo` that fetches a Cookidoo recipe URL, parses the embedded `Recipe` JSON-LD, and creates a flat-imported `source=recipe` product with ingredients, `external_url`, serving metadata, and nutriments.
- Because Cookidoo nutrition is per-serving and products store `nutriments_per_100g`, the import endpoint accepts an optional caller-supplied `serving_size_g` to convert; when absent the product is created with ingredients + metadata but **no nutriments**, flagged for the caller (the chat agent estimates serving mass and PATCHes, or the user corrects).
- The Chrome extension additionally captures `recipeIngredient` from the page JSON-LD and includes it in its `POST /products` body.
- One new MCP tool `import_cookidoo_recipe` mirroring the import endpoint.

## Capabilities

### New Capabilities

_None — server-side import extends the existing `cookidoo-importer` capability; ingredient storage extends `products`._

### Modified Capabilities

- `products`: recipe products carry an optional ordered `ingredients` text array, accepted on create and returned on read.
- `cookidoo-importer`: gains a server-side import path (`POST /products/import/cookidoo`) alongside the extension; the extension now also captures `recipeIngredient`.
- `mcp-server`: new `import_cookidoo_recipe` tool issuing exactly one HTTP call to the import endpoint.

## Impact

- **DB**: one migration (`026+` — verify head at implementation) adding `ingredients` (jsonb or text[]) to `products`, nullable.
- **Code**: `internal/products` (types, repo, handlers, validation), new fetch+JSON-LD-parse client (sibling of `internal/off` — outbound HTTP, typed errors), `internal/mcpserver` (one tool + expected-tools list bump), `internal/httpserver` wiring, `extensions/cookidoo/` popup/content script.
- **Docs**: `task swag` after handler changes.
- **Dependencies**: none new in Go (stdlib HTML/JSON handling; JSON-LD blocks are plain `<script>` tags extractable without a DOM parser).
- **Sequencing**: independent and shippable alone; `add-chat-backend` and `add-shopping-list` build on it.
