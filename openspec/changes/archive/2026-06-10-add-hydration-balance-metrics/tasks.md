# add-hydration-balance-metrics — tasks

> Third instance of the date-keyed snapshot pattern — clone `internal/recoverymetrics/`
> nearly verbatim. Verify the next free migration slot (head is `023`, so `024`
> expected) before committing.

## 1. hydration-balance capability

- [x] 1.1 Migration `024_add_hydration_balance_metrics`: CREATE TABLE `hydration_balance_metrics` (`date` DATE PRIMARY KEY; nullable `sweat_loss_ml` NUMERIC(10,1) CHECK >0, `activity_intake_ml` NUMERIC(10,1) CHECK >=0, `goal_ml` NUMERIC(10,1) CHECK >0; `created_at`/`updated_at`). Down: DROP TABLE.
- [x] 1.2 `internal/hydrationbalance/types.go`: `Snapshot` struct (Date string YYYY-MM-DD; `SweatLossML`/`ActivityIntakeML`/`GoalML` as `*float64` with omitempty) mirroring recoverymetrics
- [x] 1.3 `repo.go`: `Upsert` (`INSERT … ON CONFLICT (date) DO UPDATE` full-replace, `xmax=0` inserted flag), `GetByDate`, `List(from, to)` ordered by date asc, `DeleteByDate`; `selectCols` with `to_char(date,'YYYY-MM-DD')`, `scanSnapshot`
- [x] 1.4 `service.go`: sentinel errors (`ErrDateInvalid`, `ErrSweatLossMLInvalid`, `ErrActivityIntakeMLInvalid`, `ErrGoalMLInvalid`); validation (sweat/goal > 0, activity_intake >= 0, date parse); `Upsert`/`Get`/`ListWindow`/`Delete`
- [x] 1.5 `handlers.go`: `POST /hydration-balance` (upsert 201/200), `GET /hydration-balance` (window + 92-day cap + `window_required`/`range_too_large`), `GET /hydration-balance/{date}` (404 `hydration_balance_not_found`), `DELETE /hydration-balance/{date}`; round floats via `numfmt.Round1Ptr`; `Register(rg)`; swag annotations
- [x] 1.6 Wire into `internal/httpserver/server.go` `Run()` (repo→service→handlers→Register, behind auth+idempotency)
- [x] 1.7 Tests: upsert insert+update round-trip, omitted-field NULL on re-POST (full-replace), `activity_intake_ml: 0` stored as real zero, each range-validation error, window list + caps, get/delete 404, unit-isolation (no total_ml/kcal/weight keys)

## 2. daily-context block

- [x] 2.1 `internal/dailycontext/types.go`: add `HydrationBalance *hydrationbalance.Snapshot` field to `DailyContext` (json `hydration_balance`, same-day-or-null)
- [x] 2.2 `internal/dailycontext/service.go`: add the repo to the Service struct + constructor; fetch `GetByDate(dateStr)` in the errgroup fan-out (ErrNotFound → nil, no carryover); update `server.go` + `service_test.go` NewService call sites
- [x] 2.3 Tests: bundle includes `hydration_balance` when present, null (no carryover) when absent, stays distinct from the `hydration` block

## 3. MCP + docs

- [x] 3.1 `internal/mcpserver/tools_hydrationbalance.go`: `log_/list_/get_/delete_hydration_balance` (mirror `tools_recoverymetrics.go`); register in `server.go`
- [x] 3.2 Bump the expected-tools list in `mcp_integration_test.go` (+4); MCP wrapper tests (forwarded-when-set, omitted-when-absent, date-addressed get/delete, list window no-key)
- [x] 3.3 `task swag` — regenerate `docs/`
- [x] 3.4 README: hydration-balance section (push-by-date example) + note distinguishing it from the hydration entries; RUN_LOCAL: extend the daily Garmin push example with a `/hydration-balance` POST
- [x] 3.5 `task vet` + full `task test` green
- [x] 3.6 Out-of-repo coordination note (no code here): `garmin.py`'s existing `_push_hydration` already fetches `get_hydration_data(day)` — it gains a POST mapping `sweatLossInML`/`activityIntakeInML`/`goalInML` onto `/hydration-balance` for the same day — tracked outside this repo
