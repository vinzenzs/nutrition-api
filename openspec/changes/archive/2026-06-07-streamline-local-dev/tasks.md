## 1. Taskfile scaffolding

- [x] 1.1 Create `Taskfile.yml` at the repo root with `version: '3'`. Declare top-level variables: `POSTGRES_CONTAINER` (default `nutrition-pg`), `POSTGRES_IMAGE` (default `postgres:17-alpine`), `BIN` (default `bin/nutrition-api`), `INSTALL_PATH` (default `$HOME/.local/bin/nutrition-api`), `MIGRATIONS_DIR` (default `internal/store/migrations`), `DATABASE_URL` (default `postgres://nutrition:nutrition@localhost:5432/nutrition?sslmode=disable`).
- [x] 1.2 Mirror the existing Makefile targets one-for-one as Task targets: `build`, `run`, `test`, `vet`, `swag`, `install`, `migrate`, `migrate:up`, `migrate:down`, `migrate:new` (accepts `NAME=foo` via `--`), `mcp:install`.
- [x] 1.3 Delete `Makefile` from the repo root.

## 2. Database lifecycle tasks

- [x] 2.1 Add `db:up`: detect `docker` or `podman` in `PATH`; if neither, exit 1 with a clear message. If a container named `$POSTGRES_CONTAINER` is running, no-op. If it exists but is stopped, start it. Otherwise `run -d` with the standard env (`POSTGRES_USER=nutrition`, `POSTGRES_PASSWORD=nutrition`, `POSTGRES_DB=nutrition`) and `-p 5432:5432` against `$POSTGRES_IMAGE`.
- [x] 2.2 After (re)starting, poll `pg_isready -U nutrition` via `<runtime> exec` until success or 30 × 200ms attempts exhaust. Exit 0 on success, non-zero on timeout with a clear message.
- [x] 2.3 Add `db:down`: stop and remove `$POSTGRES_CONTAINER` if it exists; exit 0 either way.

## 3. The `dev` task

- [x] 3.1 Add internal `_write-env-local` target with a `status: [test -f .env.local]` guard so it only runs when the file is missing. Body writes the deterministic `.env.local` from the design document.
- [x] 3.2 Add `dev`:
  - `deps: [db:up]` so the container is up before the body runs.
  - Body: `task: _write-env-local`, then an inline banner echo (URL + tokens + sample curl), then `set -a; source .env.local; set +a && go run ./cmd/nutrition-api serve`.
- [x] 3.3 Smoke check by hand: `rm -rf .env.local && task dev` produces a healthy `curl http://localhost:8080/healthz` and `curl http://localhost:8080/readyz` from a second terminal.

## 4. Repository hygiene

- [x] 4.1 Add `.env.local` to `.gitignore`.
- [x] 4.2 Confirm `git status` is clean after running `task dev` (no new untracked files visible).

## 5. Documentation

- [x] 5.1 Rewrite `RUN_LOCAL.md` so the headline path is `task dev`. Move the docker-run / hand-edited-env steps into an "Under the hood" subsection for users who want to understand or override the automation.
- [x] 5.2 Update `README.md` Quickstart: replace `make run` references with `task run` (and the recommended `task dev`).
- [x] 5.3 Update `README.md` Development section: replace the make-targets list with the task-targets list.
- [x] 5.4 Add a brief Task install hint in `RUN_LOCAL.md` ("`brew install go-task/tap/go-task` on macOS").

## 6. Pre-merge checks

- [x] 6.1 `task vet` and `task test` both work (i.e., the Taskfile invokes them correctly).
- [x] 6.2 `task --list` shows every documented target.
- [x] 6.3 OpenSpec validation: `openspec status --change "streamline-local-dev"` shows 4/4 artifacts done.
