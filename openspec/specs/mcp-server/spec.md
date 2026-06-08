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

The MCP server SHALL expose one tool, `plan_carb_load`, wrapping the race-prep carb-load REST endpoints. The tool accepts an optional `apply: boolean` argument (default `false`). When `apply` is `false` (or absent), the wrapper invokes `GET /race-prep/carb-load` — the pure-compute path, unchanged from the original requirement. When `apply` is `true`, the wrapper invokes `POST /race-prep/carb-load/apply` — the side-effecting path that also writes the per-day carb targets into the goal overrides. Both branches invoke their REST endpoint with `Authorization: Bearer <AGENT_API_TOKEN>` and forward the REST response body as the tool's content via the existing `toToolResult` mapping.

The `apply: true` branch is a POST-style write. The wrapper SHALL apply the existing POST-write idempotency-key rule: if the tool input contains an explicit `idempotency_key`, that value is used; otherwise the wrapper computes a stable key as `sha256_hex("plan_carb_load|" + canonical_json(<tool_args_without_idempotency_key>))`. The `Idempotency-Key` header is set on the POST request. The `apply: false` branch remains read-only — no `Idempotency-Key` header is sent.

#### Scenario: plan_carb_load with apply=false (default) calls GET

- **WHEN** the agent calls `plan_carb_load` with `{"race_date":"2026-07-24","body_weight_kg":70}` (no `apply` arg)
- **THEN** the wrapper issues `GET /race-prep/carb-load?race_date=2026-07-24&body_weight_kg=70`
- **AND** does NOT include an `Idempotency-Key` header
- **AND** forwards the response body verbatim to the tool result

- **WHEN** the agent calls `plan_carb_load` with `{"race_date":"2026-07-24","body_weight_kg":70,"apply":false}`
- **THEN** the wrapper issues the same `GET /race-prep/carb-load?race_date=2026-07-24&body_weight_kg=70` (apply=false is equivalent to omitting apply)

#### Scenario: plan_carb_load with apply=true calls POST /apply

- **WHEN** the agent calls `plan_carb_load` with `{"race_date":"2026-07-24","body_weight_kg":70,"apply":true}`
- **THEN** the wrapper issues `POST /race-prep/carb-load/apply` with body `{"race_date":"2026-07-24","body_weight_kg":70}` (the `apply` flag is consumed by the wrapper and NOT forwarded as a query param or body field)
- **AND** sets `Idempotency-Key` (derived from the args minus the explicit `idempotency_key` field if any)
- **AND** forwards the response body — including the `applied` array — verbatim to the tool result

#### Scenario: Optional parameters are passed when supplied (both branches)

- **WHEN** the agent calls `plan_carb_load` with `{"race_date":"2026-07-24","body_weight_kg":70,"days_before":2,"carbs_per_kg_per_day":8,"race_day_carbs_per_kg":2.5}` (apply=false)
- **THEN** the wrapper appends `days_before=2&carbs_per_kg_per_day=8&race_day_carbs_per_kg=2.5` to the GET query string
- **AND** does NOT include optional params that were not supplied

- **WHEN** the same args are supplied with `apply=true`
- **THEN** the wrapper POSTs `{"race_date":"2026-07-24","body_weight_kg":70,"days_before":2,"carbs_per_kg_per_day":8,"race_day_carbs_per_kg":2.5}` (optional params included in the body when set)

#### Scenario: Explicit idempotency_key on apply=true is forwarded verbatim

- **WHEN** the agent calls `plan_carb_load` with `{"race_date":"2026-07-24","body_weight_kg":70,"apply":true,"idempotency_key":"race-week-2026-07"}`
- **THEN** the wrapper sets `Idempotency-Key: race-week-2026-07` on the POST request
- **AND** removes `idempotency_key` from the body before forwarding
- **AND** the derived key formula is NOT used (explicit key wins)

#### Scenario: idempotency_key field is absent from apply=false schema branch

