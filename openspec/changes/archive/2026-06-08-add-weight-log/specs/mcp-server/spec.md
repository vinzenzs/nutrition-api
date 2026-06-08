## ADDED Requirements

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
