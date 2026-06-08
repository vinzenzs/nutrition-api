## MODIFIED Requirements

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
