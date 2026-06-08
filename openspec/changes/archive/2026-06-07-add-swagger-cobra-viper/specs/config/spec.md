## ADDED Requirements

### Requirement: Centralized configuration loader

The system SHALL load configuration through a single `internal/config` package using Viper. Both the `serve` and `mcp` subcommands SHALL consume configuration through this loader; no subcommand SHALL read environment variables directly.

The loader SHALL return a typed `*Config` struct and an error. Validation failures SHALL be returned as errors, not logged-and-continued.

#### Scenario: Both subcommands share the loader

- **WHEN** `serve` and `mcp` both initialize their configuration
- **THEN** they call the same `config.Load()` function and receive the same `*Config` type

### Requirement: Environment variable bindings

The loader SHALL recognize the following environment variables, preserving the names and semantics used by the prior `cmd/api/config.go` and `cmd/mcp/config.go`:

- `DATABASE_URL` (string, required for `serve` and `migrate`)
- `HTTP_ADDR` (string, default `:8080`)
- `MOBILE_API_TOKEN` (string, required for `serve`)
- `AGENT_API_TOKEN` (string, required for `serve`)
- `DEFAULT_USER_TZ` (string, default `UTC`, must parse via `time.LoadLocation`)
- `OFF_TIMEOUT_SECONDS` (int, default `5`)
- `OFF_USER_AGENT_CONTACT` (string, optional)
- `IDEMPOTENCY_TTL_HOURS` (int, default `24`)
- `MIGRATE_ON_START` (bool, default `true`)
- `SWAGGER_ENABLED` (bool, default `false`)

#### Scenario: Defaults applied when env is unset

- **WHEN** the loader is called with only `DATABASE_URL`, `MOBILE_API_TOKEN`, `AGENT_API_TOKEN` set
- **THEN** the returned config has `HTTPAddr=":8080"`, `DefaultUserTZ="UTC"`, `OFFTimeout=5s`, `IdempotencyTTL=24h`, `MigrateOnStart=true`, `SwaggerEnabled=false`

#### Scenario: Required field missing

- **WHEN** the loader is called with `DATABASE_URL` unset
- **THEN** it returns an error whose message names `DATABASE_URL`

#### Scenario: Invalid timezone rejected

- **WHEN** the loader is called with `DEFAULT_USER_TZ=Not/A/Zone`
- **THEN** validation returns an error referencing the invalid timezone

### Requirement: Configuration precedence

The loader SHALL apply the following precedence, highest first: Cobra flags > environment variables > built-in defaults.

#### Scenario: Flag overrides environment

- **WHEN** the user runs `nutrition-api serve --addr :9090` with `HTTP_ADDR=:8080` set
- **THEN** the resolved `Config.HTTPAddr` is `:9090`

#### Scenario: Environment overrides default

- **WHEN** `HTTP_ADDR=:7777` is set and no flag is provided
- **THEN** the resolved `Config.HTTPAddr` is `:7777`

### Requirement: Secret values not logged

The loader and any configuration-related logging SHALL NOT emit the raw values of `MOBILE_API_TOKEN` or `AGENT_API_TOKEN`. Diagnostic output SHALL indicate presence/absence only (e.g., a boolean or a hash prefix), never the full secret.

#### Scenario: Tokens redacted in diagnostic output

- **WHEN** any code in the config loader, startup logging, or error path emits a log line referencing token configuration
- **THEN** the line contains no substring of the raw `MOBILE_API_TOKEN` or `AGENT_API_TOKEN` value
