## 1. Migration

- [x] 1.1 Add `internal/store/migrations/011_add_daily_goal_overrides.up.sql` creating `daily_goal_overrides` with `date DATE PRIMARY KEY`, the same 30 nutrient bound columns as `nutrition_goals` (kcal_min/max, protein_g_min/max, … through zinc_mg_min/max), and `created_at` / `updated_at` `TIMESTAMPTZ NOT NULL DEFAULT now()`. No FKs, no other indexes (PK is the natural index). (Renumbered from 010 because `add-hydration-tracking` already took 010.)
- [x] 1.2 Add `.down.sql`: `DROP TABLE daily_goal_overrides;`.
- [x] 1.3 Verify the migration applies cleanly against a fresh `task dev` Postgres and the column set matches `nutrition_goals` (minus `id`, plus `date`).

## 2. Backend: override repo + effective resolver

- [x] 2.1 `internal/goals/overrides_repo.go` with methods `GetOverride(ctx, date)`, `Upsert(ctx, date, *Goals)`, `Delete(ctx, date)`, `List(ctx, from, to time.Time) ([]*Override, error)`. `Override` is a tiny wrapper struct: `{Date time.Time; Goals *Goals}`. Use the existing `store.Querier`.
- [x] 2.2 Reuse the same scan/exec helpers the singleton `Repo` uses for the 30-column round-trip (extract a shared `scanGoalsRow` / `goalsColumnArgs` if duplication is meaningful; otherwise inline-mirror — judgement call during implementation). Extracted `goalColumns`, `goalValueArgs`, and `scanGoalRow` into `internal/goals/scan.go`.
- [x] 2.3 `internal/goals/effective.go` with:
  - `GoalSource` constants: `"default" | "override" | "none"`.
  - `EffectiveFor(ctx, date time.Time) (*Goals, GoalSource, error)` — override first, default fallback, none if neither.
  - `EffectiveForRange(ctx, from, to time.Time) (effective map[string]*Goals, sources map[string]GoalSource, err error)` — fetches default once, fetches all overrides in the window in one query, builds the per-date maps keyed on `YYYY-MM-DD`.
- [x] 2.4 Unit tests in `internal/goals/effective_test.go` covering: override-only, default-only, both (override wins), neither (none), range mixing all three cases.

## 3. Backend: override HTTP handlers

- [x] 3.1 `internal/goals/overrides_handlers.go` registering:
  - `PUT /goals/overrides/:date`
  - `GET /goals/overrides/:date`
  - `DELETE /goals/overrides/:date`
  - `GET /goals/overrides` (range query via `from` and `to`)
- [x] 3.2 PUT handler reuses the existing `validateGoals` from `handlers.go` for body validation. `c.GetRawData` + `DisallowUnknownFields` decoder (same as `set_goals`) so legacy `kcal_target` is rejected with `400 goal_value_invalid, field: kcal_target`. Date path segment parsed via `time.Parse("2006-01-02", ...)` → `400 date_invalid` on failure. Returns `200 OK` with the upserted goals body (rounded via `roundGoals`).
- [x] 3.3 GET single: 404 with `{"error":"override_not_found"}` when no row; 200 with `{"goals": <override>}` (rounded) otherwise.
- [x] 3.4 DELETE: 204 on success; 404 on unknown.
- [x] 3.5 List: validate `from` and `to` (date format, `from <= to`, span ≤ 366 days). Return `{"overrides": [{"date":"YYYY-MM-DD","goals":{...}}, ...]}` ordered by date ascending. Rounding applied to each goals object.
- [x] 3.6 Swag annotations for all four routes. Document the error codes (`date_invalid`, `goal_value_invalid`, `goal_range_invalid`, `override_not_found`, `range_required`, `range_too_large`).

## 4. Summary service: switch to EffectiveFor

- [x] 4.1 `internal/summary/service.go` — in `DailyFor`, replace the `s.goalsRepo.Get(ctx)` call (which fetched the singleton) with `s.goalsResolver.EffectiveFor(ctx, p.Date)`. Add the returned `GoalSource` to the `Daily` response.
- [x] 4.2 In `RangeFor`'s non-`group_by` branch, replace the single pre-fetch of goals with `s.goalsResolver.EffectiveForRange(ctx, p.From, p.To)`. Inside the day loop, look up that day's effective goals from the map for adherence; set `day.GoalSource` from the sources map.
- [x] 4.3 `Daily.GoalSource` and `RangeDay.GoalSource` JSON tags use `omitempty` so they don't appear when adherence is suppressed (meal_type filter on daily, group_by on range).
- [x] 4.4 The `computeAdherence(ctx, totals, hasMeals)` helper stays unchanged; only the goals-fetch call site moves. `computeAdherenceFor(totals, goals, hasMeals)` is unchanged. (Implementation note: the wrapper `computeAdherence` was removed; `DailyFor` now calls `computeAdherenceFor` directly after resolving effective goals — the helper was a thin convenience over the goals-fetch we no longer need.)

