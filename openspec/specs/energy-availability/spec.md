# energy-availability Specification

## Purpose

Define a pure-computation read endpoint that derives per-day Energy Availability (EA) from existing intake, workout-burn, and body-composition primitives. For an endurance athlete in a deficit, EA is the single most important number — it predicts both performance ceiling and longer-term hormonal/bone health. The Loucks bands are concrete and well-published: `< 30 kcal/kg FFM/day` is "low" (real physiological risk), `30–45` is "sub-optimal", `>= 45` is "adequate". Every input EA needs already lives in the API (meals, workouts, body weight with optional body-fat %); this capability composes them over a date window without introducing new tables or migrations. The response surfaces what was inferred vs supplied and flags days with incomplete data so missing-burn-kcal does not masquerade as healthy EA.

## Requirements

### Requirement: GET /energy/availability returns per-day Energy Availability over a window

The system SHALL expose `GET /energy/availability?from=<rfc3339>&to=<rfc3339>&tz=<iana>&lean_mass_kg=<float>&body_fat_pct=<float>` that returns the per-day Energy Availability (EA) computed from `intake_kcal` (sum of meal entries on the day in `tz`), `exercise_energy_kcal` (sum of `workouts.kcal_burned` for workouts whose `started_at` falls on the day in `tz`), and a single window-level `FFM_kg` derived per the resolution order in §Requirement: FFM resolution. The formula is `EA = (intake_kcal - exercise_energy_kcal) / FFM_kg`, expressed in `kcal / kg FFM / day`.

#### Scenario: Window with complete data on every day

- **WHEN** the client calls `GET /energy/availability?from=2026-06-01T00:00:00Z&to=2026-06-08T00:00:00Z&tz=Europe/Berlin&lean_mass_kg=62`
- **AND** every day in the window has at least one meal logged and every workout in the window has `kcal_burned` set
- **THEN** the response has shape:
  ```
  {
    "from": "...", "to": "...", "tz": "Europe/Berlin",
    "days": [
      {
        "date": "2026-06-01",
        "intake_kcal": 2400, "exercise_energy_kcal": 600,
        "ea": 29.0, "band": "low",
        "missing_burn_workout_ids": [],
        "complete_data": true
      },
      ...
    ],
    "window": {
      "avg_ea": 32.5,
      "band": "sub_optimal",
      "days_with_complete_data": 7,
      "total_days": 7
    },
    "composition": {
      "ffm_kg": 62.0,
      "source": "explicit_lean_mass",
      "body_weight_kg": 73.0,
      "body_weight_source": "rolling_7d_avg"
    }
  }
  ```
- **AND** `days` is ordered by `date` ascending with one entry per calendar day in `[from, to)` in the requested `tz`

#### Scenario: Default `tz` falls back to `DEFAULT_USER_TZ`

- **WHEN** the client omits the `tz` query parameter
- **THEN** the calendar-day buckets are computed in the server's configured `DEFAULT_USER_TZ`
- **AND** the response echoes that `tz` value

#### Scenario: Missing `from` or `to` is rejected

- **WHEN** the client omits `from` or `to`
- **THEN** the system returns `400 Bad Request` with `{"error":"window_required"}`

#### Scenario: Inverted or malformed window is rejected

- **WHEN** `from >= to`, OR either parameter is not RFC 3339
- **THEN** the system returns `400 Bad Request` with `{"error":"window_invalid"}`

#### Scenario: Window larger than 92 days is rejected

- **WHEN** the supplied window spans more than 92 days
- **THEN** the system returns `400 Bad Request` with `{"error":"range_too_large","max_days":92}`

#### Scenario: Invalid `tz` is rejected

- **WHEN** `tz` is supplied but cannot be parsed as an IANA timezone
- **THEN** the system returns `400 Bad Request` with `{"error":"tz_invalid"}`

#### Scenario: Numeric outputs are rounded at the response boundary

- **WHEN** any computed EA, FFM, intake, or burn value would have more than one decimal place
- **THEN** the response shows the value rounded to one decimal place (consistent with the existing nutrient-rounding rule)

### Requirement: Energy availability days flag workouts with missing `kcal_burned`

The system SHALL include in each day's response a `missing_burn_workout_ids` array listing every workout whose `started_at` falls on that day and whose `kcal_burned` is NULL. The day's `exercise_energy_kcal` SHALL be computed as the sum of the workouts that HAVE `kcal_burned`; the missing ones contribute zero. A day with at least one entry in `missing_burn_workout_ids` SHALL have `complete_data: false`. The window's `avg_ea` SHALL be computed across days with `complete_data: true` only.

#### Scenario: Day with one missing-burn workout is flagged

- **WHEN** a day has two workouts: workout A with `kcal_burned: 400`, workout B with `kcal_burned: NULL`
- **THEN** the day's `exercise_energy_kcal` is `400`
- **AND** `missing_burn_workout_ids` lists workout B's id
- **AND** `complete_data` is `false`
- **AND** the EA value is computed as `(intake_kcal - 400) / FFM_kg`

