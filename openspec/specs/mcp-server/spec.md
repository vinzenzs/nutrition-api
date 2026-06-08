# mcp-server Specification

## Purpose

Define the MCP server that exposes the nutrition REST API's agent-relevant endpoints as MCP tools over stdio, with idempotent writes, agent-shaped errors, and environment-driven configuration.

## Requirements

### Requirement: MCP server runs over stdio with a tool-only surface

The system SHALL provide an MCP server binary at `cmd/mcp/` that communicates with the agent runtime over stdio using JSON-RPC, exposing only tools (no resources or prompts in v1).

#### Scenario: Binary starts and registers tools on stdin connection

- **WHEN** the agent runtime spawns the MCP binary and opens stdin
- **THEN** the binary registers the twelve tools defined in this spec and announces them on the MCP `initialize` exchange
- **AND** the binary remains running, processing JSON-RPC requests over stdin

#### Scenario: Binary exits on stdin EOF or SIGTERM

- **WHEN** the agent runtime closes stdin or sends SIGTERM
- **THEN** the binary stops accepting new requests
- **AND** exits with status 0 within 2 seconds

#### Scenario: Binary refuses to start without an agent token

- **WHEN** the process starts without `AGENT_API_TOKEN` set or set to an empty string
- **THEN** the binary writes an error to stderr identifying the missing variable
- **AND** exits with a non-zero status code before announcing any tools

#### Scenario: Binary smoke-checks the REST API before announcing tools

- **WHEN** the process starts
- **THEN** the binary issues `GET <NUTRITION_API_URL>/healthz` once
- **AND** if it fails, logs the failure to stderr but still announces tools (so the agent can surface the error on the first tool call rather than the process disappearing silently)

### Requirement: Eight MCP tools mirror the agent-relevant REST endpoints

The system SHALL expose the following tools, each invoking the corresponding REST endpoint with `Authorization: Bearer <AGENT_API_TOKEN>`.

#### Scenario: lookup_product_by_barcode tool calls the REST lookup endpoint

- **WHEN** the agent calls `lookup_product_by_barcode` with `{"barcode":"3017624010701"}`
- **THEN** the wrapper issues `POST /products/lookup/3017624010701` to the REST API
- **AND** returns the REST 200 response body as the tool result content

#### Scenario: lookup_product_by_barcode forwards the refresh flag

- **WHEN** the agent calls `lookup_product_by_barcode` with `{"barcode":"X","refresh":true}`
- **THEN** the wrapper issues `POST /products/lookup/X?refresh=true`

#### Scenario: search_products tool forwards the query string

- **WHEN** the agent calls `search_products` with `{"q":"yogurt"}`
- **THEN** the wrapper issues `GET /products/search?q=yogurt`

#### Scenario: log_meal tool calls the REST meals endpoint

- **WHEN** the agent calls `log_meal` with `{"product_id":"<uuid>","quantity_g":150,"logged_at":"2026-06-06T12:30:00Z"}`
- **THEN** the wrapper issues `POST /meals` with the same JSON body
- **AND** returns the REST 201 response body as the tool result content

#### Scenario: log_meal_freeform tool calls the REST freeform endpoint

- **WHEN** the agent calls `log_meal_freeform` with `{"name":"banana","nutriments_per_100g":{"kcal":89},"quantity_g":120,"logged_at":"2026-06-06T10:00:00Z"}`
- **THEN** the wrapper issues `POST /meals/freeform` with the same JSON body

#### Scenario: patch_meal tool calls the REST patch endpoint

- **WHEN** the agent calls `patch_meal` with `{"meal_id":"<uuid>","quantity_g":200}`
- **THEN** the wrapper issues `PATCH /meals/<uuid>` with body `{"quantity_g":200}`

#### Scenario: delete_meal tool calls the REST delete endpoint

- **WHEN** the agent calls `delete_meal` with `{"meal_id":"<uuid>"}`
- **THEN** the wrapper issues `DELETE /meals/<uuid>`
- **AND** returns an empty tool result content when the REST response is 204

