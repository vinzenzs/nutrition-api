## Why

Local development today requires a handful of disconnected steps: start a Postgres container, edit `.env` with hand-generated tokens, source it, then `make run`. Each step has its own failure mode and its own thing to remember. For a personal app whose iteration loop is "edit, restart, curl," that ceremony is friction. This change collapses the entire startup story into a single command, and replaces Makefile with Taskfile while we're at it because the user prefers Task and it slots in cleanly.

## What Changes

- Replace `Makefile` with `Taskfile.yml` (Task — taskfile.dev). All existing make targets get a Task equivalent: `task build`, `task run`, `task test`, `task vet`, `task swag`, `task install`, `task migrate`, `task migrate:up`, `task migrate:down`, `task migrate:new`, `task mcp:install`. The legacy `Makefile` is removed; no shim.
- Add **`task db:up`**: idempotently starts a local Postgres container named `nutrition-pg` on `localhost:5432`. Detects `docker` or `podman` automatically (preferring `docker` if both are present). Waits for `pg_isready` to confirm Postgres is accepting connections, then returns. Re-runs are no-ops when the container is already healthy.
- Add **`task db:down`**: stops and removes the `nutrition-pg` container.
- Add **`task dev`** as the one-command local-dev entrypoint. It:
  1. Creates `.env.local` (git-ignored) on first run with deterministic dev tokens and a `DATABASE_URL` pointing at the local container.
  2. Runs `task db:up` (idempotent) so the Postgres container is up.
  3. Starts `nutrition-api serve` with `.env.local` sourced. Migrations run as part of startup (the existing `MIGRATE_ON_START=true` default), so no separate migrate step is needed.
  4. Prints a banner with the URL, dev tokens, and an example curl.
- Update `RUN_LOCAL.md` so the primary path is `task dev` (one command). The fine-grained manual steps stay as a fallback for users who want to understand or override each piece.
- Update `README.md` quickstart to reference `task` instead of `make`, and to point at `task dev` as the recommended entrypoint.
- Update `.gitignore` to exclude `.env.local`.

## Capabilities

### New Capabilities
- `local-dev-tooling`: The `task dev` one-command entrypoint, the `task db:up` / `task db:down` container lifecycle helpers, and the auto-generated `.env.local` with deterministic dev tokens.

### Modified Capabilities
<!-- None. The storage layer, repo SQL, REST API contracts, auth, idempotency, and MCP behaviour are all unchanged. -->

## Impact

- **New files**: `Taskfile.yml` at the repo root.
- **Removed files**: `Makefile`.
- **Documentation**: `RUN_LOCAL.md` rewritten around `task dev`. `README.md` quickstart and "Development" sections retargeted to Task.
- **New configuration**: none (env variable surface is unchanged). The only "new" thing is `.env.local` as a developer-local file, not a new var.
- **New external dependency**: Task (taskfile.dev) — installed by the user once via `brew install go-task/tap/go-task` or equivalent. Documented in `RUN_LOCAL.md`.
- **No code changes**: Go sources, migrations, repos, handlers, and tests are not touched. This is purely a tooling and onboarding change.

### Out of scope (explicit non-goals)
- SQLite or any alternative storage backend. (Considered briefly; the user explicitly removed it from scope. Postgres remains the only backend.)
- Embedding the Postgres process inside the binary (e.g. `embedded-postgres`). The container is fine; we just want it scripted.
- Running tests through Task. The existing `go test ./...` works fine; `task test` is just an alias.
- Backwards-compatibility shim that keeps `make` working alongside `task`. We swap, we don't bridge.
- Adding MCP server startup to `task dev`. MCP registration is a one-time setup against Claude Code/Desktop; folding it into the foreground task muddies the lifecycle. Documented separately.
