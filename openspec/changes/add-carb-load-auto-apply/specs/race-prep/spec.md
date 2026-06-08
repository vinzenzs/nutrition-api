## ADDED Requirements

### Requirement: Carb-load apply endpoint persists schedule into goal overrides

The system SHALL expose `POST /race-prep/carb-load/apply` taking the same parameters as `GET /race-prep/carb-load` (`race_date`, `body_weight_kg`, optional `days_before`, `carbs_per_kg_per_day`, `race_day_carbs_per_kg`). The endpoint computes the carb-load schedule via the same primitive as the GET endpoint, then writes the per-day carbohydrate target into the corresponding per-date goal override row. All writes happen inside a single database transaction — if any per-date write fails, the whole apply rolls back and zero overrides are persisted. The endpoint returns the schedule alongside a per-date `applied` array reporting whether each target row was newly created or merged into an existing override.

The apply step writes ONLY the `carbs_g` bound (as `{min: target_carbs_g}`, min-only — matching the existing pattern for `fiber_g` / `iron_mg`). Non-carbohydrate fields on a pre-existing override row (`kcal`, `protein_g`, etc.) are preserved verbatim; the apply step never clears or overwrites them.

#### Scenario: Apply on empty overrides creates one row per schedule day

- **WHEN** the client calls `POST /race-prep/carb-load/apply` with `{"race_date":"2026-07-24","body_weight_kg":70}` and no pre-existing override rows on the target dates
- **THEN** the response is `200 OK` with body shape `{race_date, body_weight_kg, params, schedule, applied}`
- **AND** `schedule` matches what `GET /race-prep/carb-load` would return for the same inputs (4 entries, default protocol parameters)
- **AND** `applied` contains exactly 4 entries — one per schedule day
- **AND** each `applied` entry has `{date, carbs_g_min, created: true}`
- **AND** four new rows exist in `daily_goal_overrides`, each with only the `carbs_g_min` bound populated (every other column null)
- **AND** `applied[0].carbs_g_min == 700.0` for a 70kg athlete on the default 10 g/kg load-day protocol
- **AND** the race-day entry has `carbs_g_min == 140.0` (70 × 2)

#### Scenario: Apply merges into existing override, preserving non-carb fields

- **WHEN** an override already exists on `2026-07-22` with `{kcal: {min: 2090, max: 2310}, protein_g: {min: 150, max: 190}}` and no `carbs_g` bound
- **AND** the client calls `POST /race-prep/carb-load/apply` with `{"race_date":"2026-07-24","body_weight_kg":70}`
- **THEN** the `2026-07-22` override row now has `carbs_g: {min: 700}` AND the existing `kcal` and `protein_g` bounds unchanged
- **AND** the response's `applied` entry for `2026-07-22` has `{date: "2026-07-22", carbs_g_min: 700.0, created: false}`
- **AND** the `applied` entries for dates without prior overrides have `created: true`

#### Scenario: Apply replaces an existing carbs_g bound

- **WHEN** an override on `2026-07-22` already has `{carbs_g: {min: 500, max: 600}, kcal: {min: 2200}}`
- **AND** the client calls `POST /race-prep/carb-load/apply` with `{"race_date":"2026-07-24","body_weight_kg":70}`
- **THEN** the row's `carbs_g` is replaced with `{min: 700}` (the new target; no max — the apply step writes min-only)
- **AND** `kcal` is preserved verbatim
- **AND** the response's `applied` entry has `created: false`

#### Scenario: Apply is atomic — partial failure rolls back

- **WHEN** the schedule has 4 target dates and the third per-date write fails (e.g. a constraint violation forced for tests)
- **THEN** the transaction rolls back
- **AND** zero rows have been written or modified in `daily_goal_overrides`
- **AND** the response is `500 Internal Server Error` (or the propagated error code from the failing write)
- **AND** the response does NOT include an `applied` array (or includes an empty array; either is acceptable as long as no partial state is implied)

#### Scenario: Apply uses POST semantics, accepts Idempotency-Key

- **WHEN** the client calls `POST /race-prep/carb-load/apply` with an `Idempotency-Key` header
- **THEN** the existing idempotency middleware applies (deduping replays of the same key+body, consistent with every other POST write)
- **AND** the endpoint is NOT exempted from idempotency

#### Scenario: Apply rejects same param errors as GET

- **WHEN** the client calls apply with a `race_date` in the past
- **THEN** the system returns `400 Bad Request` with `{"error":"race_date_in_past"}` (same validation as the GET endpoint)
- **AND** no transaction is opened; no overrides are touched

- **WHEN** the client calls apply with `body_weight_kg=25`
- **THEN** the system returns `400 Bad Request` with `{"error":"body_weight_kg_invalid","range":{"min":30,"max":200}}`

- **WHEN** the client calls apply with `days_before=8`
- **THEN** the system returns `400 Bad Request` with `{"error":"days_before_invalid","range":{"min":0,"max":7}}`

#### Scenario: Apply requires authentication

- **WHEN** the request omits the `Authorization: Bearer <token>` header
- **THEN** the system returns `401 Unauthorized` (same auth posture as every other API endpoint)
- **AND** no overrides are touched

#### Scenario: Apply round-trip is visible to /summary/daily

- **WHEN** the client calls `POST /race-prep/carb-load/apply` for `race_date=2026-07-24` (3 load days)
- **AND** the client then calls `GET /summary/daily?date=2026-07-22`
- **THEN** the response includes an `adherence.carbs_g` entry with `target: {min: 700}`
- **AND** `goal_source: "override"` (the apply produced an override on that date)

#### Scenario: Apply response order matches schedule order

- **WHEN** the client calls apply with default params (4 entries)
- **THEN** `applied[i].date == schedule[i].date` for every index
- **AND** both arrays are ordered by date ascending
