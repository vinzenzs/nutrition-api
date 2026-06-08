## ADDED Requirements

### Requirement: Workout-fuel tools mirror the workout-fuel REST endpoints

The MCP server SHALL expose four tools wrapping the new workout-fuel REST surface: `log_workout_fuel`, `list_workout_fuel`, `patch_workout_fuel`, and `delete_workout_fuel`. Each tool invokes its REST endpoint with `Authorization: Bearer <AGENT_API_TOKEN>` and forwards the REST response body via `toToolResult`. Write tools auto-derive an idempotency key when none is supplied; read tools never send `Idempotency-Key`.

#### Scenario: log_workout_fuel calls POST /workout-fuel

- **WHEN** the agent calls `log_workout_fuel` with `{"name":"Maurten Gel 100","logged_at":"2026-06-07T08:45:00Z","carbs_g":25,"caffeine_mg":100}`
- **THEN** the wrapper issues `POST /workout-fuel` with that JSON body
- **AND** sets `Idempotency-Key` to the agent's explicit key (if any) or to the derived stable key
- **AND** returns the REST `201` response body as the tool result content

#### Scenario: log_workout_fuel description explains the hydration vs workout-fuel routing rule

- **WHEN** the agent reads the `log_workout_fuel` tool description
- **THEN** the description explains the simple routing rule: plain water / juice (volume only) → `log_hydration`; anything with electrolytes / carbs / caffeine → `log_workout_fuel`
- **AND** notes that `name` is required (rehearsal data depends on knowing WHAT was taken)
- **AND** notes that at least one of `quantity_ml`/`carbs_g`/`sodium_mg`/`potassium_mg`/`caffeine_mg` must be supplied
- **AND** notes that `caffeine_mg: 0` is meaningful (explicitly "no caffeine") and distinct from omitting (NULL = "not measured")

#### Scenario: log_workout_fuel optional workout_id is forwarded

- **WHEN** the agent calls `log_workout_fuel` with `workout_id` set to an existing workout's UUID
- **THEN** the wrapper forwards the field in the POST body
- **AND** the REST 400 `workout_not_found` is forwarded verbatim on unknown workouts

#### Scenario: list_workout_fuel calls GET /workout-fuel with the window

- **WHEN** the agent calls `list_workout_fuel` with `{"from":"2026-06-01T00:00:00Z","to":"2026-06-08T00:00:00Z"}`
- **THEN** the wrapper issues `GET /workout-fuel?from=2026-06-01T00:00:00Z&to=2026-06-08T00:00:00Z`
- **AND** does NOT include an `Idempotency-Key` header

#### Scenario: patch_workout_fuel calls PATCH /workout-fuel/{id}

- **WHEN** the agent calls `patch_workout_fuel` with `{"id":"<uuid>","sodium_mg":420}`
- **THEN** the wrapper issues `PATCH /workout-fuel/<uuid>` with body `{"sodium_mg":420}`

#### Scenario: patch_workout_fuel forwards the empty-string clear semantic for workout_id

- **WHEN** the agent calls `patch_workout_fuel` with `{"id":"<uuid>","workout_id":""}`
- **THEN** the wrapper forwards the body verbatim
- **AND** the REST backend interprets the empty string as "clear the link"

#### Scenario: delete_workout_fuel calls DELETE /workout-fuel/{id}

- **WHEN** the agent calls `delete_workout_fuel` with `{"id":"<uuid>"}`
- **THEN** the wrapper issues `DELETE /workout-fuel/<uuid>`
- **AND** on a 204 response, the tool result content is empty and `isError = false`

### Requirement: workout_fueling_summary tool description acknowledges the new sub-object

The existing `workout_fueling_summary` tool description (from `add-meal-workout-link`) SHALL note that each window's response now includes a third sub-object `workout_fuel` carrying carbs/sodium/potassium/caffeine/ml from `workout_fuel_entries`, in addition to the existing `nutrition` and `hydration` sub-objects. No contract change to the tool's inputs; the response composition just gets richer.

#### Scenario: Updated description names all three sub-objects

- **WHEN** the agent reads the `workout_fueling_summary` tool description (after this change applies)
- **THEN** the description lists `nutrition` (from meals), `hydration` (from hydration entries), AND `workout_fuel` (from workout-fuel entries) as the three per-window sub-objects
- **AND** continues to note the time-window-vs-tag aggregation rule (no change)
- **AND** continues to note the default windows (240 pre, 60 post) and bounds (no change)