#### Scenario: daily_summary tool calls the REST daily endpoint

- **WHEN** the agent calls `daily_summary` with `{"date":"2026-06-06","tz":"Europe/Berlin"}`
- **THEN** the wrapper issues `GET /summary/daily?date=2026-06-06&tz=Europe/Berlin`

#### Scenario: daily_summary omits tz when not supplied

- **WHEN** the agent calls `daily_summary` with `{"date":"2026-06-06"}`
- **THEN** the wrapper issues `GET /summary/daily?date=2026-06-06` (no `tz` param)
- **AND** the REST API's default-tz behaviour determines the timezone

#### Scenario: range_summary tool forwards from/to/tz

- **WHEN** the agent calls `range_summary` with `{"from":"2026-06-01","to":"2026-06-07","tz":"UTC"}`
- **THEN** the wrapper issues `GET /summary/range?from=2026-06-01&to=2026-06-07&tz=UTC`

### Requirement: Write tools auto-derive idempotency keys when none is supplied

The system SHALL set the `Idempotency-Key` HTTP header on every POST-style write tool call (`log_meal`, `log_meal_freeform`, `patch_meal`, `delete_meal`, `create_recipe`, `recompute_recipe`). When the tool input contains an explicit `idempotency_key`, that value SHALL be used; otherwise the wrapper SHALL compute a stable key as `sha256_hex(<tool_name> + "|" + canonical_json(<tool_args_without_idempotency_key>))`. PUT-style write tools (`set_goals` today, plus any future PUT-backed tool) SHALL NOT expose an `idempotency_key` field in their input schema and SHALL NOT set the `Idempotency-Key` header on the backend request; the backend rejects the header on PUT with `400 idempotency_unsupported_for_put` regardless.

#### Scenario: Explicit idempotency_key is forwarded verbatim

- **WHEN** the agent calls `log_meal_freeform` with `{"name":"X","quantity_g":100,"logged_at":"…","idempotency_key":"abc-123"}`
- **THEN** the REST request carries `Idempotency-Key: abc-123`

#### Scenario: Missing idempotency_key is derived from arguments

- **WHEN** the agent calls `log_meal_freeform` twice in a row with byte-identical other arguments and no `idempotency_key`
- **THEN** both REST requests carry the same auto-derived `Idempotency-Key` header
- **AND** the second call returns the same meal id as the first

#### Scenario: Different arguments produce different auto-derived keys

- **WHEN** the agent calls `log_meal_freeform` twice with the same name but different `quantity_g`
- **THEN** the two REST requests carry different `Idempotency-Key` headers
- **AND** both meals are persisted independently

#### Scenario: Derivation excludes the idempotency_key field from the canonical form

- **WHEN** the agent calls `log_meal_freeform` once with no `idempotency_key` and once with the same args plus an explicit `idempotency_key`
- **THEN** the two REST requests carry different `Idempotency-Key` values (the explicit one wins; the auto-derived one is not used)

#### Scenario: set_goals does not expose idempotency_key

- **WHEN** the agent inspects the `set_goals` tool input schema
- **THEN** the schema does NOT include an `idempotency_key` property

#### Scenario: set_goals does not send Idempotency-Key

- **WHEN** the agent calls `set_goals` with any input
- **THEN** the wrapper issues the `PUT /goals` request without an `Idempotency-Key` header
- **AND** the wrapper does NOT auto-derive a key for this tool

#### Scenario: A PUT-style tool's description points users at retry-safety

- **WHEN** the agent reads the `set_goals` tool description
- **THEN** the description notes that retries of `set_goals` may land twice on transient network failure
- **AND** points future work at ETag/If-Match optimistic concurrency

### Requirement: REST errors are returned as MCP tool results with isError=true

The system SHALL surface REST 4xx and 5xx responses as MCP tool results carrying the REST response body verbatim, with `isError=true` so the agent runtime classifies the call as failed.

#### Scenario: product_not_found is forwarded with the next hint

