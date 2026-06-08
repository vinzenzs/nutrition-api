## ADDED Requirements

### Requirement: Workouts tools mirror the workouts REST endpoints

The MCP server SHALL expose five tools wrapping the new workouts REST surface: `log_workout`, `list_workouts`, `get_workout`, `patch_workout`, and `delete_workout`. Each tool invokes its REST endpoint with `Authorization: Bearer <AGENT_API_TOKEN>` and forwards the REST response body as the tool's content via the existing `toToolResult` mapping. Write tools auto-derive an idempotency key when none is supplied (per the existing POST-style write rule); read tools never send `Idempotency-Key`.

#### Scenario: log_workout calls POST /workouts

- **WHEN** the agent calls `log_workout` with `{"source":"manual","sport":"strength","started_at":"2026-06-07T18:00:00Z","ended_at":"2026-06-07T19:00:00Z","name":"Gym — push day"}`
- **THEN** the wrapper issues `POST /workouts` with that JSON body
- **AND** sets `Idempotency-Key` to the agent's explicit key (if any) or to the derived stable key
- **AND** returns the REST `201`/`200` response body as the tool result content

#### Scenario: log_workout description explains the external_id dedup mechanism

- **WHEN** the agent reads the `log_workout` tool description
- **THEN** the description explains that most workouts come from the Garmin importer with `source: garmin` and an `external_id`
- **AND** clarifies that the agent should use this tool for manual entries (gym sessions, sweat-rate windows, untracked workouts)
- **AND** notes that `external_id` is the dedup mechanism — agents typically do NOT set it on manual writes

#### Scenario: list_workouts calls GET /workouts with the window

- **WHEN** the agent calls `list_workouts` with `{"from":"2026-06-01T00:00:00Z","to":"2026-06-08T00:00:00Z"}`
- **THEN** the wrapper issues `GET /workouts?from=2026-06-01T00:00:00Z&to=2026-06-08T00:00:00Z`
- **AND** does NOT include an `Idempotency-Key` header

#### Scenario: get_workout calls GET /workouts/{id}

- **WHEN** the agent calls `get_workout` with `{"id":"<uuid>"}`
- **THEN** the wrapper issues `GET /workouts/<uuid>`
- **AND** forwards a 404 with `{"error":"workout_not_found"}` verbatim when no workout exists

#### Scenario: patch_workout calls PATCH /workouts/{id}

- **WHEN** the agent calls `patch_workout` with `{"id":"<uuid>","tss":85,"notes":"FTP updated"}`
- **THEN** the wrapper issues `PATCH /workouts/<uuid>` with body `{"tss":85,"notes":"FTP updated"}`

#### Scenario: patch_workout description distinguishes mutable from immutable fields

- **WHEN** the agent reads the `patch_workout` tool description
- **THEN** the description lists the PATCH-able fields (`name`, `notes`, `kcal_burned`, `avg_hr`, `tss`)
- **AND** states that `sport`, `started_at`, `ended_at`, `source`, and `external_id` are immutable — delete and re-create if those are wrong

#### Scenario: delete_workout calls DELETE /workouts/{id}

- **WHEN** the agent calls `delete_workout` with `{"id":"<uuid>"}`
- **THEN** the wrapper issues `DELETE /workouts/<uuid>`
- **AND** on a 204 response, the tool result content is empty and `isError = false`
