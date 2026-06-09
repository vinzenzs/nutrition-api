## 1. Package scaffold and types

- [x] 1.1 Create `internal/dailycontext/` package with `types.go` defining the response struct `DailyContext` and sub-types `AdherenceBlock { GoalSource string; PhaseName string omitempty; Adherence summary.Adherence omitempty }`, `NutritionBlock { Totals summary.Totals; EntriesCount int }`, `HydrationBlock { TotalMl float64; EntriesCount int }`, `WorkoutBundle { Workouts []*WorkoutLite }`, `WorkoutLite` (id, sport, started_at, ended_at, duration_min, kcal_burned?, notes?), `WorkoutFuelLite` (id, logged_at, quantity_ml?, carbs_g?, sodium_mg?, potassium_mg?, caffeine_mg?, workout_id?), `WeightBlock { LoggedAt; WeightKg; BodyFatPct?; IsCarryover bool }`, `PhaseBlock` (full phase row mirror), `GoalOverrideBlock { Present bool; Goals *goals.Goals }`. Top level `DailyContext { Date, TZ, Adherence, Nutrition, Hydration, Workouts, WorkoutFuel, Weight *WeightBlock, Phase *PhaseBlock, GoalOverride GoalOverrideBlock }`. Use `omitempty` per the design's choices.
- [x] 1.2 Decide on JSON tag policy for the empty-array slices (`workouts: []` vs absent). Per spec scenario "Empty day", use NO omitempty on `[]*WorkoutLite` and `[]*WorkoutFuelLite` so the agent always sees an array, even empty.

## 2. Service — composition + parallel reads

