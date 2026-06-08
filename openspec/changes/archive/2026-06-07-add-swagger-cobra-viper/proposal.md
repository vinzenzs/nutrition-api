## Why

The API server's entrypoint mixes ad-hoc env parsing with bespoke helpers, has no CLI surface for ops tasks (e.g., running migrations alone), and exposes no machine-readable API contract for mobile/agent clients. Adopting Cobra + Viper standardizes configuration and command structure, and adding Swagger/OpenAPI gives clients a versioned contract they can generate against.

## What Changes

- Replace the hand-rolled env loader in `cmd/api/config.go` and `cmd/mcp/config.go` with **Viper** (env + optional config file, same precedence rules; existing env var names preserved).
- Restructure `cmd/api` and `cmd/mcp` as **Cobra** commands with a shared root binary `nutrition-api` exposing subcommands: `serve` (HTTP API), `mcp` (MCP server), `migrate` (run migrations and exit), `version`.
- Add **Swagger/OpenAPI 2.0** generation via `swaggo/swag` annotations on existing handlers in `internal/products`, `internal/meals`, `internal/summary`. Serve interactive docs at `/swagger/*any` (gated off in release mode unless `SWAGGER_ENABLED=true`).
- Update `Makefile` with `make swag` (regenerate docs) and `make run` (now invokes `nutrition-api serve`).
- Update `.env.example` and `README.md` to reflect new command structure and Swagger endpoint.

No breaking changes to HTTP API surface; only the binary invocation changes (the old `cmd/api` binary becomes `nutrition-api serve`).

## Capabilities

### New Capabilities
- `api-docs`: OpenAPI/Swagger specification generation, hosting, and the rules governing what gets documented.
- `cli`: Root command structure, subcommand contract (`serve`, `mcp`, `migrate`, `version`), and exit-code conventions.
- `config`: Configuration loading rules (precedence, validation, defaults) now centralized via Viper.

### Modified Capabilities
<!-- None — existing capability specs (auth, meals, products, off-integration) describe HTTP behavior that does not change. -->

## Impact

- **Code**: `cmd/api/*`, `cmd/mcp/*` restructured under a new `cmd/nutrition-api` root command; `cmd/api/config.go` and `cmd/mcp/config.go` removed in favor of a shared `internal/config` package using Viper.
- **Handlers**: `internal/products/handlers.go`, `internal/meals/handlers.go`, `internal/summary/handlers.go` gain `swag` doc comments (non-functional).
- **New deps**: `github.com/spf13/cobra`, `github.com/spf13/viper`, `github.com/swaggo/swag`, `github.com/swaggo/gin-swagger`, `github.com/swaggo/files`.
- **Build**: `Makefile`, generated `docs/` directory (committed), `go.mod`/`go.sum`.
- **Ops**: Deployment scripts that ran `./api` must now run `./nutrition-api serve`. Same for the MCP entrypoint.
- **Docs**: `README.md` quickstart, `.env.example`.
