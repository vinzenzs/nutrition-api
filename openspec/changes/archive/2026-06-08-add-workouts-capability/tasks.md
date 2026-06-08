## 1. Migration

- [x] 1.1 Add `internal/store/migrations/012_add_workouts.up.sql`:
  - `CREATE TABLE workouts (...)` with the column set, CHECK constraints, defaults documented in the design.
  - `CREATE INDEX workouts_started_at_idx ON workouts (started_at);`
  - `CREATE UNIQUE INDEX workouts_external_id_uidx ON workouts (external_id) WHERE external_id IS NOT NULL;` (partial unique index so multiple manual rows with NULL `external_id` are allowed).
- [x] 1.2 Add `.down.sql`: `DROP TABLE workouts;`.
- [x] 1.3 Verify the migration applies cleanly against a fresh `task dev` Postgres and the schema matches the spec.

## 2. Backend: package skeleton

- [x] 2.1 Create `internal/workouts/` directory.
- [x] 2.2 `internal/workouts/types.go`:
  - `Workout` struct mirroring the table columns (UUIDs as `uuid.UUID`, timestamps as `time.Time`, all nullable fields as pointer types with `omitempty` JSON tags).
  - `Source` and `Sport` string-typed enums with `ParseSource` / `ValidSource` etc. helpers (mirroring `meals.MealType`). No `Intensity` enum — TSS replaces it.
- [x] 2.3 `internal/workouts/repo.go`: `Upsert(ctx, *Workout) (created bool, err error)` (returns whether INSERT or UPDATE for the 201/200 distinction), `GetByID(ctx, uuid) (*Workout, error)`, `Patch(ctx, uuid, PatchParams) error`, `Delete(ctx, uuid) error`, `List(ctx, from, to time.Time) ([]*Workout, error)`. `ErrNotFound` sentinel.
- [x] 2.4 `Upsert` implementation: `INSERT ... ON CONFLICT (external_id) WHERE external_id IS NOT NULL DO UPDATE SET ...` — the `WHERE external_id IS NOT NULL` clause matches the partial unique index so NULL `external_id`s always INSERT.
- [x] 2.5 `internal/workouts/service.go`: validation (sport/source enums, started_at < ended_at, started_at not >24h future, kcal_burned > 0, avg_hr > 0, tss >= 0), thin orchestration over the repo. Validation errors mapped to sentinel errors per the spec error codes. Add a `BulkUpsert(ctx, []*Workout) []BulkItemResult` helper that validates + upserts each item independently and returns per-item results (`{Index, ID?, Created, Err?}`).

## 3. Backend: HTTP handlers

- [x] 3.1 `internal/workouts/handlers.go`: `Register(rg *gin.RouterGroup)` mounting POST/GET on `/workouts`, POST on `/workouts/bulk`, and PATCH/DELETE/GET on `/workouts/:id`.
- [x] 3.2 POST handler: decode body with `c.ShouldBindJSON`; validate via the service; map errors to the documented `source_invalid`, `sport_invalid`, `window_invalid`, `started_at_too_far_future`, `kcal_burned_invalid`, `avg_hr_invalid`, `tss_invalid` codes. Returns `201` on INSERT, `200` on UPDATE — both with the persisted workout body.
- [x] 3.3 POST /workouts/bulk handler: decode `{"workouts": [...]}`; reject `bulk_invalid` if missing or non-array, `bulk_empty` if empty, `bulk_too_large` if > 100. Iterate items, calling `service.BulkUpsert`; build per-item results array; return `200 {"results": [...]}`. Per-item validation failures use the same single-item error codes (`sport_invalid`, etc.).
- [x] 3.4 GET (list) handler: validate `from` and `to` (RFC 3339, `from <= to`, span <= 92 days); map errors as `window_required` / `window_invalid` / `range_too_large` matching the meals shape. Wrap rows in `{"workouts":[...]}`.
- [x] 3.5 GET (single) handler: 404 `workout_not_found` on unknown id (and on malformed UUID, like meals).
- [x] 3.6 PATCH handler: decode partial body, reject immutable fields with `400 field_immutable, field: <name>` (DisallowUnknownFields decoder with a custom error mapper that recognises the immutable set vs truly unknown). Validate the supplied mutable fields. `404 workout_not_found` on unknown id. Accepted mutable fields: `name`, `notes`, `kcal_burned`, `avg_hr`, `tss`.
- [x] 3.7 DELETE handler: 204 on success, 404 on unknown id.
- [x] 3.8 Swag annotations for every handler (including `/workouts/bulk`), listing the documented error codes.

## 4. Wiring

