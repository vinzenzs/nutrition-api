# Tasks: rebrand-to-kazper

> **Scope pivoted mid-apply to a FULL rename** (DD1 revised): after the user renamed the GitHub repo to `kazper`, the Go module path was swept too. KEPT: Postgres DB (`nutrition`/`nutrition-pg`), env vars (`NUTRITION_API_URL`, …), domain terms (`nutrition_goals`, `nutrition_per_serving`), OpenSpec capability dirs (`nutrition-chat`, …), and the Flutter Dart **package id** `nutrition_companion` (deferred, DD9). Never a global `s/nutrition/kazper/` — only the `nutrition-api` token + `github.com/vinzenzs/nutrition-api`.

## 0. Gate

- [x] 0.1 Gate (DD7): `expand-chat-to-coach` archived; `unify-mcp-tool-registry` complete (17/17). User chose to proceed without archiving unify first (gate substantively met).

## 1. Code identity

- [x] 1.1 `cmd/nutrition-api/` → `cmd/kazper/` (`git mv`; main package). Cobra `Use`/`Short`/`Long` + version output + all path refs updated.
- [x] 1.2 Coach named Kazper in `internal/chat/prompt.go` ("You are Kazper, the user's endurance-fueling and training coach…"); behavior intact.
- [x] 1.3 REST API swag `@title` → `Kazper`; `task swag` regenerated `docs/` (title verified `"Kazper"`).
- [x] 1.4 MCP server announced name `"nutrition"` → `"kazper"` (`internal/mcpserver/server.go`). No test asserted the name (golden/announced-tools green).
- [x] 1.5 Added `internal/chat/prompt_test.go` (`TestBuildSystemPrompt_NamesKazper`) asserting the Kazper identity + injected diet/timezone.

## 1b. Module path (added by DD1 revision — full rename)

- [x] 1b.1 Swept `github.com/vinzenzs/nutrition-api` → `github.com/vinzenzs/kazper` across `go.mod` + 186 import sites + docs/chart repo URLs (199 files; archive excluded). Also swept the bare `nutrition-api` token (binary, image, OFF User-Agent, version output).
- [x] 1b.2 Verified: `go build ./...` clean, `go vet ./...` clean, full `task test` green (all packages), `cmd/kazper` integration test (`-tags=integration`) green.

## 2. Build & deploy

- [x] 2.1 Taskfile `build`/`install`/`dev` → `bin/kazper`, `~/.local/bin/kazper` (Postgres `nutrition` DB/container refs kept).
- [x] 2.2 `Dockerfile` binary refs → kazper.
- [x] 2.3 GitHub workflows (`main.yml`, `pr.yml`, `release.yml`): binary/image (`ghcr.io/…/kazper`)/chart paths → kazper.
- [x] 2.4 Helm chart `deploy/helm/nutrition-api/` → `deploy/helm/kazper/` (`git mv`); chart name + template namespace + image → kazper; Postgres DB name unchanged. **Manual: redeploy as a new release, not `helm upgrade` (DD4).**

## 3. Satellites

- [x] 3.1 Companion **display name** → Kazper (iOS `CFBundleDisplayName`, Android `android:label`). Dart **package id** `nutrition_companion` deferred (DD9 — needs a Flutter build to verify).
- [x] 3.2 `apps/garmin-bridge`: product strings → kazper; `NUTRITION_API_URL` env var kept.
- [x] 3.3 `extensions/cookidoo/manifest.json`: name/description → "Kazper".

## 4. Docs

- [x] 4.1 README (H1 → "Kazper" + tagline), RUN_LOCAL, CLAUDE.md → Kazper.
- [x] 4.2 README note documents what was deliberately kept (DB name, `NUTRITION_API_URL`).

## 5. Verify & hand off

- [x] 5.1 `task build` → `bin/kazper`; `./bin/kazper version` → `kazper version=…`; `task test` + `task vet` green.
- [x] 5.2 Stray-token sweep: no `nutrition-api` / `github.com/vinzenzs/nutrition-api` left in source; domain/DB/env terms preserved.
- [ ] 5.3 **User follow-ups (manual):** (a) local working-dir rename (breaks tooling/memory paths — DD8); (b) Helm redeploy as a new release (DD4); (c) optional Flutter Dart package-id rename `nutrition_companion` → `kazper` behind a `flutter build` (DD9); (d) optional `NUTRITION_API_URL` env-var rename (deployment-coordinated).
