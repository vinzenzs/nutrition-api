## ADDED Requirements

### Requirement: Hydration-balance tools mirror the hydration-balance REST endpoints

The MCP server SHALL expose four tools wrapping the hydration-balance REST surface: `log_hydration_balance`, `list_hydration_balance`, `get_hydration_balance`, and `delete_hydration_balance`. Each invokes its REST endpoint with `Authorization: Bearer <AGENT_API_TOKEN>` and forwards the response body via `toToolResult`. Write tools auto-derive an idempotency key when none is supplied; read tools never send `Idempotency-Key`. The integration-test expected-tools list is updated to include the four new names.

#### Scenario: log_hydration_balance upserts by date

- **WHEN** the agent calls `log_hydration_balance` with `{"date":"2026-06-09","sweat_loss_ml":2400,"activity_intake_ml":1800,"goal_ml":3000}`
- **THEN** the wrapper issues `POST /hydration-balance` with those fields in the body
- **AND** the REST response body is forwarded verbatim

#### Scenario: Optional metrics omitted from the body when nil

- **WHEN** the agent calls `log_hydration_balance` with only `date` and `sweat_loss_ml`
- **THEN** the POST body contains only those keys (the other metrics are absent, not null)

#### Scenario: list_hydration_balance passes the date window with no idempotency key

- **WHEN** the agent calls `list_hydration_balance` with `{"from":"2026-06-01","to":"2026-06-30"}`
- **THEN** the wrapper issues `GET /hydration-balance?from=2026-06-01&to=2026-06-30` with no `Idempotency-Key`

#### Scenario: get and delete address by date

- **WHEN** the agent calls `get_hydration_balance` with `{"date":"2026-06-09"}`
- **THEN** the wrapper issues `GET /hydration-balance/2026-06-09`

- **WHEN** the agent calls `delete_hydration_balance` with `{"date":"2026-06-09"}`
- **THEN** the wrapper issues `DELETE /hydration-balance/2026-06-09` and returns an empty result on `204`