- **WHEN** the agent inspects the `plan_carb_load` tool input schema
- **THEN** `idempotency_key` is listed as an optional property
- **AND** the description of `idempotency_key` notes that the field is only used when `apply: true` (the read path ignores it)

#### Scenario: Validation errors from either endpoint are forwarded verbatim

- **WHEN** the REST endpoint (either GET or POST /apply) returns `400 {"error":"body_weight_kg_invalid","range":{"min":30,"max":200}}`
- **THEN** the wrapper forwards the response body verbatim
- **AND** the tool result has `isError = true`

#### Scenario: Apply rollback errors surface to the agent

- **WHEN** the agent calls `plan_carb_load` with `apply:true` and the POST /apply endpoint returns `500 Internal Server Error` because the transaction rolled back
- **THEN** the wrapper forwards the error body
- **AND** the tool result has `isError = true`
- **AND** the agent can infer from the absence of an `applied` array in the body that nothing was persisted

#### Scenario: plan_carb_load input schema reflects the parameter contract

- **WHEN** the agent inspects the `plan_carb_load` tool input schema
- **THEN** `race_date` and `body_weight_kg` are required
- **AND** `days_before`, `carbs_per_kg_per_day`, `race_day_carbs_per_kg`, and `apply` are optional
- **AND** `apply` is typed as boolean with default `false`
- **AND** `idempotency_key` is also listed as an optional string

#### Scenario: plan_carb_load description names the apply side effect

- **WHEN** the agent reads the `plan_carb_load` tool description
- **THEN** the description notes typical `days_before` values per race distance (sprint: 1-2, 70.3: 3, Ironman: 3-4)
- **AND** notes that `carbs_per_kg_per_day` defaults sit in the documented 8-12 g/kg range, lower for athletes with GI sensitivity
- **AND** describes the `apply` flag explicitly: setting `apply: true` ALSO writes the carb_g goal bounds (min-only) for each schedule day into the per-date goal overrides, preserving any existing kcal/protein/other macros on those days
- **AND** notes that when `apply: true`, the response includes an `applied` array per date with `{date, carbs_g_min, created}`, where `created: false` means the apply merged into a pre-existing override
- **AND** notes that the original "set_daily_goal_override × N" follow-up workflow is now optional — `apply: true` is the recommended path for the standard race-prep workflow

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

### Requirement: Daily goal override tools mirror the override REST endpoints

The MCP server SHALL expose four tools wrapping the new daily-goal-override REST surface: `set_daily_goal_override`, `get_daily_goal_override`, `delete_daily_goal_override`, and `list_daily_goal_overrides`. Each tool invokes its REST endpoint with `Authorization: Bearer <AGENT_API_TOKEN>` and forwards the REST response body as the tool's content via the existing `toToolResult` mapping. `set_daily_goal_override` follows the same posture as `set_goals`: the input schema does NOT expose `idempotency_key`, the wrapper never sends one, and the REST backend would reject it on PUT regardless.

#### Scenario: set_daily_goal_override calls PUT /goals/overrides/{date}

- **WHEN** the agent calls `set_daily_goal_override` with `{"date":"2026-06-15","goals":{"kcal":{"min":2280,"max":2520},"protein_g":{"min":160,"max":200}}}`
- **THEN** the wrapper issues `PUT /goals/overrides/2026-06-15` with the goals body
- **AND** does NOT include an `Idempotency-Key` header

#### Scenario: set_daily_goal_override input schema uses the unified Range shape

- **WHEN** the agent inspects the `set_daily_goal_override` tool input schema
- **THEN** every goal field uses the unified `{min?, max?}` Range shape (matching `set_goals`)
- **AND** there is no `kcal_target` property
- **AND** there is no `idempotency_key` property

#### Scenario: set_daily_goal_override description distinguishes from set_goals

