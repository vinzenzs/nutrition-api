## MODIFIED Requirements

### Requirement: GET /workouts/{id}/fueling returns pre/intra/post intake windows

The system SHALL expose `GET /workouts/{id}/fueling?pre_window_min=<int>&post_window_min=<int>` returning three time-anchored buckets — pre, intra, post — each carrying **three** separate aggregations for entries whose `logged_at` falls within the corresponding window: a **nutrition** sub-object (from `meal_entries`), a **hydration** sub-object (from `hydration_entries`), and a **workout_fuel** sub-object (from `workout_fuel_entries`). The windows are derived from the workout's `started_at` and `ended_at` plus the supplied (or defaulted) pre/post minutes. Aggregation is time-window-based: any entry whose `logged_at` falls in a window is included regardless of its `workout_id` value.

#### Scenario: Default windows are 240 min pre / 60 min post

- **WHEN** the client calls `GET /workouts/{id}/fueling` without `pre_window_min` or `post_window_min`
- **THEN** `pre_window.minutes` is `240`
- **AND** `post_window.minutes` is `60`

#### Scenario: Response shape carries three separate sub-objects per window

- **WHEN** the response is well-formed
- **THEN** each window object has the shape `{start, end, minutes, nutrition: {totals, entry_count}, hydration: {total_ml, entry_count}, workout_fuel: {totals, entry_count}}`
- **AND** the `nutrition.totals` shape matches `/summary/daily.totals` (macros + nullable micros)
- **AND** the `workout_fuel.totals` shape carries `{quantity_ml, carbs_g, sodium_mg, potassium_mg, caffeine_mg}` — each field nullable, summed across contributing entries
- **AND** units never mix: no ml inside `nutrition.totals`; no kcal inside `hydration` or `workout_fuel`; no per-100g nutriments inside `workout_fuel`

#### Scenario: Workout-fuel sub-object sums contributing entries

- **WHEN** two workout-fuel entries fall in the intra window with `{carbs_g: 25, sodium_mg: 100}` and `{carbs_g: 25, sodium_mg: 200, quantity_ml: 500}`
- **THEN** `intra_window.workout_fuel.totals` is `{carbs_g: 50, sodium_mg: 300, quantity_ml: 500}`
- **AND** `intra_window.workout_fuel.entry_count` is `2`
- **AND** `quantity_ml` is summed across only those entries that supplied it; `null + 500 = 500`, not `null`

#### Scenario: Workout-fuel sub-object is present even when there are no contributing entries

- **WHEN** no workout-fuel entries fall in a particular window
- **THEN** `workout_fuel.entry_count` is `0`
- **AND** `workout_fuel.totals` carries zeros (or nulls) for every field — the sub-object is NOT omitted

#### Scenario: Pre-window covers [started_at − pre_window_min, started_at)

- **WHEN** a meal is logged 30 minutes before the workout's `started_at`
- **AND** `pre_window_min >= 30`
- **THEN** the meal contributes to `pre_window.nutrition.totals`
- **AND** the meal does NOT contribute to `intra_window` or `post_window`

#### Scenario: Intra-window covers [started_at, ended_at)

- **WHEN** a hydration entry is logged at a time T with `started_at <= T < ended_at`
- **THEN** the entry contributes to `intra_window.hydration.total_ml`

#### Scenario: Post-window covers [ended_at, ended_at + post_window_min)

- **WHEN** a meal is logged 30 minutes after `ended_at`
- **AND** `post_window_min >= 30`
- **THEN** the meal contributes to `post_window.nutrition.totals`

#### Scenario: Boundary at started_at lands in intra_window

- **WHEN** a workout-fuel entry is logged at exactly `workout.started_at`
- **THEN** the entry contributes to `intra_window.workout_fuel` (not `pre_window`)
- **AND** the response documents the half-open convention

#### Scenario: Boundary at ended_at lands in post_window

- **WHEN** a workout-fuel entry is logged at exactly `workout.ended_at`
- **THEN** the entry contributes to `post_window.workout_fuel` (intra window is `[started_at, ended_at)`)

#### Scenario: Entries with workout_id but outside the time window are excluded

- **WHEN** any intake row (meal, hydration, or workout-fuel) has `workout_id = X` but is logged 8 hours before workout X's `started_at`
- **AND** `pre_window_min = 240` (4h, default)
- **THEN** the row does NOT appear in the fueling totals for any window
- **AND** the response shape is unchanged (no "tagged-but-outside" bucket)

#### Scenario: Entries without workout_id but inside the time window are included

- **WHEN** any intake row has `workout_id = NULL` but `logged_at` falls inside the pre-window
- **THEN** the row contributes to `pre_window.<sub-object>.totals` (time-window matching, not tag matching)

#### Scenario: Empty windows return zero totals and entry_count

- **WHEN** a workout has no meals, hydration, or workout-fuel in any window
- **THEN** every window returns `entry_count: 0` and zero totals across all three sub-objects
- **AND** the response status is `200 OK`

#### Scenario: Workout not found returns 404

- **WHEN** the client calls `GET /workouts/<unknown-uuid>/fueling`
- **THEN** the system returns `404 Not Found` with `{"error":"workout_not_found"}`

#### Scenario: pre_window_min and post_window_min are bounded [0, 720]

- **WHEN** `pre_window_min` or `post_window_min` is outside `[0, 720]`
- **THEN** the system returns `400 Bad Request` with `{"error":"window_invalid","range":{"min":0,"max":720}}`

#### Scenario: pre_window_min = 0 returns an empty pre-window

- **WHEN** the client passes `pre_window_min=0`
- **THEN** `pre_window.minutes` is `0`
- **AND** every sub-object's `entry_count` is `0`
- **AND** the same applies symmetrically for `post_window_min=0`

#### Scenario: Numeric fields are rounded at the response boundary

- **WHEN** any aggregated total resolves to `419.7666…`
- **THEN** the response shows `419.8` (matching the existing nutrient-rounding rule)
- **AND** hydration `total_ml` and workout_fuel `quantity_ml` are rounded to 1 decimal place
