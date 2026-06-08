## Context

The project currently ships two independent main packages — `cmd/api` (Gin HTTP server) and `cmd/mcp` (MCP server) — each with its own ad-hoc env loader (`envOr`, `envBool`, `envIntDefault`). There is no CLI affordance for operational tasks (running migrations standalone, printing version, dry-run config), and no published API contract for downstream consumers (mobile app, agent tooling). All existing capability specs (`auth`, `meals`, `products`, `off-integration`) describe HTTP behavior that this change must preserve byte-for-byte.

Constraints:
- Go 1.26 module already pulls in Gin v1.12 — Swagger middleware must be Gin-compatible.
- Existing env var names (`DATABASE_URL`, `HTTP_ADDR`, `MOBILE_API_TOKEN`, `AGENT_API_TOKEN`, `DEFAULT_USER_TZ`, `OFF_TIMEOUT_SECONDS`, `OFF_USER_AGENT_CONTACT`, `IDEMPOTENCY_TTL_HOURS`, `MIGRATE_ON_START`) are referenced in `.env.example` and downstream deployment configs — they must continue to work unchanged.
- `MOBILE_API_TOKEN` and `AGENT_API_TOKEN` are secrets — config loader must never log raw values.

## Goals / Non-Goals

**Goals:**
- Single binary `nutrition-api` with discoverable subcommands and `--help` for each.
- One shared config-loading code path for `serve` and `mcp` so env var precedence, validation, and defaults are defined once.
- Machine-readable OpenAPI 2.0 spec at `/swagger/doc.json` plus interactive Swagger UI at `/swagger/index.html`, covering every public endpoint under `/products`, `/meals`, `/summary`.
- `migrate` subcommand that runs DB migrations and exits — usable in CI/CD as a pre-deploy step independent of the long-running server.

**Non-Goals:**
- Migrating to OpenAPI 3.x (swaggo emits 2.0; sufficient for the current generators in use).
- Adding YAML/TOML config files in addition to env — Viper will be wired only to env + flags this round (file support deferred until a real need arises).
- Restructuring `internal/*` packages or changing HTTP request/response shapes.
- Authentication or rate-limiting for the Swagger UI itself beyond the existing `SWAGGER_ENABLED` gate.

## Decisions

### D1: One root binary `nutrition-api` with subcommands, not two binaries

`cmd/nutrition-api/main.go` becomes the only entrypoint and wires Cobra. `cmd/api` and `cmd/mcp` are removed. Subcommands:
- `serve` — runs the Gin HTTP API (current `cmd/api/main.go` behavior).
- `mcp` — runs the MCP server (current `cmd/mcp` behavior).
- `migrate` — calls `store.Migrate(cfg.DatabaseURL)` and exits 0/1.
- `version` — prints build info (version, commit, date injected via `-ldflags`).

**Alternative considered:** Keep two binaries, just refactor config. Rejected because the operational ask (standalone `migrate`) needs a third entrypoint anyway, and Cobra makes adding it trivial. Single binary also simplifies the Dockerfile (one `COPY --from=builder`) and matches conventions in similar Go services.

### D2: Viper bound to env only, struct-tag driven

Introduce `internal/config` package exporting `Load() (*Config, error)`. Viper is configured with `AutomaticEnv()` + an explicit `BindEnv` call per field so missing env vars surface as validation errors, not silent zero values. No config-file support yet (see Non-Goals).

The `Config` struct uses `mapstructure` tags matching today's env var names. Validation (e.g., `DATABASE_URL` non-empty, `DEFAULT_USER_TZ` parseable by `time.LoadLocation`) lives in a `Validate()` method on the struct — same call site as today, just consolidated.

**Alternative considered:** `kelseyhightower/envconfig`. Lighter, but loses Viper's flag-binding which we will use for `--config` and per-subcommand overrides (`serve --addr :9090`). Sticking with the user's stated preference.

### D3: Swagger via `swaggo/swag` annotations, generated `docs/` committed

Handlers gain `@Summary`, `@Tags`, `@Param`, `@Success`, `@Failure`, `@Router` comments. `swag init -g cmd/nutrition-api/main.go -o docs` produces `docs/docs.go`, `docs/swagger.json`, `docs/swagger.yaml`. The generated package is imported for its side effect (`_ "github.com/vinzenzs/nutrition-api/docs"`) in the `serve` command.

`docs/` is **committed** so builds don't depend on `swag` being installed in CI for non-doc changes. A `make swag` target regenerates it; a CI check (out of scope here, follow-up) can run `make swag && git diff --exit-code` to catch drift.

**Alternative considered:** Hand-written OpenAPI YAML. Rejected — duplicates handler signatures and drifts immediately. Annotations stay co-located with the code they describe.

### D4: Swagger UI off by default in release builds

The `/swagger/*any` route is registered only when `SWAGGER_ENABLED=true` OR `gin.Mode() != gin.ReleaseMode`. Local dev (debug mode) gets it for free; prod must opt in. This keeps the UI off untrusted-network deployments without code changes.

Health endpoints (`/healthz`, `/readyz`) stay public and undocumented in Swagger — they're for the load balancer, not API consumers.

### D5: Backwards-compatible env vars; new flags are additive

Every existing env var keeps its current name and semantics. New Cobra flags (`--addr`, `--config`) take precedence over env when both are set (Viper's standard precedence: flag > env > default). This means rolling out the new binary requires no env changes for existing deployments.

## Risks / Trade-offs

- **Swagger annotations rot silently** → CI lint that runs `swag init` and diffs `docs/` flags drift. Add as follow-up; first cut is one-time generation.
- **Viper pulls a large dependency tree** → Accepted. The user explicitly asked for Viper, and the alternative (envconfig) trades dep weight for losing flag binding we'll likely use.
- **Removing `cmd/api` binary breaks anyone running `go run ./cmd/api`** → Mitigation: `README.md` and `Makefile` updated in the same PR; `cmd/api` symlink/shim NOT added (it would mask the breakage and accrete tech debt). Communicated as the only breaking change in the proposal's Impact section.
- **`swag init` requires Go toolchain at doc-edit time** → Acceptable; only doc authors touch annotations. Commit the generated output so consumers don't need it.
- **Swagger UI exposed by accident in prod** → Two gates (release mode + explicit env flag); covered by `api-docs` spec scenarios.

## Migration Plan

1. Land the change behind no flag (it's a pure refactor + additive feature). All existing env vars continue to work.
2. Update deployment manifests/Procfile to invoke `nutrition-api serve` instead of `api`, and `nutrition-api mcp` instead of `mcp`. Add `nutrition-api migrate` as a pre-deploy step if desired.
3. Rollback: revert the PR — no DB schema or API surface changes to undo.

## Open Questions

- Should `migrate` accept `--dry-run` to print the pending migrations without applying? Punt to a follow-up if/when ops asks.
- Do we want `nutrition-api serve --mcp` to run both servers in one process? Out of scope; current split (separate subcommands) keeps process boundaries clean.
