# Tasks: rebrand-to-kazper

> **Gate (DD7):** start only after `expand-chat-to-coach` and `unify-mcp-tool-registry` are archived. Rebrand-only — do NOT rename the Go module path, the Postgres DB, env vars, or OpenSpec capability dirs. Target specific files/symbols; never a global `s/nutrition/kazper/` (many "nutrition" strings are domain-correct).

## 0. Gate

- [ ] 0.1 Confirm `expand-chat-to-coach` + `unify-mcp-tool-registry` are archived (coach persona + agenttools/MCP surface stable).

## 1. Code identity

- [ ] 1.1 Rename `cmd/nutrition-api/` → `cmd/kazper/` (main package; no importers). Update any `./cmd/nutrition-api` references.
- [ ] 1.2 Name the coach Kazper in `internal/chat/prompt.go` — introduce the assistant as "Kazper, the user's endurance-fueling and training coach"; keep grounding/tool/confirm behavior intact.
- [ ] 1.3 REST API title: swag `@title` → `Kazper`; run `task swag` to regenerate `docs/`.
- [ ] 1.4 MCP server announced name `"nutrition"` → `"kazper"` in `internal/mcpserver/server.go`; bump `mcp_integration_test.go` if it asserts the name.
- [ ] 1.5 Update `internal/chat` prompt tests to expect the Kazper identity.

## 2. Build & deploy

- [ ] 2.1 Taskfile `build`/`install`/`dev`: output `bin/kazper`, install `~/.local/bin/kazper` (keep macOS re-sign).
- [ ] 2.2 `Dockerfile`: binary/image references → kazper.
- [ ] 2.3 GitHub workflows (`pr.yml`, `main.yml`, `release.yml`): binary/image/chart names → kazper.
- [ ] 2.4 Helm chart (DD4): `deploy/helm/nutrition-api/` → `deploy/helm/kazper/`; `Chart.yaml` name + template references → kazper. **Keep the Postgres DB name unchanged.** Document the redeploy-not-upgrade step in the chart README.

## 3. Satellites

- [ ] 3.1 `apps/companion`: `pubspec.yaml` `name:` + Android display label / app title → Kazper.
- [ ] 3.2 `apps/garmin-bridge`: user-facing names/strings → Kazper.
- [ ] 3.3 `extensions/cookidoo/manifest.json`: extension name/strings → Kazper.

## 4. Docs

- [ ] 4.1 README, RUN_LOCAL, CLAUDE.md: product/prose references → Kazper.
- [ ] 4.2 Add a one-line note (CLAUDE.md/README) explaining the deliberate module-path/product-name split (`github.com/vinzenzs/nutrition-api` stays; product is Kazper).

## 5. Verify & hand off

- [ ] 5.1 `task build` + `task install` smoke under the new `kazper` binary; `task test` + `task vet` green.
- [ ] 5.2 Grep sweep for stray user-facing "Nutrition API"/"nutrition-api" (excluding the module path, DB, env vars, and domain-correct "nutrition" usages).
- [ ] 5.3 Hand off to the user (DD8): GitHub repo rename (`gh repo rename kazper`) + local working-dir rename — these break clone URLs / tooling paths and are the user's call.
