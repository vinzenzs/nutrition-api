## ADDED Requirements

### Requirement: Recovery-metrics tools mirror the recovery-metrics REST endpoints

The MCP server SHALL expose four tools wrapping the recovery-metrics REST surface: `log_recovery_metrics`, `list_recovery_metrics`, `get_recovery_metrics`, and `delete_recovery_metrics`. Each invokes its REST endpoint with `Authorization: Bearer <AGENT_API_TOKEN>` and forwards the response body via `toToolResult`. Write tools auto-derive an idempotency key when none is supplied; read tools never send `Idempotency-Key`. The tools' integration-test expected-tools list is updated to include the four new names.

#### Scenario: log_recovery_metrics upserts by date

- **WHEN** the agent calls `log_recovery_metrics` with `{"date":"2026-06-09","sleep_seconds":27000,"resting_hr":48,"hrv_ms":61}`
- **THEN** the wrapper issues `POST /recovery-metrics` with those fields in the body
- **AND** the REST response body is forwarded verbatim

#### Scenario: Optional metrics omitted from the body when nil

- **WHEN** the agent calls `log_recovery_metrics` with only `date` and `sleep_seconds`
- **THEN** the POST body contains only those keys (the other metrics are absent, not null)

#### Scenario: list_recovery_metrics passes the date window

- **WHEN** the agent calls `list_recovery_metrics` with `{"from":"2026-06-01","to":"2026-06-30"}`
- **THEN** the wrapper issues `GET /recovery-metrics?from=2026-06-01&to=2026-06-30` with no idempotency key

#### Scenario: get_recovery_metrics and delete_recovery_metrics address by date

- **WHEN** the agent calls `get_recovery_metrics` with `{"date":"2026-06-09"}`
- **THEN** the wrapper issues `GET /recovery-metrics/2026-06-09`

- **WHEN** the agent calls `delete_recovery_metrics` with `{"date":"2026-06-09"}`
- **THEN** the wrapper issues `DELETE /recovery-metrics/2026-06-09` and returns an empty result on `204`

### Requirement: Fitness-metrics tools mirror the fitness-metrics REST endpoints

The MCP server SHALL expose four tools wrapping the fitness-metrics REST surface: `log_fitness_metrics`, `list_fitness_metrics`, `get_fitness_metrics`, and `delete_fitness_metrics`, with the same auth/idempotency/forwarding behavior as the recovery-metrics tools. The expected-tools list is updated to include the four new names.

#### Scenario: log_fitness_metrics upserts by date

- **WHEN** the agent calls `log_fitness_metrics` with `{"date":"2026-06-09","vo2max_running":54,"race_predictor_5k_seconds":1230}`
- **THEN** the wrapper issues `POST /fitness-metrics` with those fields in the body
- **AND** the REST response body is forwarded verbatim

#### Scenario: list_fitness_metrics passes the date window

- **WHEN** the agent calls `list_fitness_metrics` with `{"from":"2026-06-01","to":"2026-06-30"}`
- **THEN** the wrapper issues `GET /fitness-metrics?from=2026-06-01&to=2026-06-30`

#### Scenario: get and delete address by date

- **WHEN** the agent calls `get_fitness_metrics` / `delete_fitness_metrics` with `{"date":"2026-06-09"}`
- **THEN** the wrapper issues `GET` / `DELETE` on `/fitness-metrics/2026-06-09`

### Requirement: Weight tools accept the new smart-scale biometrics

The `log_weight` and `patch_weight` MCP tools SHALL accept optional `muscle_mass_kg`, `body_water_pct`, `bone_mass_kg`, and `bmi` arguments. When set they are forwarded to the REST endpoint verbatim; when omitted they are absent from the request body (matching the existing optional-field pattern for `body_fat_pct`).

#### Scenario: log_weight forwards the biometrics when supplied

- **WHEN** the agent calls `log_weight` with `{"weight_kg":72.5,"logged_at":"…","muscle_mass_kg":58.4,"bmi":22.4}`
- **THEN** the POST body includes `muscle_mass_kg` and `bmi`

#### Scenario: log_weight omits the biometrics when not supplied

- **WHEN** the agent calls `log_weight` without any of the four biometric args
- **THEN** the POST body contains none of those keys

### Requirement: Workout tools support the planned/completed status

The `log_workout` and `patch_workout` MCP tools SHALL accept an optional `status` argument (`planned`|`completed`); `list_workouts` SHALL accept an optional `status` filter forwarded as the `status` query parameter. Tool descriptions explain that planned workouts may be future-dated and that the default is `completed`.

#### Scenario: log_workout forwards status when supplied

- **WHEN** the agent calls `log_workout` with `{"source":"garmin","sport":"bike","status":"planned","started_at":"<future>","ended_at":"<future>"}`
- **THEN** the POST body includes `status: "planned"`

#### Scenario: list_workouts forwards the status filter

- **WHEN** the agent calls `list_workouts` with `{"from":"…","to":"…","status":"planned"}`
- **THEN** the wrapper issues `GET /workouts?from=…&to=…&status=planned`

#### Scenario: patch_workout can change status

- **WHEN** the agent calls `patch_workout` with `{"id":"…","status":"completed"}`
- **THEN** the PATCH body includes `status: "completed"`