- [x] 4.1 In `internal/httpserver/server.go`, instantiate `workouts.Repo`, `workouts.Service`, `workouts.Handlers`. Register on the existing API group (so auth + idempotency middleware applies uniformly).
- [x] 4.2 Confirm the standard idempotency middleware behaves correctly: POST /workouts WITHOUT an Idempotency-Key relies on `external_id` UPSERT (verified by the repo test); POST WITH an Idempotency-Key uses the existing replay/conflict logic (which is harmless here because the writer typically doesn't supply one).

## 5. Backend tests

- [x] 5.1 `internal/workouts/repo_test.go`:
  - Upsert with new `external_id` returns `created=true` and inserts.
  - Upsert with existing `external_id` returns `created=false` and updates the existing row (different `kcal_burned`).
  - Upsert without `external_id` always inserts (two such calls produce two rows).
  - GetByID happy path + ErrNotFound on unknown id.
  - Patch mutable fields round-trip.
  - Delete + 404-on-second-delete.
  - List window filtering, ordering by `started_at` ascending, empty-window returns empty slice.
- [x] 5.2 `internal/workouts/handlers_test.go` with the standard `storetest.NewPool` pattern. Cover:
  - POST happy path with all fields (201).
  - POST with no `external_id` returns 201 and inserts a new row each time.
  - POST with existing `external_id` returns 200 and updates (e.g. `kcal_burned` corrected).
  - One test per single-item validation failure (one per error code from the spec).
  - GET list (in-range vs out-of-range, missing/inverted window, range_too_large).
  - GET single (200 / 404).
  - PATCH happy path (e.g. updating `tss`) + immutable-field rejection + invalid-tss + 404.
  - DELETE happy + 404 on unknown.
- [x] 5.3 `internal/workouts/bulk_handlers_test.go` (or a section of the same file). Cover:
  - Bulk happy path: 3 items, 2 inserts + 1 update via existing `external_id`, per-item results match expectations.
  - Bulk with one valid + one invalid item: per-item results show the invalid item's error code; the valid item is still persisted.
  - Bulk empty array → 400 `bulk_empty`, no persistence.
  - Bulk > 100 items → 400 `bulk_too_large` with `max: 100`, no persistence.
  - Bulk with missing `workouts` field → 400 `bulk_invalid`.
  - Bulk where two items share the same `external_id`: last-write-wins; one row persisted.

## 6. MCP wrapper

- [x] 6.1 `internal/mcpserver/tools_workouts.go`. Five input structs (no bulk MCP wrapper):
  - `LogWorkoutArgs{ExternalID, Source, Sport, Name, StartedAt, EndedAt, KcalBurned, AvgHR, TSS, Notes, IdempotencyKey}` — pointer-optional for every nullable field.
  - `ListWorkoutsArgs{From, To}`.
  - `GetWorkoutArgs{ID}`.
  - `PatchWorkoutArgs{ID, Name, Notes, KcalBurned, AvgHR, TSS, IdempotencyKey}` — only the mutable subset.
  - `DeleteWorkoutArgs{ID, IdempotencyKey}`.
- [x] 6.2 Five handlers (`handleLogWorkout`, `handleListWorkouts`, `handleGetWorkout`, `handlePatchWorkout`, `handleDeleteWorkout`) following the `tools_hydration.go` patterns. Use `effectiveIdempotencyKey` for write tools; reads pass no key. `handleDeleteWorkout` uses the standard 204 → empty-content shape on success.
- [x] 6.3 `registerWorkoutsTools(server, c)` registers all five with descriptions per the mcp-server spec — particularly the `log_workout` description that explains `external_id` is the dedup mechanism and most workouts come from Garmin, and the `patch_workout` description that lists mutable vs immutable fields.
- [x] 6.4 Wire `registerWorkoutsTools` in `internal/mcpserver/server.go` (alongside the existing `register*Tools` calls).

## 7. MCP tests

- [x] 7.1 `internal/mcpserver/tools_workouts_test.go`: per-tool tests using the `newRecordingClient` / recorder pattern. Cover endpoint URLs, method, body, idempotency-key forwarding, response passthrough, 404 / 4xx forwarding with `isError=true`. Specifically:
  - log_workout posts the full body and forwards explicit or derived Idempotency-Key.
  - log_workout with same args twice produces the same derived key.
  - list_workouts builds the query string from `from` / `to` and sends no Idempotency-Key.
  - get_workout calls the path-escaped URL.
  - patch_workout omits unset fields from the body.
  - delete_workout 204 → empty content; 404 → isError.
- [x] 7.2 Update `internal/mcpserver/mcp_integration_test.go` expected-tools list to include `log_workout`, `list_workouts`, `get_workout`, `patch_workout`, `delete_workout` (now 25 total — 20 from prior changes + 5 from this).

## 8. Documentation

- [x] 8.1 `task swag` to regenerate OpenAPI for the new routes (including `/workouts/bulk`).
- [x] 8.2 `README.md`: add a "Workouts" subsection under the API examples (after "Hydration"). Show: POST with `external_id` (Garmin-shape), POST without (manual), POST /workouts/bulk (batch import), GET list, PATCH tss/notes. Add the five new MCP tools to the MCP table.
- [x] 8.3 `README.md`: add a short paragraph above the Workouts example explaining the writer-side architecture: backend exposes the endpoint; `garmin.py` (external) translates Garmin → API shape; no Garmin code lives in the backend. Note that `/workouts/bulk` exists for batch import (e.g. first-time Garmin backfill).
- [x] 8.4 `RUN_LOCAL.md`: extend the API walkthrough with a "Log a workout manually" example (a gym session) followed by `GET /workouts?from=&to=` to confirm the round-trip. Frame it as the manual path; mention `garmin.py` as the typical writer for the Garmin-sourced case.
- [x] 8.5 Add `internal/workouts/` to the project-layout section in README.

## 9. Pre-merge checks

- [x] 9.1 `task vet` clean.
- [x] 9.2 `task test` green (use `-p 1` if testcontainers parallel boot flakes surface).
- [x] 9.3 Manual e2e: with `task dev` running, POST a manual workout, POST a Garmin-shaped workout (with `external_id`), POST the same Garmin-shaped workout again with a different `kcal_burned` and confirm the response is `200` (not `201`) and the row was updated. POST `/workouts/bulk` with a mixed batch (one valid, one with `sport: "yoga"`) and confirm the per-item results array shows one success and one `sport_invalid` error. Try one single-item invalid field too and confirm the documented `400` error code.
- [x] 9.4 OpenSpec validation: `openspec status --change "add-workouts-capability"` shows 4/4 artifacts done.
