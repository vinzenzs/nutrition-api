# training-plan — delta for add-coach-methodology

## ADDED Requirements

### Requirement: A plan carries optional plan-level methodology prose

The system SHALL allow a `training_plans` row to carry an optional `methodology`
free-text Markdown field, stored in a nullable `TEXT` column distinct from the
existing operational `notes`. It holds the cross-cutting, non-phase-specific
reference (e.g. Key Principles, the Rowing Strategy table) and is stored verbatim
(no server-side rendering). `PATCH /training-plans/{id}` SHALL accept
`methodology`, and `GET /training-plans/{id}` (and the nested plan tree) SHALL
return it. A null `methodology` means none is set and SHALL serialize as null. The
MCP `patch_training_plan` tool SHALL carry `methodology` in its payload; no new MCP
tool is added.

#### Scenario: A plan stores and returns plan-level methodology

- **WHEN** a client `PATCH`es a plan with a `methodology` Markdown string (Key
  Principles + Rowing Strategy)
- **THEN** the plan persists it and `GET /training-plans/{id}` returns it verbatim

#### Scenario: Plan methodology is independent of notes

- **WHEN** a plan has both `notes` and `methodology` and a `PATCH` supplies a new
  `methodology` without `notes`
- **THEN** `methodology` is replaced and `notes` is left unchanged

#### Scenario: patch_training_plan carries methodology

- **WHEN** the agent calls `patch_training_plan` with a `methodology` field
- **THEN** the plan is updated with that methodology and the read returns it
