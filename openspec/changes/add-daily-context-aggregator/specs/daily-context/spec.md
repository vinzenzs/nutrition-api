## ADDED Requirements

### Requirement: Daily context aggregator endpoint

The system SHALL expose `GET /context/daily?date=YYYY-MM-DD&tz=…` returning a single JSON bundle that composes the day's adherence, nutrition totals, hydration ml, workouts, workout-fuel entries, body-weight state, training-phase context, and goal-override presence into one response. The endpoint SHALL perform NO writes and SHALL define NO new tables; every datum SHALL come from an existing repo's read method. `date` is required; `tz` defaults to the configured `DEFAULT_USER_TZ` when omitted.

#### Scenario: Happy path returns the full bundle

- **WHEN** the client calls `GET /context/daily?date=2026-07-15` with the day having logged meals, hydration, one workout, one workout-fuel entry, and a body-weight entry, AND a training phase covering the date with a template AND a goal override for the date
- **THEN** the response is `200 OK` with body shape `{date, tz, adherence, nutrition, hydration, workouts, workout_fuel, weight, phase, goal_override}`
- **AND** `adherence` carries `goal_source: "override"`, `phase_name: null` (omitted because goal_source is override, not phase_template), and the standard `adherence` map covering every configured goal
- **AND** `nutrition` carries the day's `totals` block (rounded per the existing rounding requirement) plus an `entries_count` integer
- **AND** `hydration` carries `total_ml` and `entries_count`
- **AND** `workouts` is a non-empty array whose entries include `id`, `sport`, `started_at`, `ended_at`, `duration_min`, `kcal_burned` (nullable), `notes` (nullable)
- **AND** `workout_fuel` is a non-empty array of fuel entries with `id`, `logged_at`, `quantity_ml?`, `carbs_g?`, `sodium_mg?`, `potassium_mg?`, `caffeine_mg?`, `workout_id?` (nullable)
- **AND** `weight` is `{logged_at, weight_kg, body_fat_pct?, is_carryover: false}` (fresh entry on the day)
- **AND** `phase` is `{id, name, type, start_date, end_date, default_template_id?, default_template_name?, notes?}`
- **AND** `goal_override` is `{present: true, goals: <the override's Goals object>}`

#### Scenario: Empty day returns the bundle with empty arrays and nulls

- **WHEN** the client calls `GET /context/daily?date=2026-07-15` on a date with NO logged data anywhere and no phase covering it and no override
- **THEN** the response is `200 OK` with the same top-level shape
- **AND** `adherence.goal_source` is `"none"` (or `"default"` if the singleton default exists) and `adherence.adherence` is absent
- **AND** `nutrition.totals` carries zero macros and absent micros; `nutrition.entries_count = 0`
- **AND** `hydration` is `{total_ml: 0, entries_count: 0}`
- **AND** `workouts` is `[]` (empty array, not null — JSON consumers branch on length)
- **AND** `workout_fuel` is `[]`
- **AND** `weight` is `null` (no entry ever logged)
- **AND** `phase` is `null`
- **AND** `goal_override` is `{present: false, goals: null}`

#### Scenario: Phase-driven adherence surfaces goal_source AND phase_name AND a phase block

- **WHEN** the client calls the endpoint on a date covered by a phase pointing at a template, AND no override exists on the date
- **THEN** `adherence.goal_source = "phase_template"` AND `adherence.phase_name` is the phase's name
- **AND** `phase` is the full phase row covering the date (same row as `goal_source` resolved against — most-recently-updated if multiple phases overlap)

#### Scenario: Phase without template still appears in the phase block

- **WHEN** a phase covers the date with `default_template_id = NULL`
- **THEN** `phase` is the phase's full row (so the agent can describe the period to the user)
- **AND** `adherence.goal_source` falls through to `"default"` or `"none"` (phase did not drive adherence)
- **AND** `adherence.phase_name` is absent

#### Scenario: Weight carryover behaviour

- **WHEN** no body-weight entry exists with `logged_at` on the requested day, but a previous entry exists
- **THEN** `weight` is the most-recent prior entry with `is_carryover: true`
- **AND** subsequent re-runs after a fresh same-day entry is logged return that entry with `is_carryover: false`

- **WHEN** no body-weight entry has ever been logged
- **THEN** `weight` is `null`

#### Scenario: Goal-override presence flag

- **WHEN** an override exists for the date
- **THEN** `goal_override.present = true` and `goal_override.goals` is the stored Goals object (rounded per the existing rounding requirement)

- **WHEN** no override exists
- **THEN** `goal_override` is `{present: false, goals: null}` (the two-field shape is stable; the agent's branch is `if context.goal_override.present`)

#### Scenario: Missing date param is rejected

- **WHEN** the client omits `date`
- **THEN** the system returns `400 Bad Request` with `{"error":"date_invalid"}`

#### Scenario: Malformed date is rejected

- **WHEN** `date` is not `YYYY-MM-DD`
- **THEN** the system returns `400 Bad Request` with `{"error":"date_invalid"}`

#### Scenario: Invalid tz is rejected

- **WHEN** `tz` is not a valid IANA timezone (e.g. `tz=NowhereLand`)
- **THEN** the system returns `400 Bad Request` with `{"error":"tz_invalid"}`

#### Scenario: Endpoint requires authentication

- **WHEN** the request omits the `Authorization: Bearer <token>` header
- **THEN** the system returns `401 Unauthorized` (same auth posture as every other API endpoint)

#### Scenario: Endpoint is read-only

- **WHEN** the client makes two identical requests
- **THEN** both responses are deeply equal (modulo timestamps on stored rows)
- **AND** no row is inserted, updated, or deleted in any table
- **AND** the endpoint does NOT require an `Idempotency-Key` header

#### Scenario: All slices fetched in parallel; a single slice failure surfaces as 500

- **WHEN** the client calls the endpoint AND any one underlying repo read fails (e.g. a transient DB error)
- **THEN** the system returns `500 Internal Server Error` (no partial bundle is returned)
- **AND** the other reads' results are not surfaced — the bundle is all-or-nothing

#### Scenario: Bundle echoes the requested date and resolved tz

- **WHEN** the client calls `GET /context/daily?date=2026-07-15&tz=Europe/Berlin`
- **THEN** the response's top-level `date` is `"2026-07-15"` and `tz` is `"Europe/Berlin"`

#### Scenario: Default tz is applied when tz is omitted

- **WHEN** the client omits `tz` and the server's `DEFAULT_USER_TZ` is `Europe/Berlin`
- **THEN** the response's top-level `tz` is `"Europe/Berlin"`
- **AND** the date-window computation uses `Europe/Berlin` for the day boundaries
