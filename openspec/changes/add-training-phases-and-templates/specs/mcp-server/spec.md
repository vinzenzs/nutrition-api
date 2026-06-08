## ADDED Requirements

### Requirement: Training phases tools mirror the phases REST endpoints

The MCP server SHALL expose five tools wrapping the new training-phases REST surface: `create_phase`, `list_phases`, `get_phase`, `update_phase`, and `delete_phase`. Each tool invokes its REST endpoint with `Authorization: Bearer <AGENT_API_TOKEN>` and forwards the REST response body as the tool's content via the existing `toToolResult` mapping. Write tools (`create_phase`, `update_phase`, `delete_phase`) auto-derive an idempotency key when none is supplied (per the existing POST-style write rule); read tools (`list_phases`, `get_phase`) never send `Idempotency-Key`.

#### Scenario: create_phase calls POST /phases

- **WHEN** the agent calls `create_phase` with `{"name":"build-block-2","type":"build","start_date":"2026-07-01","end_date":"2026-07-28","default_template_id":"<uuid>","notes":"weeks 5-8"}`
- **THEN** the wrapper issues `POST /phases` with that JSON body
- **AND** sets `Idempotency-Key` to the agent's explicit key (if any) or to the derived stable key
- **AND** returns the REST `201` response body as the tool result content

#### Scenario: create_phase tool description names the default_template_id semantic

- **WHEN** the agent reads the `create_phase` tool description
- **THEN** the description explains that `default_template_id` is the UUID of a goal template (created via `set_goal_template`) that becomes the default daily goals for every date in `[start_date, end_date]`
- **AND** notes that per-date overrides (`set_daily_goal_override`) still win over the phase's template
- **AND** notes that omitting `default_template_id` creates a phase that's visible in `list_phases` but does NOT drive adherence — useful for marking a date range with a `type` tag without committing to a template yet

#### Scenario: list_phases calls GET /phases with the window

- **WHEN** the agent calls `list_phases` with `{"from":"2026-07-01","to":"2026-07-31"}`
- **THEN** the wrapper issues `GET /phases?from=2026-07-01&to=2026-07-31`
- **AND** the response forwards the list of phases intersecting the window
- **AND** does NOT include an `Idempotency-Key` header

#### Scenario: list_phases requires from and to

- **WHEN** the agent calls `list_phases` without `from` or `to`
- **THEN** the wrapper forwards the request and the REST endpoint returns `400 range_required`
- **AND** the wrapper surfaces the error to the agent verbatim

#### Scenario: get_phase calls GET /phases/{id}

- **WHEN** the agent calls `get_phase` with `{"phase_id":"<uuid>"}`
- **THEN** the wrapper issues `GET /phases/<uuid>`
- **AND** returns the phase response body (including `default_template_name`)

#### Scenario: update_phase calls PATCH /phases/{id}

