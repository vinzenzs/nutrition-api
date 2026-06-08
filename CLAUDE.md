# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A personal nutrition + endurance-training-fueling backend (Go + Gin + Postgres). Single user, two clients: a mobile app and an LLM coaching agent over MCP. The MCP surface is a thin wrapper over the REST API — every MCP tool issues exactly one HTTP call. See `README.md` and `RUN_LOCAL.md` for end-user docs.

## Common commands

The project uses [Task](https://taskfile.dev/) — not `make`. `task --list` shows everything.

```bash
task dev               # one-command local: Postgres up + .env.local + serve
task test              # full test suite (boots Postgres via testcontainers per package)
task vet               # go vet
task swag              # regenerate docs/ from swag annotations — RUN AFTER ANY HANDLER CHANGE
task build             # compile bin/nutrition-api
task install           # build + copy to ~/.local/bin/nutrition-api (re-signs for macOS)
task db:up / db:down   # Postgres container only
task migrate:new NAME=add_widget   # scaffold a new migration pair
```

Single-package test runs: `go test -count=1 ./internal/<pkg>/...`. If you see `ping database: context deadline exceeded`, it's testcontainers parallel-boot contention — re-run with `-p 1` or just rerun the package alone. The harness already disables the Ryuk reaper (for Podman compatibility) in `internal/store/storetest/storetest.go`.

## Architecture — the big picture

**One Cobra binary, three subcommands.** `cmd/nutrition-api/{serve,mcp,migrate,version}.go` — `serve` runs the Gin REST API, `mcp` runs an MCP server over stdio that hits the REST API as a client, `migrate` runs migrations standalone. Config loads via Viper (env + flags) in `internal/config`.

**One package per capability.** Each event/domain shape lives in its own `internal/<name>/` package with the same shape:

```
internal/<capability>/
  types.go          # Entry/struct mirroring a DB row + JSON tags with omitempty for nullables
  repo.go           # CRUD against store.Querier — used by *pgxpool.Pool OR pgx.Tx
  service.go        # Validation + sentinel errors that map 1:1 to API error codes
  handlers.go       # Gin handlers with swag annotations; Register(rg *gin.RouterGroup)
  *_test.go         # Per-handler integration tests against testcontainers Postgres
```

Capabilities today: `meals`, `hydration`, `workouts`, `bodyweight`, `workoutfuel`, `workoutfueling` (the aggregation-only sibling — see below), `products`, `goals` (singleton + per-date overrides), `summary` (daily/range nutrition), `raceprep` (stateless carb-load math), `energy` (EA composition over meals + workouts + bodyweight), plus the cross-cutting `auth`, `idempotency`, `off` (Open Food Facts client), `numfmt`, `store`.

**`internal/httpserver/server.go` is the wiring trunk.** All package instantiations, repo/service wiring, FK-validation cross-injection (`mealsSvc.SetWorkoutsRepo(...)`), and route registration happen here in `Run()`. The middleware stack: request logger → auth → idempotency → handler. New endpoints land here.

**MCP server mirrors REST 1:1.** `internal/mcpserver/server.go` registers tool groups (`registerMealsTools`, `registerWorkoutsTools`, …). Each tool builds an HTTP request via `apiClient` and forwards the response body verbatim via `toToolResult`. Write tools auto-derive an idempotency key (`effectiveIdempotencyKey`) when the agent doesn't supply one. There's an integration test in `internal/mcpserver/mcp_integration_test.go` (tag `integration`) with an expected-tools list — bump it when you add tools.

## Non-obvious conventions

- **Unit isolation across capabilities.** `hydration` returns ml. `summary` returns kcal+g+mg. `workoutfuel` carries sodium_mg/caffeine_mg/carbs_g/optional ml. These are deliberately kept in separate response shapes — `/summary/daily` does NOT include `total_ml`; `/summary/hydration/daily` does NOT include workout-fuel ml. Tests assert this with `assert.NotContains(body, "total_ml")` and similar. **Do not merge units into shared Totals structs** — the explicit reasoning is in the archived `add-hydration-tracking` and `add-workout-fuel` proposals.
- **Workout-anchored fueling lives in its own package.** `internal/workoutfueling/` exists because `meals` and `hydration` both import `workouts` (for `workout_id` FK validation) — putting the aggregator inside `workouts` would create an import cycle. It depends on all three repos.
- **PATCH tri-state via empty-string sentinel.** Go's JSON decoder collapses missing-field and `null` to the same `*string == nil`, so to distinguish "leave unchanged" from "clear the link" the convention is: `{"workout_id":"<uuid>"}` sets, `{"workout_id":""}` clears, missing leaves unchanged. Applied to `workout_id` on meals/hydration/workoutfuel. The handler converts `""` into `ClearWorkoutID: true` on the service input.
- **`numfmt.Round1` at the response boundary.** Nutrient floats are stored at full precision and rounded to 1dp only when serializing, to keep status math honest at borderlines. Every response that returns nutrient/volume numbers calls it.
- **Idempotency middleware rejects `Idempotency-Key` on PUT.** Per `harden-write-paths`: PUT means full-replace, replay would lie about intermediate state. PUT endpoints (`/goals`, `/goals/overrides/{date}`) return `400 idempotency_unsupported_for_put` if the header is present.
- **Migrations are append-only and sequential** (`NNN_description.up.sql` / `.down.sql`, embedded in the binary). The current head is around `016`. When adding one, use `task migrate:new NAME=...` and check the highest existing number — out-of-band work has occasionally taken the next slot, so verify before committing.
- **`task swag` must be re-run after any change to swag annotations or handler request/response structs.** The `docs/` directory is checked in so plain `go build` doesn't require `swag`; if you forget this step, the OpenAPI spec drifts silently.

## OpenSpec is the change discipline

This repo follows a spec-driven workflow under `openspec/`:

```
openspec/
  config.yaml                # schema: spec-driven
  specs/<capability>/spec.md # authoritative current state (one per capability)
  changes/<slug>/            # in-flight: proposal.md / design.md / specs/ (deltas) / tasks.md
  changes/archive/YYYY-MM-DD-<slug>/  # implemented
  priorities.md              # tier/triage framing — "why does this matter"
```

**Every non-trivial change is proposed before it's built.** The flow is `/opsx:propose <slug>` (write artifacts) → `/opsx:apply <slug>` (implement, ticking task boxes) → `/opsx:archive <slug>` (sync delta specs into `openspec/specs/` and move to archive). Most code changes that touch the REST/MCP surface should land via this flow rather than ad-hoc.

**Commit after every `/opsx:apply`.** When an apply finishes (tasks ticked, tests green, docs regenerated), propose a commit before moving to archive or the next change — don't pile multiple changes into one squash. Convention: `feat(<scope>): <one-line summary>` with the OpenSpec change directory included alongside the code/test/doc files (the tasks.md state belongs in the same commit as the implementation it describes). The archive `mv` is its own subsequent commit. Skipping this leaves `roadmap.md` showing `_uncommitted_` for the change.

`continuity.md` (operational queue: in-progress + up-next + backlog) and `roadmap.md` (historical) are derived companion docs maintained by the `continuity` and `roadmap` skills respectively. `openspec/priorities.md` is hand-maintained tier-based triage.

When reviewing what to build next: read `openspec/priorities.md` for the framing, `continuity.md` for the operational state, and the `archive/` for precedent on shape decisions (e.g. unit isolation, tri-state PATCH, empty-string-clear).
