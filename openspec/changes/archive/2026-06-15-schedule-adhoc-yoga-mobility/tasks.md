## 1. Widen the sport vocabulary (yoga + mobility)

- [x] 1.1 Add migration `044_add_yoga_mobility_sports.up.sql` / `.down.sql` (verify 044 is still the next free slot first). Up: drop and re-add the `sport` CHECK on `workouts` and on `workout_templates` to `('run','bike','swim','strength','yoga','mobility','other')`. Down: re-add the narrow set (`('run','bike','swim','strength','other')`).
- [x] 1.2 `internal/workouts/types.go`: add `SportYoga` and `SportMobility` constants and extend `ValidSport`.
- [x] 1.3 `internal/workouttemplates/types.go`: add the matching constants and extend `validSport`.
- [x] 1.4 Update swag doc strings that enumerate allowed sports (workouts handlers, workout-templates handlers) to include `yoga | mobility`.
- [x] 1.5 Fix the existing `internal/workouts/handlers_test.go` `TestPost_InvalidSport` (and any bulk test) that uses `"yoga"` as the invalid example — switch to a still-invalid value (e.g. `"pilates"`), and add a test asserting `yoga` and `mobility` are accepted on POST.
- [x] 1.6 Add a `workouttemplates` test creating a template with `sport: "yoga"` and `sport: "mobility"`.

## 2. Ad-hoc planned workout from a template (repo)

- [x] 2.1 Add `CreateAdhocPlannedFromTemplate` to `internal/workouts/repo.go`: plain INSERT of a planned row (`source=manual`, `status=planned`, `template_id` set, `plan_slot_id=NULL`, `started_at`, `ended_at`, `sport`, `name`), returning the created `*Workout`.
- [x] 2.2 Add a helper to compute a template's summed timed-step duration (recursing repeat groups), with a 60-minute fallback when the sum is zero — reuse any existing template-duration helper if one exists, otherwise add a small walker near the template types.
- [x] 2.3 Repo test: creating an ad-hoc planned workout sets the expected fields and `ended_at = started_at + duration` (and the +60min fallback for an untimed/rep-based template).

## 3. One-shot schedule endpoint (garmincontrol)

- [x] 3.1 Add `scheduleTemplateRequest { template_id, date }` and a `scheduleTemplate` handler in `internal/garmincontrol`: guard `enabled()` → parse/validate `template_id` + `date` → load template → create the ad-hoc planned workout (Section 2) → call the existing `pushOne` → return the workout. Map errors to the documented codes (`404` unknown template, `400` invalid date, `502` bridge, `503` disabled).
- [x] 3.2 Wire `POST /garmin/schedule/template` in `internal/httpserver/server.go` and register it under the garmin route group (swag annotations on the handler).
- [x] 3.3 Confirm `pushOne` works unchanged for the slot-less row (rely on `EffectiveProgram` falling back to raw template steps when `PlanSlotID == nil`); add a regression assertion if not already covered.
- [x] 3.4 Handler/integration tests in `internal/garmincontrol`: happy path (creates row, schedules, stores ids, returns workout), unknown template → 404, invalid date → 400, disabled bridge → 503, and that a `yoga` template's sport is forwarded to the bridge compile.

## 4. MCP tool

- [x] 4.1 Register `garmin_schedule_template` in `internal/agenttools/registry_garmin.go` as `TierWriteAuto` with idempotency-key support, mapping `template_id` + `date` to the new endpoint.
- [x] 4.2 Bump the expected-tools list/count in `internal/mcpserver/mcp_integration_test.go` and regenerate `internal/mcpserver/testdata/announced_schemas.json` for the new tool.
- [x] 4.3 Extend the MCP e2e/integration coverage to schedule a yoga template end-to-end (create template → `garmin_schedule_template` → assert the planned workout + sport).

## 5. Docs and verification

- [x] 5.1 Run `task swag` to regenerate `docs/` for the new endpoint and the widened sport enums.
- [x] 5.2 Run `task test` (or the affected packages: workouts, workouttemplates, garmincontrol, mcpserver) and `task vet`; fix fallout.
- [x] 5.3 Note the bridge dependency: `garmin.py` must accept `yoga`/`mobility` and `/yoga schedule` should switch to `garmin_schedule_template` (vault follow-up, out of scope here).