- **WHEN** the agent calls `update_phase` with `{"phase_id":"<uuid>","default_template_id":"<new-template-uuid>"}`
- **THEN** the wrapper issues `PATCH /phases/<uuid>` with body `{"default_template_id":"<new-template-uuid>"}`
- **AND** sets `Idempotency-Key` (derived or explicit)
- **AND** does NOT include `phase_id` in the body (it's a URL path segment)

#### Scenario: update_phase supports partial updates

- **WHEN** the agent calls `update_phase` with only one field changed (e.g. `{"phase_id":"<uuid>","notes":"updated"}`)
- **THEN** the PATCH body contains only that field
- **AND** other fields on the phase are preserved

#### Scenario: delete_phase calls DELETE /phases/{id}

- **WHEN** the agent calls `delete_phase` with `{"phase_id":"<uuid>"}`
- **THEN** the wrapper issues `DELETE /phases/<uuid>`
- **AND** sets `Idempotency-Key` (derived or explicit)
- **AND** returns an empty tool result content when the REST response is `204`

#### Scenario: Validation errors from the endpoints are forwarded verbatim

- **WHEN** the REST endpoint returns `400 {"error":"date_range_invalid"}`
- **THEN** the wrapper forwards the response body verbatim
- **AND** the tool result has `isError = true`

### Requirement: Goal templates tools mirror the templates REST endpoints

The MCP server SHALL expose four tools wrapping the new goal-templates REST surface: `set_goal_template`, `list_goal_templates`, `get_goal_template`, and `delete_goal_template`. Each tool invokes its REST endpoint with `Authorization: Bearer <AGENT_API_TOKEN>` and forwards the REST response body verbatim. `set_goal_template` is a PUT — per the existing PUT rule it does NOT expose an `idempotency_key` input field, and the wrapper does NOT send `Idempotency-Key`. Read tools never send the header. `delete_goal_template` is a DELETE and auto-derives an idempotency key per the existing rule.

#### Scenario: set_goal_template calls PUT /goal-templates/{name}

- **WHEN** the agent calls `set_goal_template` with `{"name":"weekday-easy-training","kcal":{"min":2090,"max":2310},"protein_g":{"min":150,"max":190},"carbs_g":{"min":280,"max":340}}`
- **THEN** the wrapper issues `PUT /goal-templates/weekday-easy-training` with body containing only the nutrient bound fields (the `name` is consumed by the wrapper for the URL path, not forwarded as a body field)
- **AND** does NOT include an `Idempotency-Key` header

#### Scenario: set_goal_template input schema does not expose idempotency_key

- **WHEN** the agent inspects the `set_goal_template` tool input schema
- **THEN** there is no `idempotency_key` property (matches the existing `set_goals` / `set_daily_goal_override` posture)

#### Scenario: set_goal_template description names the reuse pattern

- **WHEN** the agent reads the `set_goal_template` tool description
- **THEN** the description names the intended use: a template is a reusable goal-set you attach to a phase via `create_phase` or `update_phase`'s `default_template_id` field
- **AND** notes that editing a template's bounds propagates to every phase pointing at it on next adherence read (template edits are intentionally cheap; no apply step required)
- **AND** notes the full-replace semantics (absent nutrient bounds are stored as NULL)

#### Scenario: list_goal_templates calls GET /goal-templates

- **WHEN** the agent calls `list_goal_templates` with `{}`
- **THEN** the wrapper issues `GET /goal-templates`
- **AND** returns the list of templates ordered by name ascending

#### Scenario: get_goal_template calls GET /goal-templates/{name}

- **WHEN** the agent calls `get_goal_template` with `{"name":"weekday-easy-training"}`
- **THEN** the wrapper issues `GET /goal-templates/weekday-easy-training`
- **AND** forwards the response body verbatim

#### Scenario: delete_goal_template calls DELETE /goal-templates/{name}

- **WHEN** the agent calls `delete_goal_template` with `{"name":"weekday-easy-training"}`
- **THEN** the wrapper issues `DELETE /goal-templates/weekday-easy-training`
- **AND** sets `Idempotency-Key` (derived or explicit)
- **AND** returns an empty tool result content when the REST response is `204`

#### Scenario: delete_goal_template forwards 409 template_in_use verbatim

- **WHEN** the template is referenced by a phase and the REST endpoint returns `409 {"error":"template_in_use","referencing_phases":[...]}`
- **THEN** the wrapper forwards the response body verbatim
- **AND** the tool result has `isError = true`
- **AND** the agent can read `referencing_phases` to decide whether to delete those phases or reassign their `default_template_id` before retrying the template delete

#### Scenario: Tool count integration test is updated

- **WHEN** the MCP integration test (`mcp_integration_test.go`) enumerates exposed tools
- **THEN** the expected-tools assertion includes the nine new tool names: `create_phase`, `list_phases`, `get_phase`, `update_phase`, `delete_phase`, `set_goal_template`, `list_goal_templates`, `get_goal_template`, `delete_goal_template`
