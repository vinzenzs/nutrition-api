## MODIFIED Requirements

### Requirement: Daily and range summaries compute adherence against goals

The summary capability SHALL include an `adherence` object in `GET /summary/daily` and in each per-day entry of `GET /summary/range`, computed against the **effective goals for that calendar date** (per the effective-goals resolution chain: per-date override > phase template > singleton default). For EVERY goal-targeted nutrient (i.e. every field where the effective goal is non-null), `adherence` includes exactly one entry with the shape `{actual: number|null, target: {min?, max?}, delta_pct?: number, status: "under" | "on" | "over" | "no_data"}`. The same code path produces adherence in both endpoints — daily and range MUST report the same shape and the same status for the same `(effective goals, daily totals)` pair. Each day's response also exposes a `goal_source` string identifying which set produced the adherence rows: `"override" | "phase_template" | "default" | "none"`. When `goal_source == "phase_template"`, the response SHALL ALSO carry a sibling `phase_name` string field naming the resolved phase. For any other `goal_source`, `phase_name` is absent (or null).

#### Scenario: Adherence row exists for every configured goal

- **WHEN** the effective goals row has 15 fields set and the day has logged meals
- **THEN** the daily summary's `adherence` object has 15 entries
- **AND** every entry has a non-empty `target` matching the effective goal's `{min?, max?}`

#### Scenario: Adherence row exists for every configured goal on empty days

- **WHEN** the effective goals row has 15 fields set and the day has zero logged meals
- **THEN** the daily summary's `adherence` object STILL has 15 entries
- **AND** every entry has `actual: null` and `status: "no_data"`

#### Scenario: Daily and range agree on shape

- **WHEN** the same effective goals and the same day's data are queried via `GET /summary/daily?date=D` and via `GET /summary/range?from=D&to=D` (single-day range)
- **THEN** the adherence object in the daily response is structurally identical (same keys, same values, same statuses) to the adherence object on the matching day in the range response
- **AND** `goal_source` is the same on both responses
- **AND** `phase_name` is the same on both responses (both present and matching, or both absent)

#### Scenario: kcal Range status uses the user-set bounds

- **WHEN** kcal goal is `{"min": 2090, "max": 2310}` and daily kcal total is 2150
- **THEN** `adherence.kcal = {actual: 2150, target: {"min": 2090, "max": 2310}, delta_pct: -2.3, status: "on"}`

- **WHEN** the daily total is 1900
- **THEN** `adherence.kcal.status = "under"`

- **WHEN** the daily total is 2400
- **THEN** `adherence.kcal.status = "over"`

#### Scenario: Range min/max nutrients status from boundaries

- **WHEN** `protein_g` goal is `{"min": 150, "max": 190}` and the daily total is 160
- **THEN** `adherence.protein_g.status = "on"`
- **AND** at 140 the status is `"under"`
- **AND** at 200 the status is `"over"`

#### Scenario: Min-only nutrients never produce "over"

- **WHEN** the goal is `{"min": 30}` (no max) and the daily total is 60
- **THEN** `adherence.fiber_g.status = "on"`

#### Scenario: Max-only nutrients never produce "under"

- **WHEN** the goal is `{"max": 50}` (no min) and the daily total is 10
- **THEN** `adherence.sugar_g.status = "on"`

#### Scenario: Macro total of zero on a day with meals is not no_data

- **WHEN** the day has logged meals but none contributed to `protein_g`, so the total is 0 (macros are always numeric in totals)
- **THEN** the `adherence.protein_g.actual` is `0`
- **AND** the status reflects `under` (assuming `min > 0`)
- **AND** the status is NOT `no_data` (the day had logged meals; this is real progress info)

#### Scenario: Micro total null produces no_data

- **WHEN** a goal exists for `iron_mg` and no contributing meal on the day had a non-null `iron_mg` value
- **THEN** `adherence.iron_mg = {actual: null, target: <the goal>, status: "no_data"}`
- **AND** `delta_pct` is absent
- **AND** `totals.iron_mg` is also absent from totals (existing behaviour preserved)

#### Scenario: No goal set means no adherence entry

- **WHEN** `kcal` goal is null in the effective goals (the user has not set one on the override, the phase template, or the default)
- **THEN** the `adherence` object has no `kcal` key
- **AND** the daily totals still include kcal as a raw number

#### Scenario: phase_name present only when goal_source is phase_template

- **WHEN** `goal_source == "phase_template"` on a day's response
- **THEN** the response includes `"phase_name": "<the resolved phase's name>"`

- **WHEN** `goal_source` is any of `"override"`, `"default"`, or `"none"`
- **THEN** `phase_name` is absent (or null) — the field is meaningful only for the phase-driven case

### Requirement: Effective goals resolve override-first, default-fallback