- [x] 2.1 Create `internal/dailycontext/service.go` with `Service` carrying the 10 read-side deps: `pool *pgxpool.Pool` (for unused; can skip if no direct queries), `mealsRepo *meals.Repo`, `hydrationRepo *hydration.Repo`, `workoutsRepo *workouts.Repo`, `workoutFuelRepo *workoutfuel.Repo`, `bodyWeightRepo *bodyweight.Repo`, `goalOverridesRepo *goals.OverridesRepo`, `phasesRepo *trainingphases.PhasesRepo`, `summarySvc *summary.Service`. Constructor takes them all positionally; the wide signature is intentional (see design §1).
- [x] 2.2 Add `BuildFor(ctx context.Context, date time.Time, loc *time.Location) (*DailyContext, error)`. Compute `dayStart := time.Date(...,loc); dayEnd := dayStart.Add(24h)`. Use `golang.org/x/sync/errgroup.WithContext(ctx)` to spawn N goroutines, each fetching one slice. After `g.Wait()`, assemble the result; on any error, return it (no partial bundle per design decision #2).
- [x] 2.3 Implement each slice fetch as its own goroutine:
  - **adherence + nutrition totals**: call `summarySvc.DailyFor(ctx, summary.DailyParams{Date: date, Loc: loc})`. From the result, populate `AdherenceBlock.GoalSource`, `AdherenceBlock.PhaseName`, `AdherenceBlock.Adherence`, `NutritionBlock.Totals` (already rounded), `NutritionBlock.EntriesCount = len(daily.Entries)`. Drop the entries themselves.
  - **hydration ml**: `hydrationRepo.List(ctx, dayStart.UTC(), dayEnd.UTC())` → sum `QuantityMl`, count entries.
  - **workouts**: `workoutsRepo.List(ctx, workouts.ListParams{From: dayStart.UTC(), To: dayEnd.UTC()})` → project each row to `WorkoutLite` (compute `duration_min` from started_at/ended_at).
  - **workout-fuel**: `workoutFuelRepo.List(ctx, workoutfuel.ListParams{From: dayStart.UTC(), To: dayEnd.UTC()})` → project each row to `WorkoutFuelLite`.
  - **weight**: try `bodyWeightRepo.List(ctx, dayStart.UTC(), dayEnd.UTC())` for fresh entries (take last); else `bodyWeightRepo.LatestBefore(ctx, dayStart.UTC())` for carryover. Empty + no prior → `nil` block.
  - **phase**: `phasesRepo.PhaseFor(ctx, date)` — convert to `PhaseBlock`, returning the full row. ErrPhaseNotFound → `nil` block.
  - **goal_override**: `goalOverridesRepo.GetOverride(ctx, date)` — ErrOverrideNotFound → `{Present: false, Goals: nil}`; hit → `{Present: true, Goals: roundGoals(stored)}`.
- [x] 2.4 Apply the `goals.Goals` rounding (1dp at the response boundary) to the override block — call the existing rounding helper from goals or duplicate the function locally; lean toward duplicating since `roundGoals` is package-private in goals.

## 3. Handlers

- [x] 3.1 Create `internal/dailycontext/handlers.go` with `Handlers { svc *Service, defaultTZ string, logger *slog.Logger }`. `NewHandlers(svc, defaultTZ, logger) *Handlers`. `Register(rg *gin.RouterGroup)` registers `GET /context/daily`.
- [x] 3.2 Handler parses `date` (`time.Parse("2006-01-02", c.Query("date"))` — empty or invalid → `400 date_invalid`), then `tz` (`time.LoadLocation` — invalid → `400 tz_invalid`, omitted → use `defaultTZ`), then calls `svc.BuildFor(ctx, date, loc)`. On error → `500 context_failed`. On success → JSON.
- [x] 3.3 Add swag annotations matching the spec scenarios (request params, response model `DailyContext`, error codes).
- [x] 3.4 Wire in `internal/httpserver/server.go` after the existing handler registrations: construct `dailycontext.NewService(...)` with all the existing repos / services, then `dailycontext.NewHandlers(svc, cfg.DefaultUserTZ, logger).Register(api)`.

## 4. Tests

- [x] 4.1 `internal/dailycontext/service_test.go` — fixture-heavy. Seed: 2 meals, 3 hydration entries totaling 1.5L, 1 workout (duration 60min, kcal=600), 2 workout-fuel rows (one inside the workout window, one outside), 1 body-weight entry on the day, 1 prior body-weight 5 days earlier, 1 phase covering the day with a template, 1 goal override on the day. Build the context, assert every nested field. This is the "shape integrity" test the design called out.
- [x] 4.2 Empty-day test: zero seeded data, no phase, no override, no weight ever. Assert `workouts: []`, `workout_fuel: []`, `weight: null`, `phase: null`, `goal_override.present: false`, and `nutrition.totals` shows zero macros.
- [x] 4.3 Weight-carryover test: no entry on the requested date but one 5 days earlier. Assert `weight.is_carryover: true` AND `weight.logged_at` is the prior entry's timestamp.
- [x] 4.4 Phase-template-drives-adherence test: phase with template, no override → `adherence.goal_source: "phase_template"`, `adherence.phase_name: "<the phase>"`, `phase: {…}`. Then add an override → `adherence.goal_source: "override"`, `adherence.phase_name` absent, `phase` STILL the same row (the phase covers the date even if the override won adherence).
- [x] 4.5 `handlers_test.go` — missing date → 400 date_invalid; invalid date format → 400 date_invalid; invalid tz → 400 tz_invalid; happy path returns 200 with the full bundle (use the fixture from 4.1); auth-required test (no Bearer → 401 when the auth middleware is wired).
- [x] 4.6 Parallel-fetch guard: smoke test that the service doesn't deadlock under errgroup. Run 4.1's fixture 100x in `-race` mode (no need for full benchmarking; just verify no goroutine-leak / racy access).

## 5. MCP wrapper

- [x] 5.1 Create `internal/mcpserver/tools_dailycontext.go` with `DailyContextArgs { Date string; TZ string omitempty }`, `handleDailyContext(ctx, c, args)` that builds the URL with `url.Values` and issues `c.Get(ctx, "/context/daily", q)`. No idempotency key.
- [x] 5.2 Register the tool with the description from the spec scenario ("first call of a session" + "for deep dives, use the dedicated tools").
- [x] 5.3 Add `registerDailyContextTools(server, client)` to `internal/mcpserver/server.go` alongside the existing register calls.
- [x] 5.4 `internal/mcpserver/tools_dailycontext_test.go` — verify GET path, query forwarding (with and without tz), no Idempotency-Key header, response body forwarded verbatim, 4xx surfaced with isError=true.
- [x] 5.5 Update `internal/mcpserver/mcp_integration_test.go` expected-tools assertion to include `daily_context`.

## 6. Documentation regen + spot updates

- [x] 6.1 Run `task swag` to regenerate `docs/swagger.{json,yaml}` and `docs/docs.go`.
- [x] 6.2 Update `README.md` "Summaries" subsection (or sibling): add a "Daily context bundle" entry with curl example and response shape excerpt.
- [x] 6.3 Update `RUN_LOCAL.md` with a short example showing `daily_context` after a few logged entries — emphasize the "one tool, many sources" framing.
- [x] 6.4 Update the MCP tools table in `README.md` to add `daily_context` row.

## 7. Verify and hand off

- [ ] 7.1 Run `task test` — all unit + handler + integration tests pass.
- [ ] 7.2 Run `task build` and exercise the round-trip via manual curl: seed a few entries via the existing endpoints, then `curl /context/daily?date=$(date +%Y-%m-%d)`. Confirm the bundle has the expected blocks.
- [ ] 7.3 Verify `openspec status --change "add-daily-context-aggregator"` reports all artifacts done and all tasks done.
- [ ] 7.4 Commit per CLAUDE.md's "commit after every /opsx:apply" convention: `feat(daily-context): add daily_context aggregator collapsing 5-7 reads into one MCP call`. Include the change dir, new `internal/dailycontext/` package, new MCP tool, doc updates.
- [ ] 7.5 Ready for `/opsx:archive add-daily-context-aggregator` — at archive time the new `openspec/specs/daily-context/spec.md` is created and `openspec/specs/mcp-server/spec.md` gains the new requirement.