- **WHEN** the agent reads the `set_daily_goal_override` tool description
- **THEN** the description states that the override is full-replace for that single date, replacing (not merging with) the default goals
- **AND** suggests typical use cases: training days, rest days, race weeks
- **AND** notes that retries are NOT safe (same constraint as set_goals)

#### Scenario: get_daily_goal_override calls GET /goals/overrides/{date}

- **WHEN** the agent calls `get_daily_goal_override` with `{"date":"2026-06-15"}`
- **THEN** the wrapper issues `GET /goals/overrides/2026-06-15`
- **AND** forwards a 404 with `{"error":"override_not_found"}` verbatim when no override exists

#### Scenario: delete_daily_goal_override calls DELETE /goals/overrides/{date}

- **WHEN** the agent calls `delete_daily_goal_override` with `{"date":"2026-06-15"}`
- **THEN** the wrapper issues `DELETE /goals/overrides/2026-06-15`
- **AND** on a 204 response, the tool result content is empty and `isError = false`
- **AND** uses the standard POST-style auto-derive idempotency rule on the DELETE

#### Scenario: list_daily_goal_overrides calls GET /goals/overrides with the range

- **WHEN** the agent calls `list_daily_goal_overrides` with `{"from":"2026-06-01","to":"2026-06-30"}`
- **THEN** the wrapper issues `GET /goals/overrides?from=2026-06-01&to=2026-06-30`
- **AND** does NOT include an `Idempotency-Key` header

#### Scenario: list_daily_goal_overrides description points at the audit use case

- **WHEN** the agent reads the `list_daily_goal_overrides` tool description
- **THEN** the description notes the typical use ("what's set for this week before I add more") and that dates without an override are omitted (the agent can infer they use the default)

### Requirement: Body-weight tools mirror the weight REST endpoints

The MCP server SHALL expose five tools wrapping the new body-weight REST surface: `log_weight`, `list_weights`, `patch_weight`, `delete_weight`, and `weight_trend`. Each tool invokes its REST endpoint with `Authorization: Bearer <AGENT_API_TOKEN>` and forwards the REST response body as the tool's content via the existing `toToolResult` mapping. Write tools auto-derive an idempotency key when none is supplied (per the existing POST-style write rule); read tools never send `Idempotency-Key`.

#### Scenario: log_weight calls POST /weight

- **WHEN** the agent calls `log_weight` with `{"weight_kg":72.5,"logged_at":"2026-06-07T07:00:00Z","body_fat_pct":14.2}`
- **THEN** the wrapper issues `POST /weight` with that JSON body
- **AND** sets `Idempotency-Key` to the agent's explicit key (if any) or to the derived stable key
- **AND** returns the REST `201` response body as the tool result content

#### Scenario: log_weight description distinguishes routine measurements from race-day context