The system SHALL expose `EffectiveFor(date)` semantics inside the summary capability: when computing adherence for a given calendar date, the system consults the effective-goals chain **in this priority order**:

1. `daily_goal_overrides` for that date — if a row exists, the override drives adherence; `goal_source = "override"`.
2. Otherwise, `training_phases` containing that date — if exactly one matches AND its `default_template_id` is non-null, the linked template's bounds drive adherence; `goal_source = "phase_template"` and `phase_name = <the phase's name>`. If multiple phases overlap, the most-recently-updated one wins. If the matched phase has no template, fall through to step 3.
3. Otherwise, the singleton default from `nutrition_goals` — if present, drives adherence; `goal_source = "default"`.
4. Otherwise, no adherence is computed; `goal_source = "none"`.

This chain is the trunk: every day's adherence in `GET /summary/daily` and in each per-day entry of `GET /summary/range` resolves through it.

#### Scenario: Override drives adherence when present

- **WHEN** the user has both a default goal of `kcal: {min:2090, max:2310}` and an override on `2026-06-15` of `kcal: {min:2280, max:2520}`
- **AND** the client calls `GET /summary/daily?date=2026-06-15`
- **THEN** the adherence row for `kcal` uses the override bounds `{min:2280, max:2520}`
- **AND** the response includes `"goal_source": "override"`

#### Scenario: Phase template drives adherence when no override but phase covers the date

- **WHEN** no override exists for `2026-07-15`
- **AND** a phase covers `2026-07-15` with `default_template_id` pointing at a template with `kcal: {min: 2400, max: 2600}`
- **AND** the singleton default also exists
- **AND** the client calls `GET /summary/daily?date=2026-07-15`
- **THEN** the adherence row for `kcal` uses the template's bounds `{min: 2400, max: 2600}`
- **AND** the response includes `"goal_source": "phase_template"`
- **AND** the response includes `"phase_name": "<the phase's name>"`

#### Scenario: Override wins over a covering phase

- **WHEN** an override exists for `2026-07-15` AND a phase also covers `2026-07-15`
- **THEN** the adherence row uses the override's bounds
- **AND** `goal_source` is `"override"`
- **AND** `phase_name` is absent

#### Scenario: Phase without template falls through to default

- **WHEN** a phase covers `2026-07-15` with `default_template_id = NULL`
- **AND** no override exists
- **AND** the singleton default has `kcal: {min: 2000}`
- **THEN** the adherence row uses the singleton default's bounds
- **AND** `goal_source` is `"default"`
- **AND** `phase_name` is absent (the phase had no template; it does not drive adherence)

#### Scenario: Default drives adherence when no override and no phase template

- **WHEN** no override exists for `2026-06-14`
- **AND** no phase covers `2026-06-14`
- **AND** the singleton default is set
- **AND** the client calls `GET /summary/daily?date=2026-06-14`
- **THEN** the adherence row uses the default bounds
- **AND** the response includes `"goal_source": "default"`

#### Scenario: Neither override nor phase nor default means no adherence

- **WHEN** no override, no phase with template, AND no singleton default exists for the date
- **AND** the client calls `GET /summary/daily?date=…`
- **THEN** the response does NOT include an `adherence` object
- **AND** the response includes `"goal_source": "none"`

#### Scenario: Range summary switches goal source day-by-day

- **WHEN** the client calls `GET /summary/range?from=2026-07-10&to=2026-07-16` with:
  - phase A covering `2026-07-12` through `2026-07-14` (with template)
  - override on `2026-07-13`
  - no other overrides or phases
- **THEN** the per-day `goal_source` values are `["default","default","phase_template","override","phase_template","default","default"]`
- **AND** `phase_name` is set per-day where goal_source is `phase_template` (days 3 and 5)
- **AND** each day's adherence rows reflect that day's effective goals

#### Scenario: Range summary fetches phases and overrides in one batch each

- **WHEN** the client calls `GET /summary/range?from=…&to=…` spanning N days
- **THEN** the system issues at most one query to fetch every override in the window AND at most one query to fetch every phase intersecting the window (NOT one per day for either)

#### Scenario: Overlapping phases — most-recently-updated wins

- **WHEN** two phases both cover `2026-07-15` — phase A (`recovery`, with template T_recovery) updated at T1, phase B (`race_week`, with template T_race) updated at T2 > T1
- **AND** no override exists
- **THEN** the resolver picks phase B
- **AND** the adherence row uses T_race's bounds
- **AND** `phase_name` is phase B's name

#### Scenario: goal_source field is omitted when adherence is suppressed

- **WHEN** a daily summary request supplies `meal_type` (which suppresses adherence)
- **THEN** the response omits the `goal_source` field
- **AND** the response omits `phase_name`
- **AND** the response does NOT include an `adherence` object (existing behaviour preserved)
