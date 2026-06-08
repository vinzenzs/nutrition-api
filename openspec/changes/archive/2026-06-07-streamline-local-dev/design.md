## Context

The current Makefile encodes the project's build/run/migrate operations. It works, but it leaves the local startup sequence as an exercise — the user reads `RUN_LOCAL.md`, copies a `docker run` command, hand-edits `.env`, then runs `make run`. Each handoff is small but each is a place where the dev loop stalls.

The user asked for two things: swap Make for Task, and collapse local startup into a single command. With Postgres staying as the storage backend, the path is straightforward: a Taskfile that mirrors the Make targets, plus a `task dev` task that orchestrates the container and the binary together.

## Goals / Non-Goals

**Goals:**

- `task dev` from a fresh clone (with Task + Docker-or-Podman installed) produces a healthy API on `http://localhost:8080` with one command.
- Every existing Make target has a same-named Task target.
- Re-running `task dev` after Ctrl-C is fast: no double-spawned containers, no surprise overwriting of the user's `.env.local`.
- `task db:up` and `task db:down` are usable standalone so the user can keep Postgres running between sessions if they want.
- Zero Go code changes. The storage layer, repos, handlers, and tests are untouched.

**Non-Goals:**

- Alternative storage backends (SQLite, embedded Postgres). Out of scope.
- Cross-platform Windows support. The Taskfile assumes a POSIX shell; macOS and Linux only.
- A native Postgres bin install path (no `brew install postgresql`). Container is the only supported local Postgres.
- Re-implementing `docker compose`. Single container, plain `docker run`.

## Decisions

### 1. Taskfile.yml at the repo root, version 3 syntax

Standard `Taskfile.yml` with `version: '3'`. Variables at the top (`POSTGRES_CONTAINER`, `POSTGRES_IMAGE`, `BIN`, `INSTALL_PATH`, etc.) so they're tweakable in one place. No nested Taskfiles.

Targets, matching the current Makefile 1:1 plus the new ones:

```
build              go build -o bin/nutrition-api ./cmd/nutrition-api
run                go run ./cmd/nutrition-api serve
test               go test ./...
vet                go vet ./...
swag               swag init -g cmd/nutrition-api/main.go -o docs
install            build + cp bin/nutrition-api to ~/.local/bin/nutrition-api
migrate            nutrition-api migrate (uses binary; matches Makefile)
migrate:up         golang-migrate up against $DATABASE_URL
migrate:down       golang-migrate down 1
migrate:new -- NAME=foo   scaffold a new migration pair
mcp:install        alias for `install` (same binary serves both REST and MCP)
db:up              start the Postgres container (idempotent)
db:down            stop and remove the Postgres container
dev                one-command local: db:up + ensure .env.local + serve
```

The Cobra-based binary already has `serve`, `migrate`, `mcp`, and `version` subcommands. Task targets just shell out to those; no command construction lives in the Taskfile beyond the obvious.

**Alternatives considered:**
- *Keep Makefile as an alias for Task.* Rejected — drift, two sources of truth. One swap is honest.
- *Use `just` instead of Task.* User specifically asked for Task.

### 2. `task db:up` detects docker vs podman, then runs Postgres

The user is on Podman with a Docker-compatible socket; my own probing during the meal-logging change found `/var/run/docker.sock` symlinked to Podman. So in practice `docker` already works on this machine. But `task db:up` should still degrade gracefully for someone without the Docker CLI installed.

Logic (POSIX shell, inside the Taskfile target):

```sh
if command -v docker >/dev/null 2>&1; then
  RUNTIME=docker
elif command -v podman >/dev/null 2>&1; then
  RUNTIME=podman
else
  echo "neither docker nor podman found in PATH" >&2
  exit 1
fi

if $RUNTIME ps --format '{{.Names}}' | grep -q '^nutrition-pg$'; then
  echo "postgres container already running"
  exit 0
fi

if $RUNTIME ps -a --format '{{.Names}}' | grep -q '^nutrition-pg$'; then
  $RUNTIME start nutrition-pg
else
  $RUNTIME run -d --name nutrition-pg \
    -e POSTGRES_USER=nutrition \
    -e POSTGRES_PASSWORD=nutrition \
    -e POSTGRES_DB=nutrition \
    -p 5432:5432 \
    postgres:17-alpine
fi

# Wait until pg_isready returns 0, max 30 attempts at 200ms each
for i in $(seq 1 30); do
  if $RUNTIME exec nutrition-pg pg_isready -U nutrition -q; then
    echo "postgres ready"
    exit 0
  fi
  sleep 0.2
done

echo "postgres did not become ready in time" >&2
exit 1
```

Three behaviours we want from this:
- Re-running is a no-op when the container is healthy.
- Re-running after `task db:down` (container removed) creates a fresh one.
- Re-running after a host reboot (container exists but stopped) starts the existing one.

**Alternatives considered:**
- *Always recreate the container.* Rejected — loses any in-flight data between dev sessions, which is annoying.
- *Always pull the image.* Rejected — adds latency for no benefit on subsequent runs.
- *Use docker-compose.* Rejected — extra dependency, extra file, no win for a single-container setup.

### 3. `task dev` writes `.env.local` once, never overwrites

`task dev` checks for `.env.local` and writes it only if it does not exist. Contents:

