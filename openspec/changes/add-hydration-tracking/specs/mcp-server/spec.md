## ADDED Requirements

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
