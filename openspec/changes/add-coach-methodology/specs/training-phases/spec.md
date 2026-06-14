# training-phases — delta for add-coach-methodology

## ADDED Requirements

### Requirement: A phase carries optional methodology prose

The system SHALL allow a `training_phases` row to carry an optional `methodology`
free-text Markdown field, stored in a nullable `TEXT` column distinct from the
existing operational `notes`. It holds the curated, cited "why this phase"
narrative and is stored verbatim (no server-side rendering or transformation). The
phase create and update paths SHALL accept `methodology`, and the phase read paths
SHALL return it. A null `methodology` means none is set and SHALL serialize as
null, not an error. The MCP phase-write tool SHALL carry `methodology` in its
payload; no new MCP tool is added.

#### Scenario: A phase stores and returns methodology

- **WHEN** a phase is created or updated with a `methodology` Markdown string
  (e.g. a Base-phase "Why" block citing Seiler)
- **THEN** the phase persists it in the `methodology` column and the phase read
  returns it verbatim

#### Scenario: Methodology is independent of notes

- **WHEN** a phase has both `notes` and `methodology` set and a write supplies a new
  `methodology` without `notes`
- **THEN** `methodology` is replaced and `notes` is left unchanged

#### Scenario: Absent methodology serializes as null

- **WHEN** a phase has no `methodology`
- **THEN** the read returns `methodology` as null and no error

#### Scenario: The phase-write MCP tool carries methodology

- **WHEN** the agent writes a phase with a `methodology` field
- **THEN** the phase is persisted with that methodology and the read returns it
