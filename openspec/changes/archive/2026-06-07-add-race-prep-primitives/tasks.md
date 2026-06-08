## 1. Service: pure carb-load math

- [x] 1.1 Create `internal/raceprep/service.go` with:
  - `type CarbLoadParams struct { RaceDate time.Time; BodyWeightKg float64; DaysBefore int; CarbsPerKgPerDay float64; RaceDayCarbsPerKg float64 }`
  - `type CarbLoadEntry struct { Date string; DaysBefore int; TargetCarbsG float64; Rationale string }`
  - `type CarbLoadSchedule struct { RaceDate string; BodyWeightKg float64; Params EchoedParams; Schedule []CarbLoadEntry }` (`EchoedParams` carries only the three protocol fields, not the entire `CarbLoadParams` shape, so the response stays clean).
  - `func PlanCarbLoad(p CarbLoadParams, today time.Time) (*CarbLoadSchedule, error)` — `today` is injected for testability.
- [x] 1.2 Implement validation in `PlanCarbLoad`: returns `ErrRaceDateInPast`, `ErrBodyWeightKgInvalid`, `ErrDaysBeforeInvalid`, `ErrCarbsPerKgPerDayInvalid`, `ErrRaceDayCarbsPerKgInvalid` (sentinel errors) with bounds per spec.
- [x] 1.3 Implement the schedule generation: for `i` in `[DaysBefore, DaysBefore-1, ..., 1]` produce a load-day entry at `RaceDate - i` with target `round1(BodyWeightKg * CarbsPerKgPerDay)`; append the race-day entry at `RaceDate` with target `round1(BodyWeightKg * RaceDayCarbsPerKg)`. Use `numfmt.Round1` for rounding.
- [x] 1.4 Rationale strings: load days use `"carb-load day N of M"` (where `N = DaysBefore - i + 1`, `M = DaysBefore`); race-day uses `"race morning, pre-race meal ~3-4h before start"`.
- [x] 1.5 Unit tests in `internal/raceprep/service_test.go`:
  - Defaults (`DaysBefore=3, CarbsPerKgPerDay=10, RaceDayCarbsPerKg=2, BodyWeightKg=70`): assert schedule length 4, exact target values (`700.0` × 3 + `140.0`), exact dates relative to RaceDate.
  - `DaysBefore=0`: schedule length 1, only race day.
  - `DaysBefore=7`: schedule length 8.
  - `RaceDayCarbsPerKg=0`: race-day entry exists with `TargetCarbsG=0.0`.
  - Custom params (`BodyWeightKg=80, CarbsPerKgPerDay=12, RaceDayCarbsPerKg=3, DaysBefore=4`): exact-math assertions.
  - One assertion per bounds rejection: weight 25 / 250, days_before -1 / 8, carbs/kg 0.5 / 25, race-day carbs/kg -1 / 11, race_date = today-1.
  - `today == RaceDate` is accepted.

## 2. Handler: GET /race-prep/carb-load

- [x] 2.1 Create `internal/raceprep/handlers.go`:
  - `type Handlers struct { svc *Service }` (Service carries the clock + `time.Location`).
  - `func (h *Handlers) Register(rg *gin.RouterGroup)` registers `GET /race-prep/carb-load` (named `Register` to match every other handler in the codebase).
  - `func (h *Handlers) carbLoad(c *gin.Context)` parses query params, calls `svc.Plan`, maps service errors to `400` codes per spec, returns `200` with the schedule.
- [x] 2.2 Required-param missing: `race_date` → `400 race_date_required`; `body_weight_kg` → `400 body_weight_kg_required`. Detect by checking `c.Query("...") == ""`.
- [x] 2.3 Each numeric param parses via `strconv.ParseFloat` / `strconv.Atoi`; parse failure → `400 <param>_invalid`.
- [x] 2.4 Apply defaults after parsing: `DaysBefore=3, CarbsPerKgPerDay=10, RaceDayCarbsPerKg=2` only when the corresponding query param is absent.
- [x] 2.5 Map service sentinel errors to `400` responses with `{"error":"...","range":{"min":...,"max":...}}` where range applies. Use `errors.Is`.
- [x] 2.6 Add swag annotations: route, params, success response shape, all `400` errors documented.

## 3. Wiring