- **WHEN** the REST API responds to `lookup_product_by_barcode` with `404` and body `{"error":"product_not_found","barcode":"X","next":"POST /meals/freeform"}`
- **THEN** the tool result has `isError=true`
- **AND** the content payload contains the JSON body unchanged, including the `next` field

#### Scenario: idempotency_key_conflict is forwarded

- **WHEN** the REST API responds with `409` and body `{"error":"idempotency_key_conflict"}`
- **THEN** the tool result has `isError=true`
- **AND** the content payload contains the JSON body unchanged

#### Scenario: upstream_timeout is forwarded

- **WHEN** the REST API responds with `504` and body `{"error":"upstream_timeout","retry_after_seconds":30}`
- **THEN** the tool result has `isError=true`
- **AND** the content payload contains the JSON body unchanged

#### Scenario: Transport-level failures produce a synthetic error envelope

- **WHEN** the wrapper cannot reach the REST API (DNS failure, connection refused, timeout)
- **THEN** the tool result has `isError=true`
- **AND** the content payload is `{"error":"transport","detail":"<description>"}`

### Requirement: Configuration is read from environment variables

The system SHALL read its configuration from environment variables at process start and refuse to start when required variables are missing.

#### Scenario: Missing AGENT_API_TOKEN halts startup

- **WHEN** the process starts with `AGENT_API_TOKEN` unset or empty
- **THEN** the binary writes an error to stderr identifying the variable
- **AND** exits with a non-zero status code

#### Scenario: NUTRITION_API_URL defaults to localhost

- **WHEN** the process starts without `NUTRITION_API_URL` set
- **THEN** tool calls target `http://localhost:8080`

#### Scenario: Per-request timeout is configurable

- **WHEN** the process starts with `MCP_REQUEST_TIMEOUT_SECONDS=20`
- **THEN** the wrapper applies a 20-second timeout on each REST call

#### Scenario: Default per-request timeout is 10 seconds

- **WHEN** the process starts without `MCP_REQUEST_TIMEOUT_SECONDS` set
- **THEN** the wrapper applies a 10-second timeout on each REST call

### Requirement: Recipe and goals tools mirror new REST endpoints

The MCP server SHALL expose four new tools that map onto the REST endpoints added by the daily-use-essentials change. Each tool invokes its REST endpoint with `Authorization: Bearer <AGENT_API_TOKEN>` and forwards the REST response body as the tool's content, using the same success/error mapping as the existing eight tools. The `set_goals` tool's input schema uses the unified `{min?, max?}` Range shape for every goal field, including `kcal` (the legacy `kcal_target` scalar field is not exposed).

#### Scenario: create_recipe tool calls the recipe creation endpoint

- **WHEN** the agent calls `create_recipe` with `{"name":"Morning skyr bowl","components":[{"product_id":"<id>","quantity_g":200}],"serving_size_g":250}`
- **THEN** the wrapper issues `POST /products/recipes` with the body
- **AND** returns the REST 201 response body as the tool result content

#### Scenario: create_recipe forwards an explicit idempotency key

- **WHEN** the agent supplies `idempotency_key` in the input
- **THEN** the wrapper sets `Idempotency-Key: <value>` on the REST request

#### Scenario: create_recipe derives an idempotency key when omitted

- **WHEN** the agent does not supply `idempotency_key`
- **THEN** the wrapper computes a deterministic key from the tool name and canonicalized input
- **AND** sets `Idempotency-Key` on the REST request to the derived value

#### Scenario: recompute_recipe tool calls the recompute endpoint

- **WHEN** the agent calls `recompute_recipe` with `{"product_id":"<id>"}`
- **THEN** the wrapper issues `POST /products/recipes/<id>/recompute`
- **AND** returns the REST response body as the tool result content

#### Scenario: get_goals tool calls the goals endpoint

- **WHEN** the agent calls `get_goals` with `{}`
- **THEN** the wrapper issues `GET /goals`
- **AND** returns the REST response body verbatim (including `{"goals": null}` when unset)

#### Scenario: set_goals tool calls the PUT goals endpoint