#### Scenario: Window aggregate excludes incomplete days

- **WHEN** the window has 7 days, 5 of which have `complete_data: true` and 2 incomplete
- **THEN** `window.avg_ea` is the mean of the 5 complete days' EA values
- **AND** `window.days_with_complete_data` is `5`
- **AND** `window.total_days` is `7`

#### Scenario: Window with zero complete days

- **WHEN** every day in the window has at least one missing-burn workout
- **THEN** `window.avg_ea` is `null`
- **AND** `window.band` is omitted (or `null`)
- **AND** `window.days_with_complete_data` is `0`
- **AND** the response status remains `200 OK` — incomplete-data is not an error condition

### Requirement: Loucks band classification

The system SHALL classify each EA value into one of three bands using the Loucks thresholds:

```
ea < 30           → "low"
30 <= ea < 45     → "sub_optimal"
ea >= 45          → "adequate"
```

The same classification is applied to the per-day `band` field and to `window.band` (computed from `window.avg_ea`).

#### Scenario: Boundary at 30.0 lands in sub_optimal

- **WHEN** a day's EA evaluates to exactly `30.0`
- **THEN** the day's `band` is `"sub_optimal"` (the lower-bound is inclusive)

#### Scenario: Boundary at 45.0 lands in adequate

- **WHEN** a day's EA evaluates to exactly `45.0`
- **THEN** the day's `band` is `"adequate"` (the upper bound of sub_optimal is exclusive)

#### Scenario: Just-under-30 lands in low

- **WHEN** a day's EA evaluates to `29.9`
- **THEN** the day's `band` is `"low"`

### Requirement: FFM resolution

The system SHALL resolve the window-level `FFM_kg` using the following ordered rules, stopping at the first that applies. The chosen rule SHALL be reported in `composition.source`.

1. If `lean_mass_kg` query param supplied: `FFM_kg = lean_mass_kg`. `composition.source = "explicit_lean_mass"`.
2. Else if `body_fat_pct` query param supplied AND there is at least one body-weight entry to derive the body weight from: `FFM_kg = body_weight_kg × (1 − body_fat_pct/100)`. `composition.source = "explicit_body_fat"`.
3. Else if the most-recent `body_weight_entries` row whose `logged_at` is in `[from, to)` carries a non-null `body_fat_pct`: `FFM_kg = body_weight_kg × (1 − body_fat_pct/100)`. `composition.source = "stored_body_fat"`.
4. Else if there is at least one body-weight entry (in or before the window) to derive the body weight from: `FFM_kg = body_weight_kg × 0.85`. `composition.source = "estimated_85pct"`. The response SHALL also include `composition.composition_estimated: true` at the composition object root.

If none of the above conditions hold (no body-weight data at all), the system SHALL return `400 Bad Request` with `{"error":"weight_data_missing"}`.

`body_weight_kg` is resolved per the §Requirement: Body weight resolution rule.

#### Scenario: Explicit lean mass wins over everything

- **WHEN** the request supplies `lean_mass_kg=58` AND the user has stored body-fat % data in the window
- **THEN** `FFM_kg` is `58.0` and `composition.source` is `"explicit_lean_mass"`
- **AND** the stored body-fat % is ignored

#### Scenario: Explicit body_fat_pct overrides stored

- **WHEN** the request supplies `body_fat_pct=15` AND the latest stored body-weight entry has `body_fat_pct: 18`
- **THEN** the explicit `15%` is used in the FFM computation
- **AND** `composition.source` is `"explicit_body_fat"`

#### Scenario: Stored body_fat_pct from most-recent in-window entry

- **WHEN** the window contains two body-weight entries — an older one with `body_fat_pct: 18` and a newer one with `body_fat_pct: 16`
- **THEN** the newer entry's `16%` is used
- **AND** `composition.source` is `"stored_body_fat"`

#### Scenario: 85% fallback when no body-fat data exists

- **WHEN** the user has body-weight entries but none carry `body_fat_pct`
- **THEN** `FFM_kg = body_weight_kg × 0.85`
- **AND** `composition.source` is `"estimated_85pct"`
- **AND** `composition.composition_estimated` is `true`

#### Scenario: No weight data at all returns 400

- **WHEN** the user has no `body_weight_entries` at all (in or before the window) AND no `lean_mass_kg` query parameter
- **THEN** the system returns `400 Bad Request` with `{"error":"weight_data_missing"}`

#### Scenario: Explicit lean_mass_kg works even when no weight entries exist

- **WHEN** the request supplies `lean_mass_kg=58` AND the user has zero body-weight entries
- **THEN** the request succeeds with `composition.source = "explicit_lean_mass"`
- **AND** `composition.body_weight_kg` is `null` (no data to report)
- **AND** `composition.body_weight_source` is `null`

