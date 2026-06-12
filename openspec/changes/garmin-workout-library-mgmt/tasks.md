## 1. No migration

- [ ] 1.1 Confirm NO migration is needed — every operation acts on Garmin via ids already stored by migration 032 (`workouts.garmin_workout_id`, `workouts.garmin_schedule_id`). Do not scaffold one.

## 2. garmin-bridge (apps/garmin-bridge)

- [ ] 2.1 `garmin_client.py`: add `delete_workout(api, garmin_workout_id)` calling the workout-service DELETE; treat a 404 / "not found" as a no-op success (mirror `unschedule_workout`)
- [ ] 2.2 `garmin_client.py`: add `get_workouts(api, start, limit)` and `get_workout_by_id(api, garmin_workout_id)` library reads
- [ ] 2.3 `garmin_client.py`: add `add_hydration_data(api, value_ml, date)` wrapping garminconnect's hydration write
- [ ] 2.4 `garmin_client.py`: add `download_activity(api, activity_id, fmt)` returning the raw bytes
- [ ] 2.5 `app.py`: `DELETE /workouts/{garmin_workout_id}` → `gc.delete_workout`, returning `{deleted}` / `{deleted:false, already_absent:true}`; `409 login_required` when token-less (via `_with_api`)
- [ ] 2.6 `app.py`: `GET /workouts` (optional `start`/`limit`) and `GET /workouts/{garmin_workout_id}` → library reads, verbatim
- [ ] 2.7 `app.py`: `POST /hydration` accepting `{value_ml, date}` → `gc.add_hydration_data`
- [ ] 2.8 `app.py`: `GET /activity/{activity_id}/export` (optional `format`, default `fit`) → base64-wrap the bytes into `{activity_id, format, filename, content_base64}`
- [ ] 2.9 Bridge unit tests: 404→no-op delete, library list/get shape, hydration write call args, export envelope base64 round-trip, token-less → `409 login_required`

## 3. garmin-control (internal/garmincontrol)

- [ ] 3.1 `scheduling.go`: add `bridgeDeleteWorkout(ctx, garminWorkoutID)` client method (idempotent — bridge 2xx incl. already-absent is success)
- [ ] 3.2 `scheduling.go` `pushOne`: when `w.GarminWorkoutID != nil`, delete the prior object before creating the new one (order: unschedule old entry → delete old object → create → schedule → store ids)
- [ ] 3.3 `scheduling.go` `unscheduleWorkout`: after removing the calendar entry, when `w.GarminWorkoutID != nil` delete the object too, then clear BOTH ids
- [ ] 3.4 New handler `DELETE /garmin/workout/{workout_id}`: load row, delete stored object, clear `garmin_workout_id` (leave `garmin_schedule_id`); no-op success when id absent; `503 garmin_disabled` when off
- [ ] 3.5 New handlers `GET /garmin/workouts` (+ `start`/`limit` passthrough) and `GET /garmin/workout/{garmin_workout_id}` forwarding verbatim (model on `calendar`)
- [ ] 3.6 New handler `POST /garmin/hydration` forwarding `{value_ml, date}` verbatim
- [ ] 3.7 New handler `GET /garmin/activity/{activity_id}/export` (+ `format`) forwarding the envelope verbatim; RAISE the export-path response cap above the 16 KB `maxBodyBytes` (blob-sized, e.g. 8 MB) — open question in design
- [ ] 3.8 Register all new routes in `Handlers.Register`
- [ ] 3.9 Add swag annotations to every new handler; `task swag`
- [ ] 3.10 Handler tests: unschedule reaps object + clears ids; re-push deletes prior object; standalone delete no-op when id absent; already-absent (bridge 404) → success; `503 garmin_disabled` on each new endpoint when bridge URL unset

## 4. MCP (internal/mcpserver)

- [ ] 4.1 `tools_garmin.go`: `garmin_delete_workout` → `DELETE /garmin/workout/{workout_id}`, `effectiveIdempotencyKey`
- [ ] 4.2 `tools_garmin.go`: `garmin_list_workouts` → `GET /garmin/workouts` (optional `start`/`limit`), read-only
- [ ] 4.3 `tools_garmin.go`: `garmin_get_workout` → `GET /garmin/workout/{garmin_workout_id}`, read-only
- [ ] 4.4 `tools_garmin.go`: `garmin_push_hydration` → `POST /garmin/hydration`, `effectiveIdempotencyKey`; description names the write direction + opt-in + overwrite semantics
- [ ] 4.5 `tools_garmin.go`: `garmin_export_activity` → `GET /garmin/activity/{activity_id}/export` (optional `format`), read-only; returns the base64 envelope verbatim
- [ ] 4.6 Register all five in `registerGarminTools`; update `garmin_unschedule_workout` description to name the dual teardown and point at `garmin_delete_workout`
- [ ] 4.7 `mcp_integration_test.go`: add the five new tool names to the expected-tools list (was ending at `garmin_list_scheduled`; +5)

## 5. Tests & docs

- [ ] 5.1 `task swag`, `task vet` green
- [ ] 5.2 `go test -count=1 ./internal/garmincontrol/...` and `./internal/mcpserver/...` (incl. the `integration` tag for the expected-tools assertion)
- [ ] 5.3 Bridge `pytest` green
- [ ] 5.4 Verify no migration was added (head stays `035`)