- **WHEN** the agent calls `set_goals` with a body like `{"kcal":{"min":2090,"max":2310},"protein_g":{"min":150,"max":190},"fiber_g":{"min":30},"sugar_g":{"max":50}}`
- **THEN** the wrapper issues `PUT /goals` with that body
- **AND** returns the REST response body as the tool result content

#### Scenario: set_goals input schema uses the unified Range shape

- **WHEN** the agent inspects the `set_goals` tool input schema
- **THEN** every goal field has the shape `{min?: number, max?: number}` (both bounds optional, at least one MUST be present per documented field)
- **AND** `kcal` appears under that shape
- **AND** there is no `kcal_target` property

#### Scenario: set_goals tool description guides the agent to construct ranges

- **WHEN** the agent reads the `set_goals` tool description
- **THEN** the description includes one sentence on how to convert a user-stated "I want N kcal a day" into an explicit `kcal: {min: N×0.95, max: N×1.05}` (or whatever tolerance the user implies)
- **AND** notes that single-bound goals (e.g. `fiber_g: {min: 30}`) are valid

#### Scenario: set_goals supports idempotency-key forwarding and derivation

- **WHEN** `set_goals` is called with or without `idempotency_key`
- **THEN** the wrapper applies the same idempotency rules as other write tools (explicit key wins; derived key otherwise)

### Requirement: Existing summary and freeform tools accept new optional parameters

The pre-existing `daily_summary`, `range_summary`, and `log_meal_freeform` tools SHALL forward new optional parameters added by the REST changes, without breaking input compatibility for agents that ignore them.

#### Scenario: daily_summary forwards meal_type when present

- **WHEN** the agent calls `daily_summary` with `{"date":"2026-06-06","meal_type":"breakfast"}`
- **THEN** the wrapper issues `GET /summary/daily?date=2026-06-06&meal_type=breakfast`

#### Scenario: daily_summary omits meal_type when not supplied

- **WHEN** the agent calls `daily_summary` without `meal_type`
- **THEN** the wrapper does not include `meal_type` in the query string

#### Scenario: range_summary forwards group_by when present

- **WHEN** the agent calls `range_summary` with `{"from":"2026-06-01","to":"2026-06-07","group_by":"meal_type"}`
- **THEN** the wrapper issues `GET /summary/range?from=2026-06-01&to=2026-06-07&group_by=meal_type`

#### Scenario: log_meal_freeform accepts micros in nutriments_per_100g

- **WHEN** the agent calls `log_meal_freeform` with `nutriments_per_100g` that includes any of `iron_mg`, `calcium_mg`, `vitamin_d_mcg`, `vitamin_b12_mcg`, `vitamin_c_mg`, `magnesium_mg`, `potassium_mg`, or `zinc_mg`
- **THEN** the wrapper forwards those fields verbatim in the REST request body
- **AND** the tool's input schema documents the micros as optional alongside macros

### Requirement: New tool descriptions guide the agent toward composite logging

The tool descriptions on `log_meal` and `log_meal_freeform` SHALL be extended to mention recipes as the preferred path for multi-ingredient meals, and `search_products` description SHALL note that recipe products appear in results just like OFF and manual products.

#### Scenario: log_meal_freeform description mentions create_recipe

- **WHEN** the agent enumerates tools via `tools/list`
- **THEN** the `log_meal_freeform` description includes a sentence pointing to `create_recipe` for "meals you eat repeatedly that have 2+ ingredients"

#### Scenario: search_products description mentions recipes

- **WHEN** the agent enumerates tools
- **THEN** the `search_products` description notes that results include products with `source` of `off`, `manual`, or `recipe`

### Requirement: Product management tools mirror the cleanup REST endpoints

The MCP server SHALL expose two new tools — `list_products` and `delete_product` — that wrap the new `GET /products` and `DELETE /products/{id}` REST endpoints. Each tool invokes its endpoint with `Authorization: Bearer <AGENT_API_TOKEN>` and forwards the REST response body as the tool's content, using the same success/error mapping as the existing tools.

#### Scenario: list_products tool calls the GET /products endpoint