```
DATABASE_URL=postgres://nutrition:nutrition@localhost:5432/nutrition?sslmode=disable
MOBILE_API_TOKEN=dev-mobile-token-0000000000aaaa
AGENT_API_TOKEN=dev-agent-token-0000000000bbbbb
DEFAULT_USER_TZ=Europe/Berlin
HTTP_ADDR=:8080
MIGRATE_ON_START=true
NUTRITION_API_URL=http://localhost:8080
MCP_REQUEST_TIMEOUT_SECONDS=10
SWAGGER_ENABLED=true
```

Notes on the choices:
- **Deterministic tokens.** They're not secret in any meaningful sense for a localhost-only binary; the auth middleware exists to prevent accidentally trusting unauthenticated clients, not to defend against an attacker who already reads your filesystem. Both tokens satisfy the existing `≥16 bytes, must differ` validation. The file is git-ignored and the README warns that production tokens must be generated with `openssl rand -hex 32`.
- **`MIGRATE_ON_START=true`.** Means `task dev` does not need a separate `task migrate` step. The `nutrition-api serve` invocation runs pending migrations before accepting traffic.
- **`SWAGGER_ENABLED=true`.** In dev we want the Swagger UI available regardless of Gin's mode. (In release mode the current default keeps it off; this opts in explicitly for local dev.)
- **Regenerating tokens is explicit.** `rm .env.local && task dev`. The principle is: surprise the user as little as possible.

**Alternatives considered:**
- *Auto-generate random tokens on every first run.* Slightly more secure, but introduces "where did my token go" confusion when the file is later regenerated. Deterministic + git-ignored is simpler and reversible.
- *Source env via Task's own env mechanism instead of a `.env.local` file.* Rejected — `.env.local` doubles as documentation for "what tokens am I using right now?", which the user can `cat` directly.

### 4. `task dev` composes db:up + serve, no extra wiring

The target uses Task's `deps:` to declare `db:up` as a prerequisite. The body of `dev`:

```yaml
dev:
  deps: [db:up]
  cmds:
    - task: _write-env-local
    - source .env.local && go run ./cmd/nutrition-api serve

_write-env-local:
  internal: true
  status:
    - test -f .env.local
  cmds:
    - cmd: |
        cat > .env.local <<'EOF'
        DATABASE_URL=postgres://nutrition:nutrition@localhost:5432/nutrition?sslmode=disable
        MOBILE_API_TOKEN=dev-mobile-token-0000000000aaaa
        AGENT_API_TOKEN=dev-agent-token-0000000000bbbbb
        DEFAULT_USER_TZ=Europe/Berlin
        HTTP_ADDR=:8080
        MIGRATE_ON_START=true
        NUTRITION_API_URL=http://localhost:8080
        MCP_REQUEST_TIMEOUT_SECONDS=10
        SWAGGER_ENABLED=true
        EOF
    - echo ".env.local created — dev tokens are not for production"
```

The `status:` check on `_write-env-local` makes it a no-op when `.env.local` already exists. Task handles re-runs idempotently.

Before `serve` blocks the terminal, we print a banner with the URL, tokens, and a sample curl. Implemented as an inline echo block right before the `go run`.

**Alternatives considered:**
- *Have `dev` always pre-build the binary (`task build`) and run it.* Rejected — `go run` is fine for dev, picks up source edits without an explicit rebuild step.
- *Background the serve with `&` so the terminal returns.* Rejected — foreground is honest. Ctrl-C kills the binary; the user can re-run.

### 5. `.gitignore` additions

`.env.local` is added to `.gitignore`. No other paths need protecting (the local Postgres lives inside the container, not on the host filesystem).

## Risks / Trade-offs

- **Deterministic dev tokens could leak.** They only work against `localhost`. The README warns that production tokens must be generated with `openssl rand -hex 32`. *Mitigation:* `.env.local` is git-ignored; tokens never reach the network.
- **`task db:up` assumes port 5432 is free.** If the user already has a host Postgres on that port, the container will fail to bind. *Mitigation:* the failure message is from Docker/Podman and is clear ("port is already allocated"); documented in RUN_LOCAL.md's troubleshooting section.
- **`task dev` does not auto-update the container when the Postgres image changes.** Pinning `postgres:17-alpine` keeps things stable; bumping the image is a manual `task db:down && task db:up` step. *Mitigation:* the version pin is in `Taskfile.yml`, one line to change.
- **No Windows support.** *Mitigation:* the project's other tooling (the binary, the tests) already assumes POSIX shells; this is consistent.
- **Re-running `task dev` does not regenerate `.env.local`.** Intentional, but might surprise someone who edits the file expecting Task to redo it. *Mitigation:* documented in `RUN_LOCAL.md` — `rm .env.local && task dev` to regenerate.

## Migration Plan

- Purely additive on the tooling side; Go code is untouched.
- `Makefile` removed in the same commit `Taskfile.yml` is added. Anyone with shell aliases on `make run` etc. needs to update them, but the change is obvious and the README is rewritten in the same diff.
- Rollback: revert the commit. The old Makefile comes back.

## Open Questions

- Whether `task dev` should optionally tail the API logs into the foreground with line-prefixed colours (gum-style). Tentative answer: no — `go run` already streams stdout/stderr, and adding a TUI layer is decoration.
- Whether `task db:up` should mount a host volume so data survives `task db:down`. Tentative answer: no — the assumption for `task dev` is that the data is throwaway. A user who wants persistence can run their own `docker run` with a volume mount; the Taskfile shouldn't impose a directory layout.
- Whether to also offer `task dev:reset` that runs `db:down`, removes `.env.local`, and restarts. Tentative answer: probably yes, low-cost; if it ends up in the implementation, the spec acquires one extra scenario.
