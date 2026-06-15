## MODIFIED Requirements

### Requirement: Training context aggregate read

The system SHALL expose `GET /context/training` returning a single composition-only bundle for grounding training advice: the training phase covering the anchor date — **including that phase's `methodology` Markdown prose when set, so the coach has the cited "why" of the current phase in the same call** — the latest fitness snapshot on/before that date within the lookback window (VO2max, acute/chronic load, training status, race predictors), the derived ACWR (acute ÷ chronic load) when both loads are present, the athlete's physiology config block (FTP, thresholds, HR/power zones) when set, a derived `watts_per_kg` (FTP ÷ the latest bodyweight on/before the anchor date) when both inputs are present, a recent-load summary and the recent completed workouts over a lookback window (default 14 days), and the upcoming planned workouts over a lookahead window (default 7 days). It SHALL accept `date` (YYYY-MM-DD, defaulting to today in the configured zone), `tz`, `lookback_days`, and `lookahead_days`; lookback and lookahead SHALL be clamped to sane bounds. The bundle SHALL be built from existing read repos in parallel with no partial result on error, and numeric fields SHALL be rounded at the response boundary. Absent snapshots (no fitness/phase/athlete-config/bodyweight) SHALL serialize as null, not errors; a covering phase with no `methodology` SHALL serialize that field as null; `watts_per_kg` SHALL be null whenever FTP or bodyweight is missing.

#### Scenario: Grounded training read

- **WHEN** the client GETs `/context/training?date=2026-06-14`
- **THEN** the response includes the covering phase (or null), the latest fitness snapshot (or null), `acwr` when acute and chronic load are both present, the `athlete_config` block when set, `watts_per_kg` when FTP and bodyweight are both present, a `recent_load` summary plus recent completed workouts within the lookback, and upcoming planned workouts within the lookahead

#### Scenario: The covering phase carries its methodology

- **WHEN** the phase covering the anchor date has a `methodology` set and the client GETs `/context/training`
- **THEN** the phase slice of the bundle includes that `methodology` prose verbatim

#### Scenario: A phase without methodology serializes null

- **WHEN** the covering phase has no `methodology`
- **THEN** the phase slice's `methodology` field is null and the response is otherwise unchanged

#### Scenario: Athlete config and W/kg are surfaced when present

- **WHEN** an athlete_config row with an FTP is set and a bodyweight has been logged on/before the anchor date, and the client GETs `/context/training`
- **THEN** the response includes the `athlete_config` block and a `watts_per_kg` equal to FTP ÷ the latest bodyweight in kg, rounded at the response boundary

#### Scenario: W/kg is null when an input is missing

- **WHEN** athlete_config has no FTP, or no bodyweight exists on/before the anchor date
- **THEN** `watts_per_kg` is null and the bundle is otherwise returned normally (200, not an error)

#### Scenario: Quiet history is not an error

- **WHEN** there are no workouts, fitness, phase, athlete-config, or bodyweight for the window
- **THEN** the response is 200 with null/empty fields, not an error

#### Scenario: Unbounded windows are clamped

- **WHEN** the client passes an absurd `lookback_days`
- **THEN** the server clamps it to the maximum rather than scanning unboundedly
