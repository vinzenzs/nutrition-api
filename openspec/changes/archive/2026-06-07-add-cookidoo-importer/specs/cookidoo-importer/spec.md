## ADDED Requirements

### Requirement: A Chrome extension imports Cookidoo recipes via Schema.org JSON-LD

The system SHALL provide a Chrome MV3 extension at `extensions/cookidoo/` that, when the user is viewing a Cookidoo recipe page, can extract the page's embedded Schema.org `Recipe` JSON-LD and save it to nutrition-api as a `source=recipe` product with the Cookidoo URL preserved in `external_url`.

#### Scenario: Extension activates on Cookidoo recipe URLs

- **WHEN** the user navigates Chrome to a URL matching `https://cookidoo.<tld>/<...>/recipes/recipe/<...>`
- **THEN** the extension's content script runs in the page
- **AND** attempts to extract a recipe from any `<script type="application/ld+json">` whose decoded JSON `@type` is `"Recipe"` (or includes `"Recipe"` in an array)

#### Scenario: Toolbar popup previews the extracted recipe

- **WHEN** the user clicks the extension toolbar button while on a Cookidoo recipe page that yielded a parsed recipe
- **THEN** the popup opens with form fields pre-populated from the JSON-LD: `name`, `servings`, `serving_size_g`, and all per-100g nutriment fields the JSON-LD provided
- **AND** the URL of the current tab is captured into a hidden `external_url` field

#### Scenario: No JSON-LD on the page

- **WHEN** the user clicks the toolbar button on a Cookidoo page whose JSON-LD does not contain a Recipe block
- **THEN** the popup opens with all fields empty
- **AND** a banner notes "no recipe detected on this page" without blocking the user from filling in the fields manually

### Requirement: All extracted fields are editable before save

The system SHALL allow the user to edit every parsed field in the popup before saving, including the recipe name, servings, serving size in grams, and every individual nutriment.

#### Scenario: User corrects the parsed servings

- **WHEN** the JSON-LD says the recipe yields 4 servings but the user knows it's really 6
- **THEN** the user can change the `servings` field in the popup
- **AND** the saved product reflects the user's value (the popup does not auto-derive `serving_size_g` from `servings` unless the user explicitly asks via a "recompute" affordance)

#### Scenario: User leaves a missing nutriment empty

- **WHEN** the JSON-LD did not include a salt value
- **THEN** the popup leaves the `salt_g` field empty
- **AND** Save submits the product with `salt_g` absent (the backend stores `null`, not `0`)

### Requirement: Save POSTs to nutrition-api as a flat recipe product

The system SHALL convert the popup's form state into a `POST /products` request body and send it to the configured API base URL using the configured token.

#### Scenario: Save produces the canonical request shape

- **WHEN** the user clicks Save with a complete form
- **THEN** the extension sends `POST <API_BASE_URL>/products` with the headers `Authorization: Bearer <TOKEN>` and `Content-Type: application/json`
- **AND** the body is `{"name": ..., "source": "recipe", "external_url": ..., "serving_size_g": ..., "nutriments_per_100g": { ... }}` where `nutriments_per_100g` contains only the fields the user provided (omitted keys for empty fields)

#### Scenario: Server-side errors are surfaced

- **WHEN** the backend responds with a non-2xx status
- **THEN** the popup shows the response status and the JSON error body verbatim
- **AND** does not close the popup, so the user can edit and retry

#### Scenario: Network failure is surfaced

- **WHEN** the request cannot complete (DNS failure, CORS, server down)
- **THEN** the popup shows "could not reach <API_BASE_URL>" with the underlying error string
- **AND** does not close the popup

### Requirement: Per-100g conversion uses the popup's serving_size_g

The system SHALL convert Schema.org per-serving nutriment values into per-100g values using the popup's `serving_size_g` field via `per_100g = per_serving × (100 / serving_size_g)`.

#### Scenario: Standard conversion from grams

- **WHEN** the JSON-LD provides `calories: "580 kcal"` per serving and the popup's `serving_size_g` is `350`
- **THEN** the saved `nutriments_per_100g.kcal` is `≈ 165.7` (580 × 100 / 350)

#### Scenario: kJ values are converted to kcal

- **WHEN** the JSON-LD provides energy in kJ (e.g. `"2425 kJ"`) and no kcal value
- **THEN** the extension converts to kcal via `kcal = kJ / 4.184` before the per-100g conversion

#### Scenario: Serving size not in grams forces manual entry

- **WHEN** the JSON-LD's `servingSize` does not parse as a gram quantity (e.g. `"1 cup"`, `"1 portion"`)
- **THEN** the popup leaves `serving_size_g` empty
- **AND** the Save button is disabled until the user enters a positive numeric value

### Requirement: Options page persists API URL, token, and token type

The system SHALL provide an options page where the user configures the API base URL, the API token, and whether the token is the mobile token or the agent token. The values SHALL be persisted to `chrome.storage.sync`.

#### Scenario: First-run options are empty

- **WHEN** the user opens the options page for the first time
- **THEN** the API URL defaults to `http://localhost:8080`
- **AND** the token field is empty
- **AND** the token type radio defaults to "mobile"

#### Scenario: Saved values survive Chrome restarts

- **WHEN** the user saves options, restarts Chrome, and opens the popup on a recipe page
- **THEN** the Save action uses the persisted API URL and token without re-prompting

#### Scenario: Missing token blocks save

- **WHEN** the user opens the popup without having set a token in the options
- **THEN** the popup shows "configure the extension options before saving" with a link to the options page
- **AND** the Save button is disabled until a token is configured

### Requirement: Extension scope is documented in the repo

The system SHALL ship a README at `extensions/cookidoo/README.md` covering installation in Chrome (Load unpacked), the options that must be set, and the explicit limits (Chrome only, Cookidoo JSON-LD dependent, no auto-sync, no Cookidoo login).

#### Scenario: README covers the install flow

- **WHEN** a new user reads `extensions/cookidoo/README.md`
- **THEN** they find step-by-step instructions to load the extension as unpacked, set the API URL and token, and import their first recipe
