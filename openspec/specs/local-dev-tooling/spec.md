# local-dev-tooling Specification

## Purpose

Define local development tooling requirements for the nutrition API, including the single-command dev entrypoint, the Postgres container lifecycle targets, and the Taskfile that replaces the legacy Makefile.

## Requirements

### Requirement: One-command local development entrypoint

The system SHALL provide a `task dev` Taskfile target that starts a fully functional REST API on `localhost:8080`, backed by a local Postgres container, in a single command — without requiring the user to manually start any service or hand-edit configuration files.

#### Scenario: Fresh clone gets a running API with one command

- **WHEN** the user runs `task dev` for the first time in a fresh clone (with Task and Docker-or-Podman installed)
- **THEN** the system starts a Postgres container named `nutrition-pg` on `localhost:5432` if it is not already running
- **AND** writes `.env.local` containing `DATABASE_URL`, deterministic dev tokens, and other defaults if the file does not already exist
- **AND** starts `kazper serve` with `.env.local` sourced
- **AND** the binary applies pending schema migrations at startup (via the existing `MIGRATE_ON_START=true` default)
- **AND** prints a banner identifying the URL, the two dev tokens, and an example curl

#### Scenario: Subsequent runs preserve existing local state

- **WHEN** the user runs `task dev` and `.env.local` already exists
- **THEN** the system does not overwrite `.env.local`
- **AND** uses the existing tokens, `DATABASE_URL`, and other settings from that file

#### Scenario: Running container is reused

- **WHEN** the user runs `task dev` and the `nutrition-pg` container is already running and healthy
- **THEN** the system does not start a new container
- **AND** does not pull the Postgres image again

#### Scenario: Stopped container is restarted

- **WHEN** the user runs `task dev` and the `nutrition-pg` container exists but is stopped (e.g. after a host reboot)
- **THEN** the system starts the existing container rather than creating a new one

### Requirement: db:up brings up Postgres idempotently

The system SHALL provide a `task db:up` target that ensures a local Postgres container is running and accepting connections, regardless of whether the container existed before, was stopped, or was running.

#### Scenario: db:up detects docker or podman

- **WHEN** the user runs `task db:up` on a host with both `docker` and `podman` available
- **THEN** the system uses `docker`
- **WHEN** the user runs `task db:up` on a host with only `podman` available
- **THEN** the system uses `podman`

#### Scenario: db:up reports neither runtime found

- **WHEN** the user runs `task db:up` on a host with neither `docker` nor `podman` in `PATH`
- **THEN** the system writes an error to stderr identifying the missing requirement
- **AND** exits with a non-zero status code

#### Scenario: db:up waits for pg_isready

- **WHEN** the system starts (or restarts) the container during `task db:up`
- **THEN** the target does not return until `pg_isready -U nutrition` reports the database accepting connections (or the timeout elapses)

#### Scenario: db:up times out gracefully

- **WHEN** the container is created but does not become ready within the wait budget (≥6 seconds)
- **THEN** the target writes a clear error message to stderr
- **AND** exits with a non-zero status code

### Requirement: db:down removes the local container

The system SHALL provide a `task db:down` target that stops and removes the `nutrition-pg` container if it exists.

#### Scenario: db:down on a running container

- **WHEN** the user runs `task db:down` and `nutrition-pg` is running
- **THEN** the container is stopped and removed
- **AND** the target exits with status code 0

#### Scenario: db:down when no container exists

- **WHEN** the user runs `task db:down` and there is no `nutrition-pg` container
- **THEN** the target exits with status code 0 without raising an error

### Requirement: Taskfile mirrors prior Makefile targets and removes the Makefile

The system SHALL provide a `Taskfile.yml` exposing every operation the prior `Makefile` exposed, plus the new local-dev targets. The legacy `Makefile` SHALL be removed in this change.

#### Scenario: Every Make target has a Task equivalent

- **WHEN** a user lists Task targets via `task --list`
- **THEN** the list includes at minimum: `build`, `run`, `test`, `vet`, `swag`, `install`, `migrate`, `migrate:up`, `migrate:down`, `migrate:new`, `mcp:install`, `db:up`, `db:down`, `dev`

#### Scenario: Makefile is absent

- **WHEN** a user runs `ls` at the repo root after this change ships
- **THEN** there is no `Makefile`

#### Scenario: task run starts the REST API

- **WHEN** the user runs `task run` with a valid environment loaded
- **THEN** the system runs `go run ./cmd/kazper serve` (equivalent to the prior `make run`)

#### Scenario: task migrate applies pending migrations

- **WHEN** the user runs `task migrate` with `DATABASE_URL` set
- **THEN** the system invokes the binary's `migrate` subcommand against that database (equivalent to the prior `make migrate`)

### Requirement: Generated local-dev files are git-ignored

The system SHALL ensure that files generated by `task dev` are not committed to version control.

#### Scenario: .env.local is git-ignored

- **WHEN** `task dev` writes `.env.local`
- **THEN** `git status` does not report `.env.local` as untracked or modified