- [x] 3.1 In `cmd/nutrition-api/serve.go` (or wherever route registration lives), instantiate `raceprep.NewService(now func() time.Time, tz *time.Location)` and `raceprep.NewHandlers(svc)`, then call `h.Register(authedAPI)` under the existing bearer-auth group. Wired in `internal/httpserver/server.go`.
- [x] 3.2 The `now` func passed to the service uses `time.Now` in prod; tests inject a fixed clock.
- [x] 3.3 The `tz` is the existing configured `DEFAULT_USER_TZ` (loaded via `time.LoadLocation(cfg.DefaultUserTZ)` in the wiring; the service exposes `TZ()` so the handler parses `race_date` in the same zone).

## 4. Handler tests

- [x] 4.1 Create `internal/raceprep/handlers_test.go` using the existing httptest harness (no DB needed — this feature is stateless).
- [x] 4.2 Happy path: `GET /race-prep/carb-load?race_date=2026-07-24&body_weight_kg=70` returns `200` with the exact JSON body (asserted via `assert.JSONEq` against a literal fixture so 1dp rounding and field ordering are pinned).
- [x] 4.3 Optional params honoured: passing `days_before=2&carbs_per_kg_per_day=8&race_day_carbs_per_kg=2.5` reflects in `params` and in the targets.
- [x] 4.4 One test per validation failure: missing race_date, missing body_weight_kg, invalid YYYY-MM-DD, weight 25, weight 250, days_before -1, days_before 8, carbs_per_kg 0.5, carbs_per_kg 25, race_day_carbs -1, race_day_carbs 11, race_date in the past. Each asserts status `400` and exact error code.
- [x] 4.5 Auth missing → `401` (single test; the rest of the suite uses the bearer-bearing test client).

## 5. MCP tool

- [x] 5.1 Create `internal/mcpserver/tools_raceprep.go`:
  - `type PlanCarbLoadArgs struct { RaceDate string; BodyWeightKg float64; DaysBefore *int; CarbsPerKgPerDay *float64; RaceDayCarbsPerKg *float64 }` — pointer-optional so the wrapper omits unset params from the query string.
  - `func handlePlanCarbLoad(ctx, c, args) *mcp.CallToolResult` issues `GET /race-prep/carb-load` with the constructed query string, no `Idempotency-Key`.
  - `func registerRacePrepTools(server, c)` registers the one tool with the description per spec.
- [x] 5.2 Wire `registerRacePrepTools` in the MCP registration site (`internal/mcpserver/server.go`).

## 6. MCP tests

- [x] 6.1 Create `internal/mcpserver/tools_raceprep_test.go` using `newRecordingClient`: assert the GET URL, that no `Idempotency-Key` header is sent, that optional params are omitted when absent and present when supplied, that the response body is forwarded verbatim, that a `400` from the endpoint surfaces as `isError = true`.
- [x] 6.2 Update `internal/mcpserver/mcp_integration_test.go` expected-tools list to include `plan_carb_load` (delta of +1; now 20 total).

## 7. Documentation

- [x] 7.1 `task swag` regenerates OpenAPI for the new route.
- [x] 7.2 `README.md`: add one row to the MCP tools table for `plan_carb_load`, and a small "Race prep" subsection under examples showing one `curl` against the endpoint plus the suggested override-loop workflow.
- [x] 7.3 `RUN_LOCAL.md`: add a short "Plan a race week" example: call `/race-prep/carb-load`, then for each schedule entry call `PUT /goals/overrides/<date>` (assumes `add-date-varying-goals` has shipped — it did).

## 8. Pre-merge checks

- [x] 8.1 `task vet` clean.
- [x] 8.2 `task test` green. Saw one transient testcontainers ping-deadline flake on an unrelated `goals` test that passed cleanly when re-run in isolation.
- [ ] 8.3 Manual e2e: with `task dev` running, `curl -H "Authorization: Bearer $API_TOKEN" "http://localhost:8080/race-prep/carb-load?race_date=2026-07-24&body_weight_kg=70"` returns the documented schedule shape. Try one out-of-range param and confirm the `400` error code matches the spec.
- [x] 8.4 OpenSpec validation: `openspec status --change "add-race-prep-primitives"` shows 4/4 artifacts done.
