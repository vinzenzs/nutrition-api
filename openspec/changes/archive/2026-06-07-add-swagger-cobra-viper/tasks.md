## 1. Dependencies and scaffolding

- [x] 1.1 Add `github.com/spf13/cobra`, `github.com/spf13/viper`, `github.com/swaggo/swag/cmd/swag`, `github.com/swaggo/gin-swagger`, `github.com/swaggo/files` to `go.mod` and run `go mod tidy`
- [x] 1.2 Create `cmd/nutrition-api/` directory with empty `main.go` placeholder so the module compiles

## 2. Shared config package (`internal/config`)

- [x] 2.1 Create `internal/config/config.go` defining `Config` struct with `mapstructure` tags matching env var names listed in `specs/config/spec.md`
- [x] 2.2 Implement `Load() (*Config, error)` using Viper: `AutomaticEnv()`, explicit `BindEnv` per field, defaults for `HTTP_ADDR`, `DEFAULT_USER_TZ`, `OFF_TIMEOUT_SECONDS`, `IDEMPOTENCY_TTL_HOURS`, `MIGRATE_ON_START`, `SWAGGER_ENABLED`
- [x] 2.3 Implement `(*Config) Validate() error` covering required fields and `time.LoadLocation(DefaultUserTZ)`
- [x] 2.4 Add `BindFlags(*pflag.FlagSet)` to wire Cobra flags (initially `--addr`) with Viper precedence flag > env > default
- [x] 2.5 Add unit tests in `internal/config/config_test.go` for: defaults, missing required, invalid TZ, flag-overrides-env precedence, token redaction in `String()`/error paths

## 3. Cobra root command and subcommands

- [x] 3.1 In `cmd/nutrition-api/main.go`, construct root `cobra.Command` `nutrition-api` with persistent `--config` flag (reserved, no-op for now) and run-with-no-subcommand exits 1 with usage
- [x] 3.2 Create `cmd/nutrition-api/serve.go` implementing the `serve` subcommand: port the body of the old `cmd/api/main.go` (logger setup, config load, auth validate, pool, off client, services, gin router, signal handling, graceful shutdown)
- [x] 3.3 Add `--addr` flag to `serve`, bound via `config.BindFlags`
- [x] 3.4 Create `cmd/nutrition-api/mcp.go` implementing the `mcp` subcommand: port body of old `cmd/mcp/main.go`
- [x] 3.5 Create `cmd/nutrition-api/migrate.go` calling `store.Migrate(cfg.DatabaseURL)` and exiting 0/1
- [x] 3.6 Create `cmd/nutrition-api/version.go` printing `version`/`commit`/`date` package vars set via `-ldflags`
- [x] 3.7 Delete `cmd/api/` and `cmd/mcp/` directories
- [x] 3.8 Update `Makefile`: `build` target compiles `cmd/nutrition-api`, `run` invokes `nutrition-api serve`, add `swag` target running `swag init -g cmd/nutrition-api/main.go -o docs`

## 4. Swagger annotations and generated docs

- [x] 4.1 Add file-level swag annotations to `cmd/nutrition-api/main.go` (`@title`, `@version`, `@description`, `@BasePath`, `@securityDefinitions.apikey`)
- [x] 4.2 Annotate `internal/products/handlers.go` handlers with `@Summary`, `@Tags`, `@Param`, `@Success`, `@Failure`, `@Router`, `@Security`
- [x] 4.3 Annotate `internal/meals/handlers.go` handlers identically
- [x] 4.4 Annotate `internal/summary/handlers.go` handlers identically
- [x] 4.5 Run `make swag` and commit generated `docs/docs.go`, `docs/swagger.json`, `docs/swagger.yaml`
- [x] 4.6 In `cmd/nutrition-api/serve.go`, import `_ "github.com/vinzenzs/nutrition-api/docs"` and register `/swagger/*any` via `ginSwagger.WrapHandler(swaggerFiles.Handler)` gated by `cfg.SwaggerEnabled || gin.Mode() != gin.ReleaseMode`

## 5. Tests

- [x] 5.1 Add `cmd/nutrition-api/serve_test.go` smoke test: build the binary, start `serve` against a testcontainer Postgres, assert `/healthz` returns 200
- [x] 5.2 Add Swagger UI test: in debug mode `/swagger/index.html` returns 200; in release mode without `SWAGGER_ENABLED` returns 404
- [x] 5.3 Add `migrate` subcommand test: run against a fresh testcontainer DB, assert exit 0 and that expected tables exist
- [x] 5.4 Add `version` subcommand test: stdout contains the injected version string
- [x] 5.5 Confirm existing `internal/e2e/e2e_test.go` still passes against the refactored `serve` path

## 6. Docs and rollout

- [x] 6.1 Update `README.md`: replace `cmd/api`/`cmd/mcp` invocation instructions with `nutrition-api serve`/`mcp`/`migrate`, document Swagger UI at `/swagger/index.html` and `SWAGGER_ENABLED` gate
- [x] 6.2 Update `.env.example` to include `SWAGGER_ENABLED=false`
- [x] 6.3 Note the binary-name change in the change PR description as the only operational migration step
- [x] 6.4 Run `openspec validate add-swagger-cobra-viper --strict` and fix any spec issues before archiving
