## ADDED Requirements

### Requirement: Root command and subcommand structure

The system SHALL ship a single binary named `nutrition-api` that exposes its functionality as Cobra subcommands.

The binary SHALL provide the following subcommands at minimum: `serve`, `mcp`, `migrate`, `version`. Running the binary with no subcommand SHALL print usage help and exit with a non-zero status.

Every subcommand SHALL accept `--help` and print its description, flags, and example invocations.

#### Scenario: Help discovery

- **WHEN** a user runs `nutrition-api --help`
- **THEN** the output lists the subcommands `serve`, `mcp`, `migrate`, `version` with one-line descriptions and exits 0

#### Scenario: Missing subcommand fails clearly

- **WHEN** a user runs `nutrition-api` with no arguments
- **THEN** the process prints usage help to stderr and exits with status code 1

### Requirement: serve subcommand

The `serve` subcommand SHALL start the Gin HTTP server with the same behavior as the previous `cmd/api` binary: load config, validate auth tokens, run migrations if `MIGRATE_ON_START=true`, open the DB pool, register routes, listen on the configured address, and shut down gracefully on SIGINT/SIGTERM.

The subcommand SHALL accept an optional `--addr` flag that overrides the `HTTP_ADDR` environment variable.

#### Scenario: serve starts the HTTP server

- **WHEN** the user runs `nutrition-api serve` with valid configuration
- **THEN** the process binds on the configured address, registers `/healthz`, `/readyz`, and the authenticated API routes, and logs `http listening`

#### Scenario: --addr overrides HTTP_ADDR

- **WHEN** the user runs `nutrition-api serve --addr :9090` with `HTTP_ADDR=:8080` set
- **THEN** the server binds on `:9090`

#### Scenario: Graceful shutdown on SIGTERM

- **WHEN** the process receives SIGTERM while serving
- **THEN** it calls `srv.Shutdown` with a 10-second timeout and exits 0

### Requirement: mcp subcommand

The `mcp` subcommand SHALL start the MCP server with the same behavior as the previous `cmd/mcp` binary.

#### Scenario: mcp starts the MCP server

- **WHEN** the user runs `nutrition-api mcp` with valid configuration
- **THEN** the MCP server starts and accepts protocol connections per the existing MCP server behavior

### Requirement: migrate subcommand

The `migrate` subcommand SHALL apply pending database migrations and exit. It SHALL NOT start the HTTP server, MCP server, or any background goroutines.

The subcommand SHALL exit with status 0 on success and a non-zero status on failure, logging the error.

#### Scenario: migrate applies pending migrations

- **WHEN** the user runs `nutrition-api migrate` against a database with pending migrations
- **THEN** the process applies the migrations, logs success, and exits 0

#### Scenario: migrate fails on bad database URL

- **WHEN** the user runs `nutrition-api migrate` with `DATABASE_URL` pointing to an unreachable host
- **THEN** the process logs the error and exits with a non-zero status

### Requirement: version subcommand

The `version` subcommand SHALL print build metadata (semantic version, git commit SHA, build date) to stdout and exit 0. These values SHALL be injected at build time via `-ldflags` and SHALL default to a placeholder (e.g., `dev`) when not injected.

#### Scenario: Version prints build metadata

- **WHEN** the user runs `nutrition-api version`
- **THEN** stdout contains the version, commit, and build date in a stable, parseable format and the process exits 0
