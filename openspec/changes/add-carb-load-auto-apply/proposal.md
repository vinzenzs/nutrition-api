## Why

`plan_carb_load` returns a deterministic per-day carb-load schedule but stops there — the agent then has to issue N separate `set_daily_goal_override` calls to actually put those numbers into the goals layer where adherence reads them. That's the only Tier-1 gap left in `openspec/priorities.md` after this session's work, and it's a real one: the original `add-race-prep-primitives` proposal explicitly fenced auto-apply off as a non-goal "pending usage data showing the friction is real." After today's archive-and-roadmap cycle, that data exists — every race-prep workflow today is "compute schedule, then mechanically loop." The original deferral can retire.

This change closes the gap with one additive flag on the existing tool. `plan_carb_load(apply: true)` computes the same schedule AND writes the carb-load targets into the per-date goal overrides atomically, returning a per-date `applied` outcome alongside the schedule. The pure-compute default behaviour (`apply: false`, the unchanged path) remains for "what-if" exploration and is what the MCP description's existing race-distance guidance covers.

## What Changes

- **New REST endpoint** `POST /race-prep/carb-load/apply` taking the same query/body params as `GET /race-prep/carb-load` (`race_date`, `body_weight_kg`, `days_before?`, `carbs_per_kg_per_day?`, `race_day_carbs_per_kg?`). Computes the schedule, then for each schedule entry merges the carb target into the existing override on that date (or creates a new one). All N upserts run in a single transaction — partial failure rolls back the whole apply.
- **Merge-only semantics** on the goals overrides: the apply step touches ONLY `carbs_g` on each target date. If the date already has an override with `kcal`, `protein_g`, etc., those are preserved verbatim; only the `carbs_g` bound is replaced. New `OverridesRepo.UpsertPatch` (or equivalent) supports this without changing the existing `PUT /goals/overrides/{date}` full-replace contract.
- **Carb target shape**: `carbs_g: {min: target_carbs_g}` (min-only, matching the existing pattern for `fiber_g` / `iron_mg`). Under-fuelling on a load day is the failure mode worth flagging via adherence; over-fuelling isn't. No max bound is written.
- **Response shape**: `{race_date, body_weight_kg, params, schedule, applied: [{date, carbs_g_min, created}]}`. The `applied` array reports per-date outcome — `created: true` for a brand-new override row, `created: false` for a merge into an existing row. Order matches the schedule.
- **`plan_carb_load` MCP tool gains optional `apply: boolean` arg** (default `false`). When `false` the wrapper hits the existing `GET /race-prep/carb-load` (unchanged). When `true` it hits the new `POST /race-prep/carb-load/apply`. Tool description gets a short paragraph naming the side effect: "When `apply: true`, also writes the carb_g goal bounds for each schedule day. Existing kcal/protein/other macros on those days are preserved."

## Capabilities

### Modified Capabilities
- `race-prep`: Gains the apply endpoint requirement (the existing carb-load schedule requirement is unchanged).
- `nutrition-goals`: Gains a requirement that the override repo supports a partial-update (carb-only) write path used by the race-prep apply step. The PUT /goals/overrides/{date} full-replace contract is unchanged.
- `mcp-server`: The existing `plan_carb_load` tool requirement is modified to add the `apply` arg + its description nudge; the existing scenarios (defaults, response-shape, error codes) all still apply when `apply: false`.

## Impact

- **No new schema migration.** The `daily_goal_overrides` table is sufficient; only the write path changes.
- **New REST handler** in `internal/raceprep/handlers.go` for `POST /race-prep/carb-load/apply`. Service-level orchestration in `internal/raceprep/apply.go` (new file) — pulls `goals.OverridesRepo` as a dependency, computes the schedule via the existing `PlanCarbLoad`, then loops N override-merges in a `pgx.Tx`.
- **New method on `goals.OverridesRepo`**: `UpsertPatch(ctx, date, patch *Goals) error` that does read-modify-write — fetches the existing override (if any), overlays only the non-nil fields from `patch`, writes back via the existing `Upsert`. The handler passes a `*Goals` with only `CarbsG` populated.
- **MCP wrapper** in `internal/mcpserver/tools_raceprep.go` gains the `apply` field on `PlanCarbLoadArgs` and routes accordingly. No new tool — the existing one branches.
- **Wiring** in `internal/httpserver/server.go`: the race-prep service constructor gains a `goals.OverridesRepo` dependency. One extra line on the registration.
- **Tests**:
  - Handler tests: happy path (no existing overrides → N inserts), merge path (existing kcal+protein on a date → carbs gets replaced, others preserved), transaction rollback (force a mid-loop failure → confirm zero rows written), validation error codes (all the same as GET).
  - Service tests for `UpsertPatch`: patch into existing row preserves other fields; patch into empty (no row) creates a new one with only the patched fields.
  - MCP wrapper tests: `apply: false` (default) hits GET unchanged; `apply: true` hits POST; response forwarding works for both.
  - One integration test addition in `internal/mcpserver/mcp_integration_test.go` — `plan_carb_load` is already in the expected-tools list; no new tool name to add.
- **Documentation**: `task swag` regenerates OpenAPI; README "Race prep" subsection gains a `?apply=true`-style example showing the side effect; RUN_LOCAL.md gains the apply-plus-summary round-trip showing the override appearing on `/summary/daily`.

### Out of scope (explicit non-goals)

- **Templates / training-phase context (T1 #5, #1A).** The merge semantics this change introduces are the same primitive that templates would use — but defining named templates and a phase concept is a separate, larger change. Revisit per priorities.md after first multi-week training block.
- **Apply over a non-contiguous date set.** The apply step always writes to the contiguous `[race_date - days_before, race_date]` range that the schedule produces. If a user wants to skip a day, they delete the override afterwards.
- **Rollback of a previously-applied carb-load.** No `unapply` endpoint. The user clears overrides via the existing `DELETE /goals/overrides/{date}` per day, or PUTs a fresh override. Adding a tracking layer to "remember which overrides this plan put there" is out of scope.
- **Apply to a non-`carbs_g` macro.** The endpoint is carb-load-specific. If protein-loading or fluid-loading primitives appear later, each gets its own apply step. No generic "plan-then-apply for any macro" abstraction in v1.
- **Validation that the existing override's non-carb bounds are sensible after the carb replacement.** E.g. if kcal was set to 2200 and the new carbs imply >2200 kcal, no warning. The agent reasons about consistency.
- **Atomicity across non-DB side effects.** If swag-gen or a future event-emit fails post-commit, the apply is still considered successful. Standard.