## 5. Backend tests

- [x] 5.1 `internal/goals/overrides_handlers_test.go`: per-endpoint tests against the testcontainers Postgres. Cover: PUT create + read-back, PUT replaces previous (clears fields), GET 404 when none, DELETE happy + 404 on unknown, list range, range_too_large, range_required, date_invalid, legacy `kcal_target` rejected, empty range object rejected, inverted min/max rejected, idempotency-key-on-PUT rejected with the documented error.
- [x] 5.2 `internal/goals/overrides_repo_test.go`: round-trip Upsert + GetOverride, List ordering by date asc, list across an empty window returns empty slice.
- [x] 5.3 Update `internal/summary/handlers_test.go`:
  - Existing tests with goals set but no overrides MUST still pass unchanged — they implicitly assert default-fallback behaviour.
  - Add: with an override set for a date, the daily summary's adherence reflects the override and `goal_source: "override"`.
  - Add: with no override and no default, daily summary returns no adherence + `goal_source: "none"`.
  - Add: range summary across three days with an override only on the middle one — `goal_source` is `["default","override","default"]` and the middle day's adherence uses override bounds.
  - Add: meal_type-filtered daily summary still omits `goal_source` (alongside the existing assertion that adherence is omitted).

## 6. MCP wrapper

- [x] 6.1 `internal/mcpserver/tools_goal_overrides.go` with four input structs:
  - `SetDailyGoalOverrideArgs` — embeds the same per-nutrient `*GoalRange` fields as `SetGoalsArgs` (flat structure rather than nesting `SetGoalsArgs` since the JSON schema reads more cleanly that way); `IdempotencyKey` not included.
  - `GetDailyGoalOverrideArgs{Date string}`.
  - `DeleteDailyGoalOverrideArgs{Date string; IdempotencyKey string}` — DELETE is a POST-style write per the existing rule.
  - `ListDailyGoalOverridesArgs{From, To string}`.
- [x] 6.2 Four handlers mirroring the existing `tools_goals.go` patterns. `handleSetDailyGoalOverride` marshals the Goals subset and PUTs to `/goals/overrides/<date>` (no Idempotency-Key). `handleDeleteDailyGoalOverride` uses `effectiveIdempotencyKey` and falls back to the 204→empty-content shape on success.
- [x] 6.3 `registerGoalOverrideTools(server, c)` registers all four with descriptions per the spec.
- [x] 6.4 Wire `registerGoalOverrideTools` in `cmd/nutrition-api/mcp.go` (or wherever existing tool registration sits). Registered in `internal/mcpserver/server.go`.

## 7. MCP tests

- [x] 7.1 `internal/mcpserver/tools_goal_overrides_test.go`: per-tool tests using `newRecordingClient` / `newRecordingBodyClient`. Cover: URL + method, body forwarding, idempotency-key absence on PUT, 404 forwarding, 204 → empty-content on DELETE, range query string construction.
- [x] 7.2 Update `internal/mcpserver/mcp_integration_test.go` expected-tools list to include `set_daily_goal_override`, `get_daily_goal_override`, `delete_daily_goal_override`, `list_daily_goal_overrides` (now 19 total — hydration shipped first, +4 from this change).

## 8. Documentation

- [x] 8.1 `task swag` regenerates OpenAPI for the new routes.
- [x] 8.2 `README.md`: add a "Daily goal overrides" subsection under Goals examples showing PUT / GET / DELETE / list. Extend the MCP tools table with the four new rows.
- [x] 8.3 `RUN_LOCAL.md`: extend the "Recipe + goals walkthrough" with one example of setting a training-day override and observing the resulting `goal_source` field in `/summary/daily`.

## 9. Pre-merge checks

- [x] 9.1 `task vet` clean.
- [x] 9.2 `task test` green (use `-p 1` if testcontainers parallel-boot flakes surface). Saw one transient testcontainers ping-deadline flake on an unrelated meals test that passed cleanly when re-run in isolation.
- [x] 9.3 Manual e2e: with `task dev` running, set a default goal, log a meal, fetch daily → `goal_source: "default"`. Set an override for the same date with different bounds, fetch again → `goal_source: "override"` and adherence reflects override. Delete the override, fetch once more → back to `"default"`.
- [x] 9.4 OpenSpec validation: `openspec status --change "add-date-varying-goals"` shows 4/4 artifacts done.
