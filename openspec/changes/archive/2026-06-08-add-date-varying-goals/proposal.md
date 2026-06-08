## Why

A real endurance-training day looks nothing like a rest day in calories or carbs. The user's working numbers are roughly 2200 kcal training / 1900 kcal rest — same target shape, different values. Today's `nutrition_goals` is a singleton row: one target set for all days, applied uniformly. That forces a choice between under-fuelling training days or over-fuelling rest days, and the adherence story is meaningless on either tail.

The simplest sufficient model is **per-date overrides** on top of the existing singleton. The default goals stay where they are; a new `daily_goal_overrides` table holds full goal sets keyed on date; the summary endpoints resolve "today's effective goals" before computing adherence. Templates ("define `training` once, assign by date") are conceptually nicer but add a second moving part — defer until the override surface proves repetitive in practice.

The agent (`add-mcp-server`, plus the future Garmin coach integration) can already classify which days are training days from the user's training data; this change just gives it a place to write the goal differences down.

## What Changes

- **New `daily_goal_overrides` table** mirroring the nutrient columns of `nutrition_goals` (all 15 ranges, both bounds optional per `unify-adherence-shape`) plus a `date DATE PRIMARY KEY`. One row per date; no singleton constraint.
- **Four REST endpoints** on the existing `/goals` group:
  - `PUT /goals/overrides/{date}` — upsert a full goal set for that date. Full-replace semantics (same as `PUT /goals`); absent fields stored as null. Idempotency-Key rejected per `harden-write-paths`.
  - `GET /goals/overrides/{date}` — fetch one (404 when no override exists).
  - `DELETE /goals/overrides/{date}` — remove the override; date falls back to default goals.
  - `GET /goals/overrides?from=YYYY-MM-DD&to=YYYY-MM-DD` — list overrides in a date range (typical use: "what's set for next week?").
- **Adherence resolution becomes per-date.** `DailyFor(date)` now calls `goals.EffectiveFor(date)` which returns the override row if present, else the default singleton, else nil. `RangeFor(from, to)` batches via `EffectiveForRange(from, to)` so an N-day range is one DB call for the override set rather than N.
- **Summary responses carry a `goal_source` field per day**: `"override" | "default" | "none"`. The agent reads it to comment ("you used training-day goals today — that's why the protein target was 190 g").
- **Four MCP tools** wrapping each endpoint: `set_daily_goal_override`, `get_daily_goal_override`, `delete_daily_goal_override`, `list_daily_goal_overrides`. Auto-derive idempotency on the writes per the existing rule — but since the underlying REST is PUT, the agent's auto-derived key gets rejected loudly (per `harden-write-paths`); the wrapper omits the key on the write per the same rule used for `set_goals` today.

## Capabilities

### Modified Capabilities
- `nutrition-goals`: Gains a date-keyed override surface (new requirement) and the "effective goals are per-date" rule (new requirement, plus an additive scenario on the existing adherence requirement noting that adherence is computed against effective goals).
- `mcp-server`: Adds a requirement covering the four new override tools. The existing `set_goals` / `get_goals` requirement is unchanged.

## Impact

- **Schema migration** `internal/store/migrations/010_add_daily_goal_overrides.{up,down}.sql`. One table, no FKs.
- **Code additions**:
  - `internal/goals/overrides_repo.go` — `GetOverride(date)`, `Upsert(date, *Goals)`, `Delete(date)`, `List(from, to) map[date]*Goals`.
  - `internal/goals/effective.go` — `EffectiveFor(ctx, date)` and `EffectiveForRange(ctx, from, to)` resolvers.
  - `internal/goals/overrides_handlers.go` — the four HTTP routes + swag.
  - `internal/summary/service.go` — replace `goalsRepo.Get` with `goalsRepo.EffectiveFor` in `DailyFor`; switch the range loop to use the batched `EffectiveForRange`.
- **Code modifications**:
  - `Daily` and `RangeDay` response types gain a `GoalSource string` field (`"default" | "override" | "none"`).
  - `mcp-server` tool registration grows four entries.
- **Tests**:
  - Repo tests for the new override CRUD.
  - Handler tests for PUT/GET/DELETE/LIST endpoints, including legacy `kcal_target` rejection (same DisallowUnknownFields path as `set_goals`).
  - Updated summary tests: default behaviour unchanged when no override exists; per-day adherence reflects override when set; range summary correctly switches goal_source day-by-day.
  - One new MCP wrapper test per tool.
- **No changes to** `nutrition_goals` table schema, the `/goals` singleton endpoints, the `set_goals` MCP tool, or hydration / products / meals / off-integration. The change is genuinely scoped to the override surface plus the adherence resolver.
- **Documentation**: `task swag`; README `Goals` examples grow an override block; RUN_LOCAL.md goals walkthrough gets one example of training-day override.

### Out of scope (explicit non-goals)
- **Templates** (named reusable goal sets, e.g. `"training"`, `"rest"`, with date-to-template assignment). Defer until override-only proves repetitive.
- **Rules / auto-classification** ("a day is a training day if Garmin says workout intensity ≥ X"). Requires Garmin integration; separate change.
- **Hydration overrides.** Hydration has no stored target; per `add-hydration-tracking` the agent computes context-sensitive intake against today's training. Not relevant here.
- **Recurring overrides** ("every Tuesday and Thursday"). Agent can issue N PUTs; bulk-apply is a v2 ergonomic that costs more to design than it saves.
- **Partial overrides** ("only change kcal, keep other macros from default"). PUT is full-replace per the existing /goals shape; partial-merge semantics were explicitly avoided in `harden-write-paths` and we're keeping that posture.
- **Validation against the default** (e.g. "warn if override differs from default by > 50%"). Agent reasons about reasonableness from context; API just records.
