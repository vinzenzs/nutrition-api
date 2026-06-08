## ADDED Requirements

### Requirement: GET /workouts/{id}/fueling returns pre/intra/post intake windows

The system SHALL expose `GET /workouts/{id}/fueling?pre_window_min=<int>&post_window_min=<int>` returning three time-anchored buckets — pre, intra, post — each carrying separate nutrition and hydration aggregations for entries whose `logged_at` falls within the corresponding window. The windows are derived from the workout's own `started_at` and `ended_at` plus the supplied (or defaulted) pre/post minutes. Aggregation is time-window-based: any meal or hydration entry whose `logged_at` falls in a window is included regardless of its `workout_id` value.

#### Scenario: Default windows are 240 min pre / 60 min post

- **WHEN** the client calls `GET /workouts/{id}/fueling` without `pre_window_min` or `post_window_min`
- **THEN** `pre_window.minutes` is `240`
- **AND** `post_window.minutes` is `60`

#### Scenario: Response shape separates nutrition and hydration

- **WHEN** the response is well-formed
- **THEN** each window object has the shape `{start, end, minutes, nutrition: {totals, entry_count}, hydration: {total_ml, entry_count}}`
- **AND** the nutrition `totals` matches the shape used by `/summary/daily.totals` (macros + nullable micros)
- **AND** units never mix (no ml inside `nutrition.totals`; no kcal inside `hydration`)

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

- **WHEN** a meal is logged at exactly `workout.started_at`
- **THEN** the meal contributes to `intra_window` (not `pre_window`)
- **AND** the response documents the half-open convention

#### Scenario: Boundary at ended_at lands in post_window

- **WHEN** a meal is logged at exactly `workout.ended_at`
- **THEN** the meal contributes to `post_window` (intra window is `[started_at, ended_at)`)

#### Scenario: Entries with workout_id but outside the time window are excluded

- **WHEN** a meal has `workout_id = X` but is logged 8 hours before workout X's `started_at`
- **AND** `pre_window_min = 240` (4h, default)
- **THEN** the meal does NOT appear in the fueling totals for any window
- **AND** the response shape is unchanged (no "tagged-but-outside" bucket in v1)

#### Scenario: Entries without workout_id but inside the time window are included

- **WHEN** a meal has `workout_id = NULL` but `logged_at` falls inside the pre-window
- **THEN** the meal contributes to `pre_window.nutrition.totals` (time-window matching, not tag matching)

#### Scenario: Empty windows return zero totals and entry_count

- **WHEN** a workout has no meals or hydration in any window
- **THEN** every window returns `entry_count: 0` and zero totals
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
- **AND** `pre_window.nutrition.entry_count` is `0`
- **AND** the same applies symmetrically for `post_window_min=0`

#### Scenario: Numeric fields are rounded at the response boundary

- **WHEN** the aggregated kcal is `419.7666…`
- **THEN** the response shows `419.8` (matching the existing nutrient-rounding rule)
- **AND** hydration `total_ml` is rounded to 1 decimal place (matching `/summary/hydration/daily`)