- **WHEN** the agent calls `list_products` with `{}` (no filters)
- **THEN** the wrapper issues `GET /products`
- **AND** returns the REST 200 response body as the tool result content

#### Scenario: list_products forwards source filter and pagination

- **WHEN** the agent calls `list_products` with `{"source":"manual","limit":20,"offset":40}`
- **THEN** the wrapper issues `GET /products?source=manual&limit=20&offset=40`

#### Scenario: list_products is read-only and never sends Idempotency-Key

- **WHEN** the agent calls `list_products`
- **THEN** the REST request does NOT include an `Idempotency-Key` header
- **AND** the wrapper does not consult or store any idempotency record

#### Scenario: delete_product tool calls the DELETE /products/{id} endpoint

- **WHEN** the agent calls `delete_product` with `{"product_id":"<uuid>"}`
- **THEN** the wrapper issues `DELETE /products/<uuid>`
- **AND** returns the REST response body as the tool result content
- **AND** on a 204 response, the tool result content is an empty body and `isError = false`

#### Scenario: delete_product surfaces the 409 in-use body verbatim

- **WHEN** the REST API responds with `409 product_in_use_as_component` and the body lists the using recipes
- **THEN** the tool result has `isError = true`
- **AND** the content payload contains the JSON body unchanged, including the `recipes` array and the `hint` field

#### Scenario: delete_product auto-derives an idempotency key when omitted

- **WHEN** the agent calls `delete_product` twice in a row with the same `product_id` and no explicit `idempotency_key`
- **THEN** both REST requests carry the same auto-derived `Idempotency-Key` header
- **AND** the second call's result reflects the backend's response to a duplicate delete (either 204 from the cached response, or 404 if the cache TTL elapsed and the row is genuinely gone)

#### Scenario: delete_product description guides the agent on the 409 path

- **WHEN** the agent reads the `delete_product` tool description
- **THEN** the description notes that products used as recipe components produce a 409 with the using recipes listed
- **AND** instructs the agent to delete or replace the listed recipes before retrying

#### Scenario: list_products description points at the cleanup pattern

- **WHEN** the agent reads the `list_products` tool description
- **THEN** the description notes the recency ordering and the source filter
- **AND** mentions that combining list_products + delete_product is the standard way to clean up leftover products from prior sessions

### Requirement: Race-prep tool wraps the carb-load endpoint

The MCP server SHALL expose one tool, `plan_carb_load`, wrapping the new `GET /race-prep/carb-load` REST endpoint. The tool invokes the endpoint with `Authorization: Bearer <AGENT_API_TOKEN>` and forwards the REST response body as the tool's content via the existing `toToolResult` mapping. The tool is read-only: the wrapper does not send an `Idempotency-Key` header, and the input schema does not expose an `idempotency_key` property.

#### Scenario: plan_carb_load calls GET /race-prep/carb-load with the supplied params

- **WHEN** the agent calls `plan_carb_load` with `{"race_date":"2026-07-24","body_weight_kg":70}`
- **THEN** the wrapper issues `GET /race-prep/carb-load?race_date=2026-07-24&body_weight_kg=70`
- **AND** does NOT include an `Idempotency-Key` header
- **AND** forwards the response body verbatim to the tool result

#### Scenario: Optional parameters are passed when supplied

- **WHEN** the agent calls `plan_carb_load` with `{"race_date":"2026-07-24","body_weight_kg":70,"days_before":2,"carbs_per_kg_per_day":8,"race_day_carbs_per_kg":2.5}`
- **THEN** the wrapper appends `days_before=2&carbs_per_kg_per_day=8&race_day_carbs_per_kg=2.5` to the query string
- **AND** does NOT include optional params that were not supplied

#### Scenario: Validation errors from the endpoint are forwarded verbatim

- **WHEN** the REST endpoint returns `400 {"error":"body_weight_kg_invalid","range":{"min":30,"max":200}}`
- **THEN** the wrapper forwards the response body verbatim
- **AND** the tool result has `isError = true`

#### Scenario: plan_carb_load input schema reflects the parameter contract

