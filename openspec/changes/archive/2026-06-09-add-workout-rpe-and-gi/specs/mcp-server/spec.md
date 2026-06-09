## ADDED Requirements

### Requirement: Workouts tools expose rehearsal signals (rpe + gi_distress_score)

The `log_workout` and `patch_workout` MCP tools SHALL accept optional `rpe` (integer 1–10) and `gi_distress_score` (integer 1–5) arguments. When set, the wrapper forwards both to the underlying REST endpoint verbatim; when omitted, the wrapper omits them from the request body (matching the existing pattern for nullable workout fields like `kcal_burned`, `avg_hr`, `tss`, `notes`). The `get_workout` and `list_workouts` tools SHALL surface the two fields on response bodies whenever the underlying rows have them set. No new tools are introduced.

#### Scenario: log_workout forwards rpe and gi_distress_score when supplied

- **WHEN** the agent calls `log_workout` with `{"source":"manual","sport":"bike","started_at":"…","ended_at":"…","rpe":7,"gi_distress_score":2}`
- **THEN** the wrapper issues `POST /workouts` with both fields included in the JSON body
- **AND** the REST response body — including `rpe` and `gi_distress_score` — is forwarded verbatim to the tool result

#### Scenario: log_workout omits the fields when not supplied

- **WHEN** the agent calls `log_workout` without `rpe` and without `gi_distress_score`
- **THEN** the wrapper issues `POST /workouts` with a body that does NOT contain those keys (matching the existing optional-field pattern)

#### Scenario: patch_workout supports setting, clearing, and leaving unchanged

- **WHEN** the agent calls `patch_workout` with `{"workout_id":"…","rpe":7,"gi_distress_score":2}`
- **THEN** the PATCH body sets both fields on the row

- **WHEN** the agent calls `patch_workout` with `{"workout_id":"…","rpe":null}`
- **THEN** the PATCH body carries explicit JSON `null` for `rpe` (clearing the field on the backend)
- **AND** `gi_distress_score` is unchanged (absent from body)

- **WHEN** the agent calls `patch_workout` with `{"workout_id":"…","notes":"felt strong"}` (no rpe / no gi_distress_score)
- **THEN** the PATCH body omits both fields and the backend leaves them unchanged

#### Scenario: log_workout and patch_workout tool descriptions name the scales

- **WHEN** the agent reads the `log_workout` tool description
- **THEN** the description explains `rpe` as "Borg CR-10 perceived effort, 1–10 integer, per session (logged after the workout)"
- **AND** explains `gi_distress_score` as "1 = no GI distress, 5 = severe (couldn't continue); per session"
- **AND** notes that both are nullable — workouts that aren't fueling rehearsals (Z1 spins, etc.) don't need either field

- **WHEN** the agent reads the `patch_workout` tool description
- **THEN** the description explains the tri-state on both fields: absent = leave unchanged, integer = set, JSON null = clear

#### Scenario: Validation errors from the endpoint are forwarded verbatim

- **WHEN** the REST endpoint returns `400 {"error":"rpe_invalid","range":{"min":1,"max":10}}`
- **THEN** the wrapper forwards the response body verbatim
- **AND** the tool result has `isError = true`

- **WHEN** the REST endpoint returns `400 {"error":"gi_distress_score_invalid","range":{"min":1,"max":5}}`
- **THEN** the wrapper forwards verbatim with `isError = true`

#### Scenario: get_workout / list_workouts response shapes include the new fields when set

- **WHEN** the agent calls `get_workout` on a row that has `rpe = 7` and `gi_distress_score = 2`
- **THEN** the response body — forwarded verbatim from the REST endpoint — includes both fields

- **WHEN** the agent calls `list_workouts` on a window that contains rows with mixed presence of the two fields
- **THEN** the response items follow the omitempty pattern: present where set, absent where NULL

#### Scenario: workout_fueling_summary tool surfaces rpe + gi_distress_score when set

- **WHEN** the agent calls `workout_fueling_summary` on a workout that has `rpe = 7` and `gi_distress_score = 2`
- **THEN** the response body — forwarded verbatim from `GET /workouts/{id}/fueling` — includes `rpe` and `gi_distress_score` at the top level alongside the pre/intra/post window breakdowns
- **AND** the agent reads "perceived effort + GI + carbs/sodium/caffeine totals" from a single tool call (the natural rehearsal-evaluation shape)
