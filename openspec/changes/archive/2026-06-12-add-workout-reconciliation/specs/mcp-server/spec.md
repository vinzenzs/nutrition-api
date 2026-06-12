# mcp-server — delta for add-workout-reconciliation

## ADDED Requirements

### Requirement: Fulfill and unfulfill tools drive manual reconciliation

The MCP server SHALL expose `fulfill_workout` (a `planned_id` + `completed_id` →
`POST /workouts/{plannedId}/fulfill`) and `unfulfill_workout` (an `id` →
`POST /workouts/{id}/unfulfill`), each issuing exactly one HTTP call and
forwarding the response verbatim, auto-deriving an idempotency key when none is
supplied. The automatic merge during Garmin sync needs no tool — it happens on
the ingestion path. The MCP integration expected-tools list SHALL include both.

#### Scenario: fulfill_workout issues one POST

- **WHEN** the agent calls `fulfill_workout` with a planned id and a completed id
- **THEN** the MCP server issues exactly one `POST /workouts/{plannedId}/fulfill`
- **AND** the tool result is the REST response verbatim

#### Scenario: unfulfill_workout reverses a merge

- **WHEN** the agent calls `unfulfill_workout` with a workout id
- **THEN** the MCP server issues exactly one `POST /workouts/{id}/unfulfill`
- **AND** the tool result is the REST response verbatim

#### Scenario: Expected-tools list includes the reconciliation tools

- **WHEN** the MCP integration test enumerates registered tools
- **THEN** `fulfill_workout` and `unfulfill_workout` are both present