- **WHEN** the agent inspects the `plan_carb_load` tool input schema
- **THEN** `race_date` and `body_weight_kg` are required
- **AND** `days_before`, `carbs_per_kg_per_day`, and `race_day_carbs_per_kg` are optional
- **AND** there is no `idempotency_key` property

#### Scenario: plan_carb_load description points the agent at the override workflow

- **WHEN** the agent reads the `plan_carb_load` tool description
- **THEN** the description names the natural follow-up: translating each schedule entry into a goal override via `set_daily_goal_override`
- **AND** notes typical `days_before` values per race distance (sprint: 1-2, 70.3: 3, Ironman: 3-4)
- **AND** notes that `carbs_per_kg_per_day` defaults sit in the documented 8-12 g/kg range, lower for athletes with GI sensitivity

### Requirement: Workouts tools mirror the workouts REST endpoints

The MCP server SHALL expose five tools wrapping the new workouts REST surface: `log_workout`, `list_workouts`, `get_workout`, `patch_workout`, and `delete_workout`. Each tool invokes its REST endpoint with `Authorization: Bearer <AGENT_API_TOKEN>` and forwards the REST response body as the tool's content via the existing `toToolResult` mapping. Write tools auto-derive an idempotency key when none is supplied (per the existing POST-style write rule); read tools never send `Idempotency-Key`.

#### Scenario: log_workout calls POST /workouts

- **WHEN** the agent calls `log_workout` with `{"source":"manual","sport":"strength","started_at":"2026-06-07T18:00:00Z","ended_at":"2026-06-07T19:00:00Z","name":"Gym — push day"}`
- **THEN** the wrapper issues `POST /workouts` with that JSON body
- **AND** sets `Idempotency-Key` to the agent's explicit key (if any) or to the derived stable key
- **AND** returns the REST `201`/`200` response body as the tool result content

#### Scenario: log_workout description explains the external_id dedup mechanism

- **WHEN** the agent reads the `log_workout` tool description
- **THEN** the description explains that most workouts come from the Garmin importer with `source: garmin` and an `external_id`
- **AND** clarifies that the agent should use this tool for manual entries (gym sessions, sweat-rate windows, untracked workouts)
- **AND** notes that `external_id` is the dedup mechanism — agents typically do NOT set it on manual writes

#### Scenario: list_workouts calls GET /workouts with the window

- **WHEN** the agent calls `list_workouts` with `{"from":"2026-06-01T00:00:00Z","to":"2026-06-08T00:00:00Z"}`
- **THEN** the wrapper issues `GET /workouts?from=2026-06-01T00:00:00Z&to=2026-06-08T00:00:00Z`
- **AND** does NOT include an `Idempotency-Key` header

#### Scenario: get_workout calls GET /workouts/{id}

- **WHEN** the agent calls `get_workout` with `{"id":"<uuid>"}`
- **THEN** the wrapper issues `GET /workouts/<uuid>`
- **AND** forwards a 404 with `{"error":"workout_not_found"}` verbatim when no workout exists

#### Scenario: patch_workout calls PATCH /workouts/{id}

- **WHEN** the agent calls `patch_workout` with `{"id":"<uuid>","tss":85,"notes":"FTP updated"}`
- **THEN** the wrapper issues `PATCH /workouts/<uuid>` with body `{"tss":85,"notes":"FTP updated"}`

#### Scenario: patch_workout description distinguishes mutable from immutable fields

- **WHEN** the agent reads the `patch_workout` tool description
- **THEN** the description lists the PATCH-able fields (`name`, `notes`, `kcal_burned`, `avg_hr`, `tss`)
- **AND** states that `sport`, `started_at`, `ended_at`, `source`, and `external_id` are immutable — delete and re-create if those are wrong

#### Scenario: delete_workout calls DELETE /workouts/{id}

- **WHEN** the agent calls `delete_workout` with `{"id":"<uuid>"}`
- **THEN** the wrapper issues `DELETE /workouts/<uuid>`
- **AND** on a 204 response, the tool result content is empty and `isError = false`
