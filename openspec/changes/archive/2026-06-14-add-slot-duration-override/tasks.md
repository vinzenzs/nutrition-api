# Tasks: add-slot-duration-override

## 1. Migration

- [x] 1.1 Confirm the migration head on disk (`041_add_garmin_misc_mirror` is current; check nothing has claimed the next slot), then `task migrate:new NAME=add_plan_slot_duration_overrides` — expected `042`
- [x] 1.2 Up: `ALTER plan_slots ADD COLUMN duration_overrides JSONB NULL`. Down: drop the column

## 2. Types + validation (reuse the templates Duration validator)

- [x] 2.1 `internal/trainingplan/types.go`: add `DurationOverrides []SlotDurationOverride` to `PlanSlot` (json `duration_overrides,omitempty`); `SlotDurationOverride{ Intent string; Duration workouttemplates.Duration }`
- [x] 2.2 `service.go`: validate overrides — each `intent` a known template intent constant; no duplicate intent; each `duration` restricted to the bounded kinds (`time`/`distance`) and delegated to the existing `workouttemplates` Duration validator (reject `open`/`lap_button` and non-positive bounds); null/empty allowed. Sentinel errors mapped 1:1
- [x] 2.3 Unit tests: valid time override accepted; duplicate intent rejected; `open`/`lap_button` rejected; `seconds<=0` / `meters<=0` / unknown kind rejected (validator delegation + bounded-kind rule holds)

## 3. Slot CRUD widening

- [x] 3.1 `repo.go` + `handlers.go`: slot create/patch accept `duration_overrides` (marshalled to/from JSONB, alongside `target_overrides`); nested plan `GET` returns it; PATCH replaces the list wholesale (`[]` clears, omitted leaves unchanged)
- [x] 3.2 Tests: create-with-overrides round-trips in the nested GET; patch replace / clear / omit semantics; a slot carrying both override lists round-trips both

## 4. Effective-program resolver (extend) + session-length derivation

- [x] 4.1 `service.go`: extend the effective-program resolver — after applying target overrides, apply per-intent **duration** replacement to the matching template steps; target+duration compose on the same intent; structure untouched
- [x] 4.2 `service.go`: add the session-length derivation used by materialize — sum of effective step durations when all are time-bounded, else `estimated_duration_sec`, else one-hour default
- [x] 4.3 Materialize: derive the planned workout's time window from the session-length derivation (4.2) instead of reading `estimated_duration_sec` directly; dates/sport/name/`plan_slot_id` keying and the `status='planned'` guard unchanged
- [x] 4.4 Tests: duration override replaces only the matching intent's duration, leaves others + targets + repeat structure intact; target+duration compose; `GET /workouts/{id}/program` reflects the overridden duration; materialize moves the window to the effective sum; distance/mixed program falls back to `estimated_duration_sec`; re-materialize stays idempotent and never reverts a `completed` row

## 5. MCP tools

- [x] 5.1 `internal/mcpserver`: widen `add_plan_slot` / `patch_plan_slot` payloads with `duration_overrides` (no new tools)
- [x] 5.2 Confirm `mcp_integration_test.go` expected-tools list is unchanged (no additions); add a payload assertion that `add_plan_slot` forwards `duration_overrides`

## 6. Cross-proposal contract note

- [x] 6.1 No edit needed in `add-garmin-scheduling`: the compile path already builds from the planned workout's **effective** steps via the resolver, so duration overrides flow to the watch automatically — note this confirmation in that change's design if it is still in flight

## 7. Docs + verification

- [x] 7.1 `task swag`; README REST table notes the `duration_overrides` slot field; README MCP table notes `add_plan_slot`/`patch_plan_slot` carry it
- [x] 7.2 `task vet` + `task test` green; `openspec validate add-slot-duration-override --strict` passes