#### Scenario: Invalid lean_mass_kg or body_fat_pct is rejected

- **WHEN** `lean_mass_kg` is `<= 0` or NaN/Inf
- **THEN** the system returns `400 Bad Request` with `{"error":"lean_mass_kg_invalid"}`
- **WHEN** `body_fat_pct` is `< 0`, `>= 100`, or NaN/Inf
- **THEN** the system returns `400 Bad Request` with `{"error":"body_fat_pct_invalid"}`

### Requirement: Body weight resolution

The system SHALL resolve a single window-level `body_weight_kg` value as follows. The chosen rule is reported in `composition.body_weight_source`.

1. If at least one body-weight entry's `logged_at` falls in the seven days preceding `to` (i.e. `[to − 7d, to)`): `body_weight_kg = rolling 7-day mean weight` of those entries. `composition.body_weight_source = "rolling_7d_avg"`.
2. Else if at least one body-weight entry's `logged_at` falls in `[from, to)`: `body_weight_kg = mean weight` of those entries (the available-data fallback when the window is shorter than 7 days). `composition.body_weight_source = "in_window_mean"`.
3. Else if at least one body-weight entry exists at all: `body_weight_kg = weight` of the most-recent entry before `from`. `composition.body_weight_source = "last_before_window"`.
4. Else: see the `weight_data_missing` rule in §Requirement: FFM resolution.

#### Scenario: Rolling 7-day average wins when in-window data exists

- **WHEN** the window is `2026-06-01..2026-06-08` and there are body-weight entries on June 2, 4, 6
- **THEN** `body_weight_kg` is the mean of those three entries
- **AND** `composition.body_weight_source` is `"rolling_7d_avg"`

#### Scenario: Falls back to last-before-window when window is empty

- **WHEN** the window is `2026-06-01..2026-06-08` and there are no body-weight entries in that range, but there is an entry on May 28
- **THEN** `body_weight_kg` is the May-28 entry's weight
- **AND** `composition.body_weight_source` is `"last_before_window"`

### Requirement: Day calendar boundaries respect the requested timezone

The system SHALL bucket meals and workouts into per-day rows using the requested `tz`. Meals' `logged_at` and workouts' `started_at` SHALL be converted to the requested TZ before extracting the calendar date. Workouts' `ended_at` is NOT used for the bucketing — a workout that spans midnight belongs to the calendar day on which it started.

#### Scenario: Meal logged at 23:30 local belongs to that day, not the next

- **WHEN** a meal is logged at `2026-06-07T22:30:00Z` and `tz=Europe/Berlin` (UTC+2 in summer, local time 00:30 of June 8)
- **AND** the request window is `2026-06-08T00:00:00Z..2026-06-09T00:00:00Z`
- **THEN** the meal contributes to the `2026-06-08` row in the response (its local date is June 8)

#### Scenario: Workout that spans midnight is attributed by start day

- **WHEN** a workout has `started_at` corresponding to `2026-06-07T23:45:00 Europe/Berlin` and `ended_at` at `2026-06-08T01:15:00 Europe/Berlin`
- **THEN** the entire workout's `kcal_burned` contributes to the `2026-06-07` row, not split across days

### Requirement: Empty calendar days appear with zero intake and zero burn

The system SHALL emit one `days` row for every calendar day in `[from, to)` regardless of whether intake or workouts exist on that day. A day with no meals has `intake_kcal: 0` and (without a corresponding burn) `ea` value equal to `-burned_kcal / FFM_kg` (which may be `0.0` or negative for rest days).

#### Scenario: Empty day still appears in the days list

- **WHEN** the window covers 7 days and day 4 has no meals and no workouts
- **THEN** the response's `days` array has 7 entries
- **AND** day 4's row has `intake_kcal: 0`, `exercise_energy_kcal: 0`, `ea: 0.0`, `band: "low"`, `complete_data: true`

#### Scenario: Empty day classifies into "low" band because EA is 0

- **WHEN** a day has zero meals and zero workouts
- **THEN** its `ea` is `0.0` and its `band` is `"low"`
- **AND** `complete_data` is `true` (no missing-burn workouts on a day with no workouts)

### Requirement: Energy availability does not modify any state

The endpoint SHALL be idempotent (in the HTTP sense) and read-only. It SHALL NOT write any rows and SHALL NOT consume an `Idempotency-Key` header.

#### Scenario: Endpoint never writes

- **WHEN** the client invokes `GET /energy/availability` an arbitrary number of times
- **THEN** no rows in `meals`, `workouts`, or `body_weight_entries` are created, updated, or deleted as a side effect

#### Scenario: Idempotency-Key header is ignored

- **WHEN** the client supplies an `Idempotency-Key` header on the GET request
- **THEN** the header is ignored and no row is added to the idempotency cache
