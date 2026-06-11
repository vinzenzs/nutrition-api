## ADDED Requirements

### Requirement: Race tools mirror the race REST endpoints

The system SHALL expose six MCP tools that each invoke exactly one race REST
endpoint with `Authorization: Bearer <AGENT_API_TOKEN>`: `create_race`,
`list_races`, `get_race`, `update_race`, `delete_race`, and `plan_race_fueling`.
The write tools (`create_race`, `update_race`, `delete_race`) SHALL follow the
existing idempotency-key derivation rule for POST/PATCH/DELETE-style write tools
(explicit `idempotency_key` forwarded verbatim; otherwise a stable key derived
from the tool name and canonical arguments). `plan_race_fueling` and the read
tools SHALL NOT send an `Idempotency-Key`. The MCP integration test's
expected-tools list SHALL include all six.

#### Scenario: create_race calls the REST races endpoint

- **WHEN** the agent calls `create_race` with a name, race date, and legs
- **THEN** the wrapper issues `POST /races` with the JSON body
- **AND** sets an `Idempotency-Key` header (explicit if supplied, else derived)

#### Scenario: plan_race_fueling forwards athlete params

- **WHEN** the agent calls `plan_race_fueling` with a race id, `body_weight_kg`,
  and `sweat_rate_ml_per_hr`
- **THEN** the wrapper issues `GET /races/{id}/fueling-plan` with `body_weight_kg`
  and `sweat_rate_ml_per_hr` as query parameters
- **AND** sends no `Idempotency-Key` header

#### Scenario: plan_race_fueling omits sweat rate when not supplied

- **WHEN** the agent calls `plan_race_fueling` with only a race id and
  `body_weight_kg`
- **THEN** the request query contains `body_weight_kg` and omits
  `sweat_rate_ml_per_hr`

#### Scenario: read tools call their endpoints

- **WHEN** the agent calls `list_races` or `get_race`
- **THEN** the wrapper issues `GET /races` or `GET /races/{id}` respectively with
  no `Idempotency-Key` header

#### Scenario: delete_race calls the REST delete endpoint

- **WHEN** the agent calls `delete_race` with a race id
- **THEN** the wrapper issues `DELETE /races/{id}` with an `Idempotency-Key` header

#### Scenario: Expected-tools list includes the race tools

- **WHEN** the MCP integration test enumerates registered tools
- **THEN** `create_race`, `list_races`, `get_race`, `update_race`, `delete_race`,
  and `plan_race_fueling` are all present
