## ADDED Requirements

### Requirement: Rolling-window summary

The system SHALL expose `GET /summary/rolling?anchor_date=<YYYY-MM-DD>&window_days=<int>&tz=<iana>` returning the trailing-window average of nutrition totals as of `anchor_date`. The window is `[anchor_date − (window_days − 1) days, anchor_date]`, both inclusive, with calendar-day buckets resolved in the requested `tz` (defaulting to `DEFAULT_USER_TZ`). The endpoint is read-only and SHALL NOT consume an `Idempotency-Key` header.

#### Scenario: Complete-data window

- **WHEN** the client calls `GET /summary/rolling?anchor_date=2026-06-08&window_days=7&tz=Europe/Berlin`
- **AND** every calendar day in the inclusive window `2026-06-02..2026-06-08` has at least one logged meal
- **THEN** the response has shape:
  ```
  {
    "anchor_date": "2026-06-08",
    "window_days": 7,
    "tz": "Europe/Berlin",
    "averages": { "kcal": 2280.5, "protein_g": 128.0, ... },
    "days_with_data": 7,
    "total_days": 7,
    "days": [
      { "date": "2026-06-02", "totals": { ... }, "has_data": true },
      ...
      { "date": "2026-06-08", "totals": { ... }, "has_data": true }
    ],
    "adherence": { ... },
    "goal_source": "default" | "override"
  }
  ```
- **AND** `days` is ordered ascending by `date` with exactly seven entries (one per calendar day in the window)
- **AND** `averages.kcal` equals the mean of the seven per-day `totals.kcal` values

#### Scenario: Default tz falls back to DEFAULT_USER_TZ

- **WHEN** the client omits the `tz` query parameter
- **THEN** the calendar-day buckets are computed in the server's configured `DEFAULT_USER_TZ`
- **AND** the response echoes that `tz` value

### Requirement: Rolling averages use days-with-data divisor

The system SHALL compute each entry in `averages` as the mean of the per-day totals across days where at least one meal was logged (`has_data: true`). Days with no meals SHALL NOT contribute to the divisor. The response SHALL expose `days_with_data` and `total_days` so the caller can detect sparse windows.

#### Scenario: Sparse window divides by logged days only

- **WHEN** the window covers seven days and the client has logged meals on only six of them (one day is empty)
- **AND** total kcal across those six days is `12 600`
- **THEN** `averages.kcal` is `2100.0` (`12600 / 6`), not `1800.0` (`12600 / 7`)
- **AND** `days_with_data` is `6`
- **AND** `total_days` is `7`

#### Scenario: Empty window returns null averages

- **WHEN** the window has zero days with data (no meals logged anywhere in the seven-day range)
- **THEN** every entry in `averages` is `0.0` (or `null` for the nullable-micro fields)
- **AND** `days_with_data` is `0`
- **AND** every per-day row has `has_data: false`
- **AND** the response status remains `200 OK`

### Requirement: Per-day rows distinguish "no data" from "logged zero"

Each entry in the response's `days` array SHALL carry a `has_data: bool` field. `has_data` is `true` when at least one meal was logged on that calendar day, regardless of whether the resulting totals are zero. `has_data` is `false` when no meal was logged on that day.

#### Scenario: No-meal day is flagged

- **WHEN** the user logged no meals on `2026-06-05`
- **THEN** the response's row for `2026-06-05` has `has_data: false`
- **AND** every numeric field in that row's `totals` is `0.0` (or `null` for nullable micros)

#### Scenario: Zero-kcal meal day is NOT flagged as missing

- **WHEN** the user logged a single meal on `2026-06-05` whose computed kcal is `0`
- **THEN** the response's row for `2026-06-05` has `has_data: true`
- **AND** `totals.kcal` is `0.0`

### Requirement: Adherence is computed against the goal resolved at the anchor

The system SHALL compute the `adherence` object against the goal that applies at `anchor_date`, honoring any per-date override in `daily_goal_overrides` for that exact date. Adherence semantics (`actual` / `target` / `delta_pct` / `status`) match the existing `daily_summary` adherence contract (`unify-adherence-shape`). `actual` is the window-average value for each nutrient; `target` is the goal range; `status` is `under` / `on` / `over` / `no_data`.

#### Scenario: Adherence against default goals

- **WHEN** no override applies at the anchor date
- **THEN** `goal_source` is `"default"`
- **AND** each adherence entry's `target` reflects the default `nutrition_goals` row

#### Scenario: Adherence honors an override at the anchor

- **WHEN** an entry exists in `daily_goal_overrides` for `anchor_date`
- **THEN** `goal_source` is `"override"`
- **AND** each adherence entry's `target` reflects that override

#### Scenario: Adherence status is no_data when the window has no logged data

- **WHEN** `days_with_data` is `0`
- **THEN** every adherence entry has `status: "no_data"` and `actual: null`

### Requirement: Window-day bounds and parameter validation

The system SHALL enforce `2 <= window_days <= 30`. Invalid windows return `400 Bad Request` with the corresponding error code.

#### Scenario: Below minimum is rejected

- **WHEN** `window_days` is `1` (or less)
- **THEN** the system returns `400 Bad Request` with `{"error":"window_days_invalid","range":{"min":2,"max":30}}`

#### Scenario: Above maximum is rejected

- **WHEN** `window_days` is `31` (or more)
- **THEN** the system returns `400 Bad Request` with `{"error":"window_days_invalid","range":{"min":2,"max":30}}`

#### Scenario: Missing anchor_date is rejected

- **WHEN** the client omits `anchor_date`
- **THEN** the system returns `400 Bad Request` with `{"error":"anchor_date_required"}`

#### Scenario: Malformed anchor_date is rejected

- **WHEN** `anchor_date` is supplied but cannot be parsed as `YYYY-MM-DD`
- **THEN** the system returns `400 Bad Request` with `{"error":"anchor_date_invalid"}`

#### Scenario: Missing window_days is rejected

- **WHEN** the client omits `window_days`
- **THEN** the system returns `400 Bad Request` with `{"error":"window_days_required"}`

#### Scenario: Malformed window_days is rejected

- **WHEN** `window_days` is supplied but cannot be parsed as an integer
- **THEN** the system returns `400 Bad Request` with `{"error":"window_days_invalid","range":{"min":2,"max":30}}`

#### Scenario: Invalid tz is rejected

- **WHEN** `tz` is supplied but cannot be parsed as an IANA timezone
- **THEN** the system returns `400 Bad Request` with `{"error":"tz_invalid"}`

### Requirement: Rolling-window numeric outputs rounded at the response boundary

The system SHALL round every numeric field in `averages`, every per-day `totals` field, and every adherence `actual` / `delta_pct` to one decimal place at the response boundary (matching the rounding rule used by `daily_summary` / `range_summary`).

#### Scenario: Average kcal rounds at the response boundary

- **WHEN** the underlying computation produces an average of `2280.4666...`
- **THEN** the response shows `2280.5`
