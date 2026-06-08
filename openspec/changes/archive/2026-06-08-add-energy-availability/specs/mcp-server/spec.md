## ADDED Requirements

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
