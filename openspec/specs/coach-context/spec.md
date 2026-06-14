# coach-context Specification

## Purpose
TBD - created by archiving change add-coach-context-endpoints. Update Purpose after archive.
## Requirements
### Requirement: Training context aggregate read

The system SHALL expose `GET /context/training` returning a single composition-only bundle for grounding training advice: the training phase covering the anchor date — **including that phase's `methodology` Markdown prose when set, so the coach has the cited "why" of the current phase in the same call** — the latest fitness snapshot on/before that date within the lookback window (VO2max, acute/chronic load, training status, race predictors), the derived ACWR (acute ÷ chronic load) when both loads are present, a recent-load summary and the recent completed workouts over a lookback window (default 14 days), and the upcoming planned workouts over a lookahead window (default 7 days). It SHALL accept `date` (YYYY-MM-DD, defaulting to today in the configured zone), `tz`, `lookback_days`, and `lookahead_days`; lookback and lookahead SHALL be clamped to sane bounds. The bundle SHALL be built from existing read repos in parallel with no partial result on error, and numeric fields SHALL be rounded at the response boundary. Absent snapshots (no fitness/phase) SHALL serialize as null, not errors; a covering phase with no `methodology` SHALL serialize that field as null.

#### Scenario: Grounded training read

- **WHEN** the client GETs `/context/training?date=2026-06-14`
- **THEN** the response includes the covering phase (or null), the latest fitness snapshot (or null), `acwr` when acute and chronic load are both present, a `recent_load` summary plus recent completed workouts within the lookback, and upcoming planned workouts within the lookahead

#### Scenario: The covering phase carries its methodology

- **WHEN** the phase covering the anchor date has a `methodology` set and the client GETs `/context/training`
- **THEN** the phase slice of the bundle includes that `methodology` prose verbatim

#### Scenario: A phase without methodology serializes null

- **WHEN** the covering phase has no `methodology`
- **THEN** the phase slice's `methodology` field is null and the response is otherwise unchanged

#### Scenario: Quiet history is not an error

- **WHEN** there are no workouts, fitness, or phase for the window
- **THEN** the response is 200 with null/empty fields, not an error

#### Scenario: Unbounded windows are clamped

- **WHEN** the client passes an absurd `lookback_days`
- **THEN** the server clamps it to the maximum rather than scanning unboundedly

### Requirement: Recovery context aggregate read

The system SHALL expose `GET /context/recovery` returning the latest recovery snapshot on/before the anchor date within the window (sleep, HRV, resting HR, body battery, training readiness, …) plus the recent trend of snapshots over a window (default 7 days). It SHALL accept `date` (YYYY-MM-DD, defaulting to today) and `days` (clamped). Absent data SHALL serialize as null/empty, not an error.

#### Scenario: Recovery trend read

- **WHEN** the client GETs `/context/recovery?date=2026-06-14&days=7`
- **THEN** the response includes `latest` (the most recent snapshot on/before the date, or null) and `recent` (the snapshots over the window, ascending)

### Requirement: Aggregate context reads are exposed as MCP tools

The MCP server SHALL expose `get_training_context` and `get_recovery_context` tools, each performing exactly one loopback call to the corresponding REST endpoint under the caller's bearer, and both names SHALL appear in the server's announced tool surface. The names SHALL be identical to those the in-app chat coach uses, so the two AI surfaces do not diverge.

#### Scenario: MCP announces the aggregate context tools

- **WHEN** an MCP client lists tools
- **THEN** `get_training_context` and `get_recovery_context` are announced alongside the existing surface