- **WHEN** the agent reads the `log_weight` tool description
- **THEN** the description explains that multiple measurements per day are fine (the trend smooths them)
- **AND** suggests the `note` field for context that affects readings (post-workout, post-meal, hotel scale, time of day if not morning)
- **AND** does NOT prescribe a default time of day for weighing (that's coaching territory)

#### Scenario: list_weights calls GET /weight with the window

- **WHEN** the agent calls `list_weights` with `{"from":"2026-06-01T00:00:00Z","to":"2026-06-08T00:00:00Z"}`
- **THEN** the wrapper issues `GET /weight?from=2026-06-01T00:00:00Z&to=2026-06-08T00:00:00Z`
- **AND** does NOT include an `Idempotency-Key` header

#### Scenario: patch_weight calls PATCH /weight/{id}

- **WHEN** the agent calls `patch_weight` with `{"id":"<uuid>","body_fat_pct":13.8}`
- **THEN** the wrapper issues `PATCH /weight/<uuid>` with body `{"body_fat_pct":13.8}`

#### Scenario: delete_weight calls DELETE /weight/{id}

- **WHEN** the agent calls `delete_weight` with `{"id":"<uuid>"}`
- **THEN** the wrapper issues `DELETE /weight/<uuid>`
- **AND** on a 204 response, the tool result content is empty and `isError = false`

#### Scenario: weight_trend calls GET /weight/trend

- **WHEN** the agent calls `weight_trend` with `{"from":"2026-05-01","to":"2026-06-07","window_days":7,"tz":"Europe/Berlin"}`
- **THEN** the wrapper issues `GET /weight/trend?from=2026-05-01&to=2026-06-07&window_days=7&tz=Europe/Berlin`
- **AND** returns the REST response body as the tool result content

#### Scenario: weight_trend omits unset optional params

- **WHEN** the agent calls `weight_trend` with `{"from":"2026-05-01","to":"2026-06-07"}` (no `window_days`, no `tz`)
- **THEN** the wrapper issues `GET /weight/trend?from=2026-05-01&to=2026-06-07` (no `window_days`, no `tz`)
- **AND** the REST server applies its defaults (`window_days=7`, `DEFAULT_USER_TZ`)

#### Scenario: weight_trend description emphasises sample_count interpretation

- **WHEN** the agent reads the `weight_trend` tool description
- **THEN** the description states that `window_days` defaults to 7 and explains it suppresses normal daily noise
- **AND** explicitly notes that each point carries `sample_count`, and that a `rolling_avg_kg` computed from `sample_count: 1` is not a trend
- **AND** suggests checking `sample_count` before basing decisions on a trend value

### Requirement: Hydration tools mirror the hydration REST endpoints

The MCP server SHALL expose five tools wrapping the new hydration REST surface: `log_hydration`, `list_hydration`, `patch_hydration`, `delete_hydration`, and `daily_hydration_summary`. Each tool invokes its REST endpoint with `Authorization: Bearer <AGENT_API_TOKEN>` and forwards the REST response body as the tool's content via the existing `toToolResult` mapping. Write tools auto-derive an idempotency key when none is supplied (per the existing POST-style write rule); read tools never send `Idempotency-Key`.

#### Scenario: log_hydration calls POST /hydration

- **WHEN** the agent calls `log_hydration` with `{"quantity_ml":500,"logged_at":"2026-06-07T08:00:00Z"}`
- **THEN** the wrapper issues `POST /hydration` with that JSON body
- **AND** sets `Idempotency-Key` to the agent's explicit key (if any) or to the derived stable key
- **AND** returns the REST 201 response body as the tool result content

#### Scenario: log_hydration description guides note usage

- **WHEN** the agent reads the `log_hydration` tool description
- **THEN** the description explains that `note` carries beverage context (e.g. `water`, `iced coffee`, `electrolytes`) and is optional free-text

#### Scenario: list_hydration calls GET /hydration with the window

- **WHEN** the agent calls `list_hydration` with `{"from":"2026-06-01T00:00:00Z","to":"2026-06-08T00:00:00Z"}`
- **THEN** the wrapper issues `GET /hydration?from=2026-06-01T00:00:00Z&to=2026-06-08T00:00:00Z`
- **AND** does NOT include an `Idempotency-Key` header

#### Scenario: patch_hydration calls PATCH /hydration/{id}

- **WHEN** the agent calls `patch_hydration` with `{"id":"<uuid>","note":"actually it was tea"}`
- **THEN** the wrapper issues `PATCH /hydration/<uuid>` with body `{"note":"actually it was tea"}`

#### Scenario: delete_hydration calls DELETE /hydration/{id}

- **WHEN** the agent calls `delete_hydration` with `{"id":"<uuid>"}`
- **THEN** the wrapper issues `DELETE /hydration/<uuid>`
- **AND** on a 204 response, the tool result content is empty and `isError = false`

#### Scenario: daily_hydration_summary calls GET /summary/hydration/daily

- **WHEN** the agent calls `daily_hydration_summary` with `{"date":"2026-06-07","tz":"Europe/Berlin"}`
- **THEN** the wrapper issues `GET /summary/hydration/daily?date=2026-06-07&tz=Europe/Berlin`
- **AND** returns the REST response body as the tool result content

#### Scenario: daily_hydration_summary omits tz when not supplied

- **WHEN** the agent calls `daily_hydration_summary` with `{"date":"2026-06-07"}`
- **THEN** the wrapper issues `GET /summary/hydration/daily?date=2026-06-07` (no `tz` param)
- **AND** the REST server's `DEFAULT_USER_TZ` resolves the day window

#### Scenario: daily_hydration_summary description distinguishes from daily_summary

- **WHEN** the agent reads the `daily_hydration_summary` tool description
- **THEN** the description states that this is the volume-only daily summary, separate from `daily_summary` (which is the nutrient-only daily summary)

### Requirement: Meal and hydration tools accept an optional workout_id

The MCP server SHALL extend the existing `log_meal`, `log_meal_freeform`, `patch_meal`, `log_hydration`, and `patch_hydration` tools with an optional `workout_id` argument. When supplied, the wrapper forwards it in the REST body verbatim. When omitted, the wrapper does not include the field. PATCH tools additionally honour the empty-string sentinel `""` to clear an existing link.

#### Scenario: log_meal forwards workout_id when supplied

- **WHEN** the agent calls `log_meal` with `{"product_id":"<p>","quantity_g":150,"logged_at":"…","workout_id":"<w>"}`
- **THEN** the wrapper issues `POST /meals` with `"workout_id":"<w>"` in the body
- **AND** the response (forwarded from the REST API) includes `workout_id`

#### Scenario: log_meal omits workout_id when not supplied

- **WHEN** the agent calls `log_meal` without `workout_id`
- **THEN** the wrapper's POST body does NOT contain a `workout_id` key
- **AND** existing behaviour is preserved

#### Scenario: log_meal_freeform and log_hydration forward workout_id the same way

- **WHEN** the agent calls `log_meal_freeform` or `log_hydration` with `workout_id`
- **THEN** the field is forwarded in the POST body verbatim

#### Scenario: patch_meal forwards empty string to clear

- **WHEN** the agent calls `patch_meal` with `{"meal_id":"<id>","workout_id":""}`
- **THEN** the wrapper issues `PATCH /meals/<id>` with body `{"workout_id":""}`
- **AND** the REST backend interprets the empty string as "clear the link"

#### Scenario: Tool descriptions explain the link semantics

- **WHEN** the agent reads the description of any of these tools
- **THEN** the description mentions that `workout_id` is an optional link to a `workouts` row
- **AND** the PATCH tools' descriptions explicitly document the empty-string clear semantic
- **AND** the descriptions note that the link is metadata — the `workout_fueling_summary` tool uses time-window matching, not tags

### Requirement: workout_fueling_summary tool wraps GET /workouts/{id}/fueling

The MCP server SHALL expose a new `workout_fueling_summary` tool that wraps `GET /workouts/{id}/fueling`. The tool takes `workout_id` (required string) plus optional `pre_window_min` and `post_window_min` integers. Read-only: no `Idempotency-Key`, no derived key.

#### Scenario: workout_fueling_summary calls the fueling endpoint

- **WHEN** the agent calls `workout_fueling_summary` with `{"workout_id":"<w>","pre_window_min":180,"post_window_min":90}`
- **THEN** the wrapper issues `GET /workouts/<w>/fueling?pre_window_min=180&post_window_min=90`
- **AND** does NOT include an `Idempotency-Key` header

#### Scenario: workout_fueling_summary omits unset optional params

- **WHEN** the agent calls `workout_fueling_summary` with only `{"workout_id":"<w>"}`
- **THEN** the wrapper issues `GET /workouts/<w>/fueling` (no query params)
- **AND** the REST server applies the default windows (240 pre, 60 post)

#### Scenario: workout_fueling_summary description spells out the semantics

- **WHEN** the agent reads the `workout_fueling_summary` tool description
- **THEN** the description states that aggregation is by time-window, not by the `workout_id` tag on intake entries
- **AND** lists the default windows (240 min pre, 60 min post) and the bounds (0..720)
- **AND** notes that `nutrition` and `hydration` are separate sub-objects per window so the agent does NOT mix units
- **AND** notes that when `workout_fuel_entries` ships (future), this tool's response will gain those contributions automatically with no contract change

#### Scenario: 404 from the endpoint is forwarded as isError

- **WHEN** the REST endpoint returns `404 {"error":"workout_not_found"}`
- **THEN** the wrapper forwards the body verbatim
- **AND** sets `isError = true` on the tool result

### Requirement: Workout-fuel tools mirror the workout-fuel REST endpoints

The MCP server SHALL expose four tools wrapping the new workout-fuel REST surface: `log_workout_fuel`, `list_workout_fuel`, `patch_workout_fuel`, and `delete_workout_fuel`. Each tool invokes its REST endpoint with `Authorization: Bearer <AGENT_API_TOKEN>` and forwards the REST response body via `toToolResult`. Write tools auto-derive an idempotency key when none is supplied; read tools never send `Idempotency-Key`.

#### Scenario: log_workout_fuel calls POST /workout-fuel

- **WHEN** the agent calls `log_workout_fuel` with `{"name":"Maurten Gel 100","logged_at":"2026-06-07T08:45:00Z","carbs_g":25,"caffeine_mg":100}`
- **THEN** the wrapper issues `POST /workout-fuel` with that JSON body
- **AND** sets `Idempotency-Key` to the agent's explicit key (if any) or to the derived stable key
- **AND** returns the REST `201` response body as the tool result content

#### Scenario: log_workout_fuel description explains the hydration vs workout-fuel routing rule

- **WHEN** the agent reads the `log_workout_fuel` tool description
- **THEN** the description explains the simple routing rule: plain water / juice (volume only) → `log_hydration`; anything with electrolytes / carbs / caffeine → `log_workout_fuel`
- **AND** notes that `name` is required (rehearsal data depends on knowing WHAT was taken)
- **AND** notes that at least one of `quantity_ml`/`carbs_g`/`sodium_mg`/`potassium_mg`/`caffeine_mg` must be supplied
- **AND** notes that `caffeine_mg: 0` is meaningful (explicitly "no caffeine") and distinct from omitting (NULL = "not measured")

#### Scenario: log_workout_fuel optional workout_id is forwarded

- **WHEN** the agent calls `log_workout_fuel` with `workout_id` set to an existing workout's UUID
- **THEN** the wrapper forwards the field in the POST body
- **AND** the REST 400 `workout_not_found` is forwarded verbatim on unknown workouts

#### Scenario: list_workout_fuel calls GET /workout-fuel with the window

- **WHEN** the agent calls `list_workout_fuel` with `{"from":"2026-06-01T00:00:00Z","to":"2026-06-08T00:00:00Z"}`
- **THEN** the wrapper issues `GET /workout-fuel?from=2026-06-01T00:00:00Z&to=2026-06-08T00:00:00Z`
- **AND** does NOT include an `Idempotency-Key` header

#### Scenario: patch_workout_fuel calls PATCH /workout-fuel/{id}

- **WHEN** the agent calls `patch_workout_fuel` with `{"id":"<uuid>","sodium_mg":420}`
- **THEN** the wrapper issues `PATCH /workout-fuel/<uuid>` with body `{"sodium_mg":420}`

#### Scenario: patch_workout_fuel forwards the empty-string clear semantic for workout_id

- **WHEN** the agent calls `patch_workout_fuel` with `{"id":"<uuid>","workout_id":""}`
- **THEN** the wrapper forwards the body verbatim
- **AND** the REST backend interprets the empty string as "clear the link"

#### Scenario: delete_workout_fuel calls DELETE /workout-fuel/{id}

- **WHEN** the agent calls `delete_workout_fuel` with `{"id":"<uuid>"}`
- **THEN** the wrapper issues `DELETE /workout-fuel/<uuid>`
- **AND** on a 204 response, the tool result content is empty and `isError = false`

### Requirement: workout_fueling_summary tool description acknowledges the new sub-object

The existing `workout_fueling_summary` tool description (from `add-meal-workout-link`) SHALL note that each window's response now includes a third sub-object `workout_fuel` carrying carbs/sodium/potassium/caffeine/ml from `workout_fuel_entries`, in addition to the existing `nutrition` and `hydration` sub-objects. No contract change to the tool's inputs; the response composition just gets richer.

#### Scenario: Updated description names all three sub-objects

- **WHEN** the agent reads the `workout_fueling_summary` tool description (after this change applies)
- **THEN** the description lists `nutrition` (from meals), `hydration` (from hydration entries), AND `workout_fuel` (from workout-fuel entries) as the three per-window sub-objects
- **AND** continues to note the time-window-vs-tag aggregation rule (no change)
- **AND** continues to note the default windows (240 pre, 60 post) and bounds (no change)

### Requirement: weekly_energy_summary tool wraps the energy-availability endpoint

The MCP server SHALL expose one tool `weekly_energy_summary` that invokes `GET /energy/availability` with `Authorization: Bearer <AGENT_API_TOKEN>`, forwards the response body via `toToolResult`, and does NOT send an `Idempotency-Key` header (the endpoint is read-only). Inputs mirror the REST query parameters; outputs are passed through verbatim.

#### Scenario: weekly_energy_summary calls GET /energy/availability with the window and optional overrides

- **WHEN** the agent calls `weekly_energy_summary` with `{"from":"2026-06-01T00:00:00Z","to":"2026-06-08T00:00:00Z","tz":"Europe/Berlin","lean_mass_kg":62}`
- **THEN** the wrapper issues `GET /energy/availability?from=2026-06-01T00:00:00Z&to=2026-06-08T00:00:00Z&tz=Europe/Berlin&lean_mass_kg=62`
- **AND** does NOT include an `Idempotency-Key` header
- **AND** returns the REST `200` response body as the tool result content

#### Scenario: Optional parameters are omitted from the query string when unset

- **WHEN** the agent calls `weekly_energy_summary` with only `from` and `to` set
- **THEN** the request URL does not include `tz`, `lean_mass_kg`, or `body_fat_pct` query parameters
- **AND** the REST backend applies its `DEFAULT_USER_TZ` and stored-composition resolution

#### Scenario: body_fat_pct is forwarded when supplied without lean_mass_kg

- **WHEN** the agent calls `weekly_energy_summary` with `{"from":"...","to":"...","body_fat_pct":15}`
- **THEN** the request URL includes `body_fat_pct=15` and not `lean_mass_kg`

#### Scenario: Tool description explains the bands, the resolution order, and the "missing burn" semantic

- **WHEN** the agent reads the `weekly_energy_summary` tool description
- **THEN** the description names the three Loucks bands (`low`, `sub_optimal`, `adequate`) with their thresholds (`< 30`, `30–45`, `>= 45 kcal/kg FFM/day`)
- **AND** explains the FFM resolution order (`lean_mass_kg` → `body_fat_pct` → stored body-fat % → 85% fallback)
- **AND** notes that days with workouts missing `kcal_burned` are flagged via `missing_burn_workout_ids` and excluded from `window.avg_ea`
- **AND** notes that this is a read tool (no idempotency-key forwarded)

#### Scenario: REST 4xx errors are forwarded as isError

- **WHEN** the REST backend returns `400 weight_data_missing` (no body-weight entries and no `lean_mass_kg` override)
- **THEN** the tool result has `isError = true`
- **AND** the response body is the verbatim REST error payload
